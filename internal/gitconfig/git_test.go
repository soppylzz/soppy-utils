package gitconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalIncludeLifecycleAndOwnership(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global.gitconfig"))
	if err := Run(home, []string{"install"}); err != nil {
		t.Fatal(err)
	}
	if err := Run(home, []string{"install"}); err != nil {
		t.Fatal(err)
	}
	b, err := git("config", "--global", "--get-all", "include.path")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), configPath(home)) != 1 {
		t.Fatalf("duplicate include: %s", b)
	}
	path := configPath(home)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(original, []byte("# user edit\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(home, []string{"uninstall"}); err == nil {
		t.Fatal("modified managed file was removed")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := Run(home, []string{"uninstall"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("managed file remains")
	}
	if b, err := git("config", "--global", "--get-all", "include.path"); err == nil && strings.Contains(string(b), path) {
		t.Fatal("include remains")
	}
}
