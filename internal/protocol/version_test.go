package protocol

import "testing"

// Bumping the version is a compatibility event, not a refactor: a client and a core
// that do not overlap refuse each other, and during an upgrade that means one of them
// restarts. This test exists so the change cannot happen by accident.
func TestProtocolVersionIsPinned(t *testing.T) {
	if Version != 1 {
		t.Errorf("Version = %d, want 1; changing it changes the wire contract", Version)
	}
}
