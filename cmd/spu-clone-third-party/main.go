// Command spu-clone-third-party is an MCP server that hands an agent a local checkout of
// a third-party package to read.
//
// It is a separate binary on purpose. It is not a mode of `spu`: it needs no daemon, it
// speaks somebody else's protocol, and a harness spawns it directly. Keeping it out of
// `spu` means a fault in it cannot take a person's shell tools down with it.
package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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

// fetchFlow is the shell script that does the fetching, or git itself.
//
// A script rather than a hardcoded clone so that a mirror, a token, or a vendored copy
// can be used without rebuilding anything. The default is the least surprising thing:
// clone the URL at the ref, into the destination.
func fetchFlow() thirdparty.Fetch {
	return func(ctx context.Context, url, dest, ref string) error {
		if script := os.Getenv("SPU_REFERENCE_FETCH"); script != "" {
			cmd := exec.CommandContext(ctx, script, url, dest, ref)
			cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
			return cmd.Run()
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		args := []string{"clone", "--depth", "1"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, url, dest)
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		// Recorded after the clone rather than before, so a checkout that failed is not
		// left looking like one that succeeded.
		return thirdparty.RecordProvenance(dest, url, ref)
	}
}
