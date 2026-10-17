package welcome

import (
	"context"
	"encoding/json"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/core"
)

func TestStatusWritesToTheWriterItWasGiven(t *testing.T) {
	// Nothing is written anywhere else: core has no stream of its own to write to, and
	// the client that does is the one that decided where it goes.
	var out strings.Builder
	if err := Run(context.Background(), sessionFor(t.TempDir()), []string{"status"}, &out, &coretest.Recorder{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "config contract:") {
		t.Errorf("output = %q, want the status report", out.String())
	}
}

func TestInstallAsksThroughTheResponderWhenAUserIsAvailable(t *testing.T) {
	home := t.TempDir()
	r := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"name": "Alice", "device": "Mac", "message": "Ready", "rc": "bash",
	}}

	if err := Run(context.Background(), sessionFor(home), []string{"install"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	asked, shown := r.LastAsked()
	if !shown {
		t.Fatal("a client that could ask was never asked")
	}
	// The form is the one the schema describes, so a field added to the schema appears
	// in it without this code being touched.
	if len(asked.Fields) != 5 {
		t.Errorf("asked for %d fields, want the five the schema declares", len(asked.Fields))
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
	r := &coretest.Recorder{Askable: false}

	if err := Run(context.Background(), sessionFor(home), []string{"install"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, shown := r.LastAsked(); shown {
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

	r := &coretest.Recorder{}
	if err := Run(context.Background(), sessionFor(home), []string{"status"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// A warning is a diagnostic, so it goes through the responder rather than into the
	// command's output, which is about to become a pipe.
	if len(r.Warnings) == 0 {
		t.Errorf("notices = %v, want the conflict reported as a warning", r.Notices)
	}
}

func TestTheHandlerHandsBackOutputRatherThanWritingIt(t *testing.T) {
	r := &coretest.Recorder{}
	cmd := core.Command{Util: "welcome", Args: []string{"status"},
		Session: core.Session{Home: t.TempDir()}}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}
	if r.Finished == nil {
		t.Fatal("the command never finished")
	}

	var got core.Result
	if err := json.Unmarshal(r.Finished, &got); err != nil {
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

	r := &coretest.Recorder{}
	cmd := core.Command{Util: "welcome", Args: []string{"status"},
		Session: core.Session{Home: home}}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}

	var got core.Result
	if err := json.Unmarshal(r.Finished, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	// The config in the session's home is the one that was read; this process's own
	// home has none at all, so reading the wrong one shows up as contract 0.
	if !strings.Contains(got.Output, "config contract: 1") {
		t.Errorf("output = %q, want the config from the session's home", got.Output)
	}
}

// sessionFor is a session on the machine running the test. A test that cares which
// platform a command is judged as builds its own.
func sessionFor(home string) core.Session {
	return core.Session{Home: home, GOOS: runtime.GOOS}
}
