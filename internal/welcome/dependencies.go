package welcome

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/clioutput"
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

func (d optionalTools) warnMissing() {
	if !d.figlet {
		clioutput.Warn("welcome", "optional figlet not found; banner text will be plain")
	} else if !d.speedFont || !d.slantFont {
		clioutput.Warn("welcome", "optional FIGlet fonts are missing; affected banner text will be plain")
	}
	if !d.fastfetch {
		clioutput.Warn("welcome", "optional fastfetch not found; system summary will be skipped")
	}
}

func (d optionalTools) printStatus() {
	if d.figlet {
		fmt.Println("figlet: available")
		fmt.Printf("figlet fonts: speed=%t smslant=%t\n", d.speedFont, d.slantFont)
	} else {
		fmt.Println("figlet: missing (plain text fallback)")
	}
	if d.fastfetch {
		fmt.Println("fastfetch: available")
	} else {
		fmt.Println("fastfetch: missing (system summary skipped)")
	}
}
