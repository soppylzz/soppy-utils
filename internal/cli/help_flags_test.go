package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// flagPackages maps a util to the package that owns its flags.
//
// It is a list rather than something derived, because the registry deliberately does not
// know where a util lives: `cli` is the composition root and knows the list, and the list
// of directories is the same decision as the list of utils.
var flagPackages = map[string]string{
	"welcome": "internal/welcome",
	"license": "internal/license",
	"git":     "internal/gitconfig",
	"agent":   "internal/agent",
	"vscode":  "internal/vscode",
}

// parsedFlag finds the flags a package hands to the flag package.
var parsedFlag = regexp.MustCompile(`fs\.(?:String|Bool|Int|StringVar|BoolVar|IntVar)\(\s*(?:[a-zA-Z]+,\s*)?"([a-zA-Z0-9-]+)"`)

func TestEveryFlagACommandParsesIsDocumented(t *testing.T) {
	// Help renders from the registry, so a flag the parser accepts and the registry does
	// not mention is a flag nobody can find. It happened: `spu license --help` listed
	// none of the nine flags the command takes, and nothing connected the two lists.
	r, err := buildRegistry()
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}

	for _, c := range r.List() {
		dir, known := flagPackages[c.ID]
		if !known {
			t.Errorf("util %q has no package listed, so its flags are not checked", c.ID)
			continue
		}

		declared := map[string]bool{}
		for _, cmd := range c.Commands {
			for _, f := range cmd.Flags {
				declared[strings.TrimLeft(f.Name, "-")] = true
			}
		}

		for _, name := range flagsParsedIn(t, dir) {
			if !declared[name] {
				t.Errorf("%s accepts --%s but its help does not mention it", c.ID, name)
			}
		}
	}
}

// repoRoot is the module root, which is two directories up from this package.
//
// A Go test runs with its own package as the working directory, so a path that reads well
// in the repository is wrong here.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("find the module root: %v", err)
	}
	return root
}

func flagsParsedIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), dir))
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(repoRoot(t), dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range parsedFlag.FindAllStringSubmatch(string(body), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

func TestHelpNamesTheValuesAListFlagTakes(t *testing.T) {
	// A flag whose values only appear in an error message is a flag people get wrong
	// first. The harnesses are a closed set, so the help can name them.
	r, err := buildRegistry()
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	c, ok := r.Lookup("agent")
	if !ok {
		t.Fatal("no agent capability")
	}
	var found registry.Flag
	for _, cmd := range c.Commands {
		for _, f := range cmd.Flags {
			if f.Name == "--harnesses" {
				found = f
			}
		}
	}
	if found.Name == "" {
		t.Fatal("--harnesses is not documented at all")
	}
	if len(found.Options) == 0 {
		t.Error("--harnesses does not say which harnesses exist")
	}
	if text := flagText(found); !strings.Contains(text, "claude") {
		t.Errorf("--harnesses reads %q, which does not name the values", text)
	}
}
