// Package item describes one thing spu agent can generate.
//
// An item is data, not code: it says which questions to ask, what the answers mean for
// the text that comes out, and what to do when a file is already there. Keeping it in
// JSON means a new document is a file beside the others rather than a new branch in Go,
// and it means the rules that decide are readable by whoever the document is for.
package item

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Policy is what to do about the files an item would write.
//
// It answers the questions that exist before anything has been rendered: is there a file
// there, and is it one of ours. What to do about one of ours that has since changed is a
// separate question with its own answer, because the two are not the same risk.
type Policy string

const (
	// Reuse writes where there is nothing and leaves alone what is already correct.
	Reuse Policy = "reuse"

	// Fresh re-renders an item's own files every time, so an edited document is brought
	// back into line rather than being left behind.
	Fresh Policy = "fresh"

	// Skip writes only what is absent and never reconsiders an item already recorded.
	Skip Policy = "skip"
)

// OnModified is what to do with a file spu wrote that no longer matches what it wrote.
//
// It is the only reader of the recorded hash. The hash itself decides nothing: it
// answers whether the file changed, and this says whether that matters.
type OnModified string

const (
	// Overwrite replaces it. Right for text that is meant to be taken as it comes.
	Overwrite OnModified = "overwrite"

	// Refuse stops and reports it. Right for text the reader was meant to adapt.
	Refuse OnModified = "refuse"

	// Ask puts the question to whoever is there, and refuses when nobody is.
	Ask OnModified = "ask"
)

// Root is which directory an output belongs in.
//
// The canonical root is the one everything is generated into; the others are the
// harnesses' own directories, which exist because a harness that does not read the
// canonical location needs a copy where it does.
type Root string

const (
	RootAgents Root = "agents"
	RootClaude Root = "claude"
	RootCodex  Root = "codex"
	RootPi     Root = "pi"
)

// Item is one definition.
type Item struct {
	ID         string          `json:"id"`
	Version    int             `json:"version"`
	Title      string          `json:"title,omitempty"`
	Policy     Policy          `json:"policy"`
	OnModified OnModified      `json:"on_modified,omitempty"`
	Questions  json.RawMessage `json:"questions,omitempty"`
	Outputs    []Output        `json:"outputs"`

	// answered is the question keys and choices the schema declares, gathered while
	// validating so that a `when` can be checked against what was actually asked.
	answered map[string]map[string]bool
}

// Output is one file an item writes.
type Output struct {
	Root    Root      `json:"root"`
	Path    string    `json:"path"`
	Content []Segment `json:"content"`
}

// Segment is one piece of an output's text.
//
// A segment is either a file from the embedded content or a literal, and may be gated on
// an answer. The gate is what makes one item serve several projects: the parts that do
// not apply are absent from the file rather than being written and qualified.
type Segment struct {
	When *Condition `json:"when,omitempty"`
	File string     `json:"file,omitempty"`
	Text string     `json:"text,omitempty"`
}

