package item

import (
	"strings"
	"testing"
)

const valid = `{
  "id": "conduct-of-commit",
  "version": 3,
  "title": "Commit message conventions",
  "policy": "reuse",
  "on_modified": "overwrite",
  "questions": {
    "type": "object",
    "properties": {
      "lint-commits": {"type": "boolean", "default": false},
      "language": {"type": "enum", "enum": ["ts", "go", "shell"]}
    }
  },
  "outputs": [{
    "root": "agents",
    "path": "prompts/conduct-of-commit.md",
    "content": [
      {"file": "commit/header.md"},
      {"when": {"answer": "lint-commits"}, "file": "commit/husky.md"},
      {"when": {"answer": "language", "is": "go"}, "text": "Go projects commit in Go."}
    ]
  }]
}`

func TestParseAnItem(t *testing.T) {
	got, err := Parse([]byte(valid))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.ID != "conduct-of-commit" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Version != 3 {
		t.Errorf("Version = %d, want 3", got.Version)
	}
	if got.Policy != Reuse {
		t.Errorf("Policy = %q, want %q", got.Policy, Reuse)
	}
	if got.OnModified != Overwrite {
		t.Errorf("OnModified = %q, want %q", got.OnModified, Overwrite)
	}
	if len(got.Outputs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(got.Outputs))
	}
	if got.Outputs[0].Root != RootAgents {
		t.Errorf("Root = %q, want %q", got.Outputs[0].Root, RootAgents)
	}
	if len(got.Outputs[0].Content) != 3 {
		t.Errorf("got %d content segments, want 3", len(got.Outputs[0].Content))
	}
}

func TestParseRejectsWhatItCannotHonour(t *testing.T) {
	// Each of these is a way for an item to be wrong that would otherwise be found at
	// the point of writing, which is the worst place: by then a person has answered
	// questions and the answer is about to become files.
	cases := []struct {
		name string
		body string
		says string
	}{
		{"no id", `{"version":1,"policy":"reuse","outputs":[]}`, "id"},
		{"bad id", `{"id":"Conduct Of Commit","version":1,"policy":"reuse","outputs":[]}`, "id"},
		{"no version", `{"id":"a","policy":"reuse","outputs":[]}`, "version"},
		{"negative version", `{"id":"a","version":-1,"policy":"reuse","outputs":[]}`, "version"},
		{"unknown policy", `{"id":"a","version":1,"policy":"maybe","outputs":[]}`, "policy"},
		{"unknown on_modified", `{"id":"a","version":1,"policy":"reuse","on_modified":"shrug","outputs":[]}`, "on_modified"},
		{"unknown root", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"emacs","path":"x.md","content":[{"text":"hi"}]}]}`, "root"},
		{"absolute path", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"/etc/passwd","content":[{"text":"hi"}]}]}`, "path"},
		{"escaping path", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"../x.md","content":[{"text":"hi"}]}]}`, "path"},
		{"empty content", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"x.md","content":[]}]}`, "content"},
		{"segment with both", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"x.md","content":[{"file":"a.md","text":"b"}]}]}`, "content"},
		{"segment with neither", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"x.md","content":[{}]}]}`, "content"},
		{"when names no question", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"x.md","content":[{"when":{"answer":"nope"},"text":"hi"}]}]}`, "nope"},
		{"when is a value not a choice", `{"id":"a","version":1,"policy":"reuse","questions":{"type":"object","properties":{"language":{"type":"enum","enum":["go"]}}},"outputs":[{"root":"agents","path":"x.md","content":[{"when":{"answer":"language","is":"rust"},"text":"hi"}]}]}`, "rust"},
		{"duplicate output path", `{"id":"a","version":1,"policy":"reuse","outputs":[{"root":"agents","path":"x.md","content":[{"text":"1"}]},{"root":"agents","path":"x.md","content":[{"text":"2"}]}]}`, "x.md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.body))
			if err == nil {
				t.Fatalf("Parse accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not mention %q", err, tc.says)
			}
		})
	}
}

func TestAMissingPolicyIsNotASilentDefault(t *testing.T) {
	// The policy is what decides whether a file the user edited gets replaced. An item
	// that forgets to say has not chosen, and a wrong guess here overwrites work.
	_, err := Parse([]byte(`{"id":"a","version":1,"outputs":[]}`))
	if err == nil {
		t.Fatal("Parse accepted an item with no policy")
	}
	if !strings.Contains(err.Error(), "policy") {
		t.Errorf("error %q does not mention the policy", err)
	}
}

func TestDefaultsReadsWhatTheItemDeclares(t *testing.T) {
	// A run with nobody to ask still has to answer the questions. The answers are the
	// ones the item declares, read explicitly here rather than left to a validator,
	// because a JSON Schema default is an annotation and nothing fills it in.
	it, err := Parse([]byte(`{
	  "id": "a", "version": 1, "policy": "reuse",
	  "questions": {"type": "object", "properties": {
	    "lint-commits": {"type": "boolean", "default": false},
	    "language": {"type": "enum", "enum": ["ts", "go"], "default": "go"},
	    "items": {"type": "array", "items": {"enum": ["x"]}, "default": ["x"]},
	    "untitled": {"type": "string"}
	  }},
	  "outputs": [{"root": "agents", "path": "x.md", "content": [{"text": "hi"}]}]
	}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	got := it.Defaults()
	if got["lint-commits"] != false {
		t.Errorf("lint-commits = %v, want the declared false", got["lint-commits"])
	}
	if got["language"] != "go" {
		t.Errorf("language = %v, want the declared go", got["language"])
	}
	if _, ok := got["items"]; !ok {
		t.Errorf("a declared list default was dropped: %v", got)
	}
	// A question with no declared default is absent rather than invented, so a caller can
	// tell "this was decided" from "nobody said".
	if _, ok := got["untitled"]; ok {
		t.Errorf("a question with no default was answered anyway: %v", got)
	}
}

func TestAnItemWithNoQuestionsHasNoDefaults(t *testing.T) {
	it, err := Parse([]byte(`{"id":"a","version":1,"policy":"reuse",
	  "outputs":[{"root":"agents","path":"x.md","content":[{"text":"hi"}]}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := it.Defaults(); len(got) != 0 {
		t.Errorf("Defaults = %v, want nothing", got)
	}
}
