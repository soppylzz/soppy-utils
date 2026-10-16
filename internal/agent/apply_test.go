package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
)

func plan(t *testing.T, project string, policy, onModified string, led ledger.Ledger) []Step {
	t.Helper()
	steps, err := Plan(PlanOptions{
		Project: project,
		Items:   []item.Item{simple(t, policy, onModified)},
		Files:   files(),
		Ledger:  led,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return steps
}

func TestApplyWritesAndRecordsTheHash(t *testing.T) {
	project := t.TempDir()
	steps := plan(t, project, "reuse", "overwrite", ledger.Ledger{Version: ledger.CurrentVersion})
	reply := &coretest.Recorder{Askable: true}

	got, err := Apply(context.Background(), project, steps, ledger.Ledger{Version: ledger.CurrentVersion}, reply)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	body, err := os.ReadFile(steps[0].Path)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if string(body) != "# One\n" {
		t.Errorf("wrote %q, want the rendered text", body)
	}

	it, ok := got.Find("one")
	if !ok {
		t.Fatal("the write was not recorded, so the next run cannot tell it is ours")
	}
	hash, ok := it.HashOf("prompts/one.md")
	if !ok {
		t.Fatal("no hash was recorded")
	}
	if hash != sha256Of([]byte("# One\n")) {
		t.Errorf("hash = %q, want the hash of what was written", hash)
	}
	if it.ItemVersion != 1 {
		t.Errorf("ItemVersion = %d, want the item's version", it.ItemVersion)
	}
}

func TestApplyRefusesAFileItDidNotWrite(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	steps := plan(t, project, "reuse", "overwrite", ledger.Ledger{Version: ledger.CurrentVersion})
	reply := &coretest.Recorder{Askable: true}

	got, err := Apply(context.Background(), project, steps, ledger.Ledger{Version: ledger.CurrentVersion}, reply)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	body, _ := os.ReadFile(steps[0].Path)
	if string(body) != "mine\n" {
		t.Errorf("the file was overwritten: %q", body)
	}
	if _, ok := got.Find("one"); ok {
		t.Error("a file this run refused to touch was recorded as ours")
	}
	warned := strings.Join(reply.Warnings, " ")
	if !strings.Contains(warned, "one.md") {
		t.Errorf("nothing said which file was in the way: %q", warned)
	}
}

func TestApplyRecordsNothingForAFileItLeftAlone(t *testing.T) {
	// skip leaves a foreign file where it is. Recording it anyway would claim ownership
	// of somebody else's work, and the next run would then feel free to replace it.
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	steps := plan(t, project, "skip", "refuse", ledger.Ledger{Version: ledger.CurrentVersion})
	reply := &coretest.Recorder{Askable: true}

	got, err := Apply(context.Background(), project, steps, ledger.Ledger{Version: ledger.CurrentVersion}, reply)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := got.Find("one"); ok {
		t.Error("a file that was left alone was recorded as generated")
	}
}

func TestAnAskWithNobodyToAnswerIsARefusal(t *testing.T) {
	// A scripted run has no terminal. Proceeding would replace a file that somebody
	// edited on the strength of a question nobody was asked.
	project := t.TempDir()
	led := ledger.Ledger{Version: ledger.CurrentVersion, Items: []ledger.Item{{
		ID: "one", ItemVersion: 1,
		Outputs: []ledger.Output{{Path: "prompts/one.md", SHA256: sha256Of([]byte("old\n"))}},
	}}}
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	steps := plan(t, project, "reuse", "ask", led)
	reply := &coretest.Recorder{} // cannot ask

	if _, err := Apply(context.Background(), project, steps, led, reply); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, _ := os.ReadFile(steps[0].Path)
	if string(body) != "edited\n" {
		t.Errorf("the file was replaced with nobody to answer: %q", body)
	}
}

func TestAnAskThatIsAnsweredYesWrites(t *testing.T) {
	project := t.TempDir()
	led := ledger.Ledger{Version: ledger.CurrentVersion, Items: []ledger.Item{{
		ID: "one", ItemVersion: 1,
		Outputs: []ledger.Output{{Path: "prompts/one.md", SHA256: sha256Of([]byte("old\n"))}},
	}}}
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	steps := plan(t, project, "reuse", "ask", led)
	reply := &coretest.Recorder{Askable: true, Confirms: []bool{true}}

	if _, err := Apply(context.Background(), project, steps, led, reply); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	body, _ := os.ReadFile(steps[0].Path)
	if string(body) != "# One\n" {
		t.Errorf("got %q, want the file replaced after the question was answered yes", body)
	}
	if len(reply.Questions) != 1 {
		t.Errorf("asked %d questions, want exactly one", len(reply.Questions))
	}
}

func TestApplyLeavesAnUnchangedFileUntouched(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("# One\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	led := ledger.Ledger{Version: ledger.CurrentVersion, Items: []ledger.Item{{
		ID: "one", ItemVersion: 1,
		Outputs: []ledger.Output{{Path: "prompts/one.md", SHA256: sha256Of([]byte("# One\n"))}},
	}}}
	steps := plan(t, project, "reuse", "overwrite", led)
	if steps[0].Decision.Action != decide.Leave {
		t.Fatalf("the plan did not leave it alone: %+v", steps[0].Decision)
	}

	got, err := Apply(context.Background(), project, steps, led, &coretest.Recorder{Askable: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	after, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !after.ModTime().Equal(info.ModTime()) {
		t.Error("a file that already matched was rewritten, churning its mtime")
	}
	if _, ok := got.Find("one"); !ok {
		t.Error("an item that is still present was dropped from the ledger")
	}
}
