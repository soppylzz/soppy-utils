package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAMissingLedgerIsAnEmptyOne(t *testing.T) {
	// A project that has never been generated into is the ordinary case, not a failure:
	// every first run meets it.
	dir := t.TempDir()

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got.Items) != 0 {
		t.Errorf("got %d items from a project with no ledger", len(got.Items))
	}
	if _, ok := got.Find("anything"); ok {
		t.Error("an empty ledger claims to know an item")
	}
}

func TestWriteThenRead(t *testing.T) {
	dir := t.TempDir()
	want := Ledger{
		Version:   CurrentVersion,
		Harnesses: []string{"claude", "pi"},
		Items: []Item{{
			ID:          "conduct-of-commit",
			ItemVersion: 3,
			Answers:     map[string]any{"lint-commits": true},
			Outputs: []Output{
				{Path: "prompts/conduct-of-commit.md", SHA256: "abc123"},
			},
		}},
	}
	if err := want.Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got.Harnesses) != 2 || got.Harnesses[0] != "claude" {
		t.Errorf("harnesses = %v, want them back in order", got.Harnesses)
	}
	it, ok := got.Find("conduct-of-commit")
	if !ok {
		t.Fatal("the item that was just written is not there")
	}
	if it.ItemVersion != 3 {
		t.Errorf("ItemVersion = %d, want 3", it.ItemVersion)
	}
	if it.Answers["lint-commits"] != true {
		t.Errorf("answers = %v, want the boolean back", it.Answers)
	}
	if hash, ok := it.HashOf("prompts/conduct-of-commit.md"); !ok || hash != "abc123" {
		t.Errorf("HashOf = %q, %v; want abc123, true", hash, ok)
	}
}

func TestAFutureVersionIsAReadOnlyConflict(t *testing.T) {
	// A newer spu wrote this. Rewriting it would drop whatever it recorded, and the
	// fields it added are exactly the ones this build does not know to preserve.
	dir := t.TempDir()
	write(t, dir, `{"version": 99, "items": []}`)

	_, err := Read(dir)
	if err == nil {
		t.Fatal("Read accepted a ledger from the future")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error is %T, want a ConflictError so the caller can tell it from corruption", err)
	}
}

func TestAMalformedLedgerIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"version": 1, "items": [`)

	_, err := Read(dir)
	if err == nil {
		t.Fatal("Read accepted a truncated ledger")
	}
	var conflict *ConflictError
	if errors.As(err, &conflict) {
		t.Error("a malformed file was reported as a version conflict")
	}
}

func TestAnUnknownItemVersionIsReadable(t *testing.T) {
	// The ledger records what an item was when it was generated. An item whose definition
	// has since moved is a thing to notice, not a reason to refuse to read the file.
	dir := t.TempDir()
	write(t, dir, `{"version": 1, "items": [{"id": "a", "item_version": 99, "outputs": []}]}`)

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	it, ok := got.Find("a")
	if !ok || it.ItemVersion != 99 {
		t.Errorf("Find = %+v, %v; want the recorded version", it, ok)
	}
}

func TestWritingKeepsWhatThisBuildDoesNotKnow(t *testing.T) {
	// Reading a ledger, changing one item, and writing it back must not drop the parts
	// this build has no opinion about. Silently losing a peer's record is how two tools
	// sharing a file corrupt each other.
	dir := t.TempDir()
	write(t, dir, `{"version": 1, "harnesses": ["claude"],
	  "items": [{"id": "a", "item_version": 1, "outputs": []}],
	  "something_new": {"kept": true}}`)

	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got.Items = append(got.Items, Item{ID: "b", ItemVersion: 1})
	if err := got.Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "something_new") {
		t.Errorf("an unknown top-level field was dropped:\n%s", body)
	}
	if !strings.Contains(string(body), `"id": "b"`) {
		t.Errorf("the item that was added is not in the file:\n%s", body)
	}
}

func TestWritingCreatesTheDirectory(t *testing.T) {
	// The ledger is the first thing an empty project gets, so it cannot assume the
	// directory it lives in is already there.
	dir := filepath.Join(t.TempDir(), "project", ".agents")

	if err := (Ledger{Version: CurrentVersion}).Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Errorf("the ledger was not written: %v", err)
	}
}

func write(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
