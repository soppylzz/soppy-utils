package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// mcpRecordID is the ledger key for the canonical MCP definition.
const mcpRecordID = "spu-mcp"

// mcpServers is what spu registers with a project's harnesses.
//
// `clone-third-party` ships beside `spu` and is named rather than pathed: a registration
// committed to a repository has to work on the machine that reads it, not only on the one
// that wrote it.
func mcpServers() []harness.Server {
	return []harness.Server{{
		Name:    "clone-third-party",
		Command: "spu-clone-third-party",
	}}
}

// mcpStep writes the canonical definition the harness files are projected from.
//
// It is the one place to see what spu registered, and it is what makes the per-harness
// files explainable: three files in two formats are three projections of this, not three
// independent statements.
func mcpStep(project string, harnesses []string, servers []harness.Server) (Step, error) {
	named := map[string]map[string]any{}
	for _, s := range servers {
		entry := map[string]any{"command": s.Command}
		if len(s.Args) > 0 {
			entry["args"] = s.Args
		}
		named[s.Name] = entry
	}

	doc := map[string]any{
		"version": 1,
		"servers": named,
		// Which harnesses these were written into. A file that does not say is a file
		// somebody has to diff three config files to understand.
		"registered_with": harnesses,
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Step{}, fmt.Errorf("encode the MCP record: %w", err)
	}

	return Step{
		Item: item.Item{
			ID:      mcpRecordID,
			Version: 1,
			Title:   "Registered MCP servers",
			// Always regenerated: the list of harnesses is not something a person edits
			// here, and a stale record would describe registrations that are gone.
			Policy:     item.Fresh,
			OnModified: item.Overwrite,
		},
		Path:     filepath.Join(canonical(project), "mcp", "spu.json"),
		Rel:      "mcp/spu.json",
		Decision: decide.Decision{Action: decide.Write, Reason: decide.Outdated},
		Content:  append(body, '\n'),
	}, nil
}

// missing returns the names in from that are not in against.
//
// Used for the harnesses a project has stopped using: a registration that outlives the
// choice keeps offering an agent a tool the project no longer wants.
func missing(from, against []string) []string {
	have := make(map[string]bool, len(against))
	for _, h := range against {
		have[h] = true
	}
	var out []string
	for _, h := range from {
		if !have[h] {
			out = append(out, h)
		}
	}
	return out
}
