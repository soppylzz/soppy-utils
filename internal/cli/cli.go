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

	"github.com/soppylzz/soppy-utils/internal/agent"
	"github.com/soppylzz/soppy-utils/internal/client"
	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/gitconfig"
	"github.com/soppylzz/soppy-utils/internal/license"
	"github.com/soppylzz/soppy-utils/internal/registry"
	"github.com/soppylzz/soppy-utils/internal/transport"
	"github.com/soppylzz/soppy-utils/internal/vscode"
	"github.com/soppylzz/soppy-utils/internal/welcome"
)

// utils is the list of utils core knows about.
//
// The registry a client reads and the dispatch that runs a command are both derived
// from this one list, so a util cannot exist in one and be missing from the other. That
// is the property the three hand-maintained lists of the previous dispatcher did not
// have, and a test checks it.
//
// Every util is here. vscode is the exception and lives in Bash, because it only opens
// files with the code command and has nothing for core to own.
var utils = []struct {
	id       string
	register func(*registry.Registry) error
	handler  core.Handler
}{
	{id: "welcome", register: welcome.Register, handler: welcome.Handler{}},
	{id: "license", register: license.Register, handler: license.Handler{}},
	{id: "git", register: gitconfig.Register, handler: gitconfig.Handler{}},
	{id: "agent", register: agent.Register, handler: agent.Handler{}},
	{id: "vscode", register: vscode.Register, handler: vscode.Handler{}},
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

	// Help is rendered here from the registry, not by core and not from a list beside it.
	// A util's options are described by the capability that owns them, so what `--help`
	// says and what the argument parser accepts cannot disagree.
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		r, err := buildRegistry()
		if err != nil {
			return 1, err
		}
		return 0, helpFor(r, os.Stdout)
	}
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") && known(args[0]) {
		r, err := buildRegistry()
		if err != nil {
			return 1, err
		}
		if args[1] == "-h" {
			return 0, briefFor(r, os.Stdout, args[0])
		}
		return 0, utilHelpFor(r, os.Stdout, args[0])
	}

	home, err := configstore.Home()
	if err != nil {
		return 1, err
	}

	switch args[0] {
	case "__core":
		return coreCommand(home, version, args[1:])

	case "config-status":
		return 0, configStatus(home)
	}

	// Everything core knows about goes to core. Asking the same list the registry is
	// built from means a util a client can see is a util that can be run, and neither
	// list can drift from the other.
	if known(args[0]) {
		return runThroughCore(home, args)
	}
	return 1, fmt.Errorf("unknown util %q", args[0])
}

func known(id string) bool {
	for _, u := range utils {
		if u.id == id {
			return true
		}
	}
	return false
}

// coreCommand is the daemon role, and the way to ask a running one to stop.
//
// Stopping is its own mode rather than something a client run does, because the callers
// that need it are the scripts about to replace or delete the installation. Going
// through a client run would start a daemon in order to stop one, which is the wrong
// direction and, during an upgrade, the wrong binary.
func coreCommand(home, version string, args []string) (int, error) {
	if len(args) == 0 {
		return 1, errors.New("__core needs a mode: serve or shutdown")
	}

	ep, err := transport.Resolve(runtime.GOOS, home, os.Getenv("SPU_CORE_ENDPOINT"))
	if err != nil {
		return 1, err
	}

	switch args[0] {
	case "serve":
		return 0, serve(home, version, ep)
	case "shutdown":
		return 0, cored.Shutdown(context.Background(), ep, shutdownBudget)
	}
	return 1, fmt.Errorf("unknown core mode %q", args[0])
}

// idleTimeout is how long a daemon stays resident with nobody connected.
//
// Overridable because the test suite starts one per spec and cannot wait fifteen minutes
// for each to leave. An unusable value falls back rather than being an error: a daemon
// that refuses to start over a malformed environment variable is worse than one that
// lives longer than asked.
func idleTimeout() time.Duration {
	const fallback = 15 * time.Minute
	raw := os.Getenv("SPU_CORE_IDLE_TIMEOUT")
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// shutdownBudget is how long the scripts wait for a daemon to finish. It has to outlast
// the drain grace the daemon was started with, or a shutdown with a session in flight
// would report failure while it was still doing the right thing.
const shutdownBudget = 10 * time.Second

func serve(home, version string, ep transport.Endpoint) error {
	welcome.Version = version

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
		IdleTimeout: idleTimeout(),
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
		Spawn:    []string{self, "__core", "serve"},
		LogPath:  filepath.Join(home, ".config", "soppy-utils", "core.log"),
		Budget:   5 * time.Second,
		Kind:     "tui",
		Capabilities: cored.Capabilities{
			// The client can always run a program: that is what a shell is. What it
			// cannot always do is ask, and the two are different questions — a script
			// that runs spu can open a file in the editor, it just cannot be asked
			// whether it wants a package installed.
			ActionExecution: true,
			Confirm:         interactive,
			Interactive:     interactive,
		},
		Env: envFor(),
	})
	if err != nil {
		return 1, err
	}
	defer conn.Close()

	c := client.New(conn, os.Stdin, os.Stdout, os.Stderr)
	c.Interactive = interactive
	return c.Run(ctx, args[0], args[1:])
}

// envFor is what this client reports about the machine it is running on.
//
// It is built here rather than in the daemon because every field is a fact about the
// client's shell: the daemon outlives whoever started it, so its own environment is an
// answer about a different process, and it never learns that it was wrong.
//
// The working directory is read rather than refused when it cannot be: a command that
// needs a project asks for one and says so, and the rest do not care.
func envFor() cored.Env {
	workdir, _ := os.Getwd()
	return cored.Env{
		GOOS: runtime.GOOS,
		// The shell that will run the banner is the one the user is in, which is what
		// the generated hook has to be written for.
		Shell:      filepath.Base(os.Getenv("SHELL")),
		Path:       os.Getenv("PATH"),
		Home:       os.Getenv("HOME"),
		HomeNative: os.Getenv("SPU_HOME_NATIVE"),
		Tmux:       os.Getenv("TMUX"),
		AppData:    os.Getenv("APPDATA"),

		WorkingDir:       workdir,
		WorkingDirNative: os.Getenv("SPU_WORKDIR_NATIVE"),
	}
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
