//go:build !darwin

package codegen

import "testing"

func verifyMSLWithXcrun(t *testing.T, _ string) {
	t.Helper()
	t.Skip("xcrun metal not available on this platform")
}
