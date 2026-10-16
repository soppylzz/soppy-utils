package cored

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

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
	id     string
	cancel context.CancelFunc
	conn   *clientConn

	mu    sync.Mutex
	ended bool
}

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
