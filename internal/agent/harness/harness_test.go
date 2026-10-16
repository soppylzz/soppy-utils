package harness

import (
	"path/filepath"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

func TestDirPerRoot(t *testing.T) {
	// Verified against the installed harnesses rather than assumed. Pi reads
	// .agents/skills natively and Codex lists it in its own configuration table, so the
	// canonical directory already reaches two of the three.
	project := filepath.Join("/home", "someone", "project")
	cases := map[item.Root]string{
		item.RootAgents: filepath.Join(project, ".agents"),
		item.RootClaude: filepath.Join(project, ".claude"),
		item.RootCodex:  filepath.Join(project, ".codex"),
		item.RootPi:     filepath.Join(project, ".pi"),
	}
	for root, want := range cases {
		if got := Dir(project, root); got != want {
			t.Errorf("Dir(%q) = %q, want %q", root, got, want)
		}
	}
}

func TestOnlyClaudeCodeNeedsALink(t *testing.T) {
	// One link, not one per harness. Claude Code is the only one of the three that does
	// not read .agents/skills; it offers to import from it, which is a copy, not a read.
	project := "/p"

	if got := Links(project, []string{"codex", "pi"}); len(got) != 0 {
		t.Errorf("Links for codex and pi = %v, want none: both read the canonical directory", got)
	}

	got := Links(project, []string{"claude"})
	if len(got) != 1 {
		t.Fatalf("Links for claude = %v, want exactly one", got)
	}
	if got[0].Path != filepath.Join(project, ".claude", "skills") {
		t.Errorf("link path = %q, want it under .claude", got[0].Path)
	}
	if got[0].Target != filepath.Join("..", ".agents", "skills") {
		t.Errorf("link target = %q, want it relative, so a moved project keeps working", got[0].Target)
	}
}

func TestLinksAreInAStableOrder(t *testing.T) {
	// The order the harnesses were chosen in is the order they were chosen in, and a
	// command that reports what it did should not have its output shuffled underneath it.
	got := Links("/p", []string{"claude", "pi", "claude"})
	if len(got) != 1 {
		t.Errorf("got %d links for a repeated harness, want one", len(got))
	}
}

func TestAProjectDirectoryIsFoundByWalkingUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := mkdirAll(deep); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := mkdirAll(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := Project(deep)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	// The temp directory may be reached through a symlink on macOS, so compare the
	// resolved forms rather than the strings.
	if !sameDir(t, got, root) {
		t.Errorf("Project = %q, want the directory holding .git (%q)", got, root)
	}
}

func TestAProjectWithNothingToFindIsTheDirectoryItself(t *testing.T) {
	// Running somewhere that is not a repository is not an error: the answer is the
	// directory, and whatever gets generated lands there.
	dir := t.TempDir()
	got, err := Project(dir)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !sameDir(t, got, dir) {
		t.Errorf("Project = %q, want %q", got, dir)
	}
}
