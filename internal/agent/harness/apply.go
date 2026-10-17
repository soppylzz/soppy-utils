package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// Apply creates what the chosen harnesses need inside a project, and returns a line for
// each thing it did.
//
// That is one thing: a link, for the one harness that cannot read the canonical directory
// on its own. It is done only for a harness that was chosen, because creating `.codex/`
// in a project that does not use Codex is a rude way to answer a question nobody asked.
//
// spu writes no permission rules for any harness. It used to write Claude Code a deny rule
// for the reference cache and skip a project that already had a settings file — which is
// most of them, silently, so the rule was absent exactly where somebody might have been
// relying on it. Rather than merge into a file the project owns, spu now writes nothing
// and says in SPU.md what to add and why two of the three harnesses cannot.
func Apply(project string, harnesses []string) ([]string, error) {
	var done []string

	// A link into a directory that is not there is a dead link, which looks like spu
	// arranged something and gives a harness nothing to read. A project with no skills
	// needs no link, and most projects have none.
	if hasSkills(project) {
		for _, link := range Links(project, harnesses) {
			made, err := ensureLink(link)
			if err != nil {
				return done, err
			}
			if made {
				done = append(done, "linked "+rel(project, link.Path))
			}
		}
	}

	return done, nil
}

// hasSkills reports whether the canonical directory actually holds skills.
func hasSkills(project string) bool {
	info, err := os.Stat(filepath.Join(Dir(project, item.RootAgents), "skills"))
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}
	// An empty directory is not worth linking to either.
	entries, err := os.ReadDir(filepath.Join(Dir(project, item.RootAgents), "skills"))
	return err == nil && len(entries) > 0
}

// ensureLink creates a link when the path is free, and leaves it alone when it already
// points where it should.
//
// The failure this exists for is Windows, where a symbolic link needs Developer Mode or
// elevation. It is reported rather than worked around: a copy that silently stops tracking
// the original is worse than an error, because the documents then diverge and nothing says
// they have.
func ensureLink(link Link) (bool, error) {
	existing, err := os.Readlink(link.Path)
	switch {
	case err == nil && existing == link.Target:
		return false, nil
	case err == nil:
		return false, fmt.Errorf("%s already points at %s, not %s; move it aside to let "+
			"spu manage it", link.Path, existing, link.Target)
	case !errors.Is(err, os.ErrNotExist):
		if _, statErr := os.Stat(link.Path); statErr == nil {
			// A real directory is there, not a link. Something else owns this path.
			return false, fmt.Errorf("%s already exists and is not a link; move it aside "+
				"to let spu manage it", link.Path)
		}
	}

	if err := os.MkdirAll(filepath.Dir(link.Path), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Dir(link.Path), err)
	}
	if err := os.Symlink(link.Target, link.Path); err != nil {
		return false, symlinkHint(link, err)
	}
	return true, nil
}

// symlinkHint adds what to do about the platform where this normally fails.
func symlinkHint(link Link, err error) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("could not create the link %s: %w. On Windows a symbolic link "+
			"needs Developer Mode or an elevated shell; without one, link the directory "+
			"by hand or leave it, and Claude Code will not see the skills", link.Path, err)
	}
	return fmt.Errorf("could not create the link %s: %w", link.Path, err)
}

func rel(project, path string) string {
	r, err := filepath.Rel(project, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

// Root maps a harness name to the root its files live under, so a harness identifier and
// a directory name cannot drift apart.
func Root(name string) item.Root {
	switch strings.ToLower(name) {
	case "claude":
		return item.RootClaude
	case "codex":
		return item.RootCodex
	case "pi":
		return item.RootPi
	default:
		return item.RootAgents
	}
}
