package clioutput

import (
	"bytes"
	"testing"
)

func TestWrite(t *testing.T) {
	for _, tc := range []struct {
		name  string
		color bool
		want  string
	}{
		{"plain", false, "warning: git: config changed\n"},
		{"color", true, "\x1b[33mwarning: git: config changed\x1b[0m\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			write(&out, tc.color, warnColor, "warning: git: config changed")
			if got := out.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}
