package client

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// notifying finishes successfully after one progress line.
func notifying(message string) cored.Handler {
	return handlerFunc(func(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
		if err := reply.Notify(message); err != nil {
			return err
		}
		return reply.Finish(nil)
	})
}

// formAsker asks one question and reports what came back.
func formAsker(answers chan map[string]any) cored.Handler {
	return handlerFunc(func(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
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

// actionAsker offers one harmless command and reports the outcome.
func actionAsker(outcomes chan action.Outcome) cored.Handler {
	return handlerFunc(func(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
		outcome, err := reply.AskActions(ctx, action.Request{
			Reason:  "welcome needs figlet",
			Actions: []action.Action{{Kind: action.Run, Argv: []string{"true"}}},
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

func failing(message string) cored.Handler {
	return handlerFunc(func(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
		return errorString(message)
	})
}

func emitting(text string) cored.Handler { return emittingTo(text, "stdout") }

func emittingTo(text, stream string) cored.Handler {
	return handlerFunc(func(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
		body, err := json.Marshal(cored.Result{Output: text, OutputTo: stream})
		if err != nil {
			return err
		}
		return reply.Finish(body)
	})
}

type handlerFunc func(context.Context, cored.Command, cored.Responder) error

func (f handlerFunc) Run(ctx context.Context, cmd cored.Command, reply cored.Responder) error {
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
