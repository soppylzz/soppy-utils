package vscode

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"github.com/soppylzz/soppy-utils/internal/registry"
)

func session(home, goos string) core.Session {
	return coretest.Session(home, goos)
}

func TestTheCapabilityNeedsTheCodeCommand(t *testing.T) {
	// Everything this does is hand a path to code. Without it there is nothing to do.
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}

	without := registry.Host{GOOS: "linux", LookPath: func(string) (string, error) { return "", io.EOF }}
	got := r.Snapshot(without).Capabilities[0]
	if got.State != registry.Unavailable || got.Reason != registry.MissingDependency {
		t.Errorf("without code: %q/%q, want unavailable/missing_dependency", got.State, got.Reason)
	}

	with := registry.Host{GOOS: "linux", LookPath: func(n string) (string, error) { return "/usr/bin/" + n, nil }}
	if got := r.Snapshot(with).Capabilities[0]; got.State != registry.Available {
		t.Errorf("with code: %q (%s), want available", got.State, got.Reason)
	}
}

func TestTheUserDirectoryIsPerPlatform(t *testing.T) {
	cases := []struct {
		goos, home, want string
	}{
		{"darwin", "/Users/u", "/Users/u/Library/Application Support/Code/User"},
		{"linux", "/home/u", "/home/u/.config/Code/User"},
	}
	for _, tc := range cases {
		got, err := userDir(session(tc.home, tc.goos))
		if err != nil {
			t.Errorf("%s: %v", tc.goos, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: user directory = %q, want %q", tc.goos, got, tc.want)
		}
	}
}

func TestWindowsWithoutAppDataCannotFindTheUserDirectory(t *testing.T) {
	// VS Code keeps its user files under the roaming application data on Windows, and a
	// session that did not say where that is cannot be guessed at.
	if _, err := userDir(session(`C:\Users\u`, "windows")); err == nil {
		t.Error("the Windows user directory was resolved without being told where to look")
	}
}

func TestAnUnsupportedPlatformSaysSo(t *testing.T) {
	if _, err := userDir(session("/home/u", "plan9")); err == nil {
		t.Error("a user directory was invented for a platform that has none")
	}
}

func TestOpeningAsksTheClientToRunCode(t *testing.T) {
	// This is the command's own effect rather than a change to the machine, so it is not
	// something a user should be asked to approve.
	r := &coretest.Recorder{}
	args := []string{"--settings"}

	if err := Run(context.Background(), session("/home/u", "linux"), args, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}

	req, offered := r.LastAction()
	if !offered {
		t.Fatal("nothing was offered to the client")
	}
	if len(req.Actions) != 1 {
		t.Fatalf("offered %d actions, want one", len(req.Actions))
	}
	action := req.Actions[0]
	if len(action.Argv) != 2 || action.Argv[0] != "code" {
		t.Errorf("argv = %v, want code and a path", action.Argv)
	}
	if action.Argv[1] != "/home/u/.config/Code/User/settings.json" {
		t.Errorf("path = %q, want the settings file", action.Argv[1])
	}
	if action.ChangesMachine {
		t.Error("opening a file claims to change the machine, so a client would ask about it")
	}
	if action.NeedsElevation {
		t.Error("opening an editor asks for privileges")
	}
}

func TestEachOptionMapsToItsFile(t *testing.T) {
	cases := map[string]string{
		"-S": "/home/u/.config/Code/User/snippets",
		"-s": "/home/u/.config/Code/User/settings.json",
		"-k": "/home/u/.config/Code/User/keybindings.json",
	}
	for flag, want := range cases {
		r := &coretest.Recorder{}
		if err := Run(context.Background(), session("/home/u", "linux"), []string{flag}, io.Discard, r); err != nil {
			t.Errorf("%s: %v", flag, err)
			continue
		}
		req, _ := r.LastAction()
		if got := req.Actions[0].Argv[1]; got != want {
			t.Errorf("%s opened %q, want %q", flag, got, want)
		}
	}
}

func TestAnUnknownOptionIsRejected(t *testing.T) {
	err := Run(context.Background(), session("/home/u", "linux"), []string{"--bogus"}, io.Discard, &coretest.Recorder{})
	if err == nil {
		t.Fatal("an unknown option was accepted")
	}
}

func TestAFailedOpenIsReported(t *testing.T) {
	// The client runs the program, so it is the one that knows it failed. A command that
	// ignored that would report success for an editor that never opened.
	r := &coretest.Recorder{ActionOutcome: failureOutcome("code: not found")}
	err := Run(context.Background(), session("/home/u", "linux"), []string{"-s"}, io.Discard, r)
	if err == nil {
		t.Fatal("a failed open was reported as success")
	}
	if !strings.Contains(err.Error(), "code: not found") {
		t.Errorf("error = %q, want what the client said", err)
	}
}

func TestNoArgumentsExplainsItself(t *testing.T) {
	// It used to print usage and exit zero, and a script that calls it with no arguments
	// should not start failing.
	var out strings.Builder
	if err := Run(context.Background(), session("/home/u", "linux"), nil, &out, &coretest.Recorder{}); err != nil {
		t.Fatalf("Run with no arguments: %v", err)
	}
	if !strings.Contains(out.String(), "--settings") {
		t.Errorf("output = %q, want the options", out.String())
	}
}

func TestTheHandlerOpensThroughCore(t *testing.T) {
	r := &coretest.Recorder{}
	cmd := core.Command{Util: "vscode", Args: []string{"-s"}, Session: session("/home/u", runtime.GOOS)}
	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}
	if _, offered := r.LastAction(); !offered {
		t.Error("the handler did not ask the client to open anything")
	}
}

func failureOutcome(why string) *action.Outcome {
	return &action.Outcome{Failures: []string{why}}
}
