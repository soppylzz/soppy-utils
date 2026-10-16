package welcome

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/action"
)

type optionalTools struct {
	figlet    bool
	speedFont bool
	slantFont bool
	fastfetch bool
}

func detectOptionalTools(home string) optionalTools {
	_, figletErr := exec.LookPath(toolFiglet)
	_, fastfetchErr := exec.LookPath(toolFastfetch)
	font := func(name string) bool {
		f, err := os.Open(filepath.Join(home, ".figlet", "fonts", name))
		if err != nil {
			return false
		}
		defer f.Close()
		info, err := f.Stat()
		return err == nil && info.Mode().IsRegular()
	}
	return optionalTools{
		figlet:    figletErr == nil,
		speedFont: font(speedFontFile),
		slantFont: font(slantFontFile),
		fastfetch: fastfetchErr == nil,
	}
}

func (d optionalTools) printStatus(out io.Writer) {
	if d.figlet {
		fmt.Fprintln(out, "figlet: available")
		fmt.Fprintf(out, "figlet fonts: speed=%t smslant=%t\n", d.speedFont, d.slantFont)
	} else {
		fmt.Fprintln(out, "figlet: missing (plain text fallback)")
	}
	if d.fastfetch {
		fmt.Fprintln(out, "fastfetch: available")
	} else {
		fmt.Fprintln(out, "fastfetch: missing (system summary skipped)")
	}
}

// dependencyActions is what would install what the banner can use.
//
// Core cannot carry any of it out: the daemon is detached, so there is no terminal for
// an elevation prompt and no way to answer one. It states what is missing and what would
// install it, and the client decides whether to. That is why no sudo appears here — how
// to obtain privileges is not core's business.
func dependencyActions(goos string, d optionalTools) action.Request {
	// Everything missing, in the order a user would look for it.
	var missing []string
	if !d.figlet {
		missing = append(missing, toolFiglet)
	} else if !d.speedFont || !d.slantFont {
		missing = append(missing, "its FIGlet fonts")
	}
	if !d.fastfetch {
		missing = append(missing, toolFastfetch)
	}
	if len(missing) == 0 {
		return action.Request{}
	}

	req := action.Request{Reason: "welcome needs " + strings.Join(missing, " and ")}

	// Only packages can be installed by a package manager. The fonts are files with no
	// agreed source, so they are named above and left there.
	var packages []string
	if !d.figlet {
		packages = append(packages, toolFiglet)
	}
	if !d.fastfetch {
		packages = append(packages, toolFastfetch)
	}

	if len(packages) == 0 {
		// Nothing a package manager could install. The fonts are the only thing missing,
		// and they are named in the reason and left there.
		return req
	}

	switch goos {
	case "darwin":
		req.Actions = append(req.Actions, action.Action{
			Kind: action.Run, Optional: true, ChangesMachine: true,
			Argv: append([]string{"brew", "install"}, packages...),
		})

	case "linux":
		// Whether the Ubuntu archive carries fastfetch has not been established, and
		// offering a command that fails is worse than saying nothing.
		var apt []string
		for _, p := range packages {
			if p == toolFiglet {
				apt = append(apt, p)
			}
		}
		if len(apt) > 0 {
			req.Actions = append(req.Actions, action.Action{
				Kind: action.Run, NeedsElevation: true, Optional: true, ChangesMachine: true,
				Argv: append([]string{"apt-get", "install", "-y"}, apt...),
			})
		}
	}
	return req
}
