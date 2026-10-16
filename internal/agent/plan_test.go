package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/soppylzz/soppy-utils/internal/agent/decide"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
)

func files() fstest.MapFS {
	return fstest.MapFS{"docs/one.md": {Data: []byte("# One\n")}}
}

func mustItem(t *testing.T, body string) item.Item {
	t.Helper()
	it, err := item.Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return it
}

func simple(t *testing.T, policy, onModified string) item.Item {
	t.Helper()
	return mustItem(t, `{
	  "id": "one", "version": 1, "policy": "`+policy+`", "on_modified": "`+onModified+`",
	  "outputs": [{"root": "agents", "path": "prompts/one.md",
	    "content": [{"file": "docs/one.md"}]}]
	}`)
}

func sha256Of(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestPlanWritesWhatIsAbsent(t *testing.T) {
	project := t.TempDir()

	steps, err := Plan(PlanOptions{
		Project: project,
		Items:   []item.Source{{Item: simple(t, "reuse", "overwrite"), Files: files()}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if steps[0].Decision.Action != decide.Write {
		t.Errorf("action = %q, want a write", steps[0].Decision.Action)
	}
	if string(steps[0].Content) != "# One\n" {
		t.Errorf("content = %q, want the rendered text", steps[0].Content)
	}
	if want := filepath.Join(project, ".agents", "prompts", "one.md"); steps[0].Path != want {
		t.Errorf("path = %q, want %q", steps[0].Path, want)
	}
}

func TestPlanRefusesAFileItDidNotWrite(t *testing.T) {
	// The whole reason the ledger exists. A file at the path that the ledger has no
	// record of is somebody else's, and no amount of confidence makes overwriting it
	// recoverable.
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	steps, err := Plan(PlanOptions{
		Project: project,
		Items:   []item.Source{{Item: simple(t, "reuse", "overwrite"), Files: files()}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if steps[0].Decision.Action != decide.Refuse {
		t.Errorf("action = %q, want a refusal", steps[0].Decision.Action)
	}
	if steps[0].Decision.Reason != decide.Foreign {
		t.Errorf("reason = %q, want %q so the message can say whose file it is",
			steps[0].Decision.Reason, decide.Foreign)
	}
}

func TestPlanLeavesAnUnchangedFileAlone(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("# One\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	steps, err := Plan(PlanOptions{
		Project: project,
		Items:   []item.Source{{Item: simple(t, "reuse", "overwrite"), Files: files()}},
		Ledger: ledger.Ledger{Version: ledger.CurrentVersion, Items: []ledger.Item{{
			ID: "one", ItemVersion: 1,
			Outputs: []ledger.Output{{Path: "prompts/one.md", SHA256: sha256Of([]byte("# One\n"))}},
		}}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if steps[0].Decision.Action != decide.Leave {
		t.Errorf("action = %q, want it left alone", steps[0].Decision.Action)
	}
}

func TestPlanSeesAnEditedFile(t *testing.T) {
	project := t.TempDir()
	target := filepath.Join(project, ".agents", "prompts", "one.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(target, []byte("# One, but I changed it\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	steps, err := Plan(PlanOptions{
		Project: project,
		Items:   []item.Source{{Item: simple(t, "reuse", "refuse"), Files: files()}},
		Ledger: ledger.Ledger{Version: ledger.CurrentVersion, Items: []ledger.Item{{
			ID: "one", ItemVersion: 1,
			Outputs: []ledger.Output{{Path: "prompts/one.md", SHA256: sha256Of([]byte("# One\n"))}},
		}}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if steps[0].Decision.Action != decide.Refuse {
		t.Errorf("action = %q, want a refusal: the recorded hash no longer matches", steps[0].Decision.Action)
	}
}
