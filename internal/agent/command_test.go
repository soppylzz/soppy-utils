package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
)

func readLedger(project string) (ledger.Ledger, error) {
	return ledger.Read(canonical(project))
}

func TestAScriptedRunIsRefusedRatherThanGuessed(t *testing.T) {
	// Nobody chose anything, so there is no right set to generate. Writing a default one
	// would put files in a project because a script ran.
	project := t.TempDir()
	reply := &coretest.Recorder{} // cannot ask

	err := Handler{}.Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply)
	if err == nil {
		t.Fatal("a run with nobody to ask generated a default set")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error %q does not say what to do about it", err)
	}
	if _, statErr := os.Stat(canonical(project)); statErr == nil {
		t.Error("the refusal still created the project directory")
	}
}

func TestTheWholeThingGeneratesAndThenLeavesItAlone(t *testing.T) {
	// The end-to-end shape: choose, answer, write, record — then run again and change
	// nothing, which is what a command that is run repeatedly has to do.
	project := t.TempDir()
	items, err := content.Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	pick := []string{"conduct-of-commit", "soppy-excalidraw"}
	if len(items) == 0 {
		t.Fatal("no items")
	}

	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items":        []any{pick[0], pick[1]},
		"lint-commits": true,
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	prompt := filepath.Join(canonical(project), "prompts", "conduct-of-commit.md")
	body, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatalf("the document was not written: %v", err)
	}
	if !strings.Contains(string(body), "Conventional Commits") {
		t.Errorf("the document does not hold the shipped text:\n%s", body)
	}
	if !strings.Contains(string(body), "commitlint") {
		t.Error("the segment gated on lint-commits was not written although it was answered yes")
	}
	skill := filepath.Join(canonical(project), "skills", "soppy-excalidraw", "SKILL.md")
	if _, err := os.Stat(skill); err != nil {
		t.Errorf("the skill was not written: %v", err)
	}
	// Only what was chosen.
	if _, err := os.Stat(filepath.Join(canonical(project), "prompts", "conduct-of-go.md")); err == nil {
		t.Error("an item that was not chosen was generated anyway")
	}

	led, err := readLedger(project)
	if err != nil {
		t.Fatalf("readLedger: %v", err)
	}
	if _, ok := led.Find("conduct-of-commit"); !ok {
		t.Error("the item was not recorded, so the next run cannot tell the file is ours")
	}

	before, err := os.Stat(prompt)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	second := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{pick[0], pick[1]}, "lint-commits": true,
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, second); err != nil {
		t.Fatalf("second run: %v", err)
	}
	after, err := os.Stat(prompt)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("running again rewrote a file that had not changed")
	}
}

func TestStatusReportsAnEditedFile(t *testing.T) {
	project := t.TempDir()
	pick := []string{"conduct-of-go"}
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{"items": []any{pick[0]}}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	target := filepath.Join(canonical(project), "prompts", "conduct-of-go.md")
	if err := os.WriteFile(target, []byte("I rewrote this\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}

	report := &coretest.Recorder{Askable: true}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Args: []string{"status"},
		Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, report); err != nil {
		t.Fatalf("status: %v", err)
	}
	said := strings.Join(report.Notices, "\n")
	if !strings.Contains(said, "changed since it was generated") {
		t.Errorf("status did not report the edit:\n%s", said)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("status touched the file it was reporting on: %v", err)
	}
}

func TestAProjectIsFoundFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := projectOf(core.Session{GOOS: "linux", WorkingDir: deep})
	if err != nil {
		t.Fatalf("projectOf: %v", err)
	}
	if got != root {
		t.Errorf("projectOf = %q, want the repository root %q", got, root)
	}
}

func TestACommandThatWarnsStillFinishes(t *testing.T) {
	// A warning is something said, not a result. Returning one ends the command with no
	// result at all, and the client reports that as a failure — which is how a status
	// report on an empty project came to look like a crash.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true}

	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Args: []string{"status"},
		Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !reply.DidFinish {
		t.Fatal("status warned and ended without a result")
	}
	if len(reply.Warnings) == 0 {
		t.Error("status said nothing about a project with nothing in it")
	}
}

