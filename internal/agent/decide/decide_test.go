package decide

import (
	"testing"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// facts is a shorthand so the table below reads as a table.
func facts(exists, recorded, identical, hashMatches bool) Facts {
	return Facts{Exists: exists, Recorded: recorded, Identical: identical, HashMatches: hashMatches}
}

func TestTheDecisionTable(t *testing.T) {
	// Every combination a file can be in, against every policy. The reason matters as
	// much as the action: it is what gets said to whoever is standing there, and an
	// action with the wrong reason is a correct behaviour explained wrongly.
	cases := []struct {
		name   string
		f      Facts
		policy item.Policy
		onMod  item.OnModified
		want   Action
		why    Reason
	}{
		// Nothing is there. Every policy writes, including skip: skip is about what to do
		// with something that exists.
		{"absent, reuse", facts(false, false, false, false), item.Reuse, item.Overwrite, Write, Absent},
		{"absent, fresh", facts(false, false, false, false), item.Fresh, item.Overwrite, Write, Absent},
		{"absent, skip", facts(false, false, false, false), item.Skip, item.Refuse, Write, Absent},

		// Deleted after we wrote it. The ledger still records it, so it is ours to put
		// back rather than a stranger's file.
		{"deleted, reuse", facts(false, true, false, false), item.Reuse, item.Overwrite, Write, Absent},
		{"deleted, skip", facts(false, true, false, false), item.Skip, item.Refuse, Write, Absent},

		// Somebody else's file at the path. Writing over it destroys work we cannot
		// recover, and skip is the only policy that has already said it will not.
		{"foreign, reuse", facts(true, false, false, false), item.Reuse, item.Overwrite, Refuse, Foreign},
		{"foreign, fresh", facts(true, false, false, false), item.Fresh, item.Overwrite, Refuse, Foreign},
		{"foreign, skip", facts(true, false, false, false), item.Skip, item.Refuse, Leave, Foreign},

		// Ours, and already exactly what we would write.
		{"unchanged, reuse", facts(true, true, true, true), item.Reuse, item.Overwrite, Leave, Unchanged},
		{"unchanged, fresh", facts(true, true, true, true), item.Fresh, item.Overwrite, Leave, Unchanged},
		{"unchanged, skip", facts(true, true, true, true), item.Skip, item.Refuse, Leave, Unchanged},

		// Ours, untouched, but our own text has moved — the item changed, not the file.
		// Nothing was edited, so there is nothing to protect and on_modified is not asked.
		{"outdated, reuse", facts(true, true, false, true), item.Reuse, item.Refuse, Write, Outdated},
		{"outdated, fresh", facts(true, true, false, true), item.Fresh, item.Refuse, Write, Outdated},
		{"outdated, skip", facts(true, true, false, true), item.Skip, item.Refuse, Leave, Kept},

		// Ours, and changed since we wrote it. This is the only case the hash exists for.
		{"edited, reuse overwrites", facts(true, true, false, false), item.Reuse, item.Overwrite, Write, Edited},
		{"edited, reuse refuses", facts(true, true, false, false), item.Reuse, item.Refuse, Refuse, Edited},
		{"edited, reuse asks", facts(true, true, false, false), item.Reuse, item.Ask, Ask, Edited},
		{"edited, fresh overwrites regardless", facts(true, true, false, false), item.Fresh, item.Refuse, Write, Edited},
		{"edited, skip keeps it", facts(true, true, false, false), item.Skip, item.Overwrite, Leave, Kept},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Plan(tc.f, tc.policy, tc.onMod)
			if got.Action != tc.want {
				t.Errorf("Action = %q, want %q", got.Action, tc.want)
			}
			if got.Reason != tc.why {
				t.Errorf("Reason = %q, want %q", got.Reason, tc.why)
			}
		})
	}
}

func TestOnlyReuseConsultsOnModified(t *testing.T) {
	// fresh and skip have each already said what an edit means, so the hash cannot move
	// their answer. If it could, on_modified would be a fourth policy wearing a
	// different name.
	edited := facts(true, true, false, false)
	for _, on := range []item.OnModified{item.Overwrite, item.Refuse, item.Ask} {
		if got := Plan(edited, item.Reuse, on); got.Action == Leave {
			t.Errorf("reuse with on_modified=%q left an edited file alone", on)
		}
	}
	for _, p := range []item.Policy{item.Fresh, item.Skip} {
		want := Plan(edited, p, item.Refuse)
		for _, on := range []item.OnModified{item.Overwrite, item.Ask} {
			if got := Plan(edited, p, on); got != want {
				t.Errorf("policy %q changed its answer with on_modified=%q: %+v, want %+v",
					p, on, got, want)
			}
		}
	}
}

func TestTheHashChangesNothingWhenItAgrees(t *testing.T) {
	// An untouched file of ours is not an edit, whatever on_modified says. Reading the
	// hash as "something happened" rather than "the file still matches" would turn every
	// ordinary re-run into a conflict.
	untouched := facts(true, true, false, true)
	for _, p := range []item.Policy{item.Reuse, item.Fresh} {
		for _, on := range []item.OnModified{item.Overwrite, item.Refuse, item.Ask} {
			if got := Plan(untouched, p, on); got.Action != Write {
				t.Errorf("policy %q with on_modified=%q on an untouched file: %+v, want a write",
					p, on, got)
			}
		}
	}
}
