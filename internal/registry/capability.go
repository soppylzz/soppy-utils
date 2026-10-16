package registry

// State is how usable a capability is in one session's environment.
//
// Four states rather than a boolean, because "missing" is three different situations
// that a user has to be told apart: not applicable here, not written for this platform
// yet, and the pieces are not installed. Today's code collapses all three into one line
// on stderr, which is why a client cannot tell the user what to do about any of them.
type State string

const (
	// Available means the capability runs.
	Available State = "available"

	// Degraded means it runs with a stated loss and is still worth offering.
	Degraded State = "degraded"

	// Unavailable means it is meaningful here but cannot run. A client should draw it
	// disabled, carrying Detail, rather than omitting it: a blank where a feature was
	// is more confusing than a disabled entry that says why.
	Unavailable State = "unavailable"

	// Hidden means it is meaningless here and should not be drawn at all.
	Hidden State = "hidden"
)

// Reason says why a capability is not Available.
type Reason string

const (
	NotApplicable     Reason = "not_applicable"
	NotImplemented    Reason = "not_implemented"
	MissingDependency Reason = "missing_dependency"
	UnsupportedShell  Reason = "unsupported_shell"
	Configuration     Reason = "configuration"
	ModifiedArtifact  Reason = "modified_artifact"
)

// Assessment is one capability's verdict for one environment.
type Assessment struct {
	State   State
	Reason  Reason
	Detail  string
	Missing []string
}

// Judge evaluates a capability against a session's environment.
//
// It takes a Host rather than reaching for the process environment, so it cannot
// accidentally depend on the daemon's own PATH and shell. Capabilities are judged on
// the requirement that gates them, never on the operating system's name: keying on
// GOOS makes "usable from Git Bash, hidden from PowerShell" inexpressible, and bakes a
// wrong assumption into the registry for good.
type Judge func(Host) Assessment

// Capability is one thing core can do.
type Capability struct {
	// ID is the name a client uses and a user types.
	ID string

	// Title is a short human name.
	Title string

	// Usage is the one-line form a client shows in help.
	Usage string

	// Commands is the command surface. Help, usage errors, and unknown-command errors
	// all render from here, so no client keeps a second hand-written list of what
	// exists.
	Commands []Command

	// Judge reports how usable this is. Nil means it depends on nothing and is
	// available wherever core runs.
	Judge Judge
}

// Command is one subcommand of a capability.
type Command struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Flags   []Flag `json:"flags,omitempty"`
}

// Flag is one option of a command.
//
// Arg is the kind of value it takes, and Options lists the accepted values for an enum,
// so a client can render a choice instead of free text.
type Flag struct {
	Name    string   `json:"name"`
	Arg     string   `json:"arg,omitempty"`
	Summary string   `json:"summary"`
	Options []string `json:"options,omitempty"`
}
