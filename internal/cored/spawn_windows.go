//go:build windows

package cored

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Detaching on Windows means a process group of its own and no console.
const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// startDetached runs a daemon that outlives the client that started it.
//
// The same two properties the Unix build needs: no inherited console, and standard
// streams pointed at a log file rather than inherited.
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
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait() }()

	return cmd.Process, nil
}
