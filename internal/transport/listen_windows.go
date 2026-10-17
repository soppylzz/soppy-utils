//go:build windows

package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows reaches the daemon through a named pipe rather than a socket.
//
// A pipe has no filesystem entry: it exists while a handle is open and goes when the
// process does. So there is no stale file to clear, nothing for a listener to leave
// behind, and no mode to narrow after the fact — the access control is the descriptor the
// pipe is created with.
const pipeBuffer = 64 * 1024

// Listener accepts connections for the daemon.
type Listener struct {
	name   string
	sa     *windows.SecurityAttributes
	handle windows.Handle
	mu     sync.Mutex
}

// Listen binds the endpoint.
//
// Nothing is left behind by a daemon that was killed, so the only question is whether one
// already answers — which is asked by connecting, the way the socket does it. The kernel
// knows the answer and cannot be mistaken for a leftover file.
func Listen(ep Endpoint) (*Listener, error) {
	if ep.Network != NetworkPipe {
		return nil, fmt.Errorf("transport: %s endpoints are not supported on this platform", ep.Network)
	}
	if conn, err := Dial(ep); err == nil {
		conn.Close()
		return nil, fmt.Errorf("%s: %w", ep.Address, ErrInUse)
	}

	sa, err := ownerOnly()
	if err != nil {
		return nil, err
	}
	h, err := createInstance(ep.Address, sa)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", ep.Address, err)
	}
	return &Listener{name: ep.Address, sa: sa, handle: h}, nil
}

// ownerOnly is a descriptor granting the current user and nobody else.
//
// A pipe created with no descriptor is reachable by every process on the machine, where
// the socket on the other platform is held by a 0700 directory and its own 0600 mode.
// This is the same intent expressed the way this platform expresses it.
func ownerOnly() (*windows.SecurityAttributes, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read the current user: %w", err)
	}
	// D:P is a protected DACL, so nothing from the parent object is inherited in.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, fmt.Errorf("build the pipe's descriptor: %w", err)
	}
	return &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}, nil
}

func createInstance(name string, sa *windows.SecurityAttributes) (windows.Handle, error) {
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	return windows.CreateNamedPipe(path,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		pipeBuffer, pipeBuffer,
		0, sa)
}

// Accept waits for the next client.
//
// The instance it accepted on is handed to the connection and replaced before this
// returns, so a caller that keeps accepting never has to make one.
//
// Closing the listener is what ends a wait: the blocked connect fails when the handle
// goes, which is how the accept loop learns to stop rather than a flag it has to poll.
func (l *Listener) Accept() (net.Conn, error) {
	l.mu.Lock()
	handle := l.handle
	l.mu.Unlock()
	if handle == 0 {
		return nil, errors.New("transport: the listener is closed")
	}

	// The next instance is made before this one is waited on, not after it is taken.
	// Windows gives a client "All pipe instances are busy" when every instance is
	// connected and none is free, and a listener that makes its next instance only after
	// a client has been accepted has exactly that gap: between the connection arriving and
	// the next instance existing, anybody else dialling is told the pipe is busy.
	next, err := createInstance(l.name, l.sa)
	if err != nil {
		return nil, fmt.Errorf("create the next instance of %s: %w", l.name, err)
	}

	for {
		err := windows.ConnectNamedPipe(handle, nil)
		if err == nil {
			break
		}
		if errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			// A client connected between the instance being made and this call. That is
			// the case this call exists to notice, not a failure.
			break
		}
		if errors.Is(err, windows.ERROR_NO_DATA) || errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) {
			// A client connected and went away before this call noticed — what a caller
			// probing for a daemon leaves behind, and what this package's own Listen does
			// before it creates its first instance. The instance is left connected by
			// that, and connecting it again fails until it is disconnected.
			if derr := windows.DisconnectNamedPipe(handle); derr != nil {
				windows.CloseHandle(next)
				return nil, derr
			}
			continue
		}
		windows.CloseHandle(next)
		return nil, err
	}

	conn, err := newPipeConn(handle)
	if err != nil {
		windows.CloseHandle(next)
		return nil, err
	}

	l.mu.Lock()
	if l.handle == 0 {
		l.mu.Unlock()
		windows.CloseHandle(next)
		conn.Close()
		return nil, errors.New("transport: the listener is closed")
	}
	l.handle = next
	l.mu.Unlock()
	return conn, nil
}

// Close stops accepting, and releases the name once nothing else holds it.
//
// There is nothing for SetUnlinkOnClose to decide here. On the other platform a socket
// file is deliberately left behind while a daemon drains, so that a caller waiting for it
// can tell "stopped accepting" from "finished"; a pipe has no file, and its name is
// released when the last handle to it closes, which is when the process goes.
func (l *Listener) Close() error {
	l.mu.Lock()
	handle := l.handle
	l.handle = 0
	l.mu.Unlock()
	if handle != 0 {
		return windows.CloseHandle(handle)
	}
	return nil
}

