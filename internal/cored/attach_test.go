package cored

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// The test binary re-executes itself as the daemon, so the spawn path is exercised
// across a real process boundary without needing an installed spu.
const (
	helperHomeEnv   = "SPU_TEST_DAEMON_HOME"
	helperReportEnv = "SPU_TEST_REPORT"
)

// TestSpawnedDaemonHelper is not a test of anything. It is the program that runs when
// the test binary is spawned as a daemon.
func TestSpawnedDaemonHelper(t *testing.T) {
	home := os.Getenv(helperHomeEnv)
	if home == "" {
		t.Skip("not the spawned daemon")
	}

	if report := os.Getenv(helperReportEnv); report != "" {
		// Reported before anything else, so the test can ask what the daemon's process
		// looked like without racing its startup.
		body, err := json.Marshal(map[string]any{
			"stdin_tty":  term.IsTerminal(os.Stdin.Fd()),
			"stdout_tty": term.IsTerminal(os.Stdout.Fd()),
			"stderr_tty": term.IsTerminal(os.Stderr.Fd()),
			"ppid":       os.Getppid(),
		})
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		if err := os.WriteFile(report, body, 0o600); err != nil {
			t.Fatalf("write report: %v", err)
		}
	}

	ep, err := transport.Resolve("linux", home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	d, err := New(Options{
		Endpoint:    ep,
		Version:     "helper",
		Handler:     nopHandler{},
		IdleTimeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Runs until the test asks it to stop through the protocol, which is also how the
	// real updater stops a daemon.
	if err := d.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}
}

// spawnAsDaemon is the argv and environment that make the test binary behave as core.
func spawnAsDaemon(t *testing.T, home, report string) (argv, env []string) {
	t.Helper()
	argv = []string{os.Args[0], "-test.run=^TestSpawnedDaemonHelper$"}
	env = append(os.Environ(), helperHomeEnv+"="+home, helperReportEnv+"="+report)
	return argv, env
}

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "spu")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// attachForTest fails with the daemon's log rather than only its absence, because the
// reason a spawned daemon never answered is in that file.
func attachForTest(t *testing.T, opts AttachOptions) net.Conn {
	t.Helper()
	conn, err := Attach(context.Background(), opts)
	if err != nil {
		if body, readErr := os.ReadFile(opts.LogPath); readErr == nil {
			t.Fatalf("Attach: %v\ndaemon log:\n%s", err, body)
		}
		t.Fatalf("Attach: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func waitForFile(t *testing.T, path string, within time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if body, err := os.ReadFile(path); err == nil {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared within %s", path, within)
	return nil
}

func TestAttachUsesADaemonThatIsAlreadyRunning(t *testing.T) {
	h := newHarness(t, Options{})
	report := filepath.Join(shortDir(t), "report.json")
	argv, env := spawnAsDaemon(t, shortDir(t), report)

	_ = attachForTest(t, AttachOptions{
		Endpoint: h.ep,
		Spawn:    argv,
		SpawnEnv: env,
		LogPath:  filepath.Join(shortDir(t), "core.log"),
		Budget:   2 * time.Second,
	})

	// A second daemon would fight the first over the same state, and the spawn would
	// have left its report behind.
	if _, err := os.Stat(report); err == nil {
		t.Error("Attach started a daemon even though one was already listening")
	}
}

func TestAttachStartsADaemonWhenNoneIsRunning(t *testing.T) {
	home := shortDir(t)
	ep, err := transport.Resolve("linux", home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	report := filepath.Join(shortDir(t), "report.json")
	argv, env := spawnAsDaemon(t, home, report)

	conn := attachForTest(t, AttachOptions{
		Endpoint: ep,
		Spawn:    argv,
		SpawnEnv: env,
		LogPath:  filepath.Join(shortDir(t), "core.log"),
		Budget:   10 * time.Second,
	})

	// The connection is to a real daemon, not to something that merely accepted a
	// socket: it has to answer the handshake.
	w := protocol.NewWriter(conn)
	if err := w.WriteMessage(protocol.NewRequest(1, "initialize", []byte(`{"client":{"protocol_versions":[1],"env":{"os":"linux"}}}`))); err != nil {
		t.Fatalf("write initialize: %v", err)
	}
	reply, err := protocol.NewReader(conn).Read()
	if err != nil {
		t.Fatalf("read initialize reply: %v", err)
	}
	if reply.Error != nil {
		t.Fatalf("initialize failed: %+v", reply.Error)
	}

	// The daemon must not have inherited the client's terminal. It outlives the client
	// that started it, and anything it wrote to an inherited stdout would appear in
	// that user's shell long after the command returned.
	var got struct {
		StdinTTY  bool `json:"stdin_tty"`
		StdoutTTY bool `json:"stdout_tty"`
		StderrTTY bool `json:"stderr_tty"`
	}
	if err := json.Unmarshal(waitForFile(t, report, 5*time.Second), &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if got.StdinTTY || got.StdoutTTY || got.StderrTTY {
		t.Errorf("the spawned daemon inherited a terminal: %+v", got)
	}

	// Stop it through the protocol, which is how the real updater stops a daemon.
	if err := w.WriteMessage(protocol.NewRequest(2, "server/shutdown", []byte(`{}`))); err != nil {
		t.Fatalf("write shutdown: %v", err)
	}
}

func TestAttachGivesUpWhenTheSpawnedDaemonNeverAnswers(t *testing.T) {
	home := shortDir(t)
	ep, err := transport.Resolve("linux", home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	started := time.Now()
	// A command that runs and exits without ever listening. Waiting forever here would
	// hang every spu command, so the budget is what turns it into a diagnosable error.
	_, err = Attach(context.Background(), AttachOptions{
		Endpoint: ep,
		Spawn:    []string{os.Args[0], "-test.run=^$"},
		LogPath:  filepath.Join(shortDir(t), "core.log"),
		Budget:   300 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Attach succeeded with no daemon, want an error")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("Attach took %s to give up, want the budget to bound it", elapsed)
	}
}

func TestAttachReportsThatNoDaemonCouldBeStarted(t *testing.T) {
	home := shortDir(t)
	ep, err := transport.Resolve("linux", home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// No spawn command at all: a caller that must not start processes has to be able
	// to say so, and must get an error rather than a hang.
	_, err = Attach(context.Background(), AttachOptions{
		Endpoint: ep,
		Budget:   time.Second,
	})
	if err == nil {
		t.Fatal("Attach succeeded with nothing to attach to and nothing to start, want an error")
	}
}

func TestAttachCreatesTheDirectoryTheDaemonLogNeeds(t *testing.T) {
	// A machine where nothing has been installed has no configuration directory yet,
	// and the daemon's log is the first thing that wants one. Failing here would mean
	// spu could not run at all until something else had created the directory.
	home := shortDir(t)
	ep, err := transport.Resolve("linux", home, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	argv, env := spawnAsDaemon(t, home, filepath.Join(shortDir(t), "report.json"))

	conn := attachForTest(t, AttachOptions{
		Endpoint: ep,
		Spawn:    argv,
		SpawnEnv: env,
		LogPath:  filepath.Join(home, ".config", "soppy-utils", "core.log"),
		Budget:   10 * time.Second,
	})
	defer conn.Close()

	if _, err := os.Stat(filepath.Join(home, ".config", "soppy-utils", "core.log")); err != nil {
		t.Errorf("no log was written: %v", err)
	}
}
