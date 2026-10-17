package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LookPathIn resolves name against the given PATH value rather than against the
// calling process's.
//
// exec.LookPath reads the process environment, which is the wrong question here. The
// daemon's PATH belongs to whichever client started it, and every client that arrives
// later has its own; judging a capability with the daemon's PATH would report the same
// answer to everyone and be wrong for all but one of them.
func LookPathIn(pathEnv, name string) (string, error) {
	if name == "" {
		return "", errors.New("empty command name")
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			// An empty entry traditionally means the current directory. Resolving a
			// command from there would let a file in whatever directory the user happens
			// to be standing in decide which program core runs.
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || !isExecutable(info) {
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf("%s: executable file not found in $PATH", name)
}
