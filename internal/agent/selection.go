package agent

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// selection is what a run was asked to do, from flags or from a file.
//
// Every util answers to flags as well as to a form, so that the same command can be run by
// a person and by a script. This is the agent util's half of that: the picker and the
// questions are what a terminal shows, and the flags are what a script uses to skip them.
type selection struct {
	items     []string
	harnesses []string
	answers   map[string]map[string]any

	// flagged is whether anything was named on the command line. It is what decides
	// between asking and not asking: a caller who named what they wanted has answered the
	// question the picker would have asked.
	flagged bool

	// interactive forces the picker and the questions even when the flags are complete.
	interactive bool
}

// answersDoc is the file `--answers` reads.
//
// One object holding the whole run, because the run is one decision: which items, for
// which harnesses, answered how. Splitting it across three files would let a project
// generate a set of documents nobody chose.
type answersDoc struct {
	Items     []string                  `json:"items"`
	Harnesses []string                  `json:"harnesses"`
	Answers   map[string]map[string]any `json:"answers"`
}

// parseSelection reads the flags a command was given.
func parseSelection(name string, args []string) (selection, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	// The message belongs on the client's stderr, and it gets there as the returned error.
	// Writing it here would put it in the daemon's log, where nobody can see it.
	fs.SetOutput(io.Discard)

	var sel selection
	var items, harnesses, answers string
	var interactive bool
	fs.StringVar(&items, "items", "", "")
	fs.StringVar(&harnesses, "harnesses", "", "")
	fs.StringVar(&answers, "answers", "", "")
	fs.BoolVar(&interactive, "interactive", false, "")
	if err := fs.Parse(args); err != nil {
		return selection{}, err
	}
	if fs.NArg() != 0 {
		return selection{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	sel.flagged = items != "" || harnesses != "" || answers != ""
	sel.interactive = interactive

	if answers != "" {
		doc, err := readAnswers(answers)
		if err != nil {
			return selection{}, err
		}
		sel.items, sel.harnesses, sel.answers = doc.Items, doc.Harnesses, doc.Answers
		if sel.answers == nil {
			sel.answers = map[string]map[string]any{}
		}
	}
	if items != "" {
		sel.items = splitList(items)
	}
	if harnesses != "" {
		sel.harnesses = splitList(harnesses)
	}
	return sel, nil
}

func readAnswers(path string) (answersDoc, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return answersDoc{}, fmt.Errorf("read answers: %w", err)
	}
	var doc answersDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return answersDoc{}, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// splitList reads a comma-separated list, ignoring the empty entries a trailing comma
// leaves behind.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// wantsForm reports whether the picker should be shown.
//
// A caller who named what they wanted has already answered the picker's question, so it is
// not asked again; `--interactive` is how somebody asks for it anyway.
func (s selection) wantsForm() bool { return s.interactive || !s.flagged }

// hasAnswers reports whether the file answered for an item.
func (s selection) hasAnswers(id string) bool {
	_, ok := s.answers[id]
	return ok
}

// checkNames rejects an item or harness nobody has heard of, before anything is written.
//
// A typo in `--items` would otherwise generate nothing and say nothing, which reads as
// success.
func (s selection) checkNames(items []item.Item, harnesses []string) error {
	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.ID] = true
	}
	for _, want := range s.items {
		if !known[want] {
			return fmt.Errorf("no item named %q; run spu agent --help for the list,\n"+
				"or run spu agent with no arguments to choose from a menu", want)
		}
	}
	for _, want := range s.harnesses {
		found := false
		for _, h := range harnesses {
			if h == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no harness named %q; the ones spu knows are: %s",
				want, strings.Join(harnesses, ", "))
		}
	}
	return nil
}
