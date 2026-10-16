package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
)

// Handler runs the agent commands.
type Handler struct{}

// Run dispatches one command.
func (Handler) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	project, err := projectOf(cmd.Session)
	if err != nil {
		return err
	}
	sub := ""
	if len(cmd.Args) > 0 {
		sub = cmd.Args[0]
	}
	switch sub {
	case "":
		return generate(ctx, project, reply)
	case "status":
		return status(project, reply)
	case "edit":
		return edit(ctx, project, reply)
	case "migrate":
		return migrate(project, reply)
	default:
		return fmt.Errorf("unknown agent command %q", sub)
	}
}

// projectOf finds the project the client was standing in.
//
// An empty working directory means the client could not tell us, and guessing would
// generate into the daemon's own directory, which belongs to whoever started it. Saying
// so costs one message; guessing writes files into a stranger's project.
func projectOf(s core.Session) (string, error) {
	if s.WorkingDir == "" {
		return "", errors.New("this client did not say which directory it was run from, " +
			"so there is no project to generate into")
	}
	return harness.Project(s.WorkingDir)
}

// canonical is the directory everything is recorded against.
func canonical(project string) string {
	return harness.Dir(project, item.RootAgents)
}

func generate(ctx context.Context, project string, reply core.Responder) error {
	items, err := content.Items()
	if err != nil {
		return err
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}

	if !reply.CanAsk() {
		// Writing a default set nobody chose would create files in a project because a
		// script ran, which is the opposite of what this command is for.
		return errors.New("choosing what to generate needs somebody to choose; " +
			"run spu agent in a terminal")
	}

	chosen, harnesses, err := choose(ctx, reply, items, led)
	if err != nil {
		return err
	}
	if len(chosen) == 0 && len(harnesses) == 0 {
		return reply.Finish(nil)
	}

	answers, err := ask(ctx, reply, items, chosen, led)
	if err != nil {
		return err
	}

	steps, err := Plan(PlanOptions{
		Project: project, Items: filter(items, chosen), Answers: answers,
		Ledger: led, Files: content.Files(),
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

	if len(harnesses) > 0 {
		mcp, err := mcpStep(project, harnesses, mcpServers())
		if err != nil {
			return err
		}
		steps = append(steps, mcp)
	}

	next, err := Apply(ctx, project, steps, led, reply)
	if err != nil {
		return err
	}
	next.Harnesses = harnesses

	// The record is written before the links and the base configuration, because the files
	// are the part that must not be lost. A link that fails afterwards is reported; a
	// ledger that was never written would make the next run treat its own files as
	// somebody else's.
	if err := next.Write(canonical(project)); err != nil {
		return err
	}
	done, err := harness.Apply(project, harnesses, content.HarnessBase())
	for _, line := range done {
		_ = reply.Notify(line)
	}
	if err != nil {
		return err
	}

	// A project that dropped a harness has its registration taken back out, and only
	// spu's own entry is ever touched.
	if gone := missing(led.Harnesses, harnesses); len(gone) > 0 {
		removed, err := harness.UnregisterMCP(project, gone, mcpServers())
		for _, line := range removed {
			_ = reply.Notify(line)
		}
		if err != nil {
			return err
		}
	}
	if len(harnesses) > 0 {
		registered, err := harness.RegisterMCP(project, harnesses, mcpServers())
		for _, line := range registered {
			_ = reply.Notify(line)
		}
		if err != nil {
			return err
		}
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
func choose(ctx context.Context, reply core.Responder, items []item.Item,
	led ledger.Ledger) ([]string, []string, error) {

	options := make([]form.Option, 0, len(items))
	preselected := make([]string, 0, len(items))
	for _, it := range items {
		label := it.Title
		if label == "" {
			label = it.ID
		}
		options = append(options, form.Option{Value: it.ID, Label: label})
		if _, ok := led.Find(it.ID); ok {
			preselected = append(preselected, it.ID)
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
	for i, it := range items {
		order[it.ID] = i
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
// An item's saved answers are the starting point, so re-running offers what was chosen
// last time instead of a blank form that has to be filled in again.
func ask(ctx context.Context, reply core.Responder, items []item.Item, chosen []string,
	led ledger.Ledger) (map[string]map[string]any, error) {

	byID := make(map[string]item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
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
		saved := map[string]any{}
		if recorded, ok := led.Find(id); ok {
			saved = recorded.Answers
		}
		spec, err := form.FromObjectSchema(it.Questions, saved)
		if err != nil {
			return nil, fmt.Errorf("item %s: %w", id, err)
		}
		spec.Title = it.Title
		values, err := reply.Ask(ctx, spec)
		if err != nil {
			return nil, err
		}
		out[id] = values
	}
	return out, nil
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

// status reports what the ledger records against what is on disk. It writes nothing.
func status(project string, reply core.Responder) error {
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	if len(led.Items) == 0 {
		if err := reply.Warn("nothing has been generated in " + canonical(project)); err != nil {
			return err
		}
		return reply.Finish(nil)
	}

	items, err := content.Items()
	if err != nil {
		return err
	}
	known := make(map[string]item.Item, len(items))
	for _, it := range items {
		known[it.ID] = it
	}

	for _, recorded := range led.Items {
		// The index is generated from the selection rather than from an item, so it is
		// not in the list to be looked up in. Reporting it as an unknown item was a
		// warning about a file spu wrote itself.
		if recorded.ID == indexID || recorded.ID == mcpRecordID {
			continue
		}
		it, still := known[recorded.ID]
		switch {
		case !still:
			_ = reply.Notify(fmt.Sprintf("%s: this spu does not know this item", recorded.ID))
		case it.Version != recorded.ItemVersion:
			_ = reply.Notify(fmt.Sprintf("%s: generated from version %d, this spu has %d",
				recorded.ID, recorded.ItemVersion, it.Version))
		}
		for _, out := range recorded.Outputs {
			path := filepath.Join(canonical(project), filepath.FromSlash(out.Path))
			body, err := os.ReadFile(path)
			if err != nil {
				_ = reply.Notify(fmt.Sprintf("%s: missing", out.Path))
				continue
			}
			if hashOf(body) != out.SHA256 {
				_ = reply.Notify(fmt.Sprintf("%s: changed since it was generated", out.Path))
			}
		}
	}
	return reply.Finish(nil)
}

// edit re-asks the questions of an item that is already recorded.
//
// It is explicit rather than folded into the ordinary run, because answering a question
// again is a decision about an item, and the run that collects the whole selection is not
// the place for a decision about one of them.
func edit(ctx context.Context, project string, reply core.Responder) error {
	if !reply.CanAsk() {
		return errors.New("editing answers needs somebody to answer; run spu agent in a terminal")
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	if len(led.Items) == 0 {
		return errors.New("nothing has been generated yet; run spu agent first")
	}
	items, err := content.Items()
	if err != nil {
		return err
	}
	chosen := make([]string, 0, len(led.Items))
	for _, it := range led.Items {
		chosen = append(chosen, it.ID)
	}

	picked, harnesses, err := choose(ctx, reply, filter(items, chosen), led)
	if err != nil {
		return err
	}
	if len(harnesses) == 0 {
		harnesses = led.Harnesses
	}
	answers, err := ask(ctx, reply, items, picked, led)
	if err != nil {
		return err
	}
	// Only the items that were answered are considered, so editing one does not rewrite
	// the others on the way past.
	steps, err := Plan(PlanOptions{
		Project: project, Items: filter(items, picked), Answers: answers,
		Ledger: led, Files: content.Files(),
	})
	if err != nil {
		return err
	}
	next, err := Apply(ctx, project, steps, led, reply)
	if err != nil {
		return err
	}
	next.Harnesses = harnesses
	if err := next.Write(canonical(project)); err != nil {
		return err
	}
	done, err := harness.Apply(project, harnesses, content.HarnessBase())
	for _, line := range done {
		_ = reply.Notify(line)
	}
	if err != nil {
		return err
	}
	return reply.Finish(nil)
}

// migrate records files that were generated before a ledger existed.
//
// It only ever records what it finds already matching what spu would write: a file that
// differs is not evidence of anything, and claiming it would let the next run replace
// something nobody agreed to.
func migrate(project string, reply core.Responder) error {
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	items, err := content.Items()
	if err != nil {
		return err
	}

	adopted := 0
	for _, it := range items {
		if _, ok := led.Find(it.ID); ok {
			continue
		}
		steps, err := Plan(PlanOptions{
			Project: project, Items: []item.Item{it}, Files: content.Files(),
		})
		if err != nil {
			return err
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
			ID: it.ID, ItemVersion: it.Version, Outputs: outputs,
		})
		adopted++
		_ = reply.Notify("adopted " + it.ID)
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

func filter(items []item.Item, keep []string) []item.Item {
	want := make(map[string]bool, len(keep))
	for _, id := range keep {
		want[id] = true
	}
	out := make([]item.Item, 0, len(items))
	for _, it := range items {
		if want[it.ID] {
			out = append(out, it)
		}
	}
	return out
}
