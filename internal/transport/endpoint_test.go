package transport

import (
	"strings"
	"testing"
)

// The endpoint is derived rather than searched for, so that a client can never
// attach to somebody else's daemon by accident and two clients can never disagree
// about where core is. These tests pin the derivation.

func TestResolveUnixEndpoint(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		got, err := Resolve(goos, "/home/u", "")
		if err != nil {
			t.Fatalf("Resolve(%s): %v", goos, err)
		}
		if got.Network != "unix" {
			t.Errorf("%s: Network = %q, want unix", goos, got.Network)
		}
		want := "/home/u/.config/soppy-utils/core.sock"
		if got.Address != want {
			t.Errorf("%s: Address = %q, want %q", goos, got.Address, want)
		}
	}
}

func TestResolveWindowsEndpointUsesAPipe(t *testing.T) {
	got, err := Resolve("windows", `C:\Users\u`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Network != "pipe" {
		t.Errorf("Network = %q, want pipe", got.Network)
	}
	if len(got.Address) < len(`\\.\pipe\`) || got.Address[:len(`\\.\pipe\`)] != `\\.\pipe\` {
		t.Errorf("Address = %q, want a pipe under %s", got.Address, `\\.\pipe\`)
	}
}

func TestResolveWindowsEndpointIsStableForOneHome(t *testing.T) {
	// The client and the daemon derive the endpoint independently. If they could
	// disagree, a second client would start a second daemon.
	first, err := Resolve("windows", `C:\Users\u`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	second, err := Resolve("windows", `C:\Users\u`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if first.Address != second.Address {
		t.Errorf("two Resolve calls disagreed: %q vs %q", first.Address, second.Address)
	}
}

func TestResolveWindowsEndpointSeparatesHomes(t *testing.T) {
	// Two users on one machine must not share a pipe name. The security descriptor
	// gates access, but a shared name would make the second daemon fail to start.
	a, err := Resolve("windows", `C:\Users\a`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	b, err := Resolve("windows", `C:\Users\b`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Address == b.Address {
		t.Errorf("both homes resolved to %q, want distinct pipes", a.Address)
	}
}

func TestResolveHonoursTheEndpointOverride(t *testing.T) {
	// Tests and a development checkout need to point at their own daemon rather than
	// at whatever is installed.
	got, err := Resolve("linux", "/home/u", "/tmp/scratch/core.sock")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Address != "/tmp/scratch/core.sock" {
		t.Errorf("Address = %q, want the override", got.Address)
	}
	if got.Network != "unix" {
		t.Errorf("Network = %q, want unix", got.Network)
	}
}

func TestResolveRejectsAnEmptyHome(t *testing.T) {
	// An empty home means the session environment is broken. Deriving a path from it
	// would quietly produce something under the filesystem root.
	if _, err := Resolve("linux", "", ""); err == nil {
		t.Fatal("Resolve accepted an empty home, want error")
	}
}

func TestResolveRejectsARelativeHome(t *testing.T) {
	if _, err := Resolve("linux", "relative/home", ""); err == nil {
		t.Fatal("Resolve accepted a relative home, want error")
	}
}

func TestResolveRejectsAnUnknownPlatform(t *testing.T) {
	if _, err := Resolve("plan9", "/home/u", ""); err == nil {
		t.Fatal("Resolve accepted an unknown platform, want error")
	}
}

func TestResolveRejectsAUnixAddressThatCannotFit(t *testing.T) {
	// The kernel caps a Unix socket path at about 104 bytes, and bind fails with a
	// message that says nothing about which of the caller's choices was too long.
	// Failing here instead names the limit and the way out.
	deep := "/tmp/" + strings.Repeat("nested/", 20) + "home"
	if _, err := Resolve("linux", deep, ""); err == nil {
		t.Fatal("Resolve accepted a home whose socket path cannot fit, want an error")
	}
}

func TestResolveRejectsAnOverlongOverride(t *testing.T) {
	if _, err := Resolve("linux", "/tmp/u", "/tmp/"+strings.Repeat("x", 200)); err == nil {
		t.Fatal("Resolve accepted an over-long override, want an error")
	}
}

func TestAShortOverrideRescuesADeepHome(t *testing.T) {
	// The override is the way out, and it must stay one: a home too deep to hold a
	// socket is still usable when the endpoint is pointed somewhere shorter.
	deep := "/tmp/" + strings.Repeat("nested/", 20) + "home"
	got, err := Resolve("linux", deep, "/tmp/spu.sock")
	if err != nil {
		t.Fatalf("Resolve with an override: %v", err)
	}
	if got.Address != "/tmp/spu.sock" {
		t.Errorf("Address = %q, want the override", got.Address)
	}
}
