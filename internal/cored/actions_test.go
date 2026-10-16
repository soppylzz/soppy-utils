package cored

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/soppylzz/soppy-utils/internal/core"
	"strings"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// awaitAction reads until core offers actions, and returns the correlation id and the
// request as the client received it.
func (c *client) awaitAction() (int64, action.Request) {
	c.t.Helper()
	for i := 0; i < 20; i++ {
		msg := c.next()
		if msg.Kind() != protocol.KindNotification || msg.Method != "session/request_action" {
			continue
		}
		var got struct {
			RequestID int64           `json:"request_id"`
			Reason    string          `json:"reason"`
			Actions   []action.Action `json:"actions"`
		}
		if err := json.Unmarshal(msg.Params, &got); err != nil {
			c.t.Fatalf("unmarshal request_action: %v", err)
		}
		return got.RequestID, action.Request{Reason: got.Reason, Actions: got.Actions}
	}
	c.t.Fatal("core never offered any actions")
	return 0, action.Request{}
}

func (c *client) reportOutcome(requestID int64, sessionID, outcome string) {
	c.t.Helper()
	params := fmt.Sprintf(`{"session_id":%q,"request_id":%d,"answer":%s}`, sessionID, requestID, outcome)
	if reply := c.call("session/respond", params); reply.Error != nil {
		c.t.Fatalf("session/respond: %+v", reply.Error)
	}
}

// withActionExecution handshakes a client that can run commands on the user's behalf.
func (c *client) withActionExecution() {
	c.t.Helper()
	c.initialized = true
	body, err := json.Marshal(map[string]any{"client": map[string]any{
		"protocol_versions": []int{protocol.Version},
		"capabilities":      map[string]any{"action_execution": true},
		"env":               map[string]any{"os": "linux", "path": ""},
	}})
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	if reply := c.call("initialize", string(body)); reply.Error != nil {
		c.t.Fatalf("initialize: %+v", reply.Error)
	}
}

func TestACommandCanOfferActionsToTheClient(t *testing.T) {
	seen := make(chan action.Outcome, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			outcome, err := reply.AskActions(ctx, action.Request{
				Reason: "welcome needs figlet",
				Actions: []action.Action{{
					Kind: action.Run, Argv: []string{"brew", "install", "figlet"},
					NeedsElevation: true, Optional: true,
				}},
			})
			if err != nil {
				return err
			}
			seen <- outcome
			return reply.Finish(nil)
		},
	}})

	c.withActionExecution()
	id := c.startSession("welcome")

	reqID, req := c.awaitAction()
	if req.Reason == "" {
		t.Error("core offered actions without saying why, so the client cannot explain them")
	}
	if len(req.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(req.Actions))
	}
	// The client has to be able to show the user exactly what will run.
	if strings.Join(req.Actions[0].Argv, " ") != "brew install figlet" {
		t.Errorf("argv = %v, want the command that will run", req.Actions[0].Argv)
	}
	if !req.Actions[0].NeedsElevation {
		t.Error("the action needs elevation but the client was not told")
	}

	c.reportOutcome(reqID, id, `{"performed":1}`)

	select {
	case got := <-seen:
		if !got.Complete() {
			t.Errorf("outcome = %+v, want complete after the client reported success", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler never saw the outcome")
	}
}

func TestACommandSeesWhichActionsFailed(t *testing.T) {
	seen := make(chan action.Outcome, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			outcome, err := reply.AskActions(ctx, action.Request{
				Actions: []action.Action{{Kind: action.Run, Argv: []string{"apt-get", "install", "figlet"}}},
			})
			if err != nil {
				return err
			}
			seen <- outcome
			return reply.Finish(nil)
		},
	}})

	c.withActionExecution()
	id := c.startSession("welcome")
	reqID, _ := c.awaitAction()

	c.reportOutcome(reqID, id, `{"performed":0,"failures":["apt-get: command not found"]}`)

	select {
	case got := <-seen:
		if got.Complete() {
			t.Error("outcome is complete although an action failed")
		}
		if len(got.Failures) != 1 || !strings.Contains(got.Failures[0], "apt-get") {
			t.Errorf("failures = %v, want the reason the client gave", got.Failures)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler never saw the outcome")
	}
}

func TestActionsAreNotOfferedToAClientThatCannotRunThem(t *testing.T) {
	// A scripted install with no terminal cannot answer a question, and asking anyway
	// would hang it. The warning still has to reach a human reading the output, and the
	// command still has to finish.
	seen := make(chan action.Outcome, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			outcome, err := reply.AskActions(ctx, action.Request{
				Reason:  "welcome needs figlet",
				Actions: []action.Action{{Kind: action.Run, Argv: []string{"brew", "install", "figlet"}}},
			})
			if err != nil {
				return err
			}
			seen <- outcome
			return reply.Finish(nil)
		},
	}})

	// A client that did not declare action_execution: no terminal, or one that chooses
	// not to run things on the user's behalf.
	c.ensureInitialized()
	id := c.startSession("welcome")

	select {
	case got := <-seen:
		if !got.Declined {
			t.Errorf("outcome = %+v, want it declined rather than reported as done", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AskActions never returned for a client that cannot run actions")
	}

	note := c.await("session/notify")
	if !strings.Contains(string(note.Params), "figlet") {
		t.Errorf("notify = %s, want the missing dependency named so a human sees it",
			note.Params)
	}

	fin := c.await("session/finish")
	if got := field(t, fin, "error"); got != "" {
		t.Errorf("the session failed instead of continuing without the dependency: %s", got)
	}
	_ = id
}

func TestAnEmptyOfferIsToldRatherThanAsked(t *testing.T) {
	// Core can name what is missing without having anything to offer. The FIGlet fonts
	// are files with no agreed source, so there is no command to propose — and asking a
	// user to approve an empty list is not a question.
	seen := make(chan action.Outcome, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			outcome, err := reply.AskActions(ctx, action.Request{
				Reason: "welcome needs its FIGlet fonts",
			})
			if err != nil {
				return err
			}
			seen <- outcome
			return reply.Finish(nil)
		},
	}})

	c.withActionExecution()
	id := c.startSession("welcome")

	select {
	case got := <-seen:
		if !got.Declined {
			t.Errorf("outcome = %+v, want it declined rather than reported as done", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AskActions never returned for an offer with nothing in it")
	}

	note := c.await("session/notify")
	if !strings.Contains(string(note.Params), "font") {
		t.Errorf("notify = %s, want what is missing named", note.Params)
	}
	_ = id
}
