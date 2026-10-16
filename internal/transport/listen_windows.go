//go:build windows

package transport

import (
	"fmt"
	"net"
)

// Windows reaches the daemon through a named pipe rather than a socket, and Go's
// standard library has no named pipe support. The pipe needs an explicit security
// descriptor restricting it to the current user, which is a dependency decision of
// its own, so it is not implemented yet.
//
// These declarations exist so the package still builds for the release target with
// the same API the other platform exposes. Returning errors rather than omitting the
// file keeps `go build ./...` honest on windows/amd64: a package whose every file is
// excluded by build tags fails to build, which would break the release rather than
// the platform.
//
// The signatures must match the Unix build exactly. A platform that changes an
// exported signature forces every caller to grow a build tag, which is the coupling
// this package exists to prevent.
type Listener struct{}

func Listen(ep Endpoint) (*Listener, error) {
	return nil, errNotImplemented()
}

func Dial(ep Endpoint) (net.Conn, error) {
	return nil, errNotImplemented()
}

func (l *Listener) Accept() (net.Conn, error) { return nil, errNotImplemented() }

func (l *Listener) Close() error { return errNotImplemented() }

func (l *Listener) Addr() string { return "" }

func errNotImplemented() error {
	return fmt.Errorf("transport: named pipes are not implemented on this platform yet")
}

// SetUnlinkOnClose decides whether closing the listener takes its endpoint with it.
//
// It exists so that this build exposes the same API as the other one. A platform that
// changes an exported signature forces every caller to grow a build tag, which is the
// coupling this package exists to prevent.
func (l *Listener) SetUnlinkOnClose(bool) {}
