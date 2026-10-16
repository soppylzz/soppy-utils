package welcome

import (
	"path/filepath"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// The tools and font files the banner can use, named once. The capability that reports
// what is missing and the status that prints it have to agree, and they cannot agree by
// each hardcoding the same string.
const (
	toolFiglet    = "figlet"
	toolFastfetch = "fastfetch"

	speedFontFile = "speed.flf"
	slantFontFile = "smslant.flf"
)

// Register adds welcome's capability.
func Register(r *registry.Registry) error {
	return r.Register(registry.Capability{
		ID:       "welcome",
		Title:    "Welcome banner",
		Usage:    "spu welcome [install|edit|migrate|status|uninstall] [flags]",
		Commands: commands,
		Judge:    judge,
	})
}

// commands is the command surface. Help, usage errors, and unknown-command errors all
// render from this rather than from a second list that has to be kept in step with the
// argument parser.
var commands = []registry.Command{
	{Name: "install", Summary: "Install or refresh the banner", Flags: []registry.Flag{
		{Name: "--name", Arg: "string", Summary: "The name shown in the banner"},
		{Name: "--device", Arg: "string", Summary: "The device name shown below it"},
		{Name: "--message", Arg: "string", Summary: "The line printed under both"},
		{Name: "--rc", Arg: "enum", Summary: "Which startup files to hook",
			Options: []string{"none", "bash", "zsh", "both"}},
		{Name: "--interactive", Arg: "bool", Summary: "Ask even when the flags are complete"},
		{Name: "--answers", Arg: "path", Summary: "Read the values from a file instead of asking"},
	}},
	{Name: "edit", Summary: "Change the saved values and refresh the script"},
	{Name: "migrate", Summary: "Adopt an older configuration and rewrite the script"},
	{Name: "status", Summary: "Report the configuration, the script, and the optional tools"},
	{Name: "uninstall", Summary: "Remove the managed hooks and the generated script", Flags: []registry.Flag{
		{Name: "--purge", Arg: "bool", Summary: "Also remove an unchanged owned configuration"},
	}},
}

// judge reports how usable the banner is on one machine.
//
// It reads only the Host, so it cannot accidentally depend on the daemon's own PATH and
// home. The requirements gate it, never the operating system's name: keying on GOOS
// makes "usable from Git Bash, hidden from PowerShell" inexpressible, and bakes a wrong
// assumption in for good.
//
// There is deliberately no check on which shell is running. The banner hooks a bash or
// zsh startup file, which is a question about which shells the user has rather than
// about which one they are in at this moment — a fish user with zsh installed can use
// it, and judging by $SHELL would hide the capability from somebody who can use it.
func judge(h registry.Host) registry.Assessment {
	switch h.GOOS {
	case "darwin", "linux":
	default:
		// Kept visible and disabled rather than absent. The entry point stays
		// discoverable, and the reason is a statement about this project rather than
		// about the machine.
		return registry.Assessment{
			State:  registry.Unavailable,
			Reason: registry.NotImplemented,
			Detail: "the banner is not written for " + platformName(h.GOOS) + " yet",
		}
	}

	var missing, losses []string

	if !hasTool(h, toolFiglet) {
		missing = append(missing, toolFiglet)
		losses = append(losses, "the banner is plain text instead of FIGlet lettering")
	} else if !hasFont(h, speedFontFile) || !hasFont(h, slantFontFile) {
		missing = append(missing, "its FIGlet fonts")
		losses = append(losses, "the banner is plain text because those fonts are missing")
	}
	if !hasTool(h, toolFastfetch) {
		missing = append(missing, toolFastfetch)
		losses = append(losses, "there is no system summary")
	}

	if len(missing) > 0 {
		// Degraded rather than unavailable: this is the state the current code already
		// reaches, expressed as data instead of as a sentence on stderr that a window
		// client can do nothing with.
		return registry.Assessment{
			State:   registry.Degraded,
			Reason:  registry.MissingDependency,
			Detail:  "without " + strings.Join(missing, " or ") + ", " + strings.Join(losses, " and "),
			Missing: missing,
		}
	}
	return registry.Assessment{State: registry.Available}
}

func hasTool(h registry.Host, name string) bool {
	if h.LookPath == nil {
		return false
	}
	_, err := h.LookPath(name)
	return err == nil
}

func hasFont(h registry.Host, name string) bool {
	if h.Stat == nil {
		return false
	}
	info, err := h.Stat(filepath.Join(h.Home, ".figlet", "fonts", name))
	return err == nil && info.Mode().IsRegular()
}

// platformName is what a user calls the platform. "darwin" is a Go constant, not
// something to show somebody.
func platformName(goos string) string {
	switch goos {
	case "":
		return "this platform"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	}
	return goos
}
