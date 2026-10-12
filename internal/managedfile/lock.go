package managedfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WithLock serializes a util's short write phase across helper processes.
func WithLock(dir string, apply func() error) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, ".spu.lock")
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Mkdir(path, 0700)
		if err == nil {
			defer os.Remove(path)
			return apply()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("config is locked: %s", dir)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
