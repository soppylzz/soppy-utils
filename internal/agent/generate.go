package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
)

func generate(ctx context.Context, project, configRoot string, args []string, reply core.Responder) error {
	sel, err := parseSelection("agent", args)
	if err != nil {
		return err
	}
	items, err := content.Library(configRoot)
	if err != nil {
		return err
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	if err := sel.checkNames(items, harnessNames); err != nil {
		return err
	}

	chosen, harnesses := sel.items, keepKnown(sel.harnesses)
	if sel.wantsForm() {
		if !reply.CanAsk() {
			// Writing a set nobody chose would create files in a project because a script
			// ran, which is the opposite of what this command is for.
			return errors.New("choosing what to generate needs somebody to choose; " +
				"run spu agent in a terminal, or name the items with --items")
		}
		picked, pickedHarnesses, err := choose(ctx, reply, items, led)
		if err != nil {
			return err
		}
		chosen, harnesses = picked, pickedHarnesses
	}
	if len(chosen) == 0 && len(harnesses) == 0 {
		return reply.Finish(nil)
	}

	answers, err := ask(ctx, reply, items, chosen, led, sel)
	if err != nil {
		return err
	}

	steps, err := Plan(PlanOptions{
		Project: project, Items: filter(items, chosen), Answers: answers,
		Ledger: led,
	})
	if err != nil {
		return err
	}
	// The index lists what was chosen, so it is built from the choice rather than from an
	// item: an item cannot describe a list that does not exist until the items are picked.

	idx, err := indexStep(project, steps)
	if err != nil {
		return err
	}
	steps = append(steps, idx)

	next, err := Apply(ctx, project, steps, led, chosen, reply)
	if err != nil {
		return err
	}
	next.Harnesses = harnesses

	// Saying what stopped being tracked. Choosing is authoritative, so an item left out of
	// the selection is handed over — and its file stays where it is, which leaves a
	// document in the project that nothing reads and nothing explains.
	//
	// Compared against what the run actually recorded, not against what was chosen: the
	// index and the MCP record are written by the run rather than chosen, and comparing
	// against the selection announced that spu had stopped tracking its own bookkeeping.
	before := make([]string, 0, len(led.Items))
	for _, it := range led.Items {
		before = append(before, it.ID)
	}
	after := make([]string, 0, len(next.Items))
	for _, it := range next.Items {
		after = append(after, it.ID)
	}
	for _, id := range missing(before, after) {
		_ = reply.Notify(fmt.Sprintf("%s is no longer tracked; whatever it generated is "+
			"still there and is now yours", id))
	}

	// The record is written before the links and the base configuration, because the files
	// are the part that must not be lost. A link that fails afterwards is reported; a
	// ledger that was never written would make the next run treat its own files as
	// somebody else's.
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

// harnessNames are the harnesses spu knows where to put things for.
var harnessNames = []string{"claude", "codex", "pi"}

// choose asks what to generate and which harnesses to arrange it for.
//
// Two questions in one form, because they are answered together and a second form for one
// more question is a second interruption. Multi-select rather than a yes for each: the
// list is known, and answering it once beats answering it once per item.
func choose(ctx context.Context, reply core.Responder, items []item.Source,
	led ledger.Ledger) ([]string, []string, error) {

	options := make([]form.Option, 0, len(items))
	preselected := make([]string, 0, len(items))
	for _, src := range items {
		label := src.Item.Title
		if label == "" {
			label = src.Item.ID
		}
		options = append(options, form.Option{Value: src.Item.ID, Label: label})
		if _, ok := led.Find(src.Item.ID); ok {
			preselected = append(preselected, src.Item.ID)
		}
	}

	harnessOptions := []form.Option{
		{Value: "claude", Label: "Claude Code"},
		{Value: "codex", Label: "Codex"},
		{Value: "pi", Label: "Pi"},
	}
	// Nothing is preselected on a first run. Creating .codex/ in a project that does not
	// use Codex is a rude way to answer a question nobody asked.
	harnessValue := append([]string(nil), led.Harnesses...)

	answers, err := reply.Ask(ctx, form.Spec{
		Title: "What should spu generate?",
		Fields: []form.Field{
			{
				Key:     "items",
				Type:    form.Multiple,
				Title:   "Documents and skills",
				Help:    "Already generated items are selected; clear one to stop tracking it.",
				Options: options,
				Value:   preselected,
			},
			{
				Key:     "harnesses",
				Type:    form.Multiple,
				Title:   "Harnesses to arrange for",
				Help:    "Pi and Codex read .agents/skills themselves; Claude Code needs a link.",
				Options: harnessOptions,
				Value:   harnessValue,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	chosen := stringsOf(answers["items"])
	harnesses := stringsOf(answers["harnesses"])

	// In the picker's order, not the order the form returned, so the walk and the report
	// read the way the list did.
	order := make(map[string]int, len(items))
	for i, src := range items {
		order[src.Item.ID] = i
	}
	sort.SliceStable(chosen, func(i, j int) bool { return order[chosen[i]] < order[chosen[j]] })
	return chosen, keepKnown(harnesses), nil
}

// keepKnown drops a harness name this build does not know, so a value from a newer ledger
// cannot put files somewhere nothing reads them.
func keepKnown(names []string) []string {
	known := make(map[string]bool, len(harnessNames))
	for _, n := range harnessNames {
		known[n] = true
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if known[n] {
			out = append(out, n)
		}
	}
	return out
}

// ask collects each chosen item's answers.
//
// Three sources, in order: what the item declares, what an answers file supplies, and what
// a person says. An item's saved answers are the starting point for the form, so re-running
// offers what was chosen last time instead of a blank form.
//
// When nobody is asked, what was decided is reported rather than decided quietly: a
// generated document whose answers nobody chose should say so.
func ask(ctx context.Context, reply core.Responder, items []item.Source, chosen []string,
	led ledger.Ledger, sel selection) (map[string]map[string]any, error) {

	byID := make(map[string]item.Item, len(items))
	for _, src := range items {
		byID[src.Item.ID] = src.Item
	}

	out := map[string]map[string]any{}
	for _, id := range chosen {
		it, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("no item named %q", id)
		}
		if len(it.Questions) == 0 {
			out[id] = nil
			continue
		}

		// Three sources, weakest first: what the item declares, what this project answered
		// last time, and what this run was told. A recorded answer outranks a declared
		// default because somebody chose it — without that, re-running with --items to add
		// one document silently resets every other answer to the default.
		declared := it.Defaults()
		if recorded, ok := led.Find(id); ok {
			for key, value := range recorded.Answers {
				declared[key] = value
			}
		}
		for key, value := range sel.answers[id] {
			declared[key] = value
		}

		if sel.interactive || !sel.flagged {
			if !reply.CanAsk() {
				return nil, fmt.Errorf("item %s asks questions and this run has nobody to "+
					"ask; give the answers with --answers", id)
			}
			spec, err := form.FromObjectSchema(it.Questions, declared)
			if err != nil {
				return nil, fmt.Errorf("item %s: %w", id, err)
			}
			spec.Title = it.Title
			answered, err := reply.Ask(ctx, spec)
			if err != nil {
				return nil, err
			}
			out[id] = answered
			continue
		}

		for _, key := range questionKeys(it) {
			if _, ok := declared[key]; ok {
				continue
			}
			return nil, fmt.Errorf("item %s asks %q and nothing answered it; give it in "+
				"--answers, or run spu agent --interactive", id, key)
		}
		chosen := map[string]bool{}
		if recorded, ok := led.Find(id); ok {
			for key := range recorded.Answers {
				chosen[key] = true
			}
		}
		for key := range sel.answers[id] {
			chosen[key] = true
		}
		for _, key := range questionKeys(it) {
			if chosen[key] {
				continue
			}
			_ = reply.Notify(fmt.Sprintf("%s: %s = %v (the item's default)", id, key, declared[key]))
		}
		out[id] = declared
	}
	return out, nil
}

// questionKeys lists what an item asks, so a scripted run can say which of them it had no
// answer for rather than discovering it in the generated file.
func questionKeys(it item.Item) []string {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(it.Questions, &schema); err != nil {
		return nil
	}
	out := make([]string, 0, len(schema.Properties))
	for key := range schema.Properties {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func stringsOf(v any) []string {
	switch got := v.(type) {
	case []string:
		return got
	case []any:
		out := make([]string, 0, len(got))
		for _, s := range got {
			if text, ok := s.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