func (l *Listener) Addr() string { return l.name }

// SetUnlinkOnClose exists so that this build exposes the same API as the other one.
// There is no filesystem entry for it to act on.
func (l *Listener) SetUnlinkOnClose(bool) {}

// Dial connects to a daemon. Its failure is how a client learns that no daemon is running
// and that it must start one.
func Dial(ep Endpoint) (net.Conn, error) {
	if ep.Network != NetworkPipe {
		return nil, fmt.Errorf("transport: %s endpoints are not supported on this platform", ep.Network)
	}
	path, err := windows.UTF16PtrFromString(ep.Address)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(path,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	return newPipeConn(h)
}

// pipeConn is one end of a connected pipe.
//
// Reads and writes are blocking, which is a choice rather than a limitation: a synchronous
// transfer needs no OVERLAPPED structure, and an OVERLAPPED one has to outlive the call
// that started it, which means memory Windows writes into that the Go runtime is free to
// move. The deadline is enforced by cancelling instead, which is what CancelIoEx is for.
type pipeConn struct {
	handle windows.Handle

	// One reader and one writer, which is what the protocol has: a message is read in one
	// goroutine and written in another, and neither is concurrent with itself. The mutex
	// guards the deadlines, not the transfers.
	mu    sync.Mutex
	rDead time.Time
	wDead time.Time
	once  sync.Once
}

func newPipeConn(h windows.Handle) (*pipeConn, error) { return &pipeConn{handle: h}, nil }

func (c *pipeConn) Read(p []byte) (int, error)  { return c.transfer(p, true) }
func (c *pipeConn) Write(p []byte) (int, error) { return c.transfer(p, false) }

func (c *pipeConn) transfer(p []byte, reading bool) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// A timer armed before the transfer rather than a timeout on the call: the call has
	// no timeout to give, so the way to stop waiting is to cancel it from outside.
	deadline := c.deadline(reading)
	stop := c.arm(deadline)
	defer stop()

	var done uint32
	var err error
	if reading {
		err = windows.ReadFile(c.handle, p, &done, nil)
	} else {
		err = windows.WriteFile(c.handle, p, &done, nil)
	}
	if err != nil {
		return 0, c.mapError(err, reading)
	}
	if !reading && done == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	return int(done), nil
}

// arm cancels the transfer when the deadline passes. The returned function disarms it.
func (c *pipeConn) arm(deadline time.Time) func() {
	if deadline.IsZero() {
		return func() {}
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		c.cancel()
		return func() {}
	}
	timer := time.AfterFunc(remaining, c.cancel)
	return func() { timer.Stop() }
}

// cancel aborts whatever is in flight on this connection.
//
// The handle carries no OVERLAPPED to name one operation with, so this cancels all of
// them. That is right for the way this is used — a handshake that reads with a budget, and
// a test that waits for something that is not coming — and wrong for a connection whose
// reader and writer are both busy, where a read timing out would take a write with it.
// Wiring a deadline onto that shape would need overlapped I/O.
func (c *pipeConn) cancel() { _ = windows.CancelIoEx(c.handle, nil) }

func (c *pipeConn) mapError(err error, reading bool) error {
	switch {
	case errors.Is(err, windows.ERROR_BROKEN_PIPE), errors.Is(err, windows.ERROR_HANDLE_EOF),
		errors.Is(err, windows.ERROR_NO_DATA):
		if reading {
			return io.EOF
		}
		return io.ErrClosedPipe
	case errors.Is(err, windows.ERROR_OPERATION_ABORTED):
		// Cancelled: either the deadline passed or the connection was closed under it.
		if deadline := c.deadline(reading); !deadline.IsZero() && !time.Now().Before(deadline) {
			return errTimeout{}
		}
		if reading {
			return io.EOF
		}
		return io.ErrClosedPipe
	default:
		return err
	}
}

func (c *pipeConn) deadline(reading bool) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if reading {
		return c.rDead
	}
	return c.wDead
}

func (c *pipeConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.rDead, c.wDead = t, t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.rDead = t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.wDead = t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) Close() error {
	var err error
	c.once.Do(func() {
		// Cancel first: a read blocked on this connection is what a closing peer is
		// waiting to be freed, and closing the handle is not guaranteed to wake it.
		c.cancel()
		err = windows.CloseHandle(c.handle)
	})
	return err
}

func (c *pipeConn) LocalAddr() net.Addr  { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return NetworkPipe }
func (pipeAddr) String() string  { return "pipe" }

// errTimeout is what a caller expects when a deadline passes. It carries the two methods
// net.Error is asked for, so a caller testing for a timeout does not have to know which
// platform it is on.
type errTimeout struct{}

func (errTimeout) Error() string   { return "transport: i/o timeout" }
func (errTimeout) Timeout() bool   { return true }
func (errTimeout) Temporary() bool { return true }
