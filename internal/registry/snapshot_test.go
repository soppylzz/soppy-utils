package registry

import (
	"errors"
	"os"
	"testing"
)

func unixHost(path string) Host {
	return Host{
		GOOS: "linux", Home: "/home/u", Shell: "bash",
		LookPath: func(name string) (string, error) {
			if path == "" {
				return "", errors.New("not found")
			}
			return path, nil
		},
		Stat: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	}
}

func TestSnapshotEvaluatesEachCapabilityAgainstTheHost(t *testing.T) {
	r := New()
	if err := r.Register(Capability{
		ID: "welcome", Title: "Welcome banner",
		Judge: func(h Host) Assessment {
			if h.GOOS == "windows" {
				return Assessment{State: Unavailable, Reason: NotImplemented,
					Detail: "welcome is not implemented for Git Bash"}
			}
			return Assessment{State: Available}
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	snap := r.Snapshot(unixHost("/usr/bin/figlet"))
	if len(snap.Capabilities) != 1 {
		t.Fatalf("snapshot has %d capabilities, want 1", len(snap.Capabilities))
	}
	if got := snap.Capabilities[0].State; got != Available {
		t.Errorf("state = %q, want %q", got, Available)
	}

	// The same registry, a different session.
	onWindows := r.Snapshot(Host{GOOS: "windows", Home: `C:\Users\u`})
	if got := onWindows.Capabilities[0].State; got != Unavailable {
		t.Errorf("state on windows = %q, want %q", got, Unavailable)
	}
	if got := onWindows.Capabilities[0].Reason; got != NotImplemented {
		t.Errorf("reason = %q, want %q", got, NotImplemented)
	}
	if onWindows.Capabilities[0].Detail == "" {
		// A client renders an unavailable entry with its reason, so a missing detail is
		// a disabled menu item with nothing to tell the user.
		t.Error("unavailable capability carries no detail")
	}
}

func TestASessionWithADifferentPathGetsADifferentAnswer(t *testing.T) {
	// This is the reason evaluation is per session rather than per daemon. A daemon
	// started from an interactive zsh would otherwise answer a Git Bash client using
	// the zsh user's PATH, and every capability that depends on a command would be
	// wrong for one of them.
	r := New()
	if err := r.Register(Capability{
		ID: "welcome",
		Judge: func(h Host) Assessment {
			if _, err := h.LookPath("figlet"); err != nil {
				return Assessment{State: Degraded, Reason: MissingDependency,
					Missing: []string{"figlet"}, Detail: "the banner will be plain text"}
			}
			return Assessment{State: Available}
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	withFiglet := r.Snapshot(unixHost("/opt/homebrew/bin/figlet")).Capabilities[0]
	withoutFiglet := r.Snapshot(unixHost("")).Capabilities[0]

	if withFiglet.State != Available {
		t.Errorf("with figlet: state = %q, want %q", withFiglet.State, Available)
	}
	if withoutFiglet.State != Degraded {
		t.Errorf("without figlet: state = %q, want %q", withoutFiglet.State, Degraded)
	}
	if len(withoutFiglet.Missing) != 1 || withoutFiglet.Missing[0] != "figlet" {
		t.Errorf("missing = %v, want [figlet]", withoutFiglet.Missing)
	}
}

func TestACapabilityWithNoJudgeIsAvailable(t *testing.T) {
	// A capability that depends on nothing is available wherever core runs.
	r := New()
	if err := r.Register(Capability{ID: "license", Title: "License"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got := r.Snapshot(Host{GOOS: "linux"}).Capabilities[0].State; got != Available {
		t.Errorf("state = %q, want %q", got, Available)
	}
}

func TestSnapshotCarriesTheCommandSurface(t *testing.T) {
	// Help, usage errors, and unknown-command errors all render from this, so a client
	// never needs a second hand-written table of what exists.
	r := New()
	if err := r.Register(Capability{
		ID: "welcome", Title: "Welcome banner",
		Usage: "spu welcome [install|edit|migrate|status|uninstall] [flags]",
		Commands: []Command{
			{Name: "install", Summary: "Install or refresh the banner", Flags: []Flag{
				{Name: "--name", Arg: "string", Summary: "Banner name"},
				{Name: "--rc", Arg: "enum", Summary: "Shell hooks",
					Options: []string{"none", "bash", "zsh", "both"}},
			}},
			{Name: "status", Summary: "Report configuration and artifact state"},
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got := r.Snapshot(Host{GOOS: "linux"}).Capabilities[0]
	if got.Usage == "" {
		t.Error("snapshot carries no usage line")
	}
	if len(got.Commands) != 2 {
		t.Fatalf("snapshot carries %d commands, want 2", len(got.Commands))
	}
	if got.Commands[0].Name != "install" {
		t.Errorf("first command = %q, want install", got.Commands[0].Name)
	}
	if len(got.Commands[0].Flags) != 2 {
		t.Fatalf("install carries %d flags, want 2", len(got.Commands[0].Flags))
	}
	if opts := got.Commands[0].Flags[1].Options; len(opts) != 4 || opts[3] != "both" {
		t.Errorf("--rc options = %v, want the four shell targets", opts)
	}
}

func TestASnapshotIsIndependentOfTheRegistry(t *testing.T) {
	// Snapshot hands out a copy. A long-lived daemon that let a caller reach back into
	// its own capability data through a returned slice would be one mutation away from
	// reporting something it does not contain.
	r := New()
	if err := r.Register(Capability{
		ID: "welcome",
		Judge: func(Host) Assessment {
			return Assessment{State: Degraded, Missing: []string{"figlet"}}
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	first := r.Snapshot(Host{GOOS: "linux"})
	first.Capabilities[0].Missing[0] = "tampered"

	second := r.Snapshot(Host{GOOS: "linux"})
	if second.Capabilities[0].Missing[0] != "figlet" {
		t.Errorf("a later snapshot saw %q, want figlet", second.Capabilities[0].Missing[0])
	}
}
