package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/action"
)

// maxFetchBytes caps a download.
//
// The URL comes from core, which got it from a util, but a client that read whatever a
// server chose to send into memory would be one redirect away from being unable to run.
const maxFetchBytes = 8 << 20

// commandFor turns an action into a program and its arguments.
func commandFor(a action.Action) (string, []string, error) {
	if len(a.Argv) == 0 {
		return "", nil, errors.New("the action names no program to run")
	}
	if a.NeedsElevation {
		// Core said privileges are needed and stopped there. This is where the "how"
		// lives, which is why sudo appears on this side and nowhere in core.
		return "sudo", a.Argv, nil
	}
	return a.Argv[0], a.Argv[1:], nil
}

// performActions carries out what core asked for and reports what became of it.
//
// Every action is attempted even after one fails, and each failure is reported
// separately: a command that needs two packages is better told which one is still
// missing than that something went wrong.
func performActions(
	ctx context.Context, in io.Reader, out io.Writer, req action.Request, confirm func() bool,
) action.Outcome {
	var outcome action.Outcome

	if len(req.Actions) > 0 && confirm != nil && !confirm() {
		outcome.Declined = true
		return outcome
	}

	for _, a := range req.Actions {
		var err error
		switch a.Kind {
		case action.Run:
			err = runOne(ctx, in, out, a)
		case action.Fetch:
			err = fetchOne(ctx, a)
		default:
			err = fmt.Errorf("cannot perform an action of kind %q", a.Kind)
		}
		if err != nil {
			outcome.Failures = append(outcome.Failures, err.Error())
			continue
		}
		outcome.Performed++
	}
	return outcome
}

func runOne(ctx context.Context, in io.Reader, out io.Writer, a action.Action) error {
	name, args, err := commandFor(a)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, name, args...)
	// The terminal is passed through, because an action that needs privileges needs
	// somewhere to ask for them.
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return nil
}

// fetchOne downloads a file, checks it against the digest core supplied, and only then
// puts it in place.
//
// The order matters: a file that is written first and checked afterwards is one that a
// failed run leaves behind for the next one to find and believe.
func fetchOne(ctx context.Context, a action.Action) error {
	if a.SHA256 == "" {
		return fmt.Errorf("%s: no digest to check the download against", a.URL)
	}
	if a.Dest == "" {
		return fmt.Errorf("%s: no destination", a.URL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", a.URL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", a.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", a.URL, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return fmt.Errorf("%s: %w", a.URL, err)
	}
	if len(body) > maxFetchBytes {
		return fmt.Errorf("%s: larger than the %d byte limit", a.URL, maxFetchBytes)
	}

	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != strings.ToLower(a.SHA256) {
		return fmt.Errorf("%s: the download does not match its digest", a.URL)
	}

	if err := os.MkdirAll(filepath.Dir(a.Dest), 0o700); err != nil {
		return fmt.Errorf("%s: %w", a.Dest, err)
	}
	if err := os.WriteFile(a.Dest, body, 0o644); err != nil {
		return fmt.Errorf("%s: %w", a.Dest, err)
	}
	return nil
}
