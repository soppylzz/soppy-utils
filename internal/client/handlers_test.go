package client

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// notifying finishes successfully after one progress line.
func notifying(message string) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		if err := reply.Notify(message); err != nil {
			return err
		}
		return reply.Finish(nil)
	})
}

// formAsker asks one question and reports what came back.
func formAsker(answers chan map[string]any) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		values, err := reply.Ask(ctx, form.Spec{
			Title:  "Welcome banner",
			Fields: []form.Field{{Key: "name", Type: form.String, Title: "Name"}},
		})
		if err != nil {
			return err
		}
		if answers != nil {
			answers <- values
		}
		return reply.Finish(nil)
	})
}

// actionAsker offers one harmless command that changes the machine, so that the client
// asks about it.
func actionAsker(outcomes chan action.Outcome) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		outcome, err := reply.AskActions(ctx, action.Request{
			Reason:  "welcome needs figlet",
			Actions: []action.Action{{Kind: action.Run, Argv: []string{"true"}, ChangesMachine: true}},
		})
		if err != nil {
			return err
		}
		if outcomes != nil {
			outcomes <- outcome
		}
		return reply.Finish(nil)
	})
}

func failing(message string) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		return errorString(message)
	})
}

func emitting(text string) core.Handler { return emittingTo(text, "stdout") }

func emittingTo(text, stream string) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		body, err := json.Marshal(core.Result{Output: text, OutputTo: stream})
		if err != nil {
			return err
		}
		return reply.Finish(body)
	})
}

type handlerFunc func(context.Context, core.Command, core.Responder) error

func (f handlerFunc) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	return f(ctx, cmd, reply)
}

type errorString string

func (e errorString) Error() string { return string(e) }

// newDaemon starts a core in-process on a private endpoint.
//
// It duplicates the harness in the cored package's own tests, which cannot be shared
// across packages without exporting test-only helpers from production code.
func newDaemon(t *testing.T, opts cored.Options) transport.Endpoint {
	t.Helper()

	root := tempDir(t)

	ep, err := transport.Resolve(runtime.GOOS, root, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	opts.Endpoint = ep
	if opts.Version == "" {
		opts.Version = "test"
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = time.Minute
	}
	if opts.DrainGrace == 0 {
		opts.DrainGrace = time.Second
	}

	d, err := cored.New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := d.Serve(ctx); err != nil {
			t.Errorf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Serve did not return")
		}
	})

	return ep
}

// confirmAsker asks one yes-or-no question and reports the answer.
func confirmAsker(question string, answers chan bool) core.Handler {
	return handlerFunc(func(ctx context.Context, cmd core.Command, reply core.Responder) error {
		ok, err := reply.Confirm(ctx, question)
		if err != nil {
			return err
		}
		if answers != nil {
			answers <- ok
		}
		return reply.Finish(nil)
	})
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
