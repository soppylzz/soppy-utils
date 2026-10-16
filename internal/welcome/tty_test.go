package welcome

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// runWithATty runs a generated banner under a controlling terminal and returns what it
// printed.
//
// The banner only speaks when a terminal is attached, so this is the only way to see
// it. The pattern comes from the test that already had to prove a banner survives
// redirected standard streams.
func runWithATty(t *testing.T, banner []byte, env ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("script does not provide a Unix controlling terminal on Windows")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script is unavailable")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "welcome.sh")
	if err := os.WriteFile(path, banner, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(dir, "runner.sh")
	script := []byte(`#!/usr/bin/env bash
exec 3>&1
exec </dev/null >"$SPU_TEST_CAPTURE" 2>&1
SPU_WELCOME_INTERACTIVE=1 bash "$SPU_TEST_SCRIPT"
exec 1>&3 2>&3
cat "$SPU_TEST_CAPTURE"
`)
	if err := os.WriteFile(runner, script, 0o700); err != nil {
		t.Fatal(err)
	}

	var args []string
	if runtime.GOOS == "darwin" {
		args = []string{"-q", "/dev/null", "bash", runner}
	} else {
		args = []string{"-q", "-c", "bash " + ShellQuote(runner), "/dev/null"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "script", args...)
	// TMUX is cleared unless a test asks for it. Inheriting it would make the result
	// depend on whether whoever ran the tests happens to be inside tmux, and a test
	// about tmux that cannot control it is not a test.
	cmd.Env = append(append(append(os.Environ(),
		"SPU_TEST_SCRIPT="+path,
		"SPU_TEST_CAPTURE="+filepath.Join(dir, "captured"),
		"HOME="+dir),
		"TMUX="), env...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("terminal run failed: %v, output: %.160q", err, out)
	}
	return string(out)
}
