//go:build !darwin

package msl

import "testing"

func verifyMSLWithXcrun(t *testing.T, _ string) {
	t.Helper()
	t.Skip("xcrun metal not available on this platform")
}
