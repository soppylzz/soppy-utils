// Package action describes work core needs done but cannot do itself.
//
// Core runs detached, with no terminal and no way to answer an elevation prompt, so
// anything requiring one is offered to the client instead. Core decides what is needed;
// the client decides how to do it. That split is what keeps core free of both terminal
// assumptions and privilege assumptions — it never invokes sudo and has no opinion
// about it.
package action

// Kind is the sort of work an action describes.
type Kind string

const (
	// Run executes a program.
	Run Kind = "run"

	// Fetch downloads a file to a destination and checks its digest.
	Fetch Kind = "fetch"
)

// Action is one thing to do.
type Action struct {
	Kind Kind `json:"kind"`

	// Argv is the program and its arguments as separate words.
	//
	// Never a command string. A shell would re-split it, and the difference between an
	// argument and an injection is exactly who does the splitting.
	Argv []string `json:"argv,omitempty"`

	// NeedsElevation says the program needs privileges. Core states that it is needed
	// and stops there; obtaining it is the client's business.
	NeedsElevation bool `json:"needs_elevation,omitempty"`

	// URL, Dest and SHA256 describe a Fetch. The digest is not optional: a client that
	// fetched without checking one would be installing whatever the network handed it.
	URL    string `json:"url,omitempty"`
	Dest   string `json:"dest,omitempty"`
	SHA256 string `json:"sha256,omitempty"`

	// Optional says the work may be skipped without the command failing.
	Optional bool `json:"optional,omitempty"`

	// ChangesMachine says the action alters the machine rather than the command's own
	// output. Installing a package does; opening a file in the editor does not.
	//
	// Core states the fact and stops there. Whether a user should be asked first is the
	// client's business, and the two answers differ: a client asks before installing and
	// does not ask before opening.
	ChangesMachine bool `json:"changes_machine,omitempty"`
}

// Request is a set of actions and the reason for them.
type Request struct {
	// Reason is why these are being asked for, in a sentence a user can act on. A client
	// shows it before anything runs.
	Reason string `json:"reason,omitempty"`

	Actions []Action `json:"actions"`
}

// Outcome is what became of a request.
type Outcome struct {
	// Performed is how many actions succeeded.
	Performed int `json:"performed"`

	// Declined is true when the actions were offered and refused, or could not be
	// offered at all.
	Declined bool `json:"declined,omitempty"`

	// Failures is one line per action that did not succeed, so a command can say which
	// piece is still missing rather than only that something went wrong.
	Failures []string `json:"failures,omitempty"`
}

// Complete reports whether every action was carried out.
//
// A command uses it to decide between proceeding and proceeding degraded; either way
// it re-probes rather than trusting this, because the answer describes what the client
// attempted rather than what the machine now looks like.
func (o Outcome) Complete() bool {
	return !o.Declined && len(o.Failures) == 0
}
