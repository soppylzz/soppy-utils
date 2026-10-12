package ui

import (
	"errors"
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
)

func Available() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stderr.Fd())
}

func Run(groups ...*huh.Group) error {
	if !Available() {
		return errors.New("interactive terminal required")
	}
	return huh.NewForm(groups...).WithInput(os.Stdin).WithOutput(os.Stderr).Run()
}
