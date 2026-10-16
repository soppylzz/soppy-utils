package cored

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// nopHandler accepts every command and finishes without doing anything.
type nopHandler struct{}

func (nopHandler) Run(ctx context.Context, cmd Command, reply Responder) error {
	return reply.Finish(nil)
}

// client keeps one Reader and one Writer for the connection.
//
// Wrapping the connection again for each message would be a bug: a Reader buffers,
// so a second Reader would start from whatever the first had already pulled off the
// socket and the buffered bytes would be lost. The real client has to hold the same
// discipline, which is why the helper does too.
type client struct {
	t    *testing.T
	ep   transport.Endpoint
	conn net.Conn
	r    *protocol.Reader
	w    *protocol.Writer
}

// dial opens a second connection to the same daemon, which is how a test asks what
// one client sees while another is doing something.
func (c *client) dial() *client {
	c.t.Helper()
	conn, err := transport.Dial(c.ep)
	if err != nil {
		c.t.Fatalf("Dial: %v", err)
	}
	c.t.Cleanup(func() { conn.Close() })
	return &client{t: c.t, ep: c.ep, conn: conn, r: protocol.NewReader(conn), w: protocol.NewWriter(conn)}
}

func (c *client) send(id int64, method string, params string) {
	c.t.Helper()
	var raw json.RawMessage
	if params != "" {
		raw = json.RawMessage(params)
	}
	if err := c.w.WriteMessage(protocol.NewRequest(id, method, raw)); err != nil {
		c.t.Fatalf("write %s: %v", method, err)
	}
}

func (c *client) read() protocol.Incoming {
	c.t.Helper()
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.t.Fatalf("SetReadDeadline: %v", err)
	}
	msg, err := c.r.Read()
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return msg
}

// call sends a request and returns the next response. Notifications that arrive
// first are dropped, since a response is what the caller is waiting for.
func (c *client) call(method string, params string) protocol.Incoming {
	c.t.Helper()
	c.send(1, method, params)
	for {
		msg := c.read()
		if msg.Kind() == protocol.KindResponse {
			return msg
		}
	}
}

// startDaemon binds a daemon on a private endpoint, serves it in the background, and
// returns a connected client. The daemon is stopped and the socket removed on
// cleanup, so no test leaves a process or a file behind.
func startDaemon(t *testing.T, opts Options) *client {
	t.Helper()

	root, err := os.MkdirTemp("/tmp", "spu")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })

	ep, err := transport.Resolve("linux", root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts.Endpoint = ep
	if opts.Version == "" {
		opts.Version = "test"
	}
	if opts.Handler == nil {
		opts.Handler = nopHandler{}
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = time.Minute
	}

	d, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan struct{})
	go func() {
		defer close(served)
		if err := d.Serve(ctx); err != nil {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-served:
		case <-time.After(5 * time.Second):
			t.Error("Serve did not return after the context was cancelled")
		}
	})

	conn, err := transport.Dial(ep)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &client{t: t, ep: ep, conn: conn, r: protocol.NewReader(conn), w: protocol.NewWriter(conn)}
}

func TestInitializeReportsTheProtocolVersion(t *testing.T) {
	c := startDaemon(t, Options{})

	reply := c.call("initialize", `{"client":{"protocol_versions":[1]}}`)
	if reply.Error != nil {
		t.Fatalf("initialize failed: %+v", reply.Error)
	}

	var got struct {
		ProtocolVersion int `json:"protocol_version"`
		Core            struct {
			Version string `json:"version"`
		} `json:"core"`
	}
	if err := json.Unmarshal(reply.Result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got.ProtocolVersion != protocol.Version {
		t.Errorf("protocol_version = %d, want %d", got.ProtocolVersion, protocol.Version)
	}
	if got.Core.Version != "test" {
		t.Errorf("core.version = %q, want test", got.Core.Version)
	}
}

func TestInitializeRefusesAClientWithNoSharedVersion(t *testing.T) {
	c := startDaemon(t, Options{})

	reply := c.call("initialize", `{"client":{"protocol_versions":[99]}}`)
	if reply.Error == nil {
		t.Fatal("initialize succeeded with no shared protocol version, want an error")
	}
	if reply.Error.Code != protocol.CodeProtocolMismatch {
		t.Errorf("code = %d, want %d", reply.Error.Code, protocol.CodeProtocolMismatch)
	}
}

func TestInitializeRefusesAClientThatDoesNotNegotiate(t *testing.T) {
	c := startDaemon(t, Options{})

	// A client that sends no versions has not negotiated, and guessing which one it
	// speaks is how two builds end up disagreeing about the shape of a message.
	reply := c.call("initialize", `{"client":{}}`)
	if reply.Error == nil {
		t.Fatal("initialize succeeded without a version list, want an error")
	}
	if reply.Error.Code != protocol.CodeProtocolMismatch {
		t.Errorf("code = %d, want %d", reply.Error.Code, protocol.CodeProtocolMismatch)
	}
}

func TestAnUnknownMethodIsRejected(t *testing.T) {
	c := startDaemon(t, Options{})

	reply := c.call("nonsense/method", `{}`)
	if reply.Error == nil {
		t.Fatal("an unknown method succeeded, want an error")
	}
	if reply.Error.Code != protocol.CodeMethodNotFound {
		t.Errorf("code = %d, want %d", reply.Error.Code, protocol.CodeMethodNotFound)
	}
}

func TestInitializeRejectsMalformedParams(t *testing.T) {
	c := startDaemon(t, Options{})

	// Valid JSON, wrong shape. This is a client bug rather than a version skew, so it
	// must not be reported as a version mismatch: the two send the client in opposite
	// directions.
	reply := c.call("initialize", `"not an object"`)
	if reply.Error == nil {
		t.Fatal("initialize accepted malformed params, want an error")
	}
	if reply.Error.Code != protocol.CodeInvalidParams {
		t.Errorf("code = %d, want %d", reply.Error.Code, protocol.CodeInvalidParams)
	}
}