func TestChoosingAHarnessArrangesForIt(t *testing.T) {
	// The link and the base configuration are the part a user cannot see was missing: the
	// documents generate correctly and Claude Code simply never reads them.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items":     []any{"soppy-excalidraw"},
		"harnesses": []any{"claude"},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	target, err := os.Readlink(filepath.Join(canonical(project), "..", ".claude", "skills"))
	if err != nil {
		t.Fatalf("no link was made for Claude Code: %v", err)
	}
	if target != filepath.Join("..", ".agents", "skills") {
		t.Errorf("link target = %q", target)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude", "settings.json")); err != nil {
		t.Errorf("the base configuration was not written: %v", err)
	}
	// A harness that was not chosen is not arranged for.
	if _, err := os.Stat(filepath.Join(project, ".pi")); err == nil {
		t.Error(".pi was created although Pi was not chosen")
	}

	led, err := readLedger(project)
	if err != nil {
		t.Fatalf("readLedger: %v", err)
	}
	if len(led.Harnesses) != 1 || led.Harnesses[0] != "claude" {
		t.Errorf("harnesses = %v, want the choice recorded so the next run remembers it",
			led.Harnesses)
	}
}

func TestAProjectWithoutAChosenHarnessGetsNoDirectories(t *testing.T) {
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go"}, "harnesses": []any{},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, dir := range []string{".claude", ".codex", ".pi"} {
		if _, err := os.Stat(filepath.Join(project, dir)); err == nil {
			t.Errorf("%s was created although the harness was not chosen", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(canonical(project), "prompts", "conduct-of-go.md")); err != nil {
		t.Errorf("the document was not written: %v", err)
	}
}

func TestTheIndexListsWhatWasChosen(t *testing.T) {
	// The one file a project references from its instruction file. If it does not name a
	// document, the agent never learns that document exists.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go", "conduct-of-commit"}, "harnesses": []any{},
		"lint-commits": true,
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(canonical(project), "SPU.md"))
	if err != nil {
		t.Fatalf("no index was written: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"@prompts/conduct-of-commit.md",
		"@prompts/conduct-of-go.md",
		"This project's agent text is managed by spu",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the index does not mention %q:\n%s", want, text)
		}
	}
	// An item that was not chosen is not in the index.
	if strings.Contains(text, "conduct-of-docs") {
		t.Error("the index names an item that was not chosen")
	}
	// The marker is a placeholder, not something the reader should see.
	if strings.Contains(text, "<!--") {
		t.Errorf("the placeholder was left in the file:\n%s", text)
	}
	// Every path it names is relative to the index, so the `@` resolves.
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "`@") {
			continue
		}
		p := strings.SplitN(strings.SplitN(line, "`@", 2)[1], "`", 2)[0]
		if _, err := os.Stat(filepath.Join(canonical(project), filepath.FromSlash(p))); err != nil {
			t.Errorf("the index points at %q, which is not there: %v", p, err)
		}
	}
}

func TestTheIndexIsNotSomethingToChoose(t *testing.T) {
	// It is generated from the selection, so offering it in the picker would ask a person
	// to choose a list of what they are choosing.
	items, err := content.Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	for _, it := range items {
		if it.ID == "spu-index" {
			t.Fatal("the index is offered as a choosable item")
		}
	}
}

func TestStatusDoesNotReportTheIndexAsUnknown(t *testing.T) {
	// The index is written by the generator, not by an item, so it is not in the list of
	// items to look up. Reporting it as one spu does not know is a warning about a file
	// spu wrote itself, which teaches the reader to ignore the warnings.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go"}, "harnesses": []any{},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	report := &coretest.Recorder{Askable: true}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Args: []string{"status"},
		Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, report); err != nil {
		t.Fatalf("status: %v", err)
	}
	said := strings.Join(report.Notices, "\n")
	if strings.Contains(said, "does not know") {
		t.Errorf("status reported a file spu wrote as unknown:\n%s", said)
	}
	if strings.TrimSpace(said) != "" {
		t.Errorf("status reported something about a healthy project:\n%s", said)
	}
}

