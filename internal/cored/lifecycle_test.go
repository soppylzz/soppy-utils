package cored

import (
	"context"
	"github.com/soppylzz/soppy-utils/internal/core"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/transport"
)

func TestShutdownStopsTheDaemon(t *testing.T) {
	h := newHarness(t, Options{})
	c := h.dial()

	if reply := c.call("server/shutdown", `{}`); reply.Error != nil {
		t.Fatalf("server/shutdown failed: %+v", reply.Error)
	}

	// A shutdown that leaves the socket behind would make the next client wait on an
	// endpoint that is not going to answer.
	h.waitServed(5 * time.Second)
	if _, err := transport.Dial(h.ep); err == nil {
		t.Error("the endpoint still accepts connections after shutdown")
	}
}

func TestShutdownWaitsForARunningSession(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})

	h := newHarness(t, Options{
		DrainGrace: 2 * time.Second,
		Handler: scriptedHandler{run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			<-release
			err := reply.Finish(nil)
			close(finished)
			return err
		}},
	})

	c := h.dial()
	c.startSession("welcome")

	// A second client asks core to stop, which is what spu update does before it
	// replaces the binary.
	admin := h.dial()
	if reply := admin.call("server/shutdown", `{}`); reply.Error != nil {
		t.Fatalf("server/shutdown failed: %+v", reply.Error)
	}

	// The session may be halfway through writing configuration. Stopping now would
	// leave the user with a half-applied change and no one to tell them.
	select {
	case <-h.done:
		t.Fatal("the daemon stopped while a session was still running")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	h.waitServed(5 * time.Second)

	select {
	case <-finished:
	default:
		t.Error("the daemon returned before the session reported its result")
	}
}

func TestShutdownDoesNotWaitForever(t *testing.T) {
	// A session that refuses to end must not keep the daemon resident: spu update is
	// waiting on this, and so is the user.
	h := newHarness(t, Options{
		DrainGrace: 100 * time.Millisecond,
		Handler: scriptedHandler{run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	})

	c := h.dial()
	c.startSession("welcome")

	admin := h.dial()
	admin.call("server/shutdown", `{}`)

	h.waitServed(5 * time.Second)
}

func TestAnIdleDaemonExits(t *testing.T) {
	// Nobody ever connects. A daemon that stayed resident anyway would accumulate on
	// a machine that spawns one per install directory.
	h := newHarness(t, Options{IdleTimeout: 60 * time.Millisecond})
	h.waitServed(5 * time.Second)
}

func TestAnActiveDaemonDoesNotExitWhileAClientIsConnected(t *testing.T) {
	h := newHarness(t, Options{IdleTimeout: 60 * time.Millisecond})
	c := h.dial()

	// Hold the connection open for several idle periods and confirm the daemon is
	// still there. Idleness is about connections, not about silence on them: a client
	// waiting for the user to fill in a form looks exactly like this.
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case <-h.done:
			t.Fatal("the daemon exited while a client was still connected")
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}

	if reply := c.call("capabilities/list", `{}`); reply.Error == nil {
		// The method is not implemented yet; reaching the daemon at all is the point.
		t.Log("call succeeded")
	}
}
