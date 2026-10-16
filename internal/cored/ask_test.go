package cored

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/soppylzz/soppy-utils/internal/core"
	"strings"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// awaitInput reads until core asks for input, and returns the correlation id and the
// raw spec.
func (c *client) awaitInput() (int64, string) {
	c.t.Helper()
	_, _, spec := c.awaitInputFor()
	return 0, spec
}

// awaitInputFor reads until core asks for input and reports which session asked. A
// client has to know that: two commands in one terminal make two forms, and an answer
// has to go back to the one that asked.
func (c *client) awaitInputFor() (requestID int64, sessionID, spec string) {
	c.t.Helper()
	for i := 0; i < 20; i++ {
		msg := c.next()
		if msg.Kind() != protocol.KindNotification || msg.Method != "session/request_input" {
			continue
		}
		var got struct {
			SessionID string          `json:"session_id"`
			RequestID int64           `json:"request_id"`
			Spec      json.RawMessage `json:"spec"`
		}
		if err := json.Unmarshal(msg.Params, &got); err != nil {
			c.t.Fatalf("unmarshal request_input: %v", err)
		}
		if got.RequestID == 0 {
			c.t.Fatal("core asked for input without a request id, so it cannot be answered")
		}
		if got.SessionID == "" {
			c.t.Fatal("core asked for input without saying which session asked")
		}
		return got.RequestID, got.SessionID, string(got.Spec)
	}
	c.t.Fatal("core never asked for input")
	return 0, "", ""
}

func (c *client) answer(requestID int64, sessionID, values string) {
	c.t.Helper()
	params := fmt.Sprintf(`{"session_id":%q,"request_id":%d,"answer":%s}`, sessionID, requestID, values)
	if reply := c.call("session/respond", params); reply.Error != nil {
		c.t.Fatalf("session/respond: %+v", reply.Error)
	}
}

func TestACommandCanAskTheClientForInput(t *testing.T) {
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			values, err := reply.Ask(ctx, form.Spec{
				Title: "Welcome banner",
				Fields: []form.Field{
					{Key: "name", Type: form.String, Title: "Name", Required: true},
					{Key: "rc", Type: form.Enum, Title: "Shell hooks",
						Options: []form.Option{{Value: "bash", Label: "Bash only"}}},
				},
			})
			if err != nil {
				return err
			}
			if values["name"] != "Alice" {
				return fmt.Errorf("name = %v, want Alice", values["name"])
			}
			return reply.Finish(json.RawMessage(`{"ok":true}`))
		},
	}})

	id := c.startSession("welcome")

	reqID, _, spec := c.awaitInputFor()
	// The spec is what the client renders, so it has to arrive intact rather than
	// summarised.
	if !strings.Contains(spec, "Welcome banner") || !strings.Contains(spec, "Bash only") {
		t.Errorf("spec reached the client without its content: %s", spec)
	}

	c.answer(reqID, id, `{"name":"Alice","rc":"bash"}`)

	fin := c.await("session/finish")
	if got := field(t, fin, "error"); got != "" {
		t.Fatalf("the handler failed: %s", got)
	}
}

func TestAnAnswerGoesToTheSessionThatAskedForIt(t *testing.T) {
	// Two sessions on one connection, each asking. An answer delivered to the wrong one
	// hands one command's user input to another command. The handler reports its own
	// name alongside the value, so a swap is visible rather than merely unlikely.
	seen := make(chan string, 2)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			values, err := reply.Ask(ctx, form.Spec{
				Fields: []form.Field{{Key: "which", Type: form.String}},
			})
			if err != nil {
				return err
			}
			seen <- cmd.Util + "=" + fmt.Sprint(values["which"])
			return reply.Finish(nil)
		},
	}})

	first := c.startSession("welcome")
	second := c.startSession("license")

	pending := make(map[string]int64, 2)
	for len(pending) < 2 {
		reqID, sessionID, _ := c.awaitInputFor()
		pending[sessionID] = reqID
	}

	c.answer(pending[first], first, `{"which":"first"}`)
	c.answer(pending[second], second, `{"which":"second"}`)

	got := make(map[string]bool, 2)
	for range 2 {
		got[<-seen] = true
	}
	if !got["welcome=first"] || !got["license=second"] {
		t.Errorf("handlers saw %v, want each answer delivered to the session that asked", got)
	}
}

