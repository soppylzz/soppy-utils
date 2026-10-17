package agent

import (
	"path"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/core"
)

// configDir is where this session's spu configuration lives.
//
// On the platforms that have a roaming application data directory that is where a
// program's own configuration belongs; elsewhere it is under ~/.config. Both are read off
// the session rather than off this process, because the daemon outlives the client that
// started it and its own environment answers for whoever started it.
//
// The library uses this and the other utils do not. Every one of them builds
// ~/.config/soppy-utils by hand today, and moving them is a change to the configuration
// contract that each of them would have to migrate — a separate decision from where a
// template somebody writes should live.
func configDir(s core.Session) string {
	if s.AppData != "" {
		return filepath.Join(s.AppData, "soppy-utils")
	}
	// path.Join: a session's platform is not necessarily this binary's, and the
	// separator has to be the session's.
	return path.Join(s.Home, ".config", "soppy-utils")
}