func TestChoosingAHarnessRegistersTheMCPServer(t *testing.T) {
	// The three registrations are three different files in two formats. Claude Code reads
	// a file at the project root, Pi reads one under .pi/, and Codex reads TOML.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items":     []any{"conduct-of-go"},
		"harnesses": []any{"claude", "pi", "codex"},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, path := range []string{
		filepath.Join(project, ".mcp.json"),
		filepath.Join(project, ".pi", "mcp.json"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("no registration at %s: %v", path, err)
			continue
		}
		if !strings.Contains(string(body), "spu-clone-third-party") {
			t.Errorf("%s does not name the binary:\n%s", path, body)
		}
	}

	codex, err := os.ReadFile(filepath.Join(project, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("no Codex registration: %v", err)
	}
	if !strings.Contains(string(codex), "[mcp_servers.clone-third-party]") {
		t.Errorf("the Codex registration is not a TOML table:\n%s", codex)
	}

	record, err := os.ReadFile(filepath.Join(canonical(project), "mcp", "spu.json"))
	if err != nil {
		t.Fatalf("the canonical definition was not written: %v", err)
	}
	if !strings.Contains(string(record), "clone-third-party") {
		t.Errorf("the record does not name the server:\n%s", record)
	}
	for _, want := range []string{"claude", "pi", "codex"} {
		if !strings.Contains(string(record), want) {
			t.Errorf("the record does not say it was registered with %s:\n%s", want, record)
		}
	}
}

func TestNoHarnessMeansNoRegistration(t *testing.T) {
	// A project that chose no harness gets no .mcp.json at its root. Writing one would be
	// answering a question nobody asked.
	project := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go"}, "harnesses": []any{},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: project},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".mcp.json")); err == nil {
		t.Error("a registration was written although no harness was chosen")
	}
	if _, err := os.Stat(filepath.Join(canonical(project), "mcp")); err == nil {
		t.Error("the MCP record was written although no harness was chosen")
	}
}

func TestTheCacheSectionOnlyAppearsWhenTheServerIsRegistered(t *testing.T) {
	// Telling a project how to protect a cache it does not use is noise, and noise in the
	// one file an agent reads is worse than elsewhere.
	withServer := t.TempDir()
	reply := &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go"}, "harnesses": []any{"claude"},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: withServer},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(canonical(withServer), "SPU.md"))
	if err != nil {
		t.Fatalf("no index: %v", err)
	}
	if !strings.Contains(string(body), "reference cache") {
		t.Errorf("the cache section is missing although the server was registered:\n%s", body)
	}
	if strings.Contains(string(body), "<!--") {
		t.Errorf("a placeholder was left behind:\n%s", body)
	}

	withoutServer := t.TempDir()
	reply = &coretest.Recorder{Askable: true, Answers: map[string]any{
		"items": []any{"conduct-of-go"}, "harnesses": []any{},
	}}
	if err := (Handler{}).Run(context.Background(), core.Command{
		Util: "agent", Session: core.Session{GOOS: "linux", WorkingDir: withoutServer},
	}, reply); err != nil {
		t.Fatalf("generate: %v", err)
	}
	body, err = os.ReadFile(filepath.Join(canonical(withoutServer), "SPU.md"))
	if err != nil {
		t.Fatalf("no index: %v", err)
	}
	if strings.Contains(string(body), "reference cache") {
		t.Errorf("the cache section is there although no server was registered:\n%s", body)
	}
	if strings.Contains(string(body), "## \n") || strings.Contains(string(body), "\n\n\n\n") {
		t.Errorf("dropping the section left a hole:\n%s", body)
	}
}
