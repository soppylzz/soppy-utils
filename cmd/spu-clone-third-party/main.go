// Command spu-clone-third-party is an MCP server that hands an agent a local checkout of
// a third-party package to read.
//
// It is a separate binary on purpose. It is not a mode of `spu`: it needs no daemon, it
// speaks somebody else's protocol, and a harness spawns it directly. Keeping it out of
// `spu` means a fault in it cannot take a person's shell tools down with it.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/thirdparty"
)

func main() {
	// Nothing may be written to stdout except protocol messages: a stray line breaks the
	// stream, and the harness reads the break as the server being broken.
	log.SetOutput(os.Stderr)

	os.Exit(run(context.Background(), os.Stdin, os.Stdout))
}

func run(ctx context.Context, in io.Reader, out io.Writer) int {
	r := thirdparty.Resolver{
		CacheDir: cacheDir(),
		Tags:     tagsFlow(),
		Fetch:    fetchFlow(),
		Log:      log.Writer(),
	}
	if err := thirdparty.Serve(ctx, in, out, r); err != nil {
		clioutput.Error("clone-third-party", err.Error())
		return 1
	}
	return 0
}

// cacheDir is where checkouts are kept.
//
// Configurable because the default is a person's own directory and an agent should be
// able to read the same reference material from wherever it already lives.
func cacheDir() string {
	if dir := os.Getenv("SPU_REFERENCE_CACHE"); dir != "" {
		return dir
	}
	return thirdparty.DefaultCacheDir
}

// tagsFlow lists the version tags a remote publishes.
//
// It is what makes a version mean something before anything is cloned: the caller says
// `8`, this says the newest 8.x is v8.1.0, and v8.1.0 is what git is asked for. It runs
// only when the cache cannot answer, so a hit still touches no network.
func tagsFlow() thirdparty.Tags {
	return func(ctx context.Context, url string) ([]string, error) {
		out, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", url).Output()
		if err != nil {
			return nil, err
		}
		var tags []string
		for _, line := range strings.Split(string(out), "\n") {
			_, ref, found := strings.Cut(line, "\t")
			if !found {
				continue
			}
			if name, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
				tags = append(tags, name)
			}
		}
		return tags, nil
	}
}

// fetchFlow is the shell script that does the fetching, or git itself.
//
// A script rather than a hardcoded clone so that a mirror, a token, or a vendored copy
// can be used without rebuilding anything. The default is the least surprising thing:
// clone the URL at the ref, into the destination.
func fetchFlow() thirdparty.Fetch {
	return func(ctx context.Context, url, dest, ref string) (string, error) {
		if script := os.Getenv("SPU_REFERENCE_FETCH"); script != "" {
			cmd := exec.CommandContext(ctx, script, url, dest, ref)
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			if err := cmd.Run(); err != nil {
				return "", err
			}
			return checkedOut(ctx, dest, ref)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", err
		}

		// git refuses to clone into a directory that is not empty, and the refusal used to
		// be the error every second call ended with. Whatever is here was put here by an
		// earlier run, and it is reported rather than deleted: it is not this program's
		// place to remove something it cannot account for.
		if entries, err := os.ReadDir(dest); err == nil && len(entries) > 0 {
			return "", fmt.Errorf("%s already holds a checkout but nothing recorded what "+
				"version it is; remove it and run again", dest)
		}

		args := []string{"clone", "--depth", "1"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, url, dest)
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return checkedOut(ctx, dest, ref)
	}
}

// checkedOut reports the version a checkout holds.
//
// A ref that was asked for is the answer, because the ref was resolved from the remote's
// own tags before the clone and the clone succeeded. A checkout of the default branch has
// no version of its own: a shallow clone carries no tags, so describe cannot name it and
// the commit is what it actually is.
func checkedOut(ctx context.Context, dest, ref string) (string, error) {
	if ref != "" {
		return ref, nil
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dest, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("read the commit that was checked out: %w", err)
	}
	return "commit:" + strings.TrimSpace(string(out)), nil
}
