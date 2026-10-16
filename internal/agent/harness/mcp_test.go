package harness

import (
	"encoding/json"

	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func servers() []Server {
	return []Server{{Name: "clone-third-party", Command: "spu-clone-third-party"}}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("%s is not JSON after we wrote it: %v\n%s", path, err, body)
	}
	return got
}

func TestClaudeRegistrationIsTheRootMcpJSON(t *testing.T) {
	// Not under .claude/: Claude Code reads the project registration from .mcp.json at the
	// project root, which is why it cannot be a link into .agents/.
	project := t.TempDir()
	done, err := RegisterMCP(project, []string{"claude"}, servers())
	if err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	got := readJSON(t, filepath.Join(project, ".mcp.json"))
	named, _ := got["mcpServers"].(map[string]any)
	entry, ok := named["clone-third-party"].(map[string]any)
	if !ok {
		t.Fatalf("no entry was written: %v", got)
	}
	if entry["command"] != "spu-clone-third-party" {
		t.Errorf("command = %v, want the binary by name, so a registration committed to a "+
			"repository works on any machine that installed spu", entry["command"])
	}
	if !contains(done, "registered clone-third-party for claude") {
		t.Errorf("the registration was not reported: %v", done)
	}
}

func TestRegisteringKeepsTheUsersOwnServers(t *testing.T) {
	// .mcp.json is a map of servers, and some of them are not ours. Writing our entry
	// means writing our entry only.
	project := t.TempDir()
	original := `{"mcpServers":{"mine":{"command":"my-server","args":["--x"]}},"otherKey":true}`
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := RegisterMCP(project, []string{"claude"}, servers()); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	got := readJSON(t, filepath.Join(project, ".mcp.json"))
	named, _ := got["mcpServers"].(map[string]any)
	if _, ok := named["mine"]; !ok {
		t.Error("a server the user registered was dropped")
	}
	if _, ok := named["clone-third-party"]; !ok {
		t.Error("our server was not added")
	}
	if got["otherKey"] != true {
		t.Error("an unrelated key in the file was dropped")
	}
}

func TestARegistrationFileThatIsNotJSONIsRefused(t *testing.T) {
	// Somebody's file, in some other dialect, or half-written. Rewriting it would discard
	// whatever they meant by it.
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := RegisterMCP(project, []string{"claude"}, servers())
	if err == nil {
		t.Fatal("a malformed registration file was overwritten")
	}
	if !strings.Contains(err.Error(), ".mcp.json") {
		t.Errorf("error %q does not name the file", err)
	}
	body, _ := os.ReadFile(filepath.Join(project, ".mcp.json"))
	if string(body) != "{ not json" {
		t.Errorf("the file was changed: %q", body)
	}
}

func TestPiRegistrationIsItsOwnFile(t *testing.T) {
	project := t.TempDir()
	if _, err := RegisterMCP(project, []string{"pi"}, servers()); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}
	got := readJSON(t, filepath.Join(project, ".pi", "mcp.json"))
	if _, ok := got["mcpServers"].(map[string]any)["clone-third-party"]; !ok {
		t.Errorf("Pi reads .pi/mcp.json and the entry is not there: %v", got)
	}
}

