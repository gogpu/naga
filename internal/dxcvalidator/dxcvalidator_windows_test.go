//go:build windows

package dxcvalidator

import (
	"os"
	"testing"
)

// TestSmokeValidateGoldenFixture verifies that the Pure Go IDxcValidator
// wrapper still returns S_OK on tmp/min1_final.dxil — the first-ever naga
// output that passed real Microsoft DXIL validation (commit 0570283,
// BUG-DXIL-019). If this regresses, the cause is either a wrapper change
// here or a regression in the underlying naga DXIL emit path.
//
// Skipped automatically when:
//   - dxil.dll cannot be located (LoadDLL fails)
//   - the golden fixture is missing
func TestSmokeValidateGoldenFixture(t *testing.T) {
	const fixture = "../../tmp/min1_final.dxil"
	blob, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("golden fixture missing: %v", err)
	}

	var got Result
	runErr := Run(func(v *Validator) error {
		var vErr error
		got, vErr = v.Validate(blob)
		return vErr
	})
	if runErr != nil {
		// Treat dxil.dll loader / COM init failure as skip rather than
		// failure: the test is informational on machines without the
		// Windows 10 SDK.
		t.Skipf("Run failed (likely no dxil.dll): %v", runErr)
	}

	if !got.OK {
		t.Fatalf("expected S_OK on golden fixture, got status=0x%08x error=%q", got.Status, got.Error)
	}
}
