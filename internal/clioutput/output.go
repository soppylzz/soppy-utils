package clioutput

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

const (
	errorColor = 31
	warnColor  = 33
	infoColor  = 36
)

func colorEnabled() bool {
	return os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" && term.IsTerminal(os.Stderr.Fd())
}

func write(w io.Writer, color bool, code int, message string) {
	if color {
		fmt.Fprintf(w, "\x1b[%dm%s\x1b[0m\n", code, message)
		return
	}
	fmt.Fprintln(w, message)
}

func Error(scope, message string) {
	write(os.Stderr, colorEnabled(), errorColor, scope+": "+message)
}

func Warn(scope, message string) {
	write(os.Stderr, colorEnabled(), warnColor, "warning: "+scope+": "+message)
}

func Info(scope, message string) {
	write(os.Stderr, colorEnabled(), infoColor, scope+": "+message)
}

func Note(message string) {
	write(os.Stderr, colorEnabled(), infoColor, message)
}
