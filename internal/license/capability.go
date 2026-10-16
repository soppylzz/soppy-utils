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
	{Name: "", Summary: "Write a license to stdout, or to a file with --output",
		Flags: []registry.Flag{
			{Name: "--mit", Arg: "bool", Summary: "Write the MIT license"},
			{Name: "--isc", Arg: "bool", Summary: "Write the ISC license"},
			{Name: "-M", Arg: "bool", Summary: "Short for --mit"},
			{Name: "-I", Arg: "bool", Summary: "Short for --isc"},
			{Name: "--name", Arg: "string", Summary: "The copyright holder"},
			{Name: "--year", Arg: "string", Summary: "The copyright year"},
			{Name: "--output", Arg: "path", Summary: "Write to this file instead of stdout"},
			{Name: "-o", Arg: "path", Summary: "Short for --output"},
			{Name: "--interactive", Arg: "bool", Summary: "Open the form even when the flags are complete"},
		}},
	{Name: "edit", Summary: "Change the saved defaults", Flags: []registry.Flag{
		{Name: "--answers", Arg: "path", Summary: "Read the values from a file instead of asking"},
	}},
	{Name: "migrate", Summary: "Adopt an older configuration", Flags: []registry.Flag{
		{Name: "--answers", Arg: "path", Summary: "Read the values from a file instead of asking"},
	}},
	{Name: "purge", Summary: "Remove the saved defaults"},
}
