package gitconfig

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/core/coretest"
)

func TestGlobalIncludeLifecycleAndOwnership(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global.gitconfig"))
	noTerminal := func() *coretest.Recorder { return &coretest.Recorder{} }
	if err := Run(context.Background(), sessionFor(home), []string{"install"}, io.Discard, noTerminal()); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), sessionFor(home), []string{"install"}, io.Discard, noTerminal()); err != nil {
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
	if err := Run(context.Background(), sessionFor(home), []string{"uninstall"}, io.Discard, noTerminal()); err == nil {
		t.Fatal("modified managed file was removed")
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), sessionFor(home), []string{"uninstall"}, io.Discard, noTerminal()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("managed file remains")
	}
	if b, err := git("config", "--global", "--get-all", "include.path"); err == nil && strings.Contains(string(b), path) {
		t.Fatal("include remains")
	}
}
