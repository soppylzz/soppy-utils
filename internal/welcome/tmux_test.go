package welcome

import (
	"context"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bannerData(inTmux bool) map[string]any {
	return map[string]any{
		"name": "Alice", "device": "Mac", "message": "Ready",
		"rc": "none", "banner_in_tmux": inTmux,
	}
}

func TestRenderCarriesTheTmuxDecision(t *testing.T) {
	off, err := Render(bannerData(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	on, err := Render(bannerData(true))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Baked into the script rather than looked up at startup: the generated banner runs
	// in the user's shell, and reading configuration there would mean the banner depends
	// on spu being installed and working.
	if !strings.Contains(string(off), "${TMUX") {
		t.Error("the script does not consult TMUX, so a new pane would print a banner")
	}
	if strings.Contains(string(on), "${TMUX") {
		t.Error("the script consults TMUX although the banner was asked for there")
	}
}

func TestTheBannerStaysQuietInsideTmuxUnlessItWasAskedFor(t *testing.T) {
	off, err := Render(bannerData(false))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	on, err := Render(bannerData(true))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	inTmux := "TMUX=/tmp/fake-tmux,1,0"

	if out := runWithATty(t, off, inTmux); strings.Contains(out, "Alice") {
		t.Errorf("a banner that was told to stay out of tmux printed anyway: %.160q", out)
	}
	// Outside tmux it still prints, so the gate is a gate rather than a mute.
	if out := runWithATty(t, off); !strings.Contains(out, "Alice") {
		t.Errorf("the banner was hidden outside tmux as well: %.160q", out)
	}
	if out := runWithATty(t, on, inTmux); !strings.Contains(out, "Alice") {
		t.Errorf("a banner that was asked for inside tmux did not print: %.160q", out)
	}
}

func TestAnOlderContractIsStillReadable(t *testing.T) {
	// A v1 configuration has no banner_in_tmux. It has to keep working, warned about
	// once, and take the default rather than being treated as broken.
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "soppy-utils", "welcome")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	body := `{"util":"welcome","schema_version":1,"data":{"name":"Alice","device":"Mac","message":"Ready","rc":"none"}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	v := Store(home).Read()
	if !v.Writable {
		t.Fatalf("an older configuration was not writable: %s", v.Warning)
	}
	if !strings.Contains(v.Warning, "2") {
		t.Errorf("warning = %q, want it to name the contract now required", v.Warning)
	}
	if got, ok := v.Data["banner_in_tmux"]; !ok || got != false {
		t.Errorf("banner_in_tmux = %v, want the default", got)
	}
	if v.Data["name"] != "Alice" {
		t.Errorf("name = %v, want the stored value kept", v.Data["name"])
	}
}

func TestTheFlagSetsItForScriptedInstalls(t *testing.T) {
	// A scripted install cannot answer a form, so anything the form can set has to be
	// settable from the command line.
	home := t.TempDir()
	r := &coretest.Recorder{}
	args := []string{"install", "--name", "Alice", "--device", "Mac",
		"--message", "Ready", "--rc", "none", "--banner-in-tmux"}

	if err := Run(context.Background(), sessionFor(home), args, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}

	script, err := os.ReadFile(scriptPath(Store(home)))
	if err != nil {
		t.Fatalf("read the generated script: %v", err)
	}
	if strings.Contains(string(script), "${TMUX") {
		t.Error("the flag was ignored and the banner still hides inside tmux")
	}
}

func TestTheCapabilityOffersTheFlag(t *testing.T) {
	// Help, usage errors, and the form all render from the registry, so a flag that is
	// only in the parser is a flag nobody can discover.
	for _, c := range commands {
		if c.Name != "install" {
			continue
		}
		for _, f := range c.Flags {
			if f.Name == "--banner-in-tmux" {
				return
			}
		}
	}
	t.Error("install does not declare --banner-in-tmux")
}

func TestTheGeneratedScriptRecordsANewerRevision(t *testing.T) {
	// The script's behaviour changed, so an installed one is out of date and has to be
	// refreshable. That is what the revision is for.
	if templateRevision != 6 {
		t.Errorf("templateRevision = %d, want 6", templateRevision)
	}
}
