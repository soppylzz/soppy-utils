// Package cli is the composition root: it decides which role an invocation is, runs it,
// and is the one place that knows which utils exist.
//
// Every spu binary is this package plus a five-line main. One executable with two roles
// is deliberate: a client that cannot reach a daemon starts another copy of itself, so
// the client and the daemon can never be different builds and there is one file to
// replace during an upgrade.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/soppylzz/soppy-utils/internal/client"
	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/gitconfig"
	"github.com/soppylzz/soppy-utils/internal/license"
	"github.com/soppylzz/soppy-utils/internal/registry"
	"github.com/soppylzz/soppy-utils/internal/transport"
	"github.com/soppylzz/soppy-utils/internal/welcome"
)

// utils is the list of utils core knows about.
//
// The registry a client reads and the dispatch that runs a command are both derived
// from this one list, so a util cannot exist in one and be missing from the other. That
// is the property the three hand-maintained lists of the previous dispatcher did not
// have, and a test checks it.
//
// A util joins this list when it moves onto core. license, git, and vscode are still on
// the direct path below, and each leaves that switch as it arrives here.
var utils = []struct {
	id       string
	register func(*registry.Registry) error
	handler  core.Handler
}{
	{id: "welcome", register: welcome.Register, handler: welcome.Handler{}},
}

// Main runs one invocation and returns the status the process should exit with.
func Main(argv []string, version string) int {
	code, err := run(argv[1:], version)
	if err != nil {
		clioutput.Error("spu", err.Error())
		if code == 0 {
			code = 1
		}
	}
	return code
}

func run(args []string, version string) (int, error) {
	if len(args) == 0 {
		return 1, errors.New("missing command")
	}
	if args[0] == "--version" {
		fmt.Println(version)
		return 0, nil
	}

	home, err := configstore.Home()
	if err != nil {
		return 1, err
	}

	switch args[0] {
	case "__core":
		// The daemon role, started by a client that could not reach one.
		welcome.Version = version
		return 0, serve(home, version)

	case "welcome":
		return runThroughCore(home, args)

	case "license", "git":
		// Not moved onto core yet. Slice 2 takes them, and these branches go with them.
		if args[0] == "license" {
			return 0, license.Run(home, args[1:])
		}
		return 0, gitconfig.Run(home, args[1:])

	case "config-status":
		return 0, configStatus(home)
	}

	return 1, fmt.Errorf("unknown command %q", args[0])
}

func serve(home, version string) error {
	ep, err := transport.Resolve(runtime.GOOS, home, os.Getenv("SPU_CORE_ENDPOINT"))
	if err != nil {
		return err
	}

	r := registry.New()
	for _, u := range utils {
		if err := u.register(r); err != nil {
			return err
		}
	}

	d, err := cored.New(cored.Options{
		Endpoint:    ep,
		Registry:    r,
		Version:     version,
		Handler:     dispatch{},
		IdleTimeout: 15 * time.Minute,
		DrainGrace:  5 * time.Second,
		Log:         logFile(home),
	})
	if err != nil {
		return err
	}
	return d.Serve(context.Background())
}

// runThroughCore runs a command the way every command will: through a daemon.
func runThroughCore(home string, args []string) (int, error) {
	ep, err := transport.Resolve(runtime.GOOS, home, os.Getenv("SPU_CORE_ENDPOINT"))
	if err != nil {
		return 1, err
	}
	self, err := os.Executable()
	if err != nil {
		return 1, err
	}

	// Whether a person can be asked is a fact about this process, and the daemon it
	// talks to cannot see one, so it is reported rather than assumed.
	interactive := term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd())

	ctx := context.Background()
	conn, err := cored.Connect(ctx, cored.AttachOptions{
		Endpoint: ep,
		Spawn:    []string{self, "__core"},
		LogPath:  filepath.Join(home, ".config", "soppy-utils", "core.log"),
		Budget:   5 * time.Second,
		Kind:     "tui",
		Capabilities: cored.Capabilities{
			ActionExecution: interactive,
			Confirm:         interactive,
			Interactive:     interactive,
		},
		Env: cored.Env{
			GOOS: runtime.GOOS,
			// The shell that will run the banner is the one the user is in, which is what
			// the generated hook has to be written for.
			Shell:      filepath.Base(os.Getenv("SHELL")),
			Path:       os.Getenv("PATH"),
			Home:       os.Getenv("HOME"),
			HomeNative: os.Getenv("SPU_HOME_NATIVE"),
			Tmux:       os.Getenv("TMUX"),
		},
	})
	if err != nil {
		return 1, err
	}
	defer conn.Close()

	c := client.New(conn, os.Stdin, os.Stdout, os.Stderr)
	c.Interactive = interactive
	return c.Run(ctx, args[0], args[1:])
}

// dispatch routes a command to the util that implements it.
type dispatch struct{}

func (dispatch) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	for _, u := range utils {
		if u.id == cmd.Util {
			return u.handler.Run(ctx, cmd, reply)
		}
	}
	return fmt.Errorf("unknown util %q", cmd.Util)
}

// logFile is where a detached daemon's output goes, since it has no terminal to write to
// and must not inherit the one its client started from.
func logFile(home string) io.Writer {
	dir := filepath.Join(home, ".config", "soppy-utils")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return io.Discard
	}
	f, err := os.OpenFile(filepath.Join(dir, "core.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return io.Discard
	}
	return f
}

func configStatus(home string) error {
	for _, s := range []configstore.Store{license.Store(home), welcome.Store(home), gitconfig.Store(home)} {
		v := s.Read()
		if v.Exists && v.Warning != "" {
			clioutput.Warn(s.Util, v.Warning)
		}
	}
	if welcome.LegacyChecksum(home) {
		clioutput.Warn("welcome", "legacy state found; run 'spu welcome migrate'")
	}
	if gitconfig.LegacyPresent(home) {
		clioutput.Warn("git", "legacy state found; run 'spu git migrate'")
	}
	return nil
}
