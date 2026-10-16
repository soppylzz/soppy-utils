package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

func TestEveryUtilIsBothRegisteredAndDispatchable(t *testing.T) {
	// The registry is what a client reads to find out what exists, and the dispatch is
	// what runs it. A util in one and not the other is either advertised and broken or
	// reachable and invisible, and both are the kind of thing that only shows up in
	// front of a user.
	r := registry.New()
	for _, u := range utils {
		if err := u.register(r); err != nil {
			t.Fatalf("register %s: %v", u.id, err)
		}
		if u.handler == nil {
			t.Errorf("%s is registered but has nothing to run it", u.id)
		}
	}

	for _, c := range r.List() {
		found := false
		for _, u := range utils {
			if u.id == c.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is advertised in the registry but not dispatchable", c.ID)
		}
	}
}

func TestEveryDispatchableUtilIsRegistered(t *testing.T) {
	r := registry.New()
	for _, u := range utils {
		if err := u.register(r); err != nil {
			t.Fatalf("register %s: %v", u.id, err)
		}
	}
	for _, u := range utils {
		if _, ok := r.Lookup(u.id); !ok {
			t.Errorf("%s has a handler but no capability, so no client can find it", u.id)
		}
	}
}

func TestMissingCommandIsReported(t *testing.T) {
	if _, err := run(nil, "test"); err == nil {
		t.Fatal("run accepted no command, want an error")
	}
}

func TestAnUnknownCommandIsReportedByName(t *testing.T) {
	if _, err := run([]string{"nonesuch"}, "test"); err == nil {
		t.Fatal("run accepted an unknown command, want an error")
	}
}

func TestTheCoreModeNeedsToSayWhichMode(t *testing.T) {
	if _, err := coreCommand(t.TempDir(), "test", nil); err == nil {
		t.Fatal("__core with no mode was accepted, want an error")
	}
}

func TestAnUnknownCoreModeIsReported(t *testing.T) {
	if _, err := coreCommand(t.TempDir(), "test", []string{"nonesuch"}); err == nil {
		t.Fatal("an unknown __core mode was accepted, want an error")
	}
}

func TestTheIdleTimeoutDefaultsToFifteenMinutes(t *testing.T) {
	t.Setenv("SPU_CORE_IDLE_TIMEOUT", "")
	if got := idleTimeout(); got != 15*time.Minute {
		t.Errorf("idle timeout = %s, want 15m", got)
	}
}

func TestTheIdleTimeoutIsOverridable(t *testing.T) {
	// The test suite starts a daemon per spec and cannot wait fifteen minutes for each.
	t.Setenv("SPU_CORE_IDLE_TIMEOUT", "2s")
	if got := idleTimeout(); got != 2*time.Second {
		t.Errorf("idle timeout = %s, want 2s", got)
	}
}

func TestAnUnusableIdleTimeoutFallsBack(t *testing.T) {
	// A daemon that refused to start over a malformed variable would be worse than one
	// that lives longer than asked.
	for _, raw := range []string{"nonsense", "-1s", "0"} {
		t.Setenv("SPU_CORE_IDLE_TIMEOUT", raw)
		if got := idleTimeout(); got != 15*time.Minute {
			t.Errorf("idle timeout for %q = %s, want the default", raw, got)
		}
	}
}

// The daemon outlives the client, so anything the client knows about where it was run has
// to be sent rather than looked up later. This is the wiring that does it: a command that
// generates files into a project is given the directory the user was standing in, and on
// Windows the spelling the native daemon can open.
func TestTheClientReportsTheDirectoryItWasRunFrom(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("SPU_WORKDIR_NATIVE", `C:\Users\someone\project`)

	env := envFor()

	got, err := filepath.EvalSymlinks(env.WorkingDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", env.WorkingDir, err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}
	if got != want {
		t.Errorf("WorkingDir = %q, want the directory the client was run from (%q)", env.WorkingDir, dir)
	}
	if env.WorkingDirNative != `C:\Users\someone\project` {
		t.Errorf("WorkingDirNative = %q, want the value the launcher exported", env.WorkingDirNative)
	}
	if got := env.WorkingDirForFilesystem(); got != `C:\Users\someone\project` {
		t.Errorf("WorkingDirForFilesystem = %q, want the native spelling when there is one", got)
	}
}
