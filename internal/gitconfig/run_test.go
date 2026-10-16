package gitconfig

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// sessionFor is a session on the machine running the test.
func sessionFor(home string) core.Session {
	return coretest.Session(home, runtime.GOOS)
}

// scratchGit points git at a global configuration inside the test's temporary home, so
// nothing here touches the real one.
func scratchGit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global.gitconfig"))
	return home
}

func TestTheCapabilityNeedsGit(t *testing.T) {
	// The whole util manages global git configuration. Without git there is nothing it
	// can do, and saying so is better than failing on the first command it runs.
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	without := registry.Host{GOOS: "linux", LookPath: func(string) (string, error) {
		return "", os.ErrNotExist
	}}
	got := r.Snapshot(without).Capabilities[0]
	if got.State != registry.Unavailable {
		t.Errorf("without git: state = %q, want unavailable", got.State)
	}
	if got.Reason != registry.MissingDependency {
		t.Errorf("without git: reason = %q, want missing_dependency", got.Reason)
	}

	with := registry.Host{GOOS: "linux", LookPath: func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}}
	if got := r.Snapshot(with).Capabilities[0]; got.State != registry.Available {
		t.Errorf("with git: state = %q (%s), want available", got.State, got.Reason)
	}
}

func TestTheCapabilityCarriesItsCommandSurface(t *testing.T) {
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	c, _ := r.Lookup("git")
	if c.Usage == "" {
		t.Error("no usage line")
	}
	want := map[string]bool{"install": false, "uninstall": false, "edit": false, "migrate": false, "status": false}
	for _, cmd := range c.Commands {
		if _, ok := want[cmd.Name]; ok {
			want[cmd.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("the command surface does not declare %s", name)
		}
	}
}

func TestTheEditFormIsTheSchemaProjection(t *testing.T) {
	// The aliases are a map, so they arrive as a map rather than as a blob of JSON the
	// command has to parse back out of a text box.
	home := scratchGit(t)
	r := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"editor": "code --wait", "autocrlf": "input",
		"diff_tool": "vscode", "merge_tool": "vscode",
		"aliases": map[string]any{"st": "status"},
	}}

	if err := Run(context.Background(), sessionFor(home), []string{"edit"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}

	asked, shown := r.LastAsked()
	if !shown {
		t.Fatal("no form was shown")
	}
	if len(asked.Fields) != 5 {
		t.Fatalf("asked for %d fields, want the five the schema declares", len(asked.Fields))
	}
	aliases, ok := asked.Fields[4].Value.(map[string]any)
	if asked.Fields[4].Type != form.KeyValue || !ok || aliases["st"] != "status" {
		t.Errorf("aliases = %+v, want the stored map as a key-value field", asked.Fields[4])
	}
}

func TestEditingWritesTheAliasMapItWasGiven(t *testing.T) {
	home := scratchGit(t)
	r := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"editor": "code --wait", "autocrlf": "input",
		"diff_tool": "vscode", "merge_tool": "vscode",
		"aliases": map[string]any{"st": "status", "co": "checkout"},
	}}

	if err := Run(context.Background(), sessionFor(home), []string{"edit"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := os.ReadFile(configPath(home))
	if err != nil {
		t.Fatalf("read the generated config: %v", err)
	}
	for _, want := range []string{"st = status", "co = checkout", "editor = code --wait"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the generated config does not contain %q:\n%s", want, body)
		}
	}
}

func TestAnAliasTheSchemaWouldRefuseIsRejectedByCore(t *testing.T) {
	// A client can check that the answer is a JSON object of strings. Whether the keys
	// and values are safe to write into a config file is core's to decide.
	home := scratchGit(t)
	r := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"editor": "code --wait", "autocrlf": "input",
		"diff_tool": "vscode", "merge_tool": "vscode",
		"aliases": map[string]any{"bad\nkey": "status"},
	}}

	if err := Run(context.Background(), sessionFor(home), []string{"edit"}, io.Discard, r); err == nil {
		t.Fatal("an alias with a newline in its key was accepted")
	}
}

func TestAConfigWarningGoesThroughTheResponder(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "soppy-utils", "git")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("99\n"), 0o600); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	body := `{"util":"git","schema_version":99,"data":{}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	r := &coretest.Recorder{}
	if err := Run(context.Background(), sessionFor(home), []string{"status"}, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !r.NoticesContain("git") {
		t.Errorf("notices = %v, want the conflict reported", r.Notices)
	}
}

func TestStatusWritesToTheWriterItWasGiven(t *testing.T) {
	home := scratchGit(t)
	var out strings.Builder
	if err := Run(context.Background(), sessionFor(home), []string{"status"}, &out, &coretest.Recorder{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "config contract:") {
		t.Errorf("output = %q, want the status report", out.String())
	}
}

func TestTheHandlerHandsBackTheConfig(t *testing.T) {
	home := scratchGit(t)
	r := &coretest.Recorder{}
	cmd := core.Command{Util: "git", Args: []string{"status"}, Session: sessionFor(home)}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}
	if got := r.Answer(t); !strings.Contains(got.Output, "config contract:") {
		t.Errorf("output = %q, want the status report", got.Output)
	}
}
