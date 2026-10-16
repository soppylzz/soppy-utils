package agent

import (
	"context"
	"errors"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// edit re-asks the questions of an item that is already recorded.
//
// It is explicit rather than folded into the ordinary run, because answering a question
// again is a decision about an item, and the run that collects the whole selection is not
// the place for a decision about one of them.
func edit(ctx context.Context, project, configRoot string, args []string, reply core.Responder) error {
	sel, err := parseSelection("agent edit", args)
	if err != nil {
		return err
	}
	items, err := content.Library(configRoot)
	if err != nil {
		return err
	}
	// Only a run that has to show something needs somebody to show it to. `--answers` is
	// how this command is used without a terminal, so requiring one whenever the client
	// cannot ask made the flag useless.
	if sel.wantsForm() && !reply.CanAsk() {
		return errors.New("editing answers needs somebody to answer; run spu agent in a " +
			"terminal, or name the items and answer them with --answers")
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	if len(led.Items) == 0 {
		return errors.New("nothing has been generated yet; run spu agent first")
	}
	chosen := make([]string, 0, len(led.Items))
	for _, it := range led.Items {
		chosen = append(chosen, it.ID)
	}

	harnesses := led.Harnesses
	picked := sel.items
	if sel.wantsForm() {
		picked, harnesses, err = choose(ctx, reply, filter(items, chosen), led)
		if err != nil {
			return err
		}
	}
	if len(picked) == 0 || len(harnesses) == 0 {
		harnesses = led.Harnesses
	}
	if err := sel.checkNames(items, harnessNames); err != nil {
		return err
	}
	answers, err := ask(ctx, reply, items, picked, led, sel)
	if err != nil {
		return err
	}
	// Only the items that were answered are considered, so editing one does not rewrite
	// the others on the way past.
	steps, err := Plan(PlanOptions{
		Project: project, Items: filter(items, picked), Answers: answers,
		Ledger: led,
	})
	if err != nil {
		return err
	}
	// Every recorded item survives an edit, not only the one being re-answered: a
	// revision is not a selection, and the others are not this command's business.
	// Deriving the survivors from which files this run touched dropped every other
	// item's record, so the next run refused to write files spu had generated itself.
	recorded := make([]string, 0, len(led.Items))
	for _, it := range led.Items {
		recorded = append(recorded, it.ID)
	}
	next, err := Apply(ctx, project, steps, led, recorded, reply)
	if err != nil {
		return err
	}
	next.Harnesses = harnesses
	if err := next.Write(canonical(project)); err != nil {
		return err
	}
	done, err := harness.Apply(project, harnesses)
	for _, line := range done {
		_ = reply.Notify(line)
	}
	if err != nil {
		return err
	}
	return reply.Finish(nil)
}
