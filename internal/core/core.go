// Package core is the contract between the daemon and the utils it runs.
//
// It is its own package because both sides need it and neither owns it: the daemon
// implements a Responder, and a util package implements a Handler and calls one.
// Putting these types in either place would make the other depend on it, which is the
// direction this arrangement exists to avoid.
package core

import (
	"context"
	"encoding/json"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/form"
)

// Session is what the session that asked is like.
//
// These are facts the daemon knows and a command needs, and they are facts about the
// session rather than about this process: the daemon outlives the client that started
// it, and a later client may be a different user session entirely. They travel together
// so that a command takes one thing rather than growing a field at a time.
//
// This is the seed of the SessionContext the design calls for. Client capabilities and
// a workspace join it when something needs them.
type Session struct {
	// Home is the session's home. Every util works on this one.
	Home string

	// GOOS is the platform the session reported. It is not read from this process,
	// because on Git Bash the two can differ.
	GOOS string

	// Shell is the shell the user is in, when the client could tell.
	Shell string

	// AppData is where this platform keeps roaming application data, when it has such a
	// thing. Windows does, and VS Code puts its user files under it; a command that needs
	// it cannot guess, because the location is configurable.
	AppData string

	// WorkingDir is the directory the client was run from.
	//
	// A command that works on a project needs to know which one, and this is the same
	// kind of fact as the home: the daemon outlives whoever started it, so its own
	// working directory answers a question about a process, not about this session.
	WorkingDir string
}

// Command is one client request for core to do something.
type Command struct {
	Util    string
	Args    []string
	Session Session
}

// Responder is how a running command talks to the client that asked for it.
type Responder interface {
	// Notify reports progress. Nobody is expected to answer.
	//
	// Warn is the same channel with a different meaning, and the difference travels so
	// that a client can present it as its own users expect. The wording is the command's;
	// whether it reads as a warning is what the user is being told.
	Notify(message string) error
	Warn(message string) error

	// Ask sends a form and waits for the values. It is how a command collects what it
	// needs without knowing how the client will ask.
	Ask(ctx context.Context, spec form.Spec) (map[string]any, error)

	// AskActions offers work core cannot do itself, such as installing a package that
	// needs privileges. A client that cannot perform it is told what is missing and
	// asked nothing, and the outcome says so.
	AskActions(ctx context.Context, req action.Request) (action.Outcome, error)

	// Confirm asks a yes or no question.
	//
	// Not a form with one checkbox: nothing is being configured, and a client may render
	// a confirmation differently from a field. A client that cannot ask answers no,
	// because a step that needed approval must not proceed without it.
	Confirm(ctx context.Context, question string) (bool, error)

	// Finish ends the session and carries its result.
	Finish(result json.RawMessage) error

	// CanAsk reports whether a person is available to answer.
	//
	// A command that needs values uses it to choose between asking and falling back to
	// documented defaults. Only the responder knows: whether a terminal is attached is
	// a fact about the client, and the daemon it is talking to cannot see one.
	CanAsk() bool
}

// Handler runs commands.
//
// A util package implements its behaviour behind this seam. The daemon knows nothing
// about what any particular util does, and a Handler knows nothing about sockets or
// JSON-RPC.
type Handler interface {
	Run(ctx context.Context, cmd Command, reply Responder) error
}

// Result is what a command reports when it finishes.
//
// The command decides which stream its output belongs on rather than the client,
// because whether output is data or a diagnostic is a fact about the command. A licence
// body and a usage message both look like text and belong in different places.
type Result struct {
	// Output is text the command produced.
	Output string `json:"output,omitempty"`

	// OutputTo names the stream Output belongs on: "stdout", the default, or "stderr".
	OutputTo string `json:"output_to,omitempty"`
}
