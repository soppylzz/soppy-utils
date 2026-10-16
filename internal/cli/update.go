package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// installRoot is where the package is installed.
//
// Two sources, because two callers know different things. The launcher knows the root it
// was run from, which is the only thing that works in a checkout: `go run` builds into a
// temporary directory, and a binary there cannot work out where the source tree is. A
// binary started directly, as an installed one always is, works it out from its own path:
// libexec/spu sits one level under the root.
func installRoot() string {
	if root := os.Getenv("SPU_ROOT"); root != "" {
		return root
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(exe))
}

// runUpdate hands the upgrade to the packaged script.
//
// update is the one command the Go side does not implement, because what it does is
// replace the installation this binary is running from, and the steps that make that safe
// — waiting for the running core to stop, swapping directories, keeping a backup until
// the new package has passed its own check — are shell and belong beside the installer
// that shares them.
//
// It is routed here rather than left to the launcher so that the command surface is the
// same however spu was invoked. A binary called directly used to answer "unknown util".
func runUpdate(args []string) (int, error) {
	root := installRoot()
	if root == "" {
		return 1, errors.New("cannot tell where this spu is installed")
	}

	script := filepath.Join(root, "scripts", "update.sh")
	if _, err := os.Stat(script); err != nil {
		return 1, fmt.Errorf("no updater beside this binary: %w", err)
	}

	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// The script writes its own diagnostics, so a failure is reported once rather than
	// twice: its exit status is the answer.
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
