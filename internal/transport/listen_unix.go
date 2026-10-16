//go:build !windows

package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// Listener accepts connections for the daemon.
type Listener struct {
	ln   *net.UnixListener
	path string
}

// Listen binds the endpoint, creating its directory when needed.
//
// Three situations share one code path, and the difference between them matters:
//
//   - Nothing is there: bind.
//   - A socket file is there and nobody answers it: the previous daemon was killed
//     rather than shut down. Remove the leftover and bind.
//   - A socket file is there and somebody answers it: a daemon is running. Report
//     ErrInUse without touching the file.
//
// The probe is a connect rather than a pidfile or a lock, because the kernel already
// knows the answer and a stale file cannot lie about it.
func Listen(ep Endpoint) (*Listener, error) {
	if ep.Network != NetworkUnix {
		return nil, fmt.Errorf("transport: %s endpoints are not supported on this platform", ep.Network)
	}

	if err := os.MkdirAll(filepath.Dir(ep.Address), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(ep.Address), err)
	}

	if _, err := os.Lstat(ep.Address); err == nil {
		if live, dialErr := dialUnix(ep.Address); dialErr == nil {
			live.Close()
			return nil, fmt.Errorf("%s: %w", ep.Address, ErrInUse)
		}
		if err := os.Remove(ep.Address); err != nil {
			return nil, fmt.Errorf("remove stale socket %s: %w", ep.Address, err)
		}
	}

	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: ep.Address, Net: "unix"})
	if errors.Is(err, syscall.EADDRINUSE) {
		// Something bound between the probe and here.
		return nil, fmt.Errorf("%s: %w", ep.Address, ErrInUse)
	}
	if err != nil {
		return nil, err
	}

	// The kernel creates the socket with the process umask applied, which is broader
	// than this wants, so it is narrowed right after. An unprivileged process cannot
	// create a Unix socket with a chosen mode portably, so the window exists; keeping
	// the containing directory 0700 is what actually holds the line, and this is the
	// second lock on the same door.
	if err := os.Chmod(ep.Address, 0o600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("restrict %s: %w", ep.Address, err)
	}

	return &Listener{ln: ln, path: ep.Address}, nil
}

// Accept returns the next client connection. Closing the listener closes the socket
// file with it.
func (l *Listener) Accept() (net.Conn, error) { return l.ln.Accept() }

func (l *Listener) Close() error { return l.ln.Close() }

// Addr is the address this listener is bound to.
func (l *Listener) Addr() string { return l.path }

// Dial connects to a daemon. Its failure is how a client learns that no daemon is
// running and that it must start one.
func Dial(ep Endpoint) (net.Conn, error) {
	if ep.Network != NetworkUnix {
		return nil, fmt.Errorf("transport: %s endpoints are not supported on this platform", ep.Network)
	}
	return dialUnix(ep.Address)
}

func dialUnix(path string) (net.Conn, error) { return net.Dial("unix", path) }
