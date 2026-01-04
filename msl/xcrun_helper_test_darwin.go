//go:build darwin

package msl

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func verifyMSLWithXcrun(t *testing.T, source string) {
	t.Helper()

	if _, err := exec.LookPath("xcrun"); err != nil {
		t.Skip("xcrun not found; skipping MSL compile check")
	}
	if err := exec.Command("xcrun", "--find", "metal").Run(); err != nil {
		t.Skip("xcrun metal tool not found; skipping MSL compile check")
	}

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "shader.metal")
	outPath := filepath.Join(dir, "shader.air")
	if err := os.WriteFile(srcPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write MSL temp file: %v", err)
	}

	cmd := exec.Command("xcrun", "-sdk", "macosx", "metal", "-c", srcPath, "-o", outPath) //nolint:gosec // G204: args are temp paths in tests
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("xcrun metal failed: %v\n%s\nMSL:\n%s", err, out, source)
	}
}
