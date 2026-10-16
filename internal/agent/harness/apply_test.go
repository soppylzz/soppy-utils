package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func base() fstest.MapFS {
	return fstest.MapFS{
		"claude/settings.json": {Data: []byte(`{"permissions":{"deny":["Read(~/code/reference/**)"]}}`)},
	}
}

func TestApplyLinksSkillsForClaudeCodeOnly(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	done, err := Apply(project, []string{"claude", "pi", "codex"}, base())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	link := filepath.Join(project, ".claude", "skills")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("no link was made: %v", err)
	}
	if target != filepath.Join("..", ".agents", "skills") {
		t.Errorf("target = %q, want it relative", target)
	}
	// Pi and Codex read .agents/skills themselves, so nothing is created for them.
	if _, err := os.Stat(filepath.Join(project, ".pi")); err == nil {
		t.Error(".pi was created although Pi needs nothing there")
	}
	if !contains(done, "linked .claude/skills") {
		t.Errorf("the link was not reported: %v", done)
	}
}

func TestApplyWritesABaseConfigurationOnlyWhenAbsent(t *testing.T) {
	// A project with its own settings must never have them replaced. The whole file, and
	// only when there is nothing there.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"),
		[]byte(`{"mine":true}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Apply(project, []string{"claude"}, base()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != `{"mine":true}` {
		t.Errorf("a project's own settings were replaced: %s", body)
	}
}

func TestApplyWritesTheBaseConfigurationWhenThereIsNone(t *testing.T) {
	project := t.TempDir()
	done, err := Apply(project, []string{"claude"}, base())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("the base configuration was not written: %v", err)
	}
	if !strings.Contains(string(body), "reference") {
		t.Errorf("the deny rule is missing: %s", body)
	}
	if !contains(done, "wrote .claude/settings.json") {
		t.Errorf("the write was not reported: %v", done)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := Apply(project, []string{"claude"}, base()); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	second, err := Apply(project, []string{"claude"}, base())
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("a second run did work again: %v", second)
	}
}

func TestApplyRefusesToReplaceALinkPointingElsewhere(t *testing.T) {
	// Something is already at that path and it is not ours. Replacing it would discard a
	// decision somebody made.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, "elsewhere"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink("../elsewhere", filepath.Join(project, ".claude", "skills")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Apply(project, []string{"claude"}, base())
	if err == nil {
		t.Fatal("a link pointing somewhere else was silently replaced")
	}
	if !strings.Contains(err.Error(), ".claude/skills") {
		t.Errorf("error %q does not name the path", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
