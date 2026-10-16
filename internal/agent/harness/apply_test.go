package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyLinksSkillsForClaudeCodeOnly(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	done, err := Apply(project, []string{"claude", "pi", "codex"})
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

func TestApplyIsIdempotent(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := Apply(project, []string{"claude"}); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	second, err := Apply(project, []string{"claude"})
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
	// Skills have to exist, or there is nothing to link and the path is never looked at.
	if err := os.MkdirAll(filepath.Join(project, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(project, "elsewhere"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink("../elsewhere", filepath.Join(project, ".claude", "skills")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Apply(project, []string{"claude"})
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

func TestNoLinkIsMadeWhenThereAreNoSkills(t *testing.T) {
	// A link into a directory that does not exist is a dead link, and it is worse than
	// nothing: it looks like spu arranged something, and a harness reading it finds
	// nothing there. Only a project that actually has skills needs the link.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".agents", "prompts"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	done, err := Apply(project, []string{"claude"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".claude", "skills")); err == nil {
		t.Error("a link was made although there are no skills to link to")
	}
	if contains(done, "linked .claude/skills") {
		t.Errorf("a link that was not made was reported as made: %v", done)
	}
}
