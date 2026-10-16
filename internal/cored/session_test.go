package cored

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// scriptedHandler runs a function, so a test can describe what a util does without
// there being a util.
type scriptedHandler struct {
	run func(ctx context.Context, cmd Command, reply Responder) error
}

func (h scriptedHandler) Run(ctx context.Context, cmd Command, reply Responder) error {
	return h.run(ctx, cmd, reply)
}

func field(t *testing.T, msg protocol.Incoming, key string) string {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(msg.Params, &got); err != nil {
		t.Fatalf("unmarshal %s params: %v", msg.Method, err)
	}
	s, _ := got[key].(string)
	return s
}

func (c *client) await(method string) protocol.Incoming {
	c.t.Helper()
	for i := 0; i < 20; i++ {
		msg := c.read()
		if msg.Kind() == protocol.KindNotification && msg.Method == method {
			return msg
		}
	}
	c.t.Fatalf("no %s notification arrived", method)
	return protocol.Incoming{}
}

// ensureInitialized handshakes once per connection. It is not folded into dial()
// because a test has to be able to ask what an un-initialized connection is allowed
// to do.
func (c *client) ensureInitialized() {
	c.t.Helper()
	if c.initialized {
		return
	}
	reply := c.call("initialize", initParams(c.t, map[string]any{"os": "linux"}))
	if reply.Error != nil {
		c.t.Fatalf("initialize: %+v", reply.Error)
	}
	c.initialized = true
}

func (c *client) startSession(util string) string {
	c.t.Helper()
	c.ensureInitialized()
	reply := c.call("command/run", `{"util":"`+util+`"}`)
	if reply.Error != nil {
		c.t.Fatalf("command/run failed: %+v", reply.Error)
	}
	var started struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(reply.Result, &started); err != nil {
		c.t.Fatalf("unmarshal command/run result: %v", err)
	}
	if started.SessionID == "" {
		// Without an id the client cannot cancel the session it just started.
		c.t.Fatal("command/run returned no session id")
	}
	return started.SessionID
}

func TestACommandRunsInASession(t *testing.T) {
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			if err := reply.Notify("checking configuration"); err != nil {
				return err
			}
			return reply.Finish(json.RawMessage(`{"ok":true}`))
		},
	}})

	id := c.startSession("welcome")

	note := c.await("session/notify")
	if got := field(t, note, "session_id"); got != id {
		t.Errorf("notify session_id = %q, want %q", got, id)
	}
	if got := field(t, note, "message"); got != "checking configuration" {
		t.Errorf("notify message = %q", got)
	}

	fin := c.await("session/finish")
	if got := field(t, fin, "session_id"); got != id {
		t.Errorf("finish session_id = %q, want %q", got, id)
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(fin.Params, &struct {
		Result *struct {
			OK bool `json:"ok"`
		} `json:"result"`
	}{Result: &result}); err != nil {
		t.Fatalf("unmarshal finish params: %v", err)
	}
	if !result.OK {
		t.Errorf("finish result = %s, want ok true", fin.Params)
	}
}

func TestTheHandlerReceivesTheCommand(t *testing.T) {
	got := make(chan Command, 1)
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			got <- cmd
			return reply.Finish(nil)
		},
	}})

	c.startSession("welcome")
	c.await("session/finish")

	select {
	case cmd := <-got:
		if cmd.Util != "welcome" {
			t.Errorf("Util = %q, want welcome", cmd.Util)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the handler was never called")
	}
}

func TestAClientDisconnectCancelsItsSession(t *testing.T) {
	cancelled := make(chan struct{})
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		},
	}})

	c.startSession("welcome")
	c.conn.Close()

	// A half-finished command must not keep running, and must not hold the state it
	// was about to write.
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("the session outlived the client that started it")
	}
}

func TestSessionsOnTwoConnectionsAreIndependent(t *testing.T) {
	c1 := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			return reply.Finish(json.RawMessage(`{"util":"` + cmd.Util + `"}`))
		},
	}})
	c2 := c1.dial()

	id1 := c1.startSession("welcome")
	id2 := c2.startSession("license")
	if id1 == id2 {
		t.Fatalf("both sessions got id %q, so they cannot be told apart", id1)
	}

	// Each connection must see only its own session.
	fin1 := c1.await("session/finish")
	if got := field(t, fin1, "session_id"); got != id1 {
		t.Errorf("first connection saw session %q, want %q", got, id1)
	}
	fin2 := c2.await("session/finish")
	if got := field(t, fin2, "session_id"); got != id2 {
		t.Errorf("second connection saw session %q, want %q", got, id2)
	}
}

func TestAHandlerErrorStillEndsTheSession(t *testing.T) {
	// A handler that fails and returns without finishing would leave the client
	// waiting on a session that is never going to speak again.
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			return errors.New("figlet is missing")
		},
	}})

	c.startSession("welcome")

	fin := c.await("session/finish")
	if !strings.Contains(string(fin.Params), "figlet is missing") {
		t.Errorf("finish params = %s, want the handler's error", fin.Params)
	}
}

func TestASessionCannotFinishTwice(t *testing.T) {
	c := startDaemon(t, Options{Handler: scriptedHandler{
		run: func(ctx context.Context, cmd Command, reply Responder) error {
			if err := reply.Finish(nil); err != nil {
				return err
			}
			// A second finish would look to the client like a second session ending,
			// or worse, silently overwrite the first result.
			return reply.Finish(json.RawMessage(`{"late":true}`))
		},
	}})

	c.startSession("welcome")
	c.await("session/finish")

	// Nothing else should arrive. The next read has to time out rather than return.
	_ = c.conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if msg, err := c.r.Read(); err == nil {
		t.Errorf("a second message arrived after finish: %s %s", msg.Method, msg.Params)
	}
}
