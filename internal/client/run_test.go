package client

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/form"
)

// newTestClient runs a real daemon in-process and connects to it, so the loop is
// exercised over the real protocol rather than against a stub of it.
func newTestClient(t *testing.T, opts cored.Options, canAct bool) (*Client, *strings.Builder, *strings.Builder) {
	t.Helper()

	daemon := newDaemon(t, opts)
	conn, err := cored.Connect(context.Background(), cored.AttachOptions{
		Endpoint: daemon,
		Budget:   2 * time.Second,
		Kind:     "tui",
		Capabilities: cored.Capabilities{
			ActionExecution: canAct, Confirm: canAct, Interactive: canAct,
		},
		Env: cored.Env{GOOS: runtime.GOOS, Path: os.Getenv("PATH"), Home: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	out, errOut := &strings.Builder{}, &strings.Builder{}
	c := New(conn, strings.NewReader(""), out, errOut)
	c.Interactive = canAct
	return c, out, errOut
}

func run(t *testing.T, c *Client) (int, error) {
	t.Helper()
	return c.Run(context.Background(), "welcome", []string{"status"})
}

func TestANotificationGoesToTheErrorStream(t *testing.T) {
	// Diagnostics belong on stderr, which is the rule the project already follows: a
	// client that wrote progress to stdout would corrupt anything being piped.
	c, out, errOut := newTestClient(t, cored.Options{Handler: notifying("checking configuration")}, true)

	code, err := run(t, c)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(errOut.String(), "checking configuration") {
		t.Errorf("stderr = %q, want the notification", errOut.String())
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want nothing on it", out.String())
	}
}

func TestAFormIsAskedAndTheAnswersAreSent(t *testing.T) {
	answers := make(chan map[string]any, 1)
	c, _, _ := newTestClient(t, cored.Options{Handler: formAsker(answers)}, true)

	var asked form.Spec
	c.Ask = func(spec form.Spec) (map[string]any, error) {
		asked = spec
		return map[string]any{"name": "Alice"}, nil
	}

	if code, err := run(t, c); err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}

	// The client draws what core described, field for field.
	if len(asked.Fields) != 1 || asked.Fields[0].Key != "name" {
		t.Errorf("the client was asked to draw %+v, want the field core sent", asked)
	}
	select {
	case got := <-answers:
		if got["name"] != "Alice" {
			t.Errorf("the command saw %v, want the answers the client collected", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the answers never reached the command")
	}
}

func TestAClientThatCannotAskEndsTheSession(t *testing.T) {
	// A question sent to a client that cannot answer must not hang it. Core should not
	// ask, but a client that assumed so would wait forever if it ever did.
	c, _, errOut := newTestClient(t, cored.Options{Handler: formAsker(nil)}, false)
	c.Ask = nil

	code, err := run(t, c)
	if err == nil {
		t.Fatal("Run reported success although it could not answer a question")
	}
	if code == 0 {
		t.Errorf("exit = %d, want non-zero", code)
	}
	if !strings.Contains(errOut.String()+err.Error(), "terminal") {
		t.Errorf("the failure does not say what was missing: %v", err)
	}
}

func TestAnActionIsConfirmedAndPerformed(t *testing.T) {
	outcomes := make(chan action.Outcome, 1)
	c, _, errOut := newTestClient(t, cored.Options{Handler: actionAsker(outcomes)}, true)

	c.Confirm = func(string) (bool, error) { return true, nil }

	if code, err := run(t, c); err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}

	// What will run is shown before it runs. A yes-or-no question about something
	// invisible is not a question, and this is the only chance the user gets to see it.
	if !strings.Contains(errOut.String(), "welcome needs figlet") {
		t.Errorf("stderr = %q, want the reason shown before the question", errOut.String())
	}
	if !strings.Contains(errOut.String(), "true") {
		t.Errorf("stderr = %q, want the command that will run", errOut.String())
	}
	select {
	case got := <-outcomes:
		if !got.Complete() {
			t.Errorf("outcome = %+v, want the client to have performed the action", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the outcome never reached the command")
	}
}

func TestAnActionIsDeclinedWhenTheUserSaysNo(t *testing.T) {
	outcomes := make(chan action.Outcome, 1)
	c, _, _ := newTestClient(t, cored.Options{Handler: actionAsker(outcomes)}, true)

	c.Confirm = func(string) (bool, error) { return false, nil }

	if code, err := run(t, c); err != nil || code != 0 {
		t.Fatalf("Run = %d, %v: declining is not a failure", code, err)
	}
	select {
	case got := <-outcomes:
		if !got.Declined {
			t.Errorf("outcome = %+v, want it declined", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the outcome never reached the command")
	}
}

func TestAFailureToFinishIsANonZeroExitThatSaysWhy(t *testing.T) {
	c, _, errOut := newTestClient(t, cored.Options{Handler: failing("figlet is not installed")}, true)

	code, err := run(t, c)
	if code == 0 {
		t.Error("exit = 0 although the command failed")
	}
	if err == nil {
		t.Fatal("Run reported no error although the command failed")
	}
	if !strings.Contains(errOut.String()+err.Error(), "figlet is not installed") {
		t.Errorf("the reason did not reach the user: %v", err)
	}
}

func TestOutputIsWrittenToTheStreamCoreChose(t *testing.T) {
	// license --mit is the case that matters: the bytes it emits are the file the user
	// saves, so they go to stdout and they go exactly as core wrote them.
	c, out, errOut := newTestClient(t, cored.Options{Handler: emitting("MIT License\n\nCopyright\n")}, true)

	if code, err := run(t, c); err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}
	if out.String() != "MIT License\n\nCopyright\n" {
		t.Errorf("stdout = %q, want the bytes core produced", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want nothing", errOut.String())
	}
}

func TestOutputCanBeSentToTheErrorStream(t *testing.T) {
	c, out, errOut := newTestClient(t, cored.Options{
		Handler: emittingTo("usage: spu welcome\n", "stderr"),
	}, true)

	if code, err := run(t, c); err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want nothing", out.String())
	}
	if !strings.Contains(errOut.String(), "usage: spu welcome") {
		t.Errorf("stderr = %q, want the text core chose to put there", errOut.String())
	}
}
