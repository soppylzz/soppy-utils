package protocol

import "testing"

// Bumping the version is a compatibility event, not a refactor: a client and a core
// that do not overlap refuse each other, and during an upgrade that means one of them
// restarts. This test exists so the change cannot happen by accident.
//
// 1 was the first version. 2 added the client's working directory to the handshake,
// which an older core drops without saying so.
func TestProtocolVersionIsPinned(t *testing.T) {
	if Version != 2 {
		t.Errorf("Version = %d, want 2; changing it changes the wire contract", Version)
	}
}
