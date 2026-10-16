package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// Apply carries out a plan and returns the ledger it leaves behind.
//
// The ledger is rebuilt from the steps rather than patched, because what it should
// contain is exactly "the outputs spu wrote and still stands behind": an item whose files
// were all refused or left as somebody else's is not one of them, and an item that was
// generated before and is unchanged still is.
func Apply(ctx context.Context, project string, steps []Step, led ledger.Ledger, reply core.Responder) (ledger.Ledger, error) {
	out := ledger.Ledger{Version: ledger.CurrentVersion}

	// An item that is still in this run keeps its answers, so a run that changes nothing
	// does not forget what was answered. An item that was not chosen this time is dropped
	// from the record: its file stays where it is, but spu stops claiming it, which is
	// what lets a person take a document over by no longer selecting it.
	for _, it := range led.Items {
		if !considered(steps, it.ID) {
			continue
		}
		out.Items = append(out.Items, ledger.Item{
			ID:          it.ID,
			ItemVersion: it.ItemVersion,
			Answers:     it.Answers,
		})
	}

	for _, step := range steps {
		action, err := resolve(ctx, step, reply)
		if err != nil {
			return ledger.Ledger{}, err
		}
		switch action {
		case decide.Refuse:
			report(reply, step)
			continue
		case decide.Leave:
			if _, recorded := led.Find(step.Item.ID); !recorded {
				// Left alone and never ours: a foreign file, or a `skip` item that found
				// something already there. Recording it would claim work we did not do.
				continue
			}
		case decide.Write:
			if err := write(step); err != nil {
				return ledger.Ledger{}, err
			}
		}

		record(&out, step)
	}
	return out, nil
}

// considered reports whether an item was part of the run.
func considered(steps []Step, id string) bool {
	for _, step := range steps {
		if step.Item.ID == id {
			return true
		}
	}
	return false
}

// resolve turns an Ask into an answer.
//
// A client that cannot ask has answered no, which is the same answer it gives to every
// other question about work that changes the machine. Proceeding instead would replace a
// file somebody edited on the strength of a question nobody was asked.
func resolve(ctx context.Context, step Step, reply core.Responder) (decide.Action, error) {
	if step.Decision.Action != decide.Ask {
		return step.Decision.Action, nil
	}
	if !reply.CanAsk() {
		return decide.Refuse, nil
	}
	question := fmt.Sprintf("%s was changed after spu generated it. Replace it?", step.Path)
	ok, err := reply.Confirm(ctx, question)
	if err != nil {
		return "", fmt.Errorf("confirm %s: %w", step.Path, err)
	}
	if !ok {
		return decide.Refuse, nil
	}
	return decide.Write, nil
}

func write(step Step) error {
	if err := os.MkdirAll(filepath.Dir(step.Path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(step.Path), err)
	}
	if err := os.WriteFile(step.Path, step.Content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", step.Path, err)
	}
	return nil
}

// record adds an output to the ledger, creating the item if this is its first output.
//
// The hash is taken over what was actually written, read back rather than assumed: a
// recorded hash that does not match the file is worse than no hash, because it makes the
// next run trust a file it should be asking about.
func record(out *ledger.Ledger, step Step) {
	written, err := os.ReadFile(step.Path)
	if err != nil {
		return
	}
	entry := ledger.Output{Path: step.Rel, SHA256: hashOf(written)}

	for i := range out.Items {
		if out.Items[i].ID != step.Item.ID {
			continue
		}
		out.Items[i].ItemVersion = step.Item.Version
		out.Items[i].Outputs = append(out.Items[i].Outputs, entry)
		return
	}
	out.Items = append(out.Items, ledger.Item{
		ID:          step.Item.ID,
		ItemVersion: step.Item.Version,
		Outputs:     []ledger.Output{entry},
	})
}

// report says what was in the way. A refusal that does not name the file leaves whoever
// reads it with nothing to do about it.
func report(reply core.Responder, step Step) {
	switch step.Decision.Reason {
	case decide.Foreign:
		_ = reply.Warn(fmt.Sprintf("%s already exists and was not written by spu; "+
			"leaving it. Move it aside or add your own document beside it.", step.Path))
	case decide.Edited:
		_ = reply.Warn(fmt.Sprintf("%s was changed after spu generated it; leaving it.", step.Path))
	default:
		_ = reply.Warn(fmt.Sprintf("%s was not written.", step.Path))
	}
}
