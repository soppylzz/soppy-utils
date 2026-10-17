package cored

import (
	"context"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/transport"
	"runtime"
)

func TestShutdownStopsARunningCore(t *testing.T) {
	h := newHarness(t, Options{})

	if err := Shutdown(context.Background(), h.ep, 5*time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// Serve returning is the whole point: spu update replaces the binary afterwards, and
	// on Windows a running executable cannot be replaced.
	h.waitServed(5 * time.Second)

	if _, err := transport.Dial(h.ep); err == nil {
		t.Error("the endpoint still accepts connections after a shutdown")
	}
}

func TestShutdownIsNotAnErrorWhenNothingIsRunning(t *testing.T) {
	// The caller wants a core that is not running, and that is the state it is already
	// in. Reporting failure would make every uninstall on a quiet machine look broken.
	root := shortDir(t)
	ep, err := transport.Resolve(runtime.GOOS, root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := Shutdown(context.Background(), ep, time.Second); err != nil {
		t.Errorf("Shutdown with no core running: %v", err)
	}
}

func TestTheShutdownCallWaitsForARunningSession(t *testing.T) {
	// The protocol already waits for a running session, and this is the call the scripts
	// make, so it has to have the same care: a session may be halfway through writing
	// configuration, and uninstall is about to delete the installation underneath it.
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

	done := make(chan error, 1)
	go func() { done <- Shutdown(context.Background(), h.ep, 5*time.Second) }()

	select {
	case <-done:
		t.Fatal("Shutdown returned while a session was still running")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown never returned")
	}

	select {
	case <-finished:
	default:
		t.Error("Shutdown returned before the session reported its result")
	}
}

func TestShutdownGivesUpOnACoreThatWillNotStop(t *testing.T) {
	// The scripts cannot wait forever, and a core that refuses to stop has to be
	// reported rather than hung on.
	h := newHarness(t, Options{
		DrainGrace: 10 * time.Second,
		Handler: scriptedHandler{run: func(ctx context.Context, cmd core.Command, reply core.Responder) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	})
	c := h.dial()
	c.startSession("welcome")

	if err := Shutdown(context.Background(), h.ep, 300*time.Millisecond); err == nil {
		t.Error("Shutdown reported success although the core was still running")
	}
}
