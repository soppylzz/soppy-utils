package agent

import (
	"os"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// migrate records files that were generated before a ledger existed.
//
// It only ever records what it finds already matching what spu would write: a file that
// differs is not evidence of anything, and claiming it would let the next run replace
// something nobody agreed to.
func migrate(project, configRoot string, args []string, reply core.Responder) error {
	if err := acceptNoFlags("agent migrate", args); err != nil {
		return err
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	items, err := content.Library(configRoot)
	if err != nil {
		return err
	}

	adopted := 0
	for _, src := range items {
		if _, ok := led.Find(src.Item.ID); ok {
			continue
		}
		// Rendered from what the item declares, because there is nobody to ask and nothing
		// recorded: a file adopted here has to be one this spu would have written anyway.
		// An item that asks something with no declared answer is skipped — no file of its
		// could have been generated without one.
		steps, err := Plan(PlanOptions{
			Project: project, Items: []item.Source{src},
			Answers: map[string]map[string]any{src.Item.ID: src.Item.Defaults()},
		})
		if err != nil {
			continue
		}
		var outputs []ledger.Output
		for _, step := range steps {
			body, err := os.ReadFile(step.Path)
			if err != nil || hashOf(body) != hashOf(step.Content) {
				continue
			}
			outputs = append(outputs, ledger.Output{Path: step.Rel, SHA256: hashOf(body)})
		}
		if len(outputs) == 0 {
			continue
		}
		led.Items = append(led.Items, ledger.Item{
			ID: src.Item.ID, ItemVersion: src.Item.Version, Outputs: outputs,
		})
		adopted++
		_ = reply.Notify("adopted " + src.Item.ID)
	}

	if adopted == 0 {
		if err := reply.Warn("nothing to adopt: no unrecorded file matches what spu would write"); err != nil {
			return err
		}
		return reply.Finish(nil)
	}
	if err := led.Write(canonical(project)); err != nil {
		return err
	}
	return reply.Finish(nil)
}
