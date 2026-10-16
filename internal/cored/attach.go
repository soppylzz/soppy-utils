package cored

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/soppylzz/soppy-utils/internal/transport"
)

// pollInterval is how often Attach retries while a spawned daemon starts.
const pollInterval = 20 * time.Millisecond

// AttachOptions configures how a client reaches core.
type AttachOptions struct {
	Endpoint transport.Endpoint

	// Spawn is the argv of a daemon to start when nothing is listening. Empty means
	// never start one, which is how a caller that must not create processes says so.
	Spawn    []string
	SpawnEnv []string

	// LogPath receives the spawned daemon's standard output and error.
	LogPath string

	// Budget bounds how long Attach waits for a spawned daemon to answer.
	Budget time.Duration

	// Env is what this client reports about the machine it is on. Capabilities are
	// judged against it, so a client that leaves it zeroed gets a core that knows
	// nothing about the environment it is being asked about.
	Env Env

	// Kind is what sort of client this is: "tui", and later "gui".
	Kind string

	// Capabilities is what this client says it can do. Core offers work only to a
	// client that said it can perform it.
	Capabilities Capabilities
}

// Attach connects to a daemon, starting one if none is listening.
//
// The endpoint is derived rather than searched for, so a client can never reach
// somebody else's core. The daemon is per-user and shared: a client that finds one
// running uses it, and only a client that finds nothing starts one. Two clients
// racing to start a daemon is expected and harmless — the loser exits, and this
// returns the connection to the winner because it only ever retries the dial.
func Attach(ctx context.Context, opts AttachOptions) (net.Conn, error) {
	if conn, err := transport.Dial(opts.Endpoint); err == nil {
		return conn, nil
	}

	if len(opts.Spawn) == 0 {
		return nil, fmt.Errorf("no core is listening on %s and none was offered to start",
			opts.Endpoint.Address)
	}
	if opts.Budget <= 0 {
		opts.Budget = 2 * time.Second
	}

	if _, err := startDetached(opts.Spawn, opts.SpawnEnv, opts.LogPath); err != nil {
		return nil, fmt.Errorf("start core: %w", err)
	}

	deadline := time.Now().Add(opts.Budget)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
		if conn, err := transport.Dial(opts.Endpoint); err == nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("core did not answer on %s within %s", opts.Endpoint.Address, opts.Budget)
}
