// Package agent generates a project's agent-facing text.
//
// The work is in three separable pieces and they are kept that way: deciding what each
// file needs is a pure function of facts (decide), producing the bytes is a pure function
// of answers (render), and this is the part that reads the filesystem, keeps the ledger,
// and asks whoever is there.
package agent

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/harness"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// Handler runs the agent commands.
type Handler struct{}

// Run dispatches one command.
func (Handler) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	project, err := projectOf(cmd.Session)
	if err != nil {
		return err
	}
	sub := ""
	var rest []string
	if len(cmd.Args) > 0 {
		sub = cmd.Args[0]
		rest = cmd.Args[1:]
		// A leading flag belongs to the run itself. `spu agent --items x` names no
		// subcommand, and reading `--items` as one turned every flag into an unknown
		// command.
		if strings.HasPrefix(sub, "-") {
			sub, rest = "", cmd.Args
		}
	}
	switch sub {
	case "":
		return generate(ctx, project, configDir(cmd.Session), rest, reply)
	case "status":
		return status(project, configDir(cmd.Session), rest, reply)
	case "edit":
		return edit(ctx, project, configDir(cmd.Session), rest, reply)
	case "migrate":
		return migrate(project, configDir(cmd.Session), rest, reply)
	case "import":
		return importItem(configDir(cmd.Session), rest, reply)
	case "remove":
		return removeItem(configDir(cmd.Session), rest, reply)
	default:
		return fmt.Errorf("unknown agent command %q; run spu agent --help", sub)
	}
}

// projectOf finds the project the client was standing in.
//
// An empty working directory means the client could not tell us, and guessing would
// generate into the daemon's own directory, which belongs to whoever started it. Saying
// so costs one message; guessing writes files into a stranger's project.
func projectOf(s core.Session) (string, error) {
	if s.WorkingDir == "" {
		return "", errors.New("this client did not say which directory it was run from, " +
			"so there is no project to generate into")
	}
	return harness.Project(s.WorkingDir)
}

// canonical is the directory everything is recorded against.
func canonical(project string) string {
	return harness.Dir(project, item.RootAgents)
}

func filter(items []item.Source, keep []string) []item.Source {
	want := make(map[string]bool, len(keep))
	for _, id := range keep {
		want[id] = true
	}
	out := make([]item.Source, 0, len(items))
	for _, src := range items {
		if want[src.Item.ID] {
			out = append(out, src)
		}
	}
	return out
}

// acceptNoFlags is for a subcommand that takes none.
//
// Silently ignoring an argument is how `spu agent status --purge` comes to look like it
// did something.
func acceptNoFlags(name string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		// The flag package says "flag provided but not defined", which is true and does
		// not say that this subcommand has no flags at all.
		return fmt.Errorf("%s takes no flags; run spu %s --help", name, name)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s takes no arguments, but was given %s", name,
			strings.Join(fs.Args(), " "))
	}
	return nil
}

// missing returns the names in from that are not in against.
//
// Used for the harnesses a project has stopped using: a registration that outlives the
// choice keeps offering an agent a tool the project no longer wants.
func missing(from, against []string) []string {
	have := make(map[string]bool, len(against))
	for _, h := range against {
		have[h] = true
	}
	var out []string
	for _, h := range from {
		if !have[h] {
			out = append(out, h)
		}
	}
	return out
}
