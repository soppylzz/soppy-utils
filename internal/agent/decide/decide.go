// Package decide answers what to do about a file an item wants to write.
//
// It is a function of facts and a policy, and nothing else: no filesystem, no terminal,
// no clock. That is what lets the whole table be tested at once, and it is why the
// reasoning behind every outcome is written down here rather than at the call site.
package decide

import "github.com/soppylzz/soppy-utils/internal/agent/item"

// Facts is what is true about one output's path. They are gathered by reading the
// filesystem and the ledger, and nothing here gathers them.
type Facts struct {
	// Exists is whether there is a file at the path.
	Exists bool

	// Recorded is whether the ledger records this item as having written that path. It
	// is the only way to tell our file from somebody else's, and existence alone never
	// implies it.
	Recorded bool

	// Identical is whether the file already holds exactly what would be written.
	Identical bool

	// HashMatches is whether the file is still what spu wrote. False with Recorded true
	// means something changed it after the fact — by a person or by another program.
	HashMatches bool
}

// Action is what to do.
type Action string

const (
	// Write replaces or creates the file.
	Write Action = "write"

	// Leave does nothing, which is a decision rather than an omission.
	Leave Action = "leave"

	// Refuse stops and reports. The caller says what it found; nothing is written.
	Refuse Action = "refuse"

	// Ask puts it to whoever is there. A caller with nobody to ask treats it as Refuse.
	Ask Action = "ask"
)

// Reason is why, so the message can say something true.
type Reason string

const (
	// Absent: there is nothing at the path.
	Absent Reason = "absent"

	// Unchanged: the file already holds what would be written.
	Unchanged Reason = "unchanged"

	// Outdated: our file, untouched, and the text we would write has moved on.
	Outdated Reason = "outdated"

	// Edited: our file, changed since we wrote it.
	Edited Reason = "edited"

	// Foreign: a file we did not write is at the path.
	Foreign Reason = "foreign"

	// Kept: the policy is to leave what is already there.
	Kept Reason = "kept"
)

// Decision is an action and the reason for it.
type Decision struct {
	Action Action
	Reason Reason
}

// Plan decides what to do about one output.
//
// The three policies differ in one question each, and the difference is deliberate:
//
//   - reuse writes what is missing or out of date and asks on_modified before replacing
//     a file somebody edited.
//   - fresh writes what is missing or out of date and replaces an edited file without
//     asking, because the canonical text is the whole point of it.
//   - skip writes only what is missing, and then the file belongs to whoever edits it.
//
// All three write when nothing is there, including skip: skip answers what to do with
// something that exists, and a file that was deleted is not something to be protected.
func Plan(f Facts, policy item.Policy, onModified item.OnModified) Decision {
	if !f.Exists {
		// Recorded or not is irrelevant here. If the ledger says we wrote it and it is
		// gone, the ledger is a record of what we did, not a claim on the path.
		return Decision{Action: Write, Reason: Absent}
	}

	if !f.Recorded {
		// Somebody else's file. Overwriting it destroys work that cannot be recovered,
		// and the safe answer does not depend on how confident we are.
		if policy == item.Skip {
			return Decision{Action: Leave, Reason: Foreign}
		}
		return Decision{Action: Refuse, Reason: Foreign}
	}

	if f.Identical {
		// Writing identical bytes changes nothing a person can see and churns the mtime
		// a watcher may be reading. Every policy leaves it.
		return Decision{Action: Leave, Reason: Unchanged}
	}

	if !f.HashMatches {
		// Ours, and different from what we wrote. Either a person edited it or something
		// else rewrote it; the recorded hash cannot tell those apart and does not try.
		switch policy {
		case item.Skip:
			return Decision{Action: Leave, Reason: Kept}
		case item.Fresh:
			// The canonical text is the point of this policy, so the edit goes. The
			// reason still says an edit was replaced rather than that our text moved on,
			// because those are different things to have happened to somebody's work.
			return Decision{Action: Write, Reason: Edited}
		}
		switch onModified {
		case item.Refuse:
			return Decision{Action: Refuse, Reason: Edited}
		case item.Ask:
			return Decision{Action: Ask, Reason: Edited}
		default:
			return Decision{Action: Write, Reason: Edited}
		}
	}

	if policy == item.Skip {
		// Ours, untouched, but the text moved on. skip said once and means it.
		return Decision{Action: Leave, Reason: Kept}
	}
	return Decision{Action: Write, Reason: Outdated}
}
