package welcome

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
)

func allPresent() optionalTools {
	return optionalTools{figlet: true, speedFont: true, slantFont: true, fastfetch: true}
}

func argvOf(t *testing.T, req action.Request) []string {
	t.Helper()
	if len(req.Actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(req.Actions))
	}
	return req.Actions[0].Argv
}

func TestADarwinMachineIsOfferedHomebrew(t *testing.T) {
	d := allPresent()
	d.figlet = false

	req := dependencyActions("darwin", d)
	got := strings.Join(argvOf(t, req), " ")
	if got != "brew install figlet" {
		t.Errorf("action = %q, want brew install figlet", got)
	}
	if req.Actions[0].NeedsElevation {
		t.Error("homebrew does not need privileges, and saying it does would make the client ask for them")
	}
	if !req.Actions[0].Optional {
		t.Error("the action is not optional, so refusing it would fail the install")
	}
	if !strings.Contains(req.Reason, "figlet") {
		t.Errorf("reason = %q, want it to name what is missing", req.Reason)
	}
}

func TestBothPackagesAreInstalledInOneGo(t *testing.T) {
	d := allPresent()
	d.figlet, d.fastfetch = false, false

	got := strings.Join(argvOf(t, dependencyActions("darwin", d)), " ")
	if got != "brew install figlet fastfetch" {
		t.Errorf("action = %q, want one command for both", got)
	}
}

func TestALinuxMachineIsOfferedAptWithElevation(t *testing.T) {
	d := allPresent()
	d.figlet = false

	req := dependencyActions("linux", d)
	got := strings.Join(argvOf(t, req), " ")
	if got != "apt-get install -y figlet" {
		t.Errorf("action = %q, want apt-get", got)
	}
	// Core says privileges are needed and stops there; how to obtain them is the
	// client's business, which is why no sudo appears in this list.
	if !req.Actions[0].NeedsElevation {
		t.Error("apt needs privileges and the client was not told")
	}
	if strings.Contains(got, "sudo") {
		t.Errorf("action = %q, want core to state the requirement rather than the mechanism", got)
	}
}

func TestFastfetchIsNotOfferedOnLinux(t *testing.T) {
	// Whether the Ubuntu archive carries fastfetch has not been established, and
	// offering a command that fails would be worse than saying nothing.
	d := allPresent()
	d.fastfetch = false

	req := dependencyActions("linux", d)
	if len(req.Actions) != 0 {
		t.Errorf("offered %v, want nothing: its availability there is unconfirmed", req.Actions)
	}
	if !strings.Contains(req.Reason, "fastfetch") {
		t.Errorf("reason = %q, want it to name what is missing even with nothing to offer",
			req.Reason)
	}
}

func TestNothingMissingIsNothingToDo(t *testing.T) {
	if req := dependencyActions("darwin", allPresent()); len(req.Actions) != 0 || req.Reason != "" {
		t.Errorf("request = %+v, want nothing", req)
	}
}

func TestAnUnknownPlatformIsOfferedNothing(t *testing.T) {
	d := allPresent()
	d.figlet = false

	req := dependencyActions("plan9", d)
	if len(req.Actions) != 0 {
		t.Errorf("offered %v on a platform with no known package manager", req.Actions)
	}
}

func TestTheFontsAreNamedEvenThoughNothingInstallsThem(t *testing.T) {
	// They are files rather than packages and no source has been settled on, so the
	// most that can be said is that they are missing.
	d := allPresent()
	d.slantFont = false

	req := dependencyActions("darwin", d)
	if len(req.Actions) != 0 {
		t.Errorf("offered %v, but nothing installs the fonts", req.Actions)
	}
	if !strings.Contains(req.Reason, "font") {
		t.Errorf("reason = %q, want the fonts named", req.Reason)
	}
}

func TestInstallOffersWhatIsMissing(t *testing.T) {
	// Nothing is installed on this machine as far as the process can tell.
	t.Setenv("PATH", t.TempDir())

	home := t.TempDir()
	r := &recorder{}
	args := []string{"install", "--name", "Alice", "--device", "Mac",
		"--message", "Ready", "--rc", "none"}

	if err := Run(context.Background(), core.Session{Home: home, GOOS: runtime.GOOS},
		args, io.Discard, r); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if r.actions == nil {
		t.Fatal("nothing was offered although both tools are missing")
	}
	if !strings.Contains(r.actions.Reason, "figlet") {
		t.Errorf("reason = %q, want it to say what is missing", r.actions.Reason)
	}

	// The install itself still happened. A dependency that cannot be installed leaves a
	// util degraded, which is a legitimate state rather than a failure, and the client
	// refusing the offer must not undo the work.
	if _, err := os.Stat(scriptPath(Store(home))); err != nil {
		t.Errorf("the script was not generated: %v", err)
	}
}
