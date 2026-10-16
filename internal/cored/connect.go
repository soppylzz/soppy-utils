package cored

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// Handshake is what core reported about itself when the client connected.
type Handshake struct {
	ProtocolVersion int
	Version         string
}

// Conn is a negotiated connection to core.
//
// It holds the Reader and Writer for the connection rather than making new ones per
// message: a Reader buffers, so a second one would start from whatever the first had
// already pulled off the socket and the buffered bytes would be lost.
type Conn struct {
	net.Conn
	r    *protocol.Reader
	w    *protocol.Writer
	Info Handshake
}

// Read returns the next message from core.
func (c *Conn) Read() (protocol.Incoming, error) { return c.r.Read() }

// Send writes one message to core.
func (c *Conn) Send(v any) error { return c.w.WriteMessage(v) }

// rpcError carries a protocol error out of an exchange so a caller can switch on its
// code instead of matching on a message.
type rpcError struct{ err *protocol.Error }

func (e rpcError) Error() string {
	return fmt.Sprintf("core: %s (code %d)", e.err.Message, e.err.Code)
}

// Connect reaches a core, agreeing on a protocol version.
//
// A core that speaks a different protocol version is replaced rather than tolerated.
// Guessing at a version is how two builds end up disagreeing about the shape of a
// message three layers into a util, so the running core is asked to stop and the
// installed binary is started in its place. That is exactly the situation spu update
// creates: the daemon that is running was started from a binary that no longer exists.
//
// The restart happens at most once. A replacement that still disagrees means the
// installed binary is not what this client expects, and starting it again cannot fix
// that.
func Connect(ctx context.Context, opts AttachOptions) (*Conn, error) {
	conn, err := attach(ctx, opts)
	if err != nil {
		return nil, err
	}
	r := protocol.NewReader(conn)
	w := protocol.NewWriter(conn)

	info, err := handshake(r, w, opts)
	if err == nil {
		return &Conn{Conn: conn, r: r, w: w, Info: info}, nil
	}

	var rpc rpcError
	if !errors.As(err, &rpc) || rpc.err.Code != protocol.CodeProtocolMismatch {
		conn.Close()
		return nil, err
	}

	conn.Close()
	if err := Shutdown(ctx, opts.Endpoint, opts.Budget/2); err != nil {
		return nil, fmt.Errorf("stop the out-of-date core: %w", err)
	}

	conn, err = attach(ctx, opts)
	if err != nil {
		return nil, err
	}
	r = protocol.NewReader(conn)
	w = protocol.NewWriter(conn)

	info, err = handshake(r, w, opts)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("the restarted core still disagrees: %w", err)
	}
	return &Conn{Conn: conn, r: r, w: w, Info: info}, nil
}

func handshake(r *protocol.Reader, w *protocol.Writer, opts AttachOptions) (Handshake, error) {
	env := opts.Env
	if env.GOOS == "" {
		// The client and the daemon are on the same machine, so this is a fact about
		// where the client is running rather than a guess about the daemon.
		env.GOOS = runtime.GOOS
	}
	params, err := json.Marshal(map[string]any{
		"client": map[string]any{
			"kind":              opts.Kind,
			"protocol_versions": []int{protocol.Version},
			"capabilities":      opts.Capabilities,
			"env":               env,
		},
	})
	if err != nil {
		return Handshake{}, fmt.Errorf("encode initialize: %w", err)
	}
	if err := w.WriteMessage(protocol.NewRequest(1, "initialize", params)); err != nil {
		return Handshake{}, err
	}

	msg, err := r.Read()
	if err != nil {
		return Handshake{}, err
	}
	if msg.Error != nil {
		return Handshake{}, rpcError{err: msg.Error}
	}
	if msg.Kind() != protocol.KindResponse {
		return Handshake{}, fmt.Errorf("core answered initialize with a %s", msg.Kind())
	}

	var out initializeResult
	if err := json.Unmarshal(msg.Result, &out); err != nil {
		return Handshake{}, fmt.Errorf("core's handshake reply: %w", err)
	}
	return Handshake{ProtocolVersion: out.ProtocolVersion, Version: out.Core.Version}, nil
}

// Shutdown asks a running core to stop and waits for the endpoint to go quiet.
//
// It is the step spu update needs before it replaces the binary, and spu uninstall needs
// before it removes the directory. On Windows a running executable cannot be replaced;
// everywhere, a daemon left behind is a process holding files that no longer exist.
//
// A core that is not running is not an error. There is nothing to stop, which is the
// state the caller was asking for.
func Shutdown(ctx context.Context, ep transport.Endpoint, budget time.Duration) error {
	if budget <= 0 {
		budget = 2 * time.Second
	}

	conn, err := transport.Dial(ep)
	if err != nil {
		return nil // nothing is listening
	}

	r := protocol.NewReader(conn)
	w := protocol.NewWriter(conn)
	if err := w.WriteMessage(protocol.NewRequest(1, "server/shutdown", json.RawMessage(`{}`))); err == nil {
		// The reply is read before the connection closes. The daemon writes its answer
		// first and drains afterwards, so dropping the connection immediately would skip
		// the drain and abandon a session that was still running.
		_ = conn.SetReadDeadline(time.Now().Add(budget))
		_, _ = r.Read()
	}
	conn.Close()

	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		// The socket file going away is what says the daemon has finished rather than
		// merely stopped accepting. It leaves the file in place while it drains, so a
		// caller that replaced the binary when the connection was refused would be
		// replacing it underneath a process still running it.
		if _, err := os.Stat(ep.Address); err != nil {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("core did not stop within %s", budget)
}
