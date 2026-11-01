package snapshot_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/spirv"
)

// spirvVulkanTargets are the SPIR-V versions consumers write, each validated
// with the Vulkan rules of the API version that takes it: the zero version,
// which gg compiles at, and the version Rust wgpu-hal (vulkan/adapter.rs)
// and wgpu pick per device, 1.3 on Vulkan 1.1, 1.5 on 1.2, 1.6 on 1.3. The
// universal spv1.6 environment of TestSpirvValBinary has no Vulkan rules, so
// it misses errors such as an explicit layout on a Function variable's type
// (VUID-StandaloneSpirv-None-10684), which Vulkan forbids from SPIR-V 1.5.
var spirvVulkanTargets = []struct {
	version spirv.Version
	env     string
}{
	{spirv.Version1_0, "vulkan1.0"},
	{spirv.Version1_3, "vulkan1.1"},
	{spirv.Version1_5, "vulkan1.2"},
	{spirv.Version1_6, "vulkan1.3"},
}

// spirvVulkanKnown are the Vulkan validation errors still open, by shader:
// the message spirv-val gives at every target, and the card that fixes it. An
// entry that no longer fails is an error, so the list shrinks with the fixes.
var spirvVulkanKnown = map[string]struct{ message, card string }{
	"subgroup-barrier":                        {"ControlBarrier", "P2-SPV-013"},
	"8820-multiple-local-invocation-index-id": {"VUID-StandaloneSpirv-OpEntryPoint-09658", "P2-SPV-014"},
}

// TestSpirvValVulkan validates the corpus at every target with the zero
// options (and the binding map of each shader's Rust parameters). A shader
// that needs a newer version than the target is skipped there: the backend
// refuses it with an error naming the version. So is a module without a
// vertex, fragment or compute entry point: it declares Linkage, which no
// Vulkan pipeline takes (mesh shaders are not written yet).
func TestSpirvValVulkan(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	shaders := loadCorpus(t)
	for _, target := range spirvVulkanTargets {
		t.Run(target.env, func(t *testing.T) {
			valid, skipped, known := 0, 0, 0
			for i := range shaders {
				shader := &shaders[i]
				bin, err := compileSPIRVAt(shader.name, shader.source, target.version)
				if err != nil {
					skipped++ // compile failures are TestSpirvValBinary's
					continue
				}
				msg := runSpirvVal(t, spirvVal, target.env, bin)
				if strings.Contains(msg, "Capability Linkage") {
					skipped++
					continue
				}
				entry, isKnown := spirvVulkanKnown[shader.name]
				switch {
				case isKnown && strings.Contains(msg, entry.message):
					known++
				case isKnown:
					t.Errorf("%s at %s: known failure (%s) %q no longer occurs: %q", shader.name, target.env,
						entry.card, entry.message, msg)
				case msg != "":
					t.Errorf("%s at SPIR-V %d.%d (%s): %s", shader.name,
						target.version.Major, target.version.Minor, target.env, msg)
				default:
					valid++
				}
			}
			t.Logf("%s: %d valid, %d known failures, %d not compiled or not a pipeline", target.env, valid, known, skipped)
		})
	}
}

// compileSPIRVAt compiles a corpus shader as TestSpirvValBinary does, at
// version.
func compileSPIRVAt(name, source string, version spirv.Version) ([]byte, error) {
	module, err := parseWGSL(source)
	if err != nil {
		return nil, err
	}
	module, err = naga.ProcessOverrides(module, naga.OverrideOptions{Constants: readSPVPipelineConstants(name)})
	if err != nil {
		return nil, err
	}
	rust := readRustSPVConfig(name)
	opts := spirv.Options{Version: version, BindingMap: rust.BindingMap, FakeMissingBindings: rust.FakeMissingBindings}
	return spirvBinary(module, opts)
}

// runSpirvVal returns spirv-val's first message for bin under env, "" when
// it is valid.
func runSpirvVal(t *testing.T, spirvVal, env string, bin []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shader.spv")
	if err := os.WriteFile(path, bin, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(spirvVal, "--target-env", env, "--uniform-buffer-standard-layout", path).CombinedOutput()
	if err == nil {
		return ""
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return fmt.Sprintf("%s (%v)", first, err)
}
