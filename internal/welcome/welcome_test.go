package welcome

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderQuotesInputAndStaysSilentWithoutTTY(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "should-not-exist")
	data := map[string]any{"name": "'$(touch " + sentinel + ")", "device": "a\nb", "message": "$HOME `whoami`"}
	script, err := Render(data)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "welcome.sh")
	if err := os.WriteFile(path, script, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", path)
	cmd.Env = append(os.Environ(), "SPU_WELCOME_INTERACTIVE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("noninteractive script failed: %v, output: %.120q", err, out)
	}
	if len(out) != 0 {
		t.Fatalf("noninteractive script wrote %d bytes: %.120q", len(out), out)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("input was executed")
	}
	if !strings.Contains(string(script), ShellQuote(data["name"].(string))) {
		t.Fatal("name is not shell quoted")
	}
	assignments := []string{}
	for _, line := range strings.Split(string(script), "\n") {
		if strings.HasPrefix(line, "_spu_welcome_name=") {
			assignments = append(assignments, line)
		}
	}
	probe := strings.Join(assignments, "\n") + "\nprintf '%s\\n' \"$_spu_welcome_name\"\n"
	actual, err := exec.Command("bash", "-c", probe).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSuffix(string(actual), "\n") != data["name"] {
		t.Fatalf("quoted value changed: %q", actual)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("input was executed")
	}
}

func TestRenderWithRedirectedStreamsAndControllingTTY(t *testing.T) {
	// The banner has to survive stdout being redirected, because Powerlevel10k's instant
	// prompt does exactly that while the shell loads its startup file.
	banner, err := Render(map[string]any{"name": "Alice", "device": "Mac", "message": "Ready"})
	if err != nil {
		t.Fatal(err)
	}
	out := runWithATty(t, banner)
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Ready") {
		t.Fatalf("banner was hidden while standard streams were redirected: %.160q", out)
	}
}

func TestMalformedHooksPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	original := []byte(start + "\n" + start + "\n" + end + "\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := editHook(path, true); err == nil {
		t.Fatal("expected marker error")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("modified malformed hook")
	}
}

func TestWelcomeHookMovesBeforePowerlevel10k(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	preamble := `# Enable Powerlevel10k instant prompt. Should stay close to the top of ~/.zshrc.
# Initialization code that may require console input must go above this block.
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi
`
	original := "export PATH=\"$HOME/bin:$PATH\"\n\n" + preamble + "\nexport EDITOR=vim\n" + hook
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := editHook(path, true); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents := string(updated)
	if strings.Count(contents, start) != 1 || strings.Index(contents, hook) > strings.Index(contents, preamble) {
		t.Fatal("welcome hook was not moved before the instant prompt")
	}
	if !strings.Contains(contents, "export EDITOR=vim\n") {
		t.Fatal("unmanaged zshrc content was removed")
	}
	if err := editHook(path, true); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != contents {
		t.Fatal("reinstall changed the already placed hook")
	}
}

func TestLegacyBannerMigration(t *testing.T) {
	if _, err := exec.LookPath("cksum"); err != nil {
		t.Skip("cksum unavailable")
	}
	home := t.TempDir()
	base := filepath.Join(home, ".config", "soppy-utils")
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("#!/usr/bin/env bash\n# Managed by soppy-utils: welcome\nprintf 'legacy\\n'\n")
	if err := os.WriteFile(filepath.Join(base, "welcome.sh"), legacy, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cksum")
	cmd.Stdin = strings.NewReader(string(legacy))
	checksum, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "welcome.cksum"), checksum, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(legacyHook), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), home,
		[]string{"migrate", "--name", "Alice", "--device", "Mac", "--message", "Ready", "--rc", "bash"},
		io.Discard, &recorder{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "welcome.sh")); !os.IsNotExist(err) {
		t.Fatal("legacy script remains")
	}
	if _, err := os.Stat(scriptPath(Store(home))); err != nil {
		t.Fatal(err)
	}
	rc, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rc), hook) || strings.Contains(string(rc), legacyHook) {
		t.Fatal("legacy hook not replaced")
	}
}
