package license

import (
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// Register adds license's capability.
func Register(r *registry.Registry) error {
	return r.Register(registry.Capability{
		ID:       "license",
		Title:    "License",
		Usage:    "spu license [--mit|--isc] [--name NAME] [--year YEAR] [--output FILE]",
		Commands: commands,
		// No judge: it depends on nothing beyond the filesystem. The author falls back to
		// the user when git is absent or has no name set, so a missing git is not a
		// reason to report it degraded.
	})
}

var commands = []registry.Command{
	{Name: "edit", Summary: "Change the saved defaults"},
	{Name: "migrate", Summary: "Adopt an older configuration", Flags: []registry.Flag{
		{Name: "--answers", Arg: "path", Summary: "Read the values from a file instead of asking"},
	}},
	{Name: "purge", Summary: "Remove the saved defaults"},
}
