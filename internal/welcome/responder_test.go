package welcome

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
)

// recorder stands in for a client: it remembers what a command reported and answers
// whatever the test told it to. The tests below are the proof that the command no
// longer reaches for a terminal of its own.
type recorder struct {
	canAsk  bool
	answers map[string]any

	asked   *form.Spec
	notices []string
	actions *action.Request
	finish  json.RawMessage
}

func (r *recorder) CanAsk() bool { return r.canAsk }

func (r *recorder) Notify(message string) error {
	r.notices = append(r.notices, message)
	return nil
}

func (r *recorder) Ask(_ context.Context, spec form.Spec) (map[string]any, error) {
	r.asked = &spec
	if r.answers == nil {
		return nil, errors.New("this test's client has nothing to answer with")
	}
	return r.answers, nil
}

func (r *recorder) AskActions(_ context.Context, req action.Request) (action.Outcome, error) {
	r.actions = &req
	return action.Outcome{Declined: true}, nil
}

func (r *recorder) Finish(result json.RawMessage) error {
	r.finish = result
	return nil
}

func noticesContain(r *recorder, want string) bool {
	for _, n := range r.notices {
		if strings.Contains(n, want) {
			return true
		}
	}
	return false
}

func TestStatusWritesToTheWriterItWasGiven(t *testing.T) {
	// Nothing is written anywhere else: core has no stream of its own to write to, and
	// the client that does is the one that decided where it goes.
	var out strings.Builder
	if err := Run(context.Background(), t.TempDir(), []string{"status"}, &out, &recorder{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "config contract:") {
		t.Errorf("output = %q, want the status report", out.String())
	}
}

func TestInstallAsksThroughTheResponderWhenAUserIsAvailable(t *testing.T) {
	home := t.TempDir()
	r := &recorder{canAsk: true, answers: map[string]any{
		"name": "Alice", "device": "Mac", "message": "Ready", "rc": "bash",
	}}

	if err := Run(context.Background(), home, []string{"install"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.asked == nil {
		t.Fatal("a client that could ask was never asked")
	}
	// The form is the one the schema describes, so a field added to the schema appears
	// in it without this code being touched.
	if len(r.asked.Fields) != 4 {
		t.Errorf("asked for %d fields, want the four the schema declares", len(r.asked.Fields))
	}

	script, err := os.ReadFile(scriptPath(Store(home)))
	if err != nil {
		t.Fatalf("read the generated script: %v", err)
	}
	if !strings.Contains(string(script), "Alice") {
		t.Error("the script was generated without the answers")
	}
}

func TestANonInteractiveClientIsNotAsked(t *testing.T) {
	// A scripted install has no terminal to answer with. Asking anyway would hang it,
	// and today's behaviour is to fall back to documented defaults.
	home := t.TempDir()
	r := &recorder{canAsk: false}

	if err := Run(context.Background(), home, []string{"install"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.asked != nil {
		t.Error("a client that cannot ask was asked anyway")
	}

	script, err := os.ReadFile(scriptPath(Store(home)))
	if err != nil {
		t.Fatalf("read the generated script: %v", err)
	}
	if !strings.Contains(string(script), "Ready to hack the universe.") {
		t.Error("the documented default message was not used")
	}
}

func TestAFutureContractIsReportedNotRepaired(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "soppy-utils", "welcome")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("99\n"), 0o600); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	body := `{"util":"welcome","schema_version":99,"data":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	r := &recorder{}
	if err := Run(context.Background(), home, []string{"status"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// A warning is a diagnostic, so it goes through the responder rather than into the
	// command's output, which is about to become a pipe.
	if !noticesContain(r, "welcome") {
		t.Errorf("notices = %v, want the conflict reported", r.notices)
	}
}

func TestTheHandlerHandsBackOutputRatherThanWritingIt(t *testing.T) {
	r := &recorder{}
	cmd := core.Command{Util: "welcome", Args: []string{"status"}, Home: t.TempDir()}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}
	if r.finish == nil {
		t.Fatal("the command never finished")
	}

	var got core.Result
	if err := json.Unmarshal(r.finish, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !strings.Contains(got.Output, "config contract:") {
		t.Errorf("output = %q, want the status report", got.Output)
	}
}

func TestTheHandlerRunsAgainstTheSessionsHome(t *testing.T) {
	// Not this process's home. The daemon outlives the client that started it, and a
	// later client may be a different user session entirely.
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "soppy-utils", "welcome")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	body := `{"util":"welcome","schema_version":1,"data":{"name":"Session","device":"Host","message":"Hi","rc":"none"}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	r := &recorder{}
	cmd := core.Command{Util: "welcome", Args: []string{"status"}, Home: home}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}

	var got core.Result
	if err := json.Unmarshal(r.finish, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	// The config in the session's home is the one that was read; this process's own
	// home has none at all, so reading the wrong one shows up as contract 0.
	if !strings.Contains(got.Output, "config contract: 1") {
		t.Errorf("output = %q, want the config from the session's home", got.Output)
	}
}
