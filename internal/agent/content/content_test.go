package content

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/agent/render"
)

func TestEveryShippedItemParsesAndRenders(t *testing.T) {
	// The items are embedded, so a broken one is shipped rather than discovered. This is
	// the test that turns that into a build failure.
	items, err := Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("no items are shipped")
	}

	ids := map[string]bool{}
	for _, it := range items {
		if ids[it.ID] {
			t.Errorf("two items share the id %q", it.ID)
		}
		ids[it.ID] = true

		for _, out := range it.Outputs {
			body, err := render.Output(out, nil, Files())
			if err != nil {
				t.Errorf("item %s: %v", it.ID, err)
				continue
			}
			if len(strings.TrimSpace(string(body))) == 0 {
				t.Errorf("item %s: output %s renders to nothing with no answers",
					it.ID, out.Path)
			}
		}
	}
}

func TestTheItemsAreInAStableOrder(t *testing.T) {
	// The picker's order. A list that reshuffles makes the user re-read it every run.
	first, err := Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	second, err := Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("order changed between reads at %d: %q then %q", i, first[i].ID, second[i].ID)
		}
	}
}

func TestTheDenyRuleIsOneClaudeCodeActuallyMatches(t *testing.T) {
	// Claude Code matches file permission rules against Edit(path) only, and an Edit rule
	// covers every editing tool. A rule spelled for another tool is not rejected — it is
	// simply never matched, so it reads like protection and gives none. Claude Code says
	// so at startup, which is how this was found.
	body, err := fs.ReadFile(HarnessBase(), "claude/settings.json")
	if err != nil {
		t.Fatalf("the base configuration is missing: %v", err)
	}
	var got struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("the base configuration is not JSON: %v", err)
	}
	if len(got.Permissions.Deny) == 0 {
		t.Fatal("the base configuration denies nothing, so the cache is writable")
	}
	for _, rule := range got.Permissions.Deny {
		if !strings.HasPrefix(rule, "Edit(") {
			t.Errorf("deny rule %q is not an Edit rule, so Claude Code never matches it", rule)
		}
		if !strings.Contains(rule, "reference") {
			t.Errorf("deny rule %q does not name the cache directory", rule)
		}
	}
}
