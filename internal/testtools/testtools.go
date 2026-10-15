// Package testtools gives tests one policy for external tools (spirv-val,
// spirv-dis, dxc, ...), in the spirit of Go's internal/testenv.
//
// Locally, a missing tool skips the check that needs it, with an explicit
// reason. In CI, NAGA_REQUIRE_TOOLS lists tools that must be present
// ("spirv-val,spirv-dis", or "all"): a missing listed tool fails the test, so
// a runner without the tool cannot report green while checking nothing.
package testtools

import (
	"os"
	"os/exec"
	"strings"
)

// EnvRequired names the environment variable listing required tools.
const EnvRequired = "NAGA_REQUIRE_TOOLS"

// T is the subset of testing.TB these helpers use.
type T interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
	Logf(format string, args ...any)
}

// lookPath is exec.LookPath, replaceable in tests.
var lookPath = exec.LookPath

// Required reports whether name is listed in NAGA_REQUIRE_TOOLS.
func Required(name string) bool {
	for item := range strings.SplitSeq(os.Getenv(EnvRequired), ",") {
		item = strings.TrimSpace(item)
		if item == "all" || item == name {
			return true
		}
	}
	return false
}

// Look returns the path of a tool the test cannot run without. If it is
// missing, the test fails when the tool is required and is skipped otherwise.
func Look(t T, name, install string) string {
	t.Helper()
	path, err := lookPath(name)
	if err == nil {
		return path
	}
	Missing(t, name, install)
	return ""
}

// Optional returns the path of a tool one part of a test needs, and whether
// it was found. If it is missing, the test fails when the tool is required;
// otherwise the absence is logged and the rest of the test goes on.
func Optional(t T, name, install string) (string, bool) {
	t.Helper()
	path, err := lookPath(name)
	if err == nil {
		return path, true
	}
	if Required(name) {
		t.Fatalf("%s not found in PATH but required by %s (%s)", name, EnvRequired, install)
		return "", false
	}
	t.Logf("%s not found in PATH (%s): its checks did not run; set %s=%s to make this a failure",
		name, install, EnvRequired, name)
	return "", false
}

// Missing reports a tool that a test-specific lookup could not find, with
// Look's policy: fail when required, skip otherwise.
func Missing(t T, name, install string) {
	t.Helper()
	if Required(name) {
		t.Fatalf("%s not found but required by %s (%s)", name, EnvRequired, install)
		return
	}
	t.Skipf("%s not found (%s); set %s=%s to make this a failure", name, install, EnvRequired, name)
}

// Install hints shared by callers.
const (
	InstallSPIRVTools = "install SPIRV-Tools, e.g. from the Vulkan SDK"
	InstallDXC        = "install the Windows SDK or put dxc in PATH"
)
