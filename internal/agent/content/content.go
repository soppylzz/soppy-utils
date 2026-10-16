// Package content holds the items spu ships and the text they generate.
//
// It is embedded rather than downloaded: the documents an agent follows are text the
// user's agent will act on, and fetching them at run time would make every project's
// conventions depend on a network that is up and a host that is honest. The cost is that
// changing a document takes a release, which is the same trade `welcome` made.
package content

import (
	"embed"
	"io/fs"
	"sort"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

//go:embed items/*.json
var itemFiles embed.FS

//go:embed docs
var docs embed.FS

//go:embed harnesses
var harnesses embed.FS

// Items parses the shipped items, in a stable order.
//
// Order matters: the picker shows them in this order, and a list that reshuffles between
// runs makes the user re-read it every time. Sorted by id, which is also the order the
// files are embedded in on every platform.
func Items() ([]item.Item, error) {
	entries, err := fs.Glob(itemFiles, "items/*.json")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	out := make([]item.Item, 0, len(entries))
	for _, name := range entries {
		body, err := itemFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		it, err := item.Parse(body)
		if err != nil {
			// A shipped item that does not parse is a build error, not a user error:
			// it is caught by the test that reads every one of them.
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// SPUProse is the text of the index file the agent reads.
//
// It is not an item: its content is the list of what was chosen, and that list does not
// exist until the choices are made. The prose is here and the list is filled in.
func SPUProse() ([]byte, error) {
	return docs.ReadFile("docs/spu.md")
}

// HarnessBase is the configuration a harness needs in a project.
//
// The reference cache is readable and not writable: an agent may read the checkouts and
// may not change them, and that deny rule is the whole of the safety. Written for Claude
// Code only, because its format is the one that has been checked — Codex and Pi express
// the same intent in formats this does not know, and guessing would write a file that
// looks right and does nothing.
func HarnessBase() fs.FS {
	sub, err := fs.Sub(harnesses, "harnesses")
	if err != nil {
		panic(err)
	}
	return sub
}

// Files is the text the items name in their content segments.
//
// Paths in an item are relative to `docs`, so a segment says which document it wants
// without knowing where the documents live.
func Files() fs.FS {
	sub, err := fs.Sub(docs, "docs")
	if err != nil {
		// Impossible: the directory is embedded above, and fs.Sub only fails when it is
		// not a directory.
		panic(err)
	}
	return sub
}