func TestAskEndsWhenTheClientDisconnects(t *testing.T) {
	done := make(chan error, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			_, err := reply.Ask(ctx, form.Spec{
				Fields: []form.Field{{Key: "name", Type: form.String}},
			})
			done <- err
			return err
		},
	}})

	c.startSession("welcome")
	c.awaitInput()
	c.conn.Close()

	// A handler waiting on a user who is no longer there must not wait forever, and must
	// not go on to write anything.
	select {
	case err := <-done:
		if err == nil {
			t.Error("Ask returned no error when the client went away")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ask never returned after the client disconnected")
	}
}

func TestTheClientCanCancelASession(t *testing.T) {
	cancelled := make(chan struct{})
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		},
	}})

	id := c.startSession("welcome")
	reply := c.call("session/cancel", `{"session_id":"`+id+`"}`)
	if reply.Error != nil {
		t.Fatalf("session/cancel: %+v", reply.Error)
	}

	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled session kept running")
	}
}

func TestAnAnswerForAnUnknownSessionDoesNotBreakTheConnection(t *testing.T) {
	// A late answer is ordinary: the user filled a form and the session was cancelled
	// first. It has to be reported without taking the connection down with it.
	c := startDaemon(t, Options{})
	c.ensureInitialized()

	reply := c.call("session/respond", `{"session_id":"nonesuch","request_id":99,"values":{}}`)
	if reply.Error == nil {
		t.Error("an answer for a session that does not exist was accepted")
	}
	if follow := c.call("capabilities/list", `{}`); follow.Error != nil {
		t.Errorf("the connection broke after a stray answer: %+v", follow.Error)
	}
}

func confirmAsker(question string, answers chan bool) core.Handler {
	return scriptedHandler{run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		ok, err := reply.Confirm(ctx, question)
		if err != nil {
			return err
		}
		answers <- ok
		return reply.Finish(nil)
	}}
}

func (c *client) answerConfirm(requestID int64, sessionID string, confirmed bool) {
	c.t.Helper()
	params := fmt.Sprintf(`{"session_id":%q,"request_id":%d,"answer":{"confirmed":%t}}`,
		sessionID, requestID, confirmed)
	if reply := c.call("session/respond", params); reply.Error != nil {
		c.t.Fatalf("session/respond: %+v", reply.Error)
	}
}

func TestACommandCanAskForConfirmation(t *testing.T) {
	for _, want := range []bool{true, false} {
		answers := make(chan bool, 1)
		c := startDaemon(t, Options{Handler: confirmAsker("Replace existing file?", answers)})
		c.withCapabilities()

		id := c.startSession("license")

		var got struct {
			SessionID string `json:"session_id"`
			RequestID int64  `json:"request_id"`
			Question  string `json:"question"`
		}
		for i := 0; i < 20; i++ {
			msg := c.next()
			if msg.Method != "session/request_confirm" {
				continue
			}
			if err := json.Unmarshal(msg.Params, &got); err != nil {
				t.Fatalf("unmarshal request_confirm: %v", err)
			}
			break
		}
		if got.Question == "" {
			t.Fatal("no confirmation was asked for")
		}
		if got.SessionID != id {
			t.Errorf("session_id = %q, want %q", got.SessionID, id)
		}

		c.answerConfirm(got.RequestID, id, want)

		select {
		case answer := <-answers:
			if answer != want {
				t.Errorf("the command saw %v, want %v", answer, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the answer never reached the command")
		}
	}
}

func TestAClientThatCannotConfirmIsAnsweredNo(t *testing.T) {
	// A destructive step with nobody to approve it must not proceed. Answering no is the
	// only safe reading of "nobody could be asked".
	answers := make(chan bool, 1)
	c := startDaemon(t, Options{Handler: confirmAsker("Replace existing file?", answers)})
	// No action execution and no confirmation in the handshake.
	c.ensureInitialized()
	c.startSession("license")

	select {
	case answer := <-answers:
		if answer {
			t.Error("a client that cannot ask was answered yes")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Confirm never returned for a client that cannot ask")
	}
}
