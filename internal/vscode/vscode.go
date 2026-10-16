// Package vscode opens the files a person edits in VS Code.
//
// It has no state of its own and nothing to configure, which makes it the smallest util
// here and the clearest example of what core does: it works out a path and asks the
// client to hand it to the editor. Core cannot run the editor itself, because the daemon
// is detached and the editor belongs in the session somebody is looking at.
package vscode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// target is which of the user's files to open.
type target string

const (
	snippets target = "snippets"
	settings target = "settings"
	keys     target = "keys"
)

// Run performs one vscode command.
func Run(ctx context.Context, session core.Session, args []string, out io.Writer, reply core.Responder) error {
	if len(args) == 0 {
		// It used to print this and exit zero, and a script that calls it with nothing
		// should not start failing.
		fmt.Fprintln(out, "vscode: open one of the VS Code files: -S/--snippets, -s/--settings, -k/--keys")
		return nil
	}

	want, err := parse(args)
	if err != nil {
		return err
	}

	dir, err := userDir(session)
	if err != nil {
		return err
	}

	path := pathFor(dir, want)

	// The editor is the user's program in the user's session, so core declares it and the
	// client runs it. Nothing here changes the machine, which is why a client does not
	// ask before doing it.
	outcome, err := reply.AskActions(ctx, action.Request{
		Reason: "open " + path,
		Actions: []action.Action{{
			Kind: action.Run, Optional: true,
			Argv: []string{"code", path},
		}},
	})
	if err != nil {
		return err
	}
	// The client ran it, so the client is the one that knows whether it worked. A command
	// that ignored that would report success for an editor that never opened.
	if len(outcome.Failures) > 0 {
		return errors.New(strings.Join(outcome.Failures, "; "))
	}
	return nil
}

// parse reads the one option this util takes.
func parse(args []string) (target, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("expected one option, got %d", len(args))
	}
	switch args[0] {
	case "-S", "--snippets":
		return snippets, nil
	case "-s", "--settings":
		return settings, nil
	case "-k", "--keys":
		return keys, nil
	}
	return "", fmt.Errorf("unknown option %q", args[0])
}

// userDir is where VS Code keeps the files a person edits.
func userDir(s core.Session) (string, error) {
	switch s.GOOS {
	case "darwin":
		return filepath.Join(s.Home, "Library", "Application Support", "Code", "User"), nil
	case "linux":
		return filepath.Join(s.Home, ".config", "Code", "User"), nil
	case "windows":
		if s.AppData == "" {
			return "", errors.New("the session did not say where the roaming application " +
				"data is, which is where VS Code keeps its user files on Windows")
		}
		return filepath.Join(s.AppData, "Code", "User"), nil
	}
	return "", fmt.Errorf("there is nowhere to look for VS Code configuration on %s", s.GOOS)
}

func pathFor(dir string, want target) string {
	switch want {
	case snippets:
		return filepath.Join(dir, "snippets")
	case settings:
		return filepath.Join(dir, "settings.json")
	}
	return filepath.Join(dir, "keybindings.json")
}
