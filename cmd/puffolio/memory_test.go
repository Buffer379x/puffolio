package main

import (
	"math"
	"runtime/debug"
	"testing"
)

func TestStartupMemoryBudget(t *testing.T) {
	previous := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(previous) })
	for _, tc := range []struct {
		name, env     string
		current, want int64
	}{
		{"default", "", math.MaxInt64, 384 << 20},
		{"stricter_runtime", "", 128 << 20, 128 << 20},
		{"explicit_higher", "1GiB", 1 << 30, 1 << 30},
		{"explicit_off", "off", math.MaxInt64, math.MaxInt64},
		{"explicit_stricter", "256MiB", 256 << 20, 256 << 20},
		{"explicit_preserves_runtime", "1GiB", 128 << 20, 128 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GOMEMLIMIT", tc.env)
			debug.SetMemoryLimit(tc.current)
			if err := run([]string{"--version"}); err != nil {
				t.Fatal(err)
			}
			if got := debug.SetMemoryLimit(-1); got != tc.want {
				t.Fatalf("startup memory limit = %d, want %d", got, tc.want)
			}
		})
	}
}
