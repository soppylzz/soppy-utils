package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// indexID is the ledger key for the index file.
//
// Reserving an id rather than inventing an item keeps the picker from offering it: the
// index is not something anyone chooses, it is the list of what they chose.
const indexID = "spu-index"

// indexMarker is where the generated list goes in the prose.
const indexMarker = "<!-- spu:index -->"

// mcpMarker is where the reference-cache section goes. It is only kept when a server was
// actually registered: a project that registered none should not be told how to protect a
// cache it does not use.
const mcpMarker = "<!-- spu:mcp -->"

// indexStep builds SPU.md from the prose and the files this run is generating.
//
// It is generated rather than written as an item because its content is the selection
// itself, and an item cannot describe a list that does not exist until the items are
// chosen.
//
// It is always written when anything is chosen, and never when nothing is: a project
// whose files were all deselected should not keep an index pointing at them.
func indexStep(project string, steps []Step, registeredMCP bool) (Step, error) {
	prose, err := content.SPUProse()
	if err != nil {
		return Step{}, err
	}

	// Grouped by the item that wrote them, in the order the items were chosen, so the
	// list reads the way the picker did.
	type entry struct {
		title string
		paths []string
	}
	seen := map[string]int{}
	var order []string
	byItem := map[string]*entry{}

	for _, step := range steps {
		if step.Item.ID == indexID {
			continue
		}
		idx, ok := seen[step.Item.ID]
		if !ok {
			idx = len(order)
			seen[step.Item.ID] = idx
			order = append(order, step.Item.ID)
			byItem[step.Item.ID] = &entry{title: titleOf(step.Item)}
		}
		byItem[step.Item.ID].paths = append(byItem[step.Item.ID].paths, step.Rel)
	}
	sort.Strings(byItem[order[0]].paths)

	var list strings.Builder
	for _, id := range order {
		e := byItem[id]
		sort.Strings(e.paths)
		if len(e.paths) == 1 {
			fmt.Fprintf(&list, "- `@%s` — %s\n", e.paths[0], e.title)
			continue
		}
		fmt.Fprintf(&list, "- %s:\n", e.title)
		for _, p := range e.paths {
			fmt.Fprintf(&list, "  - `@%s`\n", p)
		}
	}

	// Paths in the index are relative to the index, which sits beside the directories the
	// documents are in, so an `@` resolves without the reader knowing where they are.
	body := strings.Replace(string(prose), indexMarker, strings.TrimRight(list.String(), "\n"), 1)
	if registeredMCP {
		// The section stays; only the marker that placed it goes.
		body = strings.Replace(body, "\n\n"+mcpMarker, "", 1)
	} else {
		body = dropSection(body, mcpMarker)
	}

	return Step{
		Item: item.Item{
			ID:      indexID,
			Version: 1,
			Title:   "This project's agent text",
			// Always regenerated: the list of what was chosen is not something a person
			// edits, and a stale index points at documents that are no longer generated.
			Policy:     item.Fresh,
			OnModified: item.Overwrite,
		},
		Path:     filepath.Join(canonical(project), "SPU.md"),
		Rel:      "SPU.md",
		Decision: decide.Decision{Action: decide.Write, Reason: decide.Outdated},
		Content:  []byte(body),
	}, nil
}

// dropSection removes a whole section — its heading, its body, and the marker that ends
// it — so a section that does not apply leaves no trace rather than an empty heading.
//
// The blank line before the heading is kept: it is what separates the paragraph above
// from the heading below once the section between them is gone.
func dropSection(body, marker string) string {
	lines := strings.Split(body, "\n")
	at := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			at = i
			break
		}
	}
	if at < 0 {
		return body
	}

	start := at
	for start > 0 && !strings.HasPrefix(lines[start], "## ") {
		start--
	}
	end := at + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}

	out := make([]string, 0, len(lines))
	out = append(out, lines[:start]...)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

func titleOf(it item.Item) string {
	if it.Title != "" {
		return it.Title
	}
	return it.ID
}
