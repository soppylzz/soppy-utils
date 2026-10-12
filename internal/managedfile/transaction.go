package managedfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/configstore"
)

type snapshot struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

// WithRollback restores a set of owned files when a later step fails.
// Callers validate ownership and marker structure before entering the transaction.
func WithRollback(paths []string, apply func() error) error {
	if len(paths) == 0 {
		return apply()
	}
	return WithLock(filepath.Dir(paths[0]), func() error { return withRollback(paths, apply) })
}

func withRollback(paths []string, apply func() error) (err error) {
	snapshots := make([]snapshot, 0, len(paths))
	for _, path := range paths {
		st, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			snapshots = append(snapshots, snapshot{path: path})
			continue
		}
		if e != nil {
			return e
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		snapshots = append(snapshots, snapshot{path: path, data: b, mode: st.Mode().Perm(), exists: true})
	}
	if err = apply(); err == nil {
		return nil
	}
	for i := len(snapshots) - 1; i >= 0; i-- {
		s := snapshots[i]
		var restoreErr error
		if s.exists {
			restoreErr = configstore.AtomicWrite(s.path, s.data, s.mode)
		} else {
			restoreErr = os.Remove(s.path)
			if errors.Is(restoreErr, os.ErrNotExist) {
				restoreErr = nil
			}
		}
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback %s: %w", s.path, restoreErr))
		}
	}
	return err
}
