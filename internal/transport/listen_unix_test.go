//go:build !windows

package transport

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// Unix socket paths are capped by the kernel at around 104 bytes. t.TempDir() on
// macOS returns a path deep enough to blow that budget, so these tests build their
// own short root.
func tempEndpoint(t *testing.T) Endpoint {
	t.Helper()
	root := tempDir(t)

	ep, err := Resolve(runtime.GOOS, root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return ep
}

func TestListenCreatesAnOwnerOnlySocket(t *testing.T) {
	ep := tempEndpoint(t)

	l, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	dirInfo, err := os.Stat(filepath.Dir(ep.Address))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode = %o, want 700", got)
	}

	sockInfo, err := os.Stat(ep.Address)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if got := sockInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("socket mode = %o, want 600: any process could talk to core", got)
	}
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	ep := tempEndpoint(t)
	if err := os.MkdirAll(filepath.Dir(ep.Address), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// A daemon that was killed rather than shut down leaves exactly this behind: a
	// socket file nobody is listening on.
	dead, err := net.Listen("unix", ep.Address)
	if err != nil {
		t.Fatalf("seed listener: %v", err)
	}
	dead.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := dead.Close(); err != nil {
		t.Fatalf("close seed listener: %v", err)
	}
	if _, err := os.Stat(ep.Address); err != nil {
		t.Fatalf("expected a leftover socket file: %v", err)
	}

	l, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen on a stale socket: %v", err)
	}
	defer l.Close()

	if conn, err := Dial(ep); err == nil {
		conn.Close()
	} else {
		t.Errorf("Dial after replacing the stale socket: %v", err)
	}
}

func TestListenRefusesASecondDaemon(t *testing.T) {
	ep := tempEndpoint(t)

	first, err := Listen(ep)
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer first.Close()

	// Two daemons on one endpoint would each think they own the state. The loser has
	// to exit quietly rather than unlink the winner's socket.
	_, err = Listen(ep)
	if !errors.Is(err, ErrInUse) {
		t.Fatalf("second Listen = %v, want ErrInUse", err)
	}
	if _, err := os.Stat(ep.Address); err != nil {
		t.Errorf("the loser removed the winner's socket: %v", err)
	}
	if _, err := Dial(ep); err != nil {
		t.Errorf("the winner stopped accepting after the race: %v", err)
	}
}

func TestRoundTripThroughTheTransport(t *testing.T) {
	ep := tempEndpoint(t)

	l, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		msg, err := protocol.NewReader(conn).Read()
		if err != nil {
			return
		}
		_ = protocol.NewWriter(conn).WriteMessage(
			protocol.NewResult(*msg.ID, []byte(`{"protocol_version":1}`)))
	}()

	conn, err := Dial(ep)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := protocol.NewWriter(conn).WriteMessage(
		protocol.NewRequest(1, "initialize", nil)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reply, err := protocol.NewReader(conn).Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if reply.Kind() != protocol.KindResponse {
		t.Errorf("Kind() = %q, want response", reply.Kind())
	}
}

func TestDialFailsWhenNothingIsListening(t *testing.T) {
	// The client uses this to decide that it must start a daemon.
	ep := tempEndpoint(t)
	if _, err := Dial(ep); err == nil {
		t.Fatal("Dial succeeded with no listener, want error")
	}
}

func TestCloseRemovesTheSocket(t *testing.T) {
	ep := tempEndpoint(t)

	l, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(ep.Address); !os.IsNotExist(err) {
		t.Errorf("socket file still present after Close: %v", err)
	}
}

func TestListenRejectsANonUnixEndpoint(t *testing.T) {
	ep := Endpoint{Network: NetworkPipe, Address: `\\.\pipe\x`}
	if _, err := Listen(ep); err == nil {
		t.Fatal("Listen accepted a pipe endpoint on a unix build, want error")
	}
}

func TestAListenerCanLeaveItsSocketBehind(t *testing.T) {
	// A daemon that has stopped accepting but is still finishing its work needs the file
	// to stay, so that a caller can tell the two apart.
	ep := tempEndpoint(t)

	l, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	l.SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(ep.Address); err != nil {
		t.Errorf("the socket file was removed although the listener was told to leave it: %v", err)
	}
}

// tempDir is where a test may put a socket.
//
// A Unix socket path is capped at about a hundred bytes, and a per-test temporary
// directory under /var/folders on macOS is longer than that before the socket's own name
// is added. /tmp is short and exists on every Unix. It does not exist on Windows, where
// there is no such cap and the ordinary temporary directory is the right answer.
func tempDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return t.TempDir()
	}
	dir, err := os.MkdirTemp("/tmp", "spu")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
