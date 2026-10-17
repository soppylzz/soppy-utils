//go:build !windows

package registry

import "os"

// isExecutable reports whether a file can be run. On Unix that is a permission bit
// rather than a property of the name.
func isExecutable(info os.FileInfo) bool {
	return info.Mode().Perm()&0o111 != 0
}
