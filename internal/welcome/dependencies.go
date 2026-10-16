package welcome

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/core"
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

func (d optionalTools) warnMissing(reply core.Responder) {
	if !d.figlet {
		reply.Notify("welcome: optional figlet not found; banner text will be plain")
	} else if !d.speedFont || !d.slantFont {
		reply.Notify("welcome: optional FIGlet fonts are missing; affected banner text will be plain")
	}
	if !d.fastfetch {
		reply.Notify("welcome: optional fastfetch not found; system summary will be skipped")
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
