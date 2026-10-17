package render

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

func content() fstest.MapFS {
	return fstest.MapFS{
		"commit/header.md": {Data: []byte("# Commits\n")},
		"commit/husky.md":  {Data: []byte("Run commitlint before committing.\n")},
		"plain.md":         {Data: []byte("no trailing newline")},
	}
}

func output(t *testing.T, body string) item.Output {
	t.Helper()
	it, err := item.Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return it.Outputs[0]
}

func TestSegmentsRenderInOrder(t *testing.T) {
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"file": "commit/header.md"},
	    {"text": "written inline\n"},
	    {"file": "commit/husky.md"}
	  ]}]
	}`)

	got, err := Output(out, nil, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	want := "# Commits\nwritten inline\nRun commitlint before committing.\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestABooleanConditionGatesASegment(t *testing.T) {
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {"lint-commits": {"type": "boolean"}}},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"file": "commit/header.md"},
	    {"when": {"answer": "lint-commits"}, "file": "commit/husky.md"}
	  ]}]
	}`)

	on, err := Output(out, map[string]any{"lint-commits": true}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if !strings.Contains(string(on), "commitlint") {
		t.Errorf("the segment was left out although the answer was yes: %q", on)
	}

	off, err := Output(out, map[string]any{"lint-commits": false}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if strings.Contains(string(off), "commitlint") {
		t.Errorf("the segment was written although the answer was no: %q", off)
	}
}

func TestAConditionOnAChoiceMatchesTheValue(t *testing.T) {
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {"language": {"type": "enum", "enum": ["go", "ts"]}}},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"when": {"answer": "language", "is": "go"}, "text": "Go.\n"},
	    {"when": {"answer": "language", "is": "ts"}, "text": "TypeScript.\n"}
	  ]}]
	}`)

	got, err := Output(out, map[string]any{"language": "go"}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "Go.\n" {
		t.Errorf("got %q, want only the branch that matched", got)
	}
}

func TestAConditionOnASetMatchesMembership(t *testing.T) {
	// The item picker answers with a set, and a segment gated on one of its members is
	// how a document that belongs to one item is written into that item's file.
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {"items": {"type": "array", "items": {"enum": ["docs"]}}}},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"when": {"answer": "items", "is": "docs"}, "text": "Docs.\n"}
	  ]}]
	}`)

	got, err := Output(out, map[string]any{"items": []any{"commit", "docs"}}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "Docs.\n" {
		t.Errorf("got %q, want the segment whose member was chosen", got)
	}

	none, err := Output(out, map[string]any{"items": []any{"commit"}}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(none) != "" {
		t.Errorf("got %q, want nothing when the member was not chosen", none)
	}
}

func TestAnAbsentAnswerIsNotTrue(t *testing.T) {
	// An item that was recorded before a question existed must not gain the segment that
	// question guards: nobody answered it, and yes is not the safe reading.
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {"lint-commits": {"type": "boolean"}}},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"file": "commit/header.md"},
	    {"when": {"answer": "lint-commits"}, "text": "new\n"}
	  ]}]
	}`)

	got, err := Output(out, nil, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "# Commits\n" {
		t.Errorf("got %q, want the ungated segment only", got)
	}
}

func TestAMissingContentFileIsAnError(t *testing.T) {
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [{"file": "gone.md"}]}]
	}`)

	_, err := Output(out, nil, content())
	if err == nil {
		t.Fatal("Output accepted a segment whose file is missing")
	}
	if !strings.Contains(err.Error(), "gone.md") {
		t.Errorf("error %q does not name the missing file", err)
	}
}

func TestAnAnswerIsWrittenIntoTheText(t *testing.T) {
	// A document that names a directory somebody chose cannot say which one until the
	// choice is made, and the choice is not known when the template is written.
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {"dir": {"type": "string"}}},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"text": "read from {{dir}} and nowhere else\n"},
	    {"file": "plain.md"}
	  ]}]
	}`)

	got, err := Output(out, map[string]any{"dir": "~/code/reference"}, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "read from ~/code/reference and nowhere else\nno trailing newline" {
		t.Errorf("got %q", got)
	}
}

func TestAnAnswerNobodyGaveIsRefused(t *testing.T) {
	// What gets written otherwise is a document containing `{{dir}}`, which reads as a
	// document that meant to say something and did not.
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"text": "read from {{dir}}\n"}]}]
	}`)

	_, err := Output(out, nil, content())
	if err == nil {
		t.Fatal("a placeholder with no answer was written out")
	}
	if !strings.Contains(err.Error(), "dir") {
		t.Errorf("error %q does not name the placeholder", err)
	}
}

func TestTextWithoutAPlaceholderIsCopiedThrough(t *testing.T) {
	// The documents are full of braces — `${var^^}` is in the shell conventions — so a
	// substitution that looked at single braces would corrupt them.
	out := output(t, `{
	  "id": "a", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [
	    {"text": "no ${var} here\n"}]}]
	}`)

	got, err := Output(out, nil, content())
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if string(got) != "no ${var} here\n" {
		t.Errorf("got %q", got)
	}
}
