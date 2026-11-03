package store

import (
	"fmt"
	"syscall"
)

// RequireSpace fails before staging large archives; runtime I/O errors still
// propagate so a concurrent disk consumer cannot cause a partial transaction.
func RequireSpace(dir string, needed int64) error {
	if needed < 0 {
		return fmt.Errorf("invalid disk budget")
	}
	var stat syscall.Statfs_t
	if e := syscall.Statfs(dir, &stat); e != nil {
		return e
	}
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	if uint64(needed) > available {
		return fmt.Errorf("insufficient free disk space: need %d bytes, available %d", needed, available)
	}
	return nil
}
