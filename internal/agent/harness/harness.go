// Package harness knows where each coding agent keeps the text it reads.
//
// The paths here were checked against the installed harnesses rather than taken from
// documentation that may have moved: Claude Code 2.1.286, Codex 0.163.0, and Pi 1.2.0.
// Where they disagree, this is what the program does about the disagreement.
package harness

import (
	"os"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// dirNames is the directory each root names, inside a project.
var dirNames = map[item.Root]string{
	item.RootAgents: ".agents",
	item.RootClaude: ".claude",
	item.RootCodex:  ".codex",
	item.RootPi:     ".pi",
}

// Dir is where a root's files live for a project.
func Dir(project string, root item.Root) string {
	name, ok := dirNames[root]
	if !ok {
		// An unknown root cannot reach here through Parse, which refuses one. Returning
		// the canonical directory beats returning "" and writing to the project root.
		name = dirNames[item.RootAgents]
	}
	return filepath.Join(project, name)
}

// Link is a symbolic link to create, and what it should point at.
type Link struct {
	Path   string
	Target string
}

// Links is the links a set of harnesses needs, in the order they were given.
//
// It is usually empty, and that is the finding rather than an oversight. `.agents/skills`
// is a convention both Pi and Codex already read, so the canonical directory reaches
// them without help; Claude Code does not read it and offers only to import from it,
// which copies rather than tracks. So Claude Code is the single harness that needs a
// link, and a project that does not use it needs none.
//
// Targets are relative, so that moving or copying a project does not leave a link
// pointing into wherever it used to be.
func Links(project string, harnesses []string) []Link {
	var out []Link
	seen := map[string]bool{}
	for _, h := range harnesses {
		if h != "claude" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, Link{
			Path:   filepath.Join(Dir(project, item.RootClaude), "skills"),
			Target: filepath.Join("..", dirNames[item.RootAgents], "skills"),
		})
	}
	return out
}

// Project finds the project a directory belongs to.
//
// It walks up to the nearest directory holding `.agents` or `.git`, which is what git
// does and what nobody has to be told. A directory with neither is its own project, so
// running somewhere that is not a repository generates into where you are standing
// rather than failing.
func Project(from string) (string, error) {
	dir, err := filepath.Abs(from)
	if err != nil {
		return "", err
	}
	for {
		for _, marker := range []string{dirNames[item.RootAgents], ".git"} {
			if info, err := os.Stat(filepath.Join(dir, marker)); err == nil && info.IsDir() {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Abs(from)
		}
		dir = parent
	}
}
