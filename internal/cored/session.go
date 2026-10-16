package cored

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// clientConn serializes writes to one client.
//
// A session runs in its own goroutine and writes notifications while serveConn may be
// writing the response to another request on the same connection. Two goroutines
// sharing a Writer interleave their bytes and produce a line neither end can parse,
// so every write to a connection goes through here.
type clientConn struct {
	conn net.Conn
	w    *protocol.Writer
	mu   sync.Mutex
}

func newClientConn(conn net.Conn) *clientConn {
	return &clientConn{conn: conn, w: protocol.NewWriter(conn)}
}

func (c *clientConn) write(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.w.WriteMessage(v)
}

// session is one running command, and the Responder its handler talks to.
//
// A session can end exactly once. That is not a formality: a second finish would
// look to the client like a second session ending, or silently replace the result it
// already acted on.
type session struct {
	id           string
	cancel       context.CancelFunc
	conn         *clientConn
	capabilities Capabilities

	mu       sync.Mutex
	ended    bool
	requests int64
	waiting  map[int64]chan json.RawMessage
}

// Ask sends a form to the client and waits for the answers.
//
// This is the heart of what core does not do: it decides which fields it needs and what
// each accepts, and the client decides how to ask. A terminal renders one with a form
// library and a window renders the same description with markup, which is why the two
// clients can be equal rather than one of them degrading into a prompt.
func (s *session) Ask(ctx context.Context, spec form.Spec) (map[string]any, error) {
	answer, err := s.question(ctx, "session/request_input", map[string]any{"spec": spec})
	if err != nil {
		return nil, err
	}
	if len(answer) == 0 {
		return nil, nil
	}
	var values map[string]any
	if err := json.Unmarshal(answer, &values); err != nil {
		return nil, fmt.Errorf("answer to session %s: %w", s.id, err)
	}
	return values, nil
}

// AskActions offers work that core cannot do itself.
//
// A client that did not say it can run actions is asked nothing. It is told what is
// missing instead, because the warning still has to reach a human reading the output,
// and the session continues: a dependency that cannot be installed leaves a util
// degraded, which is a legitimate state rather than a failure.
func (s *session) AskActions(ctx context.Context, req action.Request) (action.Outcome, error) {
	// Nothing to offer, or nobody to offer it to. Both end the same way: the user is
	// told what is missing, and the session continues. Asking somebody to approve an
	// empty list is not a question.
	if len(req.Actions) == 0 || !s.capabilities.ActionExecution {
		if err := s.Notify(missingNote(req)); err != nil {
			return action.Outcome{}, err
		}
		return action.Outcome{Declined: true}, nil
	}

	answer, err := s.question(ctx, "session/request_action", map[string]any{
		"reason":  req.Reason,
		"actions": req.Actions,
	})
	if err != nil {
		return action.Outcome{}, err
	}
	var out action.Outcome
	if len(answer) > 0 {
		if err := json.Unmarshal(answer, &out); err != nil {
			return action.Outcome{}, fmt.Errorf("answer to session %s: %w", s.id, err)
		}
	}
	return out, nil
}

// question sends something to the client and waits for the answer.
//
// The answer comes back on a correlation id rather than as a reply to this message.
// Both directions of this connection carry requests, and sharing one id space between
// them would make a client's request 1 and core's request 1 the same number.
func (s *session) question(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %s has already finished", s.id)
	}
	s.requests++
	id := s.requests
	if s.waiting == nil {
		s.waiting = make(map[int64]chan json.RawMessage)
	}
	answer := make(chan json.RawMessage, 1)
	s.waiting[id] = answer
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.waiting, id)
		s.mu.Unlock()
	}()

	params["session_id"] = s.id
	params["request_id"] = id
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", method, err)
	}
	if err := s.conn.write(protocol.NewNotification(method, payload)); err != nil {
		return nil, err
	}

	select {
	case raw := <-answer:
		return raw, nil
	case <-ctx.Done():
		// The client went away, or the session was cancelled. Either way nobody is going
		// to answer, and a handler left waiting here would hold its session open.
		return nil, ctx.Err()
	}
}

// missingNote is what a user sees when nothing can be installed for them.
func missingNote(req action.Request) string {
	what := req.Reason
	if what == "" {
		parts := make([]string, 0, len(req.Actions))
		for _, a := range req.Actions {
			switch {
			case len(a.Argv) > 0:
				parts = append(parts, strings.Join(a.Argv, " "))
			case a.URL != "":
				parts = append(parts, a.URL)
			}
		}
		what = "this needs " + strings.Join(parts, ", ")
	}
	return what + "; this client cannot install it"
}

// deliver hands answers to the request that asked for them, reporting whether one was
// waiting.
func (s *session) deliver(requestID int64, values json.RawMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.waiting[requestID]
	if !ok {
		return false
	}
	select {
	case ch <- values:
	default:
		// Buffered with room for one. A second answer to the same question is a client
		// bug, not something to block on.
	}
	return true
}

// CanAsk reports whether the client that started this session said a person is
// available to answer.
func (s *session) CanAsk() bool { return s.capabilities.Interactive }

// Notify reports progress. It is a notification rather than a request because a util
// saying what it is doing is not asking a question.
func (s *session) Notify(message string) error {
	if s.hasEnded() {
		return fmt.Errorf("session %s has already finished", s.id)
	}
	payload, err := json.Marshal(map[string]any{"session_id": s.id, "message": message})
	if err != nil {
		return fmt.Errorf("encode session/notify: %w", err)
	}
	return s.conn.write(protocol.NewNotification("session/notify", payload))
}

// Finish ends the session and carries its result.
func (s *session) Finish(result json.RawMessage) error {
	payload, err := json.Marshal(map[string]any{"session_id": s.id, "result": result})
	if err != nil {
		return fmt.Errorf("encode session/finish: %w", err)
	}
	if !s.end() {
		return fmt.Errorf("session %s has already finished", s.id)
	}
	return s.conn.write(protocol.NewNotification("session/finish", payload))
}

// fail ends the session with an error.
//
// The daemon calls it whenever a handler returns without finishing, so that a client
// is never left waiting on a session that will not speak again. It is a no-op when the
// session already ended, which is the normal case for a handler that finished and then
// returned an error from its own last write.
func (s *session) fail(cause error) error {
	if !s.end() {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"session_id": s.id, "error": cause.Error()})
	if err != nil {
		return fmt.Errorf("encode session/finish: %w", err)
	}
	return s.conn.write(protocol.NewNotification("session/finish", payload))
}

// end marks the session finished and reports whether this call is the one that did it.
func (s *session) end() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return false
	}
	s.ended = true
	return true
}

func (s *session) hasEnded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}
