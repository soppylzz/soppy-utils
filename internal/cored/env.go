package cored

import (
	"os"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// Capabilities is what a client says it can do.
//
// It is a negotiation rather than a courtesy. Core offers work to a client that can
// perform it and describes the same work as a warning to one that cannot, because a
// scripted install has no terminal to answer a question with, and asking anyway would
// hang it.
type Capabilities struct {
	ActionExecution bool `json:"action_execution"`
	Confirm         bool `json:"confirm"`
	Interactive     bool `json:"interactive"`
}

// Env is what a client reports about the machine it is running on.
//
// It travels with the connection rather than with the daemon. The daemon is long-lived
// and was started by whichever client came first, so its own PATH, shell, and home are
// the wrong answer for every client that arrives later — and the wrongness is silent,
// because a capability judged with the wrong PATH still returns a confident answer.
type Env struct {
	GOOS  string `json:"os"`
	Shell string `json:"shell"`
	Path  string `json:"path"`
	Home  string `json:"home"`

	// HomeNative is the home spelled the way this platform's file APIs want it. A
	// client running under Git Bash reports a POSIX spelling the native daemon cannot
	// open, so it also reports the converted form. The project has carried
	// SPU_HOME_NATIVE for this since the previous migration.
	HomeNative string `json:"home_native,omitempty"`

	Tmux string `json:"tmux,omitempty"`

	// WorkingDir is the directory the client was run from, carried for the same reason
	// the home is: a command that works on a project asks the session which project,
	// and the daemon's own directory is an answer about a different process.
	WorkingDir string `json:"workdir,omitempty"`

	// WorkingDirNative is that directory spelled the way this platform's file APIs want
	// it, for the reason HomeNative exists.
	WorkingDirNative string `json:"workdir_native,omitempty"`

	// AppData is the roaming application data directory on the platforms that have one.
	AppData string `json:"appdata,omitempty"`
}

// HomeForFilesystem is the home the daemon should use for file checks.
func (e Env) HomeForFilesystem() string {
	if e.HomeNative != "" {
		return e.HomeNative
	}
	return e.Home
}

// WorkingDirForFilesystem is the working directory the daemon should use for file access.
func (e Env) WorkingDirForFilesystem() string {
	if e.WorkingDirNative != "" {
		return e.WorkingDirNative
	}
	return e.WorkingDir
}

// host builds the view a capability is judged against.
func (e Env) host() registry.Host {
	return registry.Host{
		GOOS:  e.GOOS,
		Home:  e.HomeForFilesystem(),
		Shell: e.Shell,
		LookPath: func(name string) (string, error) {
			return registry.LookPathIn(e.Path, name)
		},
		// The client and the daemon are on the same machine, so the daemon's own view
		// of the filesystem is the right one — as long as the paths are in a form it
		// can open, which is what HomeForFilesystem is for.
		Stat: os.Stat,
	}
}
