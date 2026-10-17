package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/action"
)

func TestCommandForUsesArgvAsGiven(t *testing.T) {
	name, args, err := commandFor(action.Action{
		Kind: action.Run, Argv: []string{"brew", "install", "figlet"},
	})
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	// The program and its arguments stay separate. Joining them into a string and
	// handing that to a shell is how an argument becomes an injection.
	if name != "brew" || strings.Join(args, " ") != "install figlet" {
		t.Errorf("got %q %v, want brew [install figlet]", name, args)
	}
}

func TestCommandForAsksForElevationItsOwnWay(t *testing.T) {
	name, args, err := commandFor(action.Action{
		Kind: action.Run, Argv: []string{"apt-get", "install", "-y", "figlet"},
		NeedsElevation: true,
	})
	if err != nil {
		t.Fatalf("commandFor: %v", err)
	}
	// Core said privileges are needed and stopped there. How to obtain them is this
	// side's business, which is why sudo appears here and nowhere in core.
	if name != "sudo" {
		t.Errorf("program = %q, want sudo", name)
	}
	if strings.Join(args, " ") != "apt-get install -y figlet" {
		t.Errorf("args = %v, want the original argv after sudo", args)
	}
}

func TestCommandForRejectsAnActionWithNoProgram(t *testing.T) {
	if _, _, err := commandFor(action.Action{Kind: action.Run}); err == nil {
		t.Fatal("commandFor accepted an action with nothing to run")
	}
}

func TestPerformingARunReportsSuccess(t *testing.T) {
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out,
		action.Request{Actions: []action.Action{{Kind: action.Run, Argv: []string{"true"}}}},
		nil)

	if !outcome.Complete() {
		t.Errorf("outcome = %+v, want complete", outcome)
	}
	if outcome.Performed != 1 {
		t.Errorf("performed = %d, want 1", outcome.Performed)
	}
}

func TestPerformingAFailingRunSaysWhy(t *testing.T) {
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out,
		action.Request{Actions: []action.Action{{Kind: action.Run, Argv: []string{"false"}}}},
		nil)

	if outcome.Complete() {
		t.Error("outcome is complete although the program failed")
	}
	if len(outcome.Failures) != 1 {
		t.Fatalf("failures = %v, want one line", outcome.Failures)
	}
}

func TestPerformingAMissingProgramSaysSo(t *testing.T) {
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out,
		action.Request{Actions: []action.Action{
			{Kind: action.Run, Argv: []string{"spu-no-such-program-anywhere"}},
		}}, nil)

	if len(outcome.Failures) != 1 || !strings.Contains(outcome.Failures[0], "spu-no-such-program-anywhere") {
		t.Errorf("failures = %v, want the missing program named", outcome.Failures)
	}
}

func TestFetchWritesTheFileWhenTheDigestMatches(t *testing.T) {
	body := []byte("speed font bytes")
	sum := sha256.Sum256(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "speed.flf")
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out, action.Request{Actions: []action.Action{{
		Kind: action.Fetch, URL: server.URL, Dest: dest,
		SHA256: hex.EncodeToString(sum[:]),
	}}}, nil)

	if !outcome.Complete() {
		t.Fatalf("outcome = %+v, want complete", outcome)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("wrote %q, want %q", got, body)
	}
}

func TestFetchRefusesAFileWhoseDigestDoesNotMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("something else entirely"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "speed.flf")
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out, action.Request{Actions: []action.Action{{
		Kind: action.Fetch, URL: server.URL, Dest: dest,
		SHA256: strings.Repeat("0", 64),
	}}}, nil)

	if outcome.Complete() {
		t.Error("outcome is complete although the download did not match its digest")
	}
	// Leaving the file behind would mean the next run finds it and believes it.
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("a file that failed its digest was left at %s", dest)
	}
}

func TestFetchRequiresADigest(t *testing.T) {
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out, action.Request{Actions: []action.Action{{
		Kind: action.Fetch, URL: "https://example.invalid/x", Dest: filepath.Join(t.TempDir(), "x"),
	}}}, nil)

	if outcome.Complete() {
		t.Error("a download with no digest was treated as done")
	}
}

func TestANonSuccessStatusIsNotADownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "x")
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out, action.Request{Actions: []action.Action{{
		Kind: action.Fetch, URL: server.URL, Dest: dest, SHA256: strings.Repeat("0", 64),
	}}}, nil)

	if outcome.Complete() {
		t.Error("a 404 was treated as a successful download")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("an error response left a file at %s", dest)
	}
}

func TestPerformingNothingIsComplete(t *testing.T) {
	out := &strings.Builder{}
	if outcome := performActions(context.Background(), strings.NewReader(""), out, action.Request{}, nil); !outcome.Complete() {
		t.Errorf("outcome = %+v, want complete", outcome)
	}
}

func TestFetchRefusesAnOversizedDownload(t *testing.T) {
	// A client that read whatever a server chose to send into memory would be one
	// redirect away from being unable to run.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 1<<20)
		for range 9 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "big")
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out,
		action.Request{Actions: []action.Action{{
			Kind: action.Fetch, URL: server.URL, Dest: dest, SHA256: strings.Repeat("0", 64),
		}}}, nil)

	if outcome.Complete() {
		t.Error("an oversized download was treated as done")
	}
	if len(outcome.Failures) != 1 || !strings.Contains(outcome.Failures[0], "limit") {
		t.Errorf("failures = %v, want the size limit named", outcome.Failures)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("an oversized download left a file at %s", dest)
	}
}

func TestOnlyActionsThatChangeTheMachineAreApproved(t *testing.T) {
	// Asking before opening a file in the editor would be a question nobody wants, and
	// not asking before installing a package would be a question somebody does.
	asked := false
	prompt := func() bool { asked = true; return true }

	opening := action.Request{Actions: []action.Action{
		{Kind: action.Run, Argv: []string{"true"}},
	}}
	if outcome := performActions(context.Background(), strings.NewReader(""), io.Discard, opening, prompt); !outcome.Complete() {
		t.Errorf("outcome = %+v, want the action performed", outcome)
	}
	if asked {
		t.Error("the client asked before doing something that changes nothing")
	}

	asked = false
	installing := action.Request{Actions: []action.Action{
		{Kind: action.Run, Argv: []string{"true"}, ChangesMachine: true},
	}}
	if outcome := performActions(context.Background(), strings.NewReader(""), io.Discard, installing, prompt); !outcome.Complete() {
		t.Errorf("outcome = %+v, want the action performed", outcome)
	}
	if !asked {
		t.Error("the client installed something without asking")
	}
}

func TestWorkThatChangesTheMachineIsNotDoneWithoutBeingAsked(t *testing.T) {
	// A scripted run has nobody to ask. Installing a package without an answer is not
	// the safe default, so it is declined and the command carries on degraded.
	out := &strings.Builder{}
	outcome := performActions(context.Background(), strings.NewReader(""), out,
		action.Request{Actions: []action.Action{
			{Kind: action.Run, Argv: []string{"true"}, ChangesMachine: true},
		}}, nil)

	if !outcome.Declined {
		t.Errorf("outcome = %+v, want it declined", outcome)
	}
	if outcome.Performed != 0 {
		t.Error("something ran although nobody could approve it")
	}
}
