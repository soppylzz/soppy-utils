//go:build windows

package registry

import (
	"os"
	"path/filepath"
	"strings"
)

// executableExtensions are the suffixes Windows treats as runnable.
var executableExtensions = []string{".exe", ".bat", ".cmd", ".com"}

// isExecutable reports whether a file can be run. Windows decides by extension rather
// than by a permission bit, so this cannot share the Unix implementation.
func isExecutable(info os.FileInfo) bool {
	ext := strings.ToLower(filepath.Ext(info.Name()))
	for _, want := range executableExtensions {
		if ext == want {
			return true
		}
	}
	return false
}
