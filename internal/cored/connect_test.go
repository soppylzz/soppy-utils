package cored

import (
	"context"
	"encoding/json"
	"runtime"

	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

const fakeCoreEnv = "SPU_TEST_FAKE_CORE"

// fakeCore is a core that speaks a protocol version this client does not, and stops
// when asked. It exists so the version-skew path can be driven from the wire rather
// than from a stub of our own code.
func fakeCore(ep transport.Endpoint, protocolVersion int) (*transport.Listener, error) {
	l, err := transport.Listen(ep)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := protocol.NewReader(conn)
				w := protocol.NewWriter(conn)
				for {
					msg, err := r.Read()
					if err != nil {
						return
					}
					if msg.Kind() != protocol.KindRequest {
						continue
					}
					if msg.Method == "server/shutdown" {
						_ = w.WriteMessage(protocol.NewResult(*msg.ID, nil))
						l.Close()
						return
					}
					_ = w.WriteMessage(protocol.NewErrorResponse(*msg.ID,
						protocol.CodeProtocolMismatch,
						"this core speaks protocol "+strconv.Itoa(protocolVersion)))
				}
			}()
		}
	}()
	return l, nil
}

// TestFakeCoreHelper is the program that runs when the test binary is spawned as a
// core that speaks the wrong version.
func TestFakeCoreHelper(t *testing.T) {
	home := os.Getenv(fakeCoreEnv)
	if home == "" {
		t.Skip("not the spawned fake core")
	}
	version, err := strconv.Atoi(os.Getenv("SPU_TEST_FAKE_PROTOCOL"))
	if err != nil {
		t.Fatalf("SPU_TEST_FAKE_PROTOCOL: %v", err)
	}
	ep, err := transport.Resolve(runtime.GOOS, home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := fakeCore(ep, version); err != nil {
		t.Fatalf("fakeCore: %v", err)
	}
	select {}
}

func spawnAsFakeCore(t *testing.T, home string, version int) (argv, env []string) {
	t.Helper()
	argv = []string{os.Args[0], "-test.run=^TestFakeCoreHelper$"}
	env = append(os.Environ(),
		fakeCoreEnv+"="+home,
		"SPU_TEST_FAKE_PROTOCOL="+strconv.Itoa(version))
	return argv, env
}

func endpointsFor(t *testing.T) (home string, ep transport.Endpoint) {
	t.Helper()
	home = shortDir(t)
	ep, err := transport.Resolve(runtime.GOOS, home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return home, ep
}

func TestConnectNegotiatesWithARunningCore(t *testing.T) {
	h := newHarness(t, Options{Version: "0.3.0"})

	conn, err := Connect(context.Background(), AttachOptions{
		Endpoint: h.ep,
		Budget:   time.Second,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if conn.Info.Version != "0.3.0" {
		t.Errorf("core version = %q, want 0.3.0", conn.Info.Version)
	}
	if conn.Info.ProtocolVersion != protocol.Version {
		t.Errorf("protocol version = %d, want %d", conn.Info.ProtocolVersion, protocol.Version)
	}
}

func TestConnectRestartsACoreThatSpeaksAnotherVersion(t *testing.T) {
	home, ep := endpointsFor(t)

	old, err := fakeCore(ep, 99)
	if err != nil {
		t.Fatalf("fakeCore: %v", err)
	}
	defer old.Close()

	// The replacement is a real daemon, which is what an upgrade leaves installed.
	argv, env := spawnAsDaemon(t, home, filepath.Join(shortDir(t), "report.json"))

	conn, err := Connect(context.Background(), AttachOptions{
		Endpoint: ep,
		Spawn:    argv,
		SpawnEnv: env,
		LogPath:  filepath.Join(shortDir(t), "core.log"),
		Budget:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	// The point of the restart is that the client ends up talking to the build that is
	// installed now, not to the one that was running when it started.
	if conn.Info.Version != "helper" {
		t.Errorf("core version = %q, want helper: the client kept talking to the old core",
			conn.Info.Version)
	}
}

func TestConnectGivesUpWhenTheReplacementAlsoMismatches(t *testing.T) {
	home, ep := endpointsFor(t)

	old, err := fakeCore(ep, 99)
	if err != nil {
		t.Fatalf("fakeCore: %v", err)
	}
	defer old.Close()

	argv, env := spawnAsFakeCore(t, home, 98)

	// One restart, not a loop. A replacement that also disagrees means the installed
	// binary is not what the client expected, and restarting it again cannot fix that.
	started := time.Now()
	_, err = Connect(context.Background(), AttachOptions{
		Endpoint: ep,
		Spawn:    argv,
		SpawnEnv: env,
		LogPath:  filepath.Join(shortDir(t), "core.log"),
		Budget:   5 * time.Second,
	})
	if err == nil {
		t.Fatal("Connect succeeded against a core that still disagrees, want an error")
	}
	if elapsed := time.Since(started); elapsed > 15*time.Second {
		t.Errorf("Connect took %s, want a bounded number of attempts", elapsed)
	}
}

func TestConnectReportsAMalformedHandshakeReply(t *testing.T) {
	_, ep := endpointsFor(t)

	// A core that answers with the right shape but a broken body: the client must not
	// treat it as "no shared version" and start restarting things over a parse error.
	l, err := transport.Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := protocol.NewReader(conn)
		w := protocol.NewWriter(conn)
		for {
			msg, err := r.Read()
			if err != nil {
				return
			}
			if msg.Kind() == protocol.KindRequest {
				_ = w.WriteMessage(protocol.NewResult(*msg.ID, json.RawMessage(`"not an object"`)))
			}
		}
	}()

	if _, err := Connect(context.Background(), AttachOptions{Endpoint: ep, Budget: time.Second}); err == nil {
		t.Fatal("Connect accepted a malformed handshake reply, want an error")
	}
}