func TestCodexRegistrationIsTOML(t *testing.T) {
	project := t.TempDir()
	if _, err := RegisterMCP(project, []string{"codex"}, servers()); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(project, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("no Codex registration was written: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "[mcp_servers.clone-third-party]") {
		t.Errorf("the TOML table is missing:\n%s", text)
	}
	if !strings.Contains(text, `command = "spu-clone-third-party"`) {
		t.Errorf("the command is missing:\n%s", text)
	}
}

func TestCodexRegistrationMergesIntoAnExistingConfig(t *testing.T) {
	// .codex/config.toml is the project's own configuration, and the entry is added to it
	// rather than the file being rewritten. A rewrite would drop the comments and the
	// ordering somebody put there for their own reasons.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	original := `# Kept for the sandbox review, see the ticket.
sandbox_mode = "workspace-write"

[projects."/somewhere"]
trust_level = "trusted"
`
	path := filepath.Join(project, ".codex", "config.toml")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	done, err := RegisterMCP(project, []string{"codex"}, servers())
	if err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(body)

	// Everything that was there is still there, byte for byte.
	if !strings.Contains(text, original) {
		t.Errorf("the project's own config was not preserved:\n%s", text)
	}
	if !strings.Contains(text, "[mcp_servers.clone-third-party]") {
		t.Errorf("the server was not added:\n%s", text)
	}
	if !strings.Contains(text, `command = "spu-clone-third-party"`) {
		t.Errorf("the command is missing:\n%s", text)
	}
	if !contains(done, "registered clone-third-party for codex") {
		t.Errorf("the registration was not reported: %v", done)
	}

	// And the result is still TOML that Codex can read.
	var parsed map[string]any
	if _, err := toml.Decode(text, &parsed); err != nil {
		t.Fatalf("the file we wrote is not valid TOML: %v\n%s", err, text)
	}
	servers, _ := parsed["mcp_servers"].(map[string]any)
	if _, ok := servers["clone-third-party"]; !ok {
		t.Errorf("the table did not end up under mcp_servers: %v", parsed)
	}
	if parsed["sandbox_mode"] != "workspace-write" {
		t.Errorf("an existing key was lost: %v", parsed)
	}
}

func TestCodexConfigThatIsNotValidTOMLIsRefused(t *testing.T) {
	// Somebody's file, half-written or broken. Adding to it would make it worse, and
	// rewriting it would discard whatever they meant.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	original := "sandbox_mode = [unclosed\n"
	path := filepath.Join(project, ".codex", "config.toml")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := RegisterMCP(project, []string{"codex"}, servers())
	if err == nil {
		t.Fatal("a broken config was written to")
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("error %q does not name the file", err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != original {
		t.Errorf("the file was changed: %q", body)
	}
}

func TestCodexRegistrationRemovesItsTableWhenDeselected(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	kept := "sandbox_mode = \"read-only\"\n"
	if err := os.WriteFile(path, []byte(kept), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := RegisterMCP(project, []string{"codex"}, servers()); err != nil {
		t.Fatalf("register: %v", err)
	}

	removed, err := UnregisterMCP(project, []string{"codex"}, servers())
	if err != nil {
		t.Fatalf("UnregisterMCP: %v", err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "mcp_servers") {
		t.Errorf("our table was left behind:\n%s", body)
	}
	if !strings.Contains(string(body), `sandbox_mode = "read-only"`) {
		t.Errorf("the project's own setting was removed too:\n%s", body)
	}
	if len(removed) == 0 {
		t.Error("the removal was not reported")
	}
}

func TestCodexRegistrationLeavesAHandEditAlone(t *testing.T) {
	// The table is there with different values: somebody changed it on purpose, or this
	// build would write something else. Either way it is not ours to overwrite.
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	original := "[mcp_servers.clone-third-party]\ncommand = \"/somewhere/else\"\n"
	path := filepath.Join(project, ".codex", "config.toml")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	done, err := RegisterMCP(project, []string{"codex"}, servers())
	if err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != original {
		t.Errorf("a table somebody set was overwritten:\n%s", body)
	}
	if !strings.Contains(strings.Join(done, "\n"), "different values") {
		t.Errorf("nothing said the table was already there with other values: %v", done)
	}
}

func TestRegisteringTwiceChangesNothingTheSecondTime(t *testing.T) {
	project := t.TempDir()
	if _, err := RegisterMCP(project, []string{"claude", "pi"}, servers()); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(project, ".mcp.json"))

	second, err := RegisterMCP(project, []string{"claude", "pi"}, servers())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("a second run reported work: %v", second)
	}
	again, _ := os.ReadFile(filepath.Join(project, ".mcp.json"))
	if string(first) != string(again) {
		t.Error("a file that already said the right thing was rewritten")
	}
}

func TestDeselectingAHarnessRemovesOurRegistration(t *testing.T) {
	// A registration that outlives the choice keeps offering an agent a tool the project
	// no longer uses. Only our own entry is removed; everything else in the file stays.
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"),
		[]byte(`{"mcpServers":{"mine":{"command":"x"}}}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := RegisterMCP(project, []string{"claude"}, servers()); err != nil {
		t.Fatalf("register: %v", err)
	}

	removed, err := UnregisterMCP(project, []string{"claude"}, servers())
	if err != nil {
		t.Fatalf("UnregisterMCP: %v", err)
	}
	got := readJSON(t, filepath.Join(project, ".mcp.json"))
	named, _ := got["mcpServers"].(map[string]any)
	if _, ok := named["clone-third-party"]; ok {
		t.Error("our registration was left behind")
	}
	if _, ok := named["mine"]; !ok {
		t.Error("the user's own server was removed along with ours")
	}
	if len(removed) == 0 {
		t.Error("the removal was not reported")
	}
}

func TestUnregisteringLeavesAFileThatWasNeverOurs(t *testing.T) {
	project := t.TempDir()
	removed, err := UnregisterMCP(project, []string{"claude", "pi", "codex"}, servers())
	if err != nil {
		t.Fatalf("UnregisterMCP: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("removing from files that do not exist reported work: %v", removed)
	}
	if _, err := os.Stat(filepath.Join(project, ".mcp.json")); err == nil {
		t.Error("unregistering created a file")
	}
}
