package harness

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// Apply creates what the chosen harnesses need inside a project, and returns a line for
// each thing it did.
//
// Two things, both of which must not happen when they are not wanted: a link, for the one
// harness that cannot read the canonical directory, and a base configuration, for a
// project that does not have its own. Creating `.codex/` in a project that does not use
// Codex is a rude way to answer a question nobody asked, so neither happens for a harness
// that was not chosen.
func Apply(project string, harnesses []string, base fs.FS) ([]string, error) {
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

	for _, h := range harnesses {
		wrote, err := writeBaseConfig(project, h, base)
		if err != nil {
			return done, err
		}
		done = append(done, wrote...)
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

// writeBaseConfig copies a harness's base configuration in, and only when there is
// nothing at the path.
//
// The whole file rather than a merge: a project's own settings are the project's, and
// writing into a file somebody also edits is how a tool loses their work. The cost is
// named in the design — a value that changes later is not carried into a file already
// written.
func writeBaseConfig(project, h string, base fs.FS) ([]string, error) {
	if base == nil {
		return nil, nil
	}
	var written []string
	err := fs.WalkDir(base, h, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(h, p)
		if err != nil {
			return err
		}
		target := filepath.Join(Dir(project, item.Root(h)), relPath)
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		body, err := fs.ReadFile(base, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return err
		}
		written = append(written, "wrote "+rel(project, target))
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return written, err
	}
	return written, nil
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
