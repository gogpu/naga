//go:build windows

package dxcvalidator

import "testing"

// TestRunManyTimes calls Run more often than the Go runtime has callback
// slots (syscall.NewCallback allows 2000 per process and never frees one).
// dxil.Validate calls Run once per container, so a process that validates
// more containers than that must not run out.
func TestRunManyTimes(t *testing.T) {
	if testing.Short() {
		t.Skip("2100 validator threads take about 30 s")
	}
	const runs = 2100
	for i := 0; i < runs; i++ {
		calls := 0
		if err := Run(func(*Validator) error {
			calls++
			return nil
		}); err != nil {
			t.Skipf("Run %d failed (likely no dxil.dll): %v", i, err)
		}
		if calls != 1 {
			t.Fatalf("Run %d invoked its callback %d times, want 1", i, calls)
		}
	}
}
