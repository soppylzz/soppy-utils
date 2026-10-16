// Package transport carries protocol messages between a client and the core daemon.
//
// It knows about sockets and pipes; the protocol package knows about bytes and
// messages. Neither learns the other's concern, and no util package imports either,
// so domain code never hears about a file descriptor.
package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// maxUnixPath bounds a Unix socket path.
//
// The kernel's sun_path is 104 bytes on macOS and 108 on Linux including the
// terminator, so the path itself is capped at 103 characters there. The smaller bound
// is used everywhere so that a path is never accepted on one platform and rejected on
// another, and so that the limit is the same in a test as in production.
const maxUnixPath = 103

// Network names the kind of endpoint an address refers to.
const (
	NetworkUnix = "unix"
	NetworkPipe = "pipe"
)

// Endpoint is where a client connects and where core listens.
type Endpoint struct {
	Network string
	Address string
}

// Resolve derives the endpoint for one session's environment.
//
// goos and home are inputs rather than facts about this process, because the daemon
// is long-lived and was started by whichever client happened to be first. Its own
// environment is the wrong answer for everyone who arrives later, so the caller
// always supplies the session's.
//
// override is SPU_CORE_ENDPOINT. A development checkout and the test suite set it so
// they reach their own daemon rather than whatever is installed on the machine.
func Resolve(goos, home, override string) (Endpoint, error) {
	if home == "" {
		return Endpoint{}, fmt.Errorf("home is empty")
	}
	if !isAbsolute(goos, home) {
		return Endpoint{}, fmt.Errorf("home %q is not absolute", home)
	}

	switch goos {
	case "darwin", "linux":
		address := override
		if address == "" {
			address = filepath.Join(home, ".config", "soppy-utils", "core.sock")
		}
		// Checked on the chosen address rather than on the home, so that a home too
		// deep to hold a socket stays usable when the endpoint is pointed elsewhere.
		// Without this the caller gets a bind failure that says nothing about which of
		// its choices was too long.
		if len(address) > maxUnixPath {
			return Endpoint{}, fmt.Errorf(
				"endpoint %s is %d bytes, over the %d byte limit for a Unix socket; "+
					"point SPU_CORE_ENDPOINT at a shorter path",
				address, len(address), maxUnixPath)
		}
		return Endpoint{Network: NetworkUnix, Address: address}, nil

	case "windows":
		address := override
		if address == "" {
			address = pipeName(home)
		}
		return Endpoint{Network: NetworkPipe, Address: address}, nil

	default:
		return Endpoint{}, fmt.Errorf("unsupported platform %q", goos)
	}
}

// pipeName derives a per-user pipe name from the home directory.
//
// The name has to differ between users: a shared name would make the second user's
// daemon fail to start, or worse, attach to the first user's. The access control is
// the pipe's security descriptor, not the name, so the name only needs to be unique
// and agreed on by both ends — which a hash of the home is.
//
// The home is hashed rather than embedded because it can contain characters a pipe
// name cannot, and can be longer than one allows.
func pipeName(home string) string {
	sum := sha256.Sum256([]byte(home))
	return `\\.\pipe\soppy-utils-` + hex.EncodeToString(sum[:8])
}

// isAbsolute reports whether p is absolute for goos.
//
// filepath.IsAbs answers for the host this binary was built for, which is the wrong
// question: the Windows endpoint is derived on every platform by the tests, and by a
// client running under Git Bash.
func isAbsolute(goos, p string) bool {
	if goos != "windows" {
		return strings.HasPrefix(p, "/")
	}
	// `C:\...` or `C:/...`
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	// A UNC path such as `\\host\share`.
	return strings.HasPrefix(p, `\\`)
}
