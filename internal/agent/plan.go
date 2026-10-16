// Package agent generates a project's agent-facing text.
//
// The work is in three separable pieces and they are kept that way: deciding what each
// file needs is a pure function of facts (decide), producing the bytes is a pure function
// of answers (render), and this is the part that reads the filesystem, keeps the ledger,
// and asks whoever is there.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/agent/render"
)

// PlanOptions is what a plan needs.
type PlanOptions struct {
	// Project is the directory the project's files are generated into.
	Project string

	// Items are the items to consider, in the order they were chosen, each with the
	// files its content reads.
	Items []item.Source

	// Answers is each item's answers, keyed by item id.
	Answers map[string]map[string]any

	// Ledger is what was recorded before this run.
	Ledger ledger.Ledger
}

// Step is one output and what should happen to it.
type Step struct {
	Item item.Item

	// Path is where the output goes, absolute.
	Path string

	// Rel is that path relative to the canonical directory, which is what the ledger
	// records — a ledger that moved with the project would be a ledger that broke when
	// the project was renamed.
	Rel string

	Decision decide.Decision

	// Content is the text that would be written. It is computed even for a step that
	// will not be written, because the decision needs it to know whether the file
	// already matches.
	Content []byte
}

// Plan reads the project and decides what each output needs.
//
// Nothing is written here, which is what lets `status` share it: the same questions are
// asked and the same answers given, and only the writing differs.
func Plan(opts PlanOptions) ([]Step, error) {
	var steps []Step
	for _, src := range opts.Items {
		it := src.Item
		answers := opts.Answers[it.ID]
		recorded, _ := opts.Ledger.Find(it.ID)
		for _, out := range it.Outputs {
			body, err := render.Output(out, answers, src.Files)
			if err != nil {
				return nil, fmt.Errorf("item %s: %w", it.ID, err)
			}
			// A skill that the harness would load without its fields is worse than one
			// that fails here, because nothing says it happened.
			if err := render.Check(out.Path, body); err != nil {
				return nil, fmt.Errorf("item %s: %w", it.ID, err)
			}
			dir := harness.Dir(opts.Project, out.Root)
			path := filepath.Join(dir, filepath.FromSlash(out.Path))

			facts, err := gather(path, recorded, out.Path, body)
			if err != nil {
				return nil, err
			}
			steps = append(steps, Step{
				Item:     it,
				Path:     path,
				Rel:      out.Path,
				Decision: decide.Plan(facts, it.Policy, it.OnModified),
				Content:  body,
			})
		}
	}
	return steps, nil
}

// gather reads the four facts about one path.
//
// Every fact is read here rather than carried from an earlier snapshot: a person can
// create, edit, or delete a file between the picker being drawn and the write happening,
// and a decision made on a stale fact is a decision about a file that is no longer there.
func gather(path string, recorded ledger.Item, rel string, body []byte) (decide.Facts, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return decide.Facts{}, fmt.Errorf("read %s: %w", path, err)
	}
	exists := err == nil

	hash, isOurs := recorded.HashOf(rel)
	facts := decide.Facts{
		Exists:   exists,
		Recorded: isOurs,
	}
	if !exists {
		return facts, nil
	}
	facts.Identical = string(existing) == string(body)
	if isOurs {
		facts.HashMatches = hashOf(existing) == hash
	}
	return facts, nil
}

func hashOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
