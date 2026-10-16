package gitconfig

import (
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// Register adds git's capability.
func Register(r *registry.Registry) error {
	return r.Register(registry.Capability{
		ID:       "git",
		Title:    "Global Git defaults",
		Usage:    "spu git install|uninstall [--purge]|edit|migrate|status",
		Commands: commands,
		Judge:    judge,
	})
}

var commands = []registry.Command{
	{Name: "install", Summary: "Write the managed config and register it with Git", Flags: []registry.Flag{
		{Name: "--editor", Arg: "string", Summary: "The editor Git opens"},
		{Name: "--autocrlf", Arg: "enum", Summary: "Line-ending handling",
			Options: []string{"input", "true", "false"}},
		{Name: "--diff-tool", Arg: "string", Summary: "The diff tool Git uses"},
		{Name: "--merge-tool", Arg: "string", Summary: "The merge tool Git uses"},
	}},
	{Name: "edit", Summary: "Change the saved defaults and rewrite the managed config"},
	{Name: "migrate", Summary: "Adopt an older configuration", Flags: []registry.Flag{
		{Name: "--answers", Arg: "path", Summary: "Read the values from a file instead of asking"},
	}},
	{Name: "status", Summary: "Report the configuration and whether Git still includes it"},
	{Name: "uninstall", Summary: "Remove the managed config and its include", Flags: []registry.Flag{
		{Name: "--purge", Arg: "bool", Summary: "Also remove an unchanged owned configuration"},
	}},
}

// judge reports whether the global Git configuration can be managed here.
//
// The whole util is about git's own configuration, and every operation it performs runs
// git. Without it there is nothing to do, and saying so is better than failing on the
// first command, which is the shape of the answer a user can act on.
func judge(h registry.Host) registry.Assessment {
	if h.LookPath == nil {
		return registry.Assessment{
			State: registry.Unavailable, Reason: registry.MissingDependency,
			Missing: []string{"git"}, Detail: "managing global Git configuration needs git",
		}
	}
	if _, err := h.LookPath("git"); err != nil {
		return registry.Assessment{
			State: registry.Unavailable, Reason: registry.MissingDependency,
			Missing: []string{"git"}, Detail: "managing global Git configuration needs git",
		}
	}
	return registry.Assessment{State: registry.Available}
}
