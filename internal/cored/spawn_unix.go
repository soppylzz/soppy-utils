//go:build !windows

package cored

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// startDetached runs a daemon that outlives the client that started it.
//
// Two things separate a daemon from a child, and both are load-bearing:
//
//   - A new session, so it has no controlling terminal. Otherwise closing the
//     terminal that ran the first spu sends SIGHUP and ends the daemon that a window
//     client is still using.
//   - Standard streams pointed at a log file rather than inherited. A daemon writing
//     to an inherited stdout would print into the user's shell minutes after their
//     command returned, and its stdin would be whatever that shell had.
func startDetached(argv, env []string, logPath string) (*os.Process, error) {
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer stdin.Close()

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", logPath, err)
	}
	defer logFile.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Reaped in the background. Without this the child stays a zombie for as long as
	// the client lives, and a client that lives a while is an ordinary case.
	go func() { _ = cmd.Wait() }()

	return cmd.Process, nil
}
