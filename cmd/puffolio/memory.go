package main

import (
	"os"
	"runtime/debug"
)

// Leave cgroup headroom for executable mappings, tmpfs and other memory
// outside the Go soft limit. Explicit operator settings always take precedence.
func configureMemoryLimit() {
	const defaultLimit = 384 << 20
	if os.Getenv("GOMEMLIMIT") == "" && debug.SetMemoryLimit(-1) > defaultLimit {
		debug.SetMemoryLimit(defaultLimit)
	}
}