// Condition gates a segment on an answer.
type Condition struct {
	Answer string `json:"answer"`
	Is     string `json:"is,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Parse reads an item and refuses one it could not honour.
//
// Everything that can be checked without rendering is checked here, because the
// alternative is discovering it after somebody has answered questions and the answer is
// about to be written to their project.
func Parse(body []byte) (Item, error) {
	var it Item
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&it); err != nil {
		return Item{}, fmt.Errorf("item: %w", err)
	}

	if !idPattern.MatchString(it.ID) {
		return Item{}, fmt.Errorf("item: id %q is not a lowercase hyphenated name", it.ID)
	}
	if it.Version < 1 {
		return Item{}, fmt.Errorf("item %s: version %d is not a positive number", it.ID, it.Version)
	}
	switch it.Policy {
	case Reuse, Fresh, Skip:
	case "":
		return Item{}, fmt.Errorf("item %s: no policy, and the policy is what decides "+
			"whether a file somebody edited is replaced", it.ID)
	default:
		return Item{}, fmt.Errorf("item %s: unknown policy %q", it.ID, it.Policy)
	}
	switch it.OnModified {
	case Overwrite, Refuse, Ask:
	case "":
		it.OnModified = defaultOnModified(it.Policy)
	default:
		return Item{}, fmt.Errorf("item %s: unknown on_modified %q", it.ID, it.OnModified)
	}

	questions, err := readQuestions(it.Questions)
	if err != nil {
		return Item{}, fmt.Errorf("item %s: %w", it.ID, err)
	}
	it.answered = questions

	seen := make(map[string]bool, len(it.Outputs))
	for _, out := range it.Outputs {
		if err := validateOutput(it.ID, out, questions); err != nil {
			return Item{}, err
		}
		key := string(out.Root) + "/" + out.Path
		if seen[key] {
			return Item{}, fmt.Errorf("item %s: two outputs both write %s", it.ID, out.Path)
		}
		seen[key] = true
	}
	return it, nil
}

// defaultOnModified is the answer an item gets when it does not give one.
//
// Overwriting by default is a decision rather than an oversight, and it is confined to
// the two policies that already mean "this file is ours". An item that skips is left
// alone by definition, so refusing costs it nothing.
func defaultOnModified(p Policy) OnModified {
	if p == Skip {
		return Refuse
	}
	return Overwrite
}

func validateOutput(id string, out Output, questions map[string]map[string]bool) error {
	switch out.Root {
	case RootAgents, RootClaude, RootCodex, RootPi:
	default:
		return fmt.Errorf("item %s: unknown root %q", id, out.Root)
	}
	if out.Path == "" {
		return fmt.Errorf("item %s: an output has no path", id)
	}
	if strings.HasPrefix(out.Path, "/") || strings.Contains(out.Path, `\`) {
		return fmt.Errorf("item %s: path %q must be relative", id, out.Path)
	}
	if strings.HasPrefix(out.Path, "../") || strings.Contains(out.Path, "/../") || out.Path == ".." {
		return fmt.Errorf("item %s: path %q escapes the output directory", id, out.Path)
	}
	if len(out.Content) == 0 {
		return fmt.Errorf("item %s: output %q has no content to write", id, out.Path)
	}

	for _, seg := range out.Content {
		switch {
		case seg.File != "" && seg.Text != "":
			return fmt.Errorf("item %s: a content segment has both a file and text", id)
		case seg.File == "" && seg.Text == "":
			return fmt.Errorf("item %s: a content segment has neither a file nor text", id)
		}
		if seg.When == nil {
			continue
		}
		choices, ok := questions[seg.When.Answer]
		if !ok {
			return fmt.Errorf("item %s: a condition asks about %q, which is not a question",
				id, seg.When.Answer)
		}
		if seg.When.Is != "" && !choices[seg.When.Is] {
			return fmt.Errorf("item %s: a condition expects %q, which is not a choice of %q",
				id, seg.When.Is, seg.When.Answer)
		}
	}
	return nil
}

// Defaults is the answers an item declares, for a run with nobody to ask.
//
// A JSON Schema default is an annotation and nothing fills it in, so these are read out
// explicitly. A question that declares no default is left out rather than invented, which
// is what lets a caller tell a decision from a guess.
func (it Item) Defaults() map[string]any {
	out := map[string]any{}
	if len(it.Questions) == 0 {
		return out
	}
	var schema struct {
		Properties map[string]struct {
			Default json.RawMessage `json:"default"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(it.Questions, &schema); err != nil {
		return out
	}
	for key, q := range schema.Properties {
		if len(q.Default) == 0 {
			continue
		}
		var value any
		if err := json.Unmarshal(q.Default, &value); err != nil {
			continue
		}
		out[key] = value
	}
	return out
}

// readQuestions gathers what the schema asks, so a condition can be checked against it.
//
// Only a boolean or a fixed list of choices can be asked about: a condition has to be
// decidable from the answer alone, and free text is not.
func readQuestions(raw json.RawMessage) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	if len(raw) == 0 {
		return out, nil
	}
	var schema struct {
		Properties map[string]struct {
			Type  string   `json:"type"`
			Enum  []string `json:"enum"`
			Items struct {
				Enum []string `json:"enum"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("questions: %w", err)
	}
	for key, q := range schema.Properties {
		choices := map[string]bool{}
		switch q.Type {
		case "boolean":
			choices["true"] = true
			choices["false"] = true
		case "array":
			// A set is decidable: a condition asks whether one of its members is in it,
			// which is what an item picker's answer needs.
			for _, v := range q.Items.Enum {
				choices[v] = true
			}
		case "enum", "string":
			for _, v := range q.Enum {
				choices[v] = true
			}
		default:
			return nil, fmt.Errorf("question %q is of type %q, which a condition cannot decide",
				key, q.Type)
		}
		out[key] = choices
	}
	return out, nil
}
