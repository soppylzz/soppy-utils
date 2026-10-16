package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updater is a stand-in for the packaged script: it records what it was given and exits
// with a status the test chooses.
func updater(t *testing.T, exitWith int) (root, record string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	record = filepath.Join(root, "argv")

	body := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >" + record + "\nexit " + itoa(exitWith) + "\n"
	if err := os.WriteFile(filepath.Join(root, "scripts", "update.sh"), []byte(body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("SPU_ROOT", root)
	return root, record
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	return string(rune('0' + n))
}

func TestUpdateRunsThePackagedScript(t *testing.T) {
	// update is the one command the Go side does not implement, because what it does is
	// replace the installation this binary runs from. What it must do is be reachable
	// however spu was invoked: a binary called directly used to answer "unknown util".
	_, record := updater(t, 0)

	code, err := runUpdate([]string{"--status"})
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}

	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the script did not run: %v", err)
	}
	if strings.TrimSpace(string(got)) != "--status" {
		t.Errorf("the script was given %q, want the arguments passed through", got)
	}
}

func TestUpdateReportsTheScriptsExitStatus(t *testing.T) {
	// The script writes its own diagnostics, so a failure is reported once rather than
	// twice: its exit status is the answer.
	updater(t, 3)

	code, err := runUpdate(nil)
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if code != 3 {
		t.Errorf("exit = %d, want the script's 3", code)
	}
}

func TestUpdateSaysSoWhenThereIsNoUpdater(t *testing.T) {
	// A binary with nothing beside it cannot upgrade itself, and saying that beats
	// failing at something the user cannot see.
	t.Setenv("SPU_ROOT", t.TempDir())

	code, err := runUpdate(nil)
	if err == nil {
		t.Fatal("runUpdate succeeded with no updater, want an error")
	}
	if code == 0 {
		t.Errorf("exit = %d, want non-zero", code)
	}
	if !strings.Contains(err.Error(), "updater") {
		t.Errorf("error = %q, want it to say what is missing", err)
	}
}

func TestTheInstallRootPrefersWhatTheLauncherSays(t *testing.T) {
	// A checkout runs the binary from a temporary build directory and only the launcher
	// knows where the source tree is.
	t.Setenv("SPU_ROOT", "/somewhere/installed")
	if got := installRoot(); got != "/somewhere/installed" {
		t.Errorf("installRoot = %q, want what the launcher said", got)
	}
}

func TestTheInstallRootFallsBackToTheBinarysOwnPath(t *testing.T) {
	// libexec/spu sits one level under the root, which is all an installed binary needs.
	t.Setenv("SPU_ROOT", "")

	got := installRoot()
	if got == "" {
		t.Fatal("installRoot found nothing")
	}
	if filepath.Base(got) == "libexec" {
		t.Errorf("installRoot = %q, want the directory above libexec", got)
	}
}
