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
func Apply(ctx context.Context, project string, steps []Step, led ledger.Ledger,
	keep []string, reply core.Responder) (ledger.Ledger, error) {
	out := ledger.Ledger{Version: ledger.CurrentVersion}

	// Which entries survive is decided by the caller rather than by which files this run
	// happened to touch. Choosing everything and re-answering one item are different
	// questions: the first is a selection, where an item left out is being handed over,
	// and the second is a revision, where every other item is simply not this command's
	// business. Deriving it from the steps made editing one item drop every other item's
	// record, so the next run refused to write files spu had generated itself.
	for _, it := range led.Items {
		if !inList(keep, it.ID) {
			continue
		}
		out.Items = append(out.Items, ledger.Item{
			ID:          it.ID,
			ItemVersion: it.ItemVersion,
			Answers:     it.Answers,
			// The outputs carry the hashes, and a hash is what makes a file spu's. An
			// item carried forward without them stays in the ledger while none of its
			// files does, so the next run reads its own output as somebody else's.
			Outputs: it.Outputs,
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

// inList reports whether an id is among those whose record survives.
func inList(ids []string, id string) bool {
	for _, have := range ids {
		if have == id {
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
		// Replaced rather than appended: an item writes one file at one path, and an item
		// carried forward already has that path recorded.
		replaced := false
		for j := range out.Items[i].Outputs {
			if out.Items[i].Outputs[j].Path == step.Rel {
				out.Items[i].Outputs[j] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			out.Items[i].Outputs = append(out.Items[i].Outputs, entry)
		}
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
		// A file holding exactly what spu would write is one spu wrote before the item was
		// handed over — dropping an item leaves its output behind, and choosing it again
		// finds that output with nothing recorded to explain it. Telling the reader to
		// move it aside is wrong advice for a file that is already correct.
		if existing, err := os.ReadFile(step.Path); err == nil && string(existing) == string(step.Content) {
			_ = reply.Warn(fmt.Sprintf("%s already holds what spu would write, but nothing "+
				"records it; run 'spu agent migrate' to take it back under management",
				step.Path))
			return
		}
		_ = reply.Warn(fmt.Sprintf("%s already exists and was not written by spu; "+
			"leaving it. Move it aside or add your own document beside it.", step.Path))
	case decide.Edited:
		_ = reply.Warn(fmt.Sprintf("%s was changed after spu generated it; leaving it.", step.Path))
	default:
		_ = reply.Warn(fmt.Sprintf("%s was not written.", step.Path))
	}
}
