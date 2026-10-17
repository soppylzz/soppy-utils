package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLookPathInFindsAnExecutableOnTheGivenPath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "figlet")
	writeExecutable(t, want)

	got, err := LookPathIn(dir, "figlet")
	if err != nil {
		t.Fatalf("LookPathIn: %v", err)
	}
	if got != want {
		t.Errorf("resolved %q, want %q", got, want)
	}
}

func TestLookPathInIgnoresTheProcessPath(t *testing.T) {
	// The daemon's own PATH belongs to whichever client started it. Answering a later
	// client's question with it is how every session gets the same wrong answer.
	mine := t.TempDir()
	writeExecutable(t, filepath.Join(mine, "figlet"))
	t.Setenv("PATH", mine)

	elsewhere := t.TempDir()
	if _, err := LookPathIn(elsewhere, "figlet"); err == nil {
		t.Fatal("LookPathIn found a command that is not on the path it was given")
	}
}

func TestLookPathInRejectsANonExecutableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "figlet"), []byte("not a program"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := LookPathIn(dir, "figlet"); err == nil {
		t.Fatal("LookPathIn accepted a file that cannot be run")
	}
}

func TestLookPathInRejectsADirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "figlet"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if _, err := LookPathIn(dir, "figlet"); err == nil {
		t.Fatal("LookPathIn accepted a directory as a command")
	}
}

func TestLookPathInRejectsAnEmptyName(t *testing.T) {
	if _, err := LookPathIn(t.TempDir(), ""); err == nil {
		t.Fatal("LookPathIn accepted an empty command name")
	}
}

func TestLookPathInSearchesEveryEntry(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	want := filepath.Join(second, "figlet")
	writeExecutable(t, want)

	got, err := LookPathIn(first+string(os.PathListSeparator)+second, "figlet")
	if err != nil {
		t.Fatalf("LookPathIn: %v", err)
	}
	if got != want {
		t.Errorf("resolved %q, want %q", got, want)
	}
}

func TestLookPathInSkipsAnEmptyEntry(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "probe"))
	t.Chdir(dir)

	// An empty entry in PATH traditionally means the current directory. Resolving a
	// command from there would let a file in whatever directory the user happens to be
	// standing in decide which program core runs.
	if _, err := LookPathIn("", "probe"); err == nil {
		t.Fatal("LookPathIn resolved a command from an empty path entry")
	}

	// A directory the user wrote out is still honoured; only the implicit one is not.
	if _, err := LookPathIn(".", "probe"); err != nil {
		t.Errorf("LookPathIn with an explicit directory entry: %v", err)
	}
}
