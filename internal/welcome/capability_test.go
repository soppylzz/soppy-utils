package welcome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// host describes a machine without needing one. It is what capability judging is
// built to be driven by.
type host struct {
	goos  string
	home  string
	shell string
	tools []string
	files []string
}

// fakeFile is a regular file as far as anything here can tell.
type fakeFile struct{}

func (fakeFile) Name() string       { return "file" }
func (fakeFile) Size() int64        { return 0 }
func (fakeFile) Mode() os.FileMode  { return 0o644 }
func (fakeFile) ModTime() time.Time { return time.Time{} }
func (fakeFile) IsDir() bool        { return false }
func (fakeFile) Sys() any           { return nil }

func (h host) asRegistryHost() registry.Host {
	return registry.Host{
		GOOS:  h.goos,
		Home:  h.home,
		Shell: h.shell,
		LookPath: func(name string) (string, error) {
			for _, tool := range h.tools {
				if tool == name {
					return "/usr/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		Stat: func(path string) (os.FileInfo, error) {
			for _, f := range h.files {
				if path == f {
					return fakeFile{}, nil
				}
			}
			return nil, os.ErrNotExist
		},
	}
}

func everything() host {
	home := "/home/u"
	return host{
		goos: "linux", home: home, shell: "zsh",
		tools: []string{"figlet", "fastfetch"},
		files: []string{
			filepath.Join(home, ".figlet", "fonts", "speed.flf"),
			filepath.Join(home, ".figlet", "fonts", "smslant.flf"),
		},
	}
}

func assess(t *testing.T, h host) registry.Assessment {
	t.Helper()
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	c, ok := r.Lookup("welcome")
	if !ok {
		t.Fatal("welcome did not register a capability")
	}
	return c.Judge(h.asRegistryHost())
}

func TestAWelcomingMachineIsAvailable(t *testing.T) {
	got := assess(t, everything())
	if got.State != registry.Available {
		t.Errorf("state = %q (%s: %s), want available", got.State, got.Reason, got.Detail)
	}
}

func TestMissingToolsDegradeRatherThanFail(t *testing.T) {
	// This is the state today's code already reaches, expressed as data instead of as a
	// sentence on stderr that a window client cannot do anything with.
	for _, tool := range []string{"figlet", "fastfetch"} {
		h := everything()
		kept := h.tools[:0]
		for _, have := range h.tools {
			if have != tool {
				kept = append(kept, have)
			}
		}
		h.tools = kept

		got := assess(t, h)
		if got.State != registry.Degraded {
			t.Errorf("without %s: state = %q, want degraded", tool, got.State)
		}
		if got.Reason != registry.MissingDependency {
			t.Errorf("without %s: reason = %q, want missing_dependency", tool, got.Reason)
		}
		if len(got.Missing) == 0 || got.Missing[0] != tool {
			t.Errorf("without %s: missing = %v, want it named", tool, got.Missing)
		}
		if got.Detail == "" {
			t.Errorf("without %s: no detail, so a client cannot say what is lost", tool)
		}
	}
}

func TestMissingFigletFontsDegradeToo(t *testing.T) {
	h := everything()
	h.files = h.files[:1]

	got := assess(t, h)
	if got.State != registry.Degraded {
		t.Errorf("state = %q, want degraded", got.State)
	}
	if !strings.Contains(got.Detail, "font") {
		t.Errorf("detail = %q, want it to name what is lost", got.Detail)
	}
}

func TestWindowsSaysNotImplementedRatherThanHidden(t *testing.T) {
	h := everything()
	h.goos = "windows"

	got := assess(t, h)
	if got.State != registry.Unavailable {
		t.Errorf("state = %q, want unavailable", got.State)
	}
	if got.Reason != registry.NotImplemented {
		t.Errorf("reason = %q, want not_implemented", got.Reason)
	}
	// Kept visible and disabled rather than absent: the entry point stays discoverable,
	// and the reason is a statement about this project rather than about the machine.
	if !strings.Contains(got.Detail, "Windows") {
		t.Errorf("detail = %q, want it to name the platform a user knows", got.Detail)
	}
}

func TestTheShellRunningNowDoesNotDecideAnything(t *testing.T) {
	// The banner hooks a bash or zsh startup file. That is a question about which shells
	// the user has, not about which one is running, and a fish user with zsh installed
	// can still use it. Judging by $SHELL would hide the capability from somebody who
	// can use it.
	for _, shell := range []string{"zsh", "bash", "fish", ""} {
		h := everything()
		h.shell = shell
		if got := assess(t, h); got.State != registry.Available {
			t.Errorf("shell %q: state = %q, want available", shell, got.State)
		}
	}
}

func TestTheCapabilityCarriesItsCommandSurface(t *testing.T) {
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	c, _ := r.Lookup("welcome")

	if c.Usage == "" {
		t.Error("no usage line, so help has to be written out a second time")
	}

	want := []string{"install", "edit", "migrate", "status", "uninstall"}
	if len(c.Commands) != len(want) {
		t.Fatalf("registered %d commands, want %d", len(c.Commands), len(want))
	}
	for i, name := range want {
		if c.Commands[i].Name != name {
			t.Errorf("Commands[%d] = %q, want %q", i, c.Commands[i].Name, name)
		}
	}

	// The flags are what a client validates against and what usage errors are written
	// from, so they have to be here rather than only in the argument parser.
	var install registry.Command
	for _, cmd := range c.Commands {
		if cmd.Name == "install" {
			install = cmd
		}
	}
	for _, flag := range []string{"--name", "--device", "--message", "--rc"} {
		found := false
		for _, f := range install.Flags {
			if f.Name == flag {
				found = true
			}
		}
		if !found {
			t.Errorf("install does not declare %s", flag)
		}
	}
}

func TestPlatformNamesAreWhatAUserWouldSay(t *testing.T) {
	cases := map[string]string{
		"darwin":  "macOS",
		"windows": "Windows",
		"linux":   "linux",
		"":        "this platform",
	}
	for goos, want := range cases {
		if got := platformName(goos); got != want {
			t.Errorf("platformName(%q) = %q, want %q", goos, got, want)
		}
	}
}
