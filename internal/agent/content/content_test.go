package content

import (
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

		// With the answers the item declares: that is what a run with nobody to ask
		// produces, and an item whose questions have no defaults is one that cannot be
		// generated without somebody there.
		answers := it.Defaults()
		for _, out := range it.Outputs {
			body, err := render.Output(out, answers, Files())
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
