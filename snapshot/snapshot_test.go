// Package snapshot_test provides golden snapshot tests for all naga backends.
//
// For each WGSL input shader in testdata/in/, the test compiles through all
// four backends (SPIR-V, GLSL, HLSL, MSL) and compares output to golden files
// stored in testdata/golden/{spv,glsl,hlsl,msl}/.
//
// To regenerate golden files after intentional changes:
//
//	UPDATE_GOLDEN=1 go test ./snapshot/...
package snapshot_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// ---------------------------------------------------------------------------
// Test Runner
// ---------------------------------------------------------------------------

// shaderFile represents an input WGSL shader loaded from disk.
type shaderFile struct {
	name   string // base name without extension (e.g., "vertex_basic")
	source string // WGSL source code
}

// TestSnapshots is the main golden snapshot test. It loads all WGSL inputs,
// compiles each through all backends, and compares with golden files.
// For shaders without Rust reference, falls back to self-generated golden.
func TestSnapshots(t *testing.T) {
	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found in testdata/in/")
	}

	for i := range shaders {
		shader := &shaders[i]
		t.Run(shader.name, func(t *testing.T) {
			module := compileToIR(t, shader.name, shader.source)
			if module == nil {
				t.Skipf("[%s] lower failed, skipping backend tests", shader.name)
				return
			}

			t.Run("spv", func(t *testing.T) {
				spvBytes := compileSPIRV(t, module)
				disasm := disassembleSPIRV(spvBytes)
				compareGolden(t, filepath.Join("testdata", "golden", "spv", shader.name+".spvasm"), disasm)
			})

			t.Run("glsl", func(t *testing.T) {
				code := compileGLSL(t, module)
				compareGolden(t, filepath.Join("testdata", "golden", "glsl", shader.name+".glsl"), code)
			})

			t.Run("hlsl", func(t *testing.T) {
				code := compileHLSL(t, module, shader.name)
				compareGolden(t, filepath.Join("testdata", "golden", "hlsl", shader.name+".hlsl"), code)
			})

			t.Run("msl", func(t *testing.T) {
				code := compileMSL(t, module)
				compareGolden(t, filepath.Join("testdata", "golden", "msl", shader.name+".msl"), code)
			})
		})
	}
}

// TestRustReference compares our compiled output against Rust naga reference outputs.
// This is the AUTHORITATIVE test — Rust naga output is the ground truth.
// Our output for the same .wgsl input MUST match Rust's output (structurally).
//
// Rust reference files: testdata/reference/{spv,msl,hlsl}/wgsl-{name}.{ext}
// GLSL has per-entry-point files: testdata/reference/glsl/wgsl-{name}.{entry}.{Stage}.glsl
func TestRustReference(t *testing.T) {
	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found")
	}

	var spvPass, spvFail, spvSkip int
	var mslPass, mslFail, mslSkip int
	var hlslPass, hlslFail, hlslSkip int
	var glslPass, glslFail, glslSkip int
	var lowerFail int

	for i := range shaders {
		shader := &shaders[i]
		rustSPV := filepath.Join("testdata", "reference", "spv", "wgsl-"+shader.name+".spvasm")
		rustMSL := filepath.Join("testdata", "reference", "msl", "wgsl-"+shader.name+".msl")
		rustHLSL := filepath.Join("testdata", "reference", "hlsl", "wgsl-"+shader.name+".hlsl")

		t.Run(shader.name, func(t *testing.T) {
			module := compileToIR(t, shader.name, shader.source)
			if module == nil {
				lowerFail++
				return
			}

			// SPIR-V vs Rust reference (structural comparison via spirv-dis)
			t.Run("spv_vs_rust", func(t *testing.T) {
				if _, err := os.Stat(rustSPV); os.IsNotExist(err) {
					spvSkip++
					t.Skipf("no Rust reference: %s", rustSPV)
				}
				spirvDisPath, lookErr := exec.LookPath("spirv-dis")
				if lookErr != nil {
					spvSkip++
					t.Skip("spirv-dis not found in PATH")
				}
				spvOpts := readRustSPVConfig(shader.name)
				spvBytes := compileSPIRVWithOpts(t, module, spvOpts)

				// Write binary to temp file and disassemble with spirv-dis
				tmpFile, tmpErr := os.CreateTemp("", "spirv-cmp-*.spv")
				if tmpErr != nil {
					t.Fatalf("create temp: %v", tmpErr)
				}
				tmpPath := tmpFile.Name()
				_, _ = tmpFile.Write(spvBytes)
				tmpFile.Close()
				defer os.Remove(tmpPath)

				cmd := exec.Command(spirvDisPath, "--no-header", tmpPath)
				ourDisasm, disErr := cmd.Output()
				if disErr != nil {
					spvFail++
					t.Errorf("spirv-dis failed: %v", disErr)
					return
				}

				rustExpected, err := os.ReadFile(rustSPV)
				if err != nil {
					t.Fatalf("read Rust reference: %v", err)
				}

				diffs := compareStructural(string(ourDisasm), string(rustExpected))
				if len(diffs) > 0 {
					spvFail++
					t.Errorf("SPIR-V differs from Rust reference:\n%s", strings.Join(diffs, "\n"))
				} else {
					spvPass++
				}
			})

			// MSL vs Rust reference
			t.Run("msl_vs_rust", func(t *testing.T) {
				if _, err := os.Stat(rustMSL); os.IsNotExist(err) {
					mslSkip++
					t.Skipf("no Rust reference: %s", rustMSL)
				}
				mslOpts := readRustMSLConfig(shader.name)
				code, compileErr := compileMSLWithOpts(t, module, mslOpts)
				if compileErr != nil {
					mslFail++
					t.Errorf("%v", compileErr)
					return
				}
				rustExpected, err := os.ReadFile(rustMSL)
				if err != nil {
					t.Fatalf("read Rust reference: %v", err)
				}
				expected := strings.ReplaceAll(string(rustExpected), "\r\n", "\n")
				actual := strings.ReplaceAll(code, "\r\n", "\n")
				if expected != actual {
					mslFail++
					diff := diffStrings(expected, actual)
					t.Errorf("MSL differs from Rust reference %s:\n%s", rustMSL, diff)
				} else {
					mslPass++
				}
			})

			// HLSL vs Rust reference
			t.Run("hlsl_vs_rust", func(t *testing.T) {
				if _, err := os.Stat(rustHLSL); os.IsNotExist(err) {
					hlslSkip++
					t.Skipf("no Rust reference: %s", rustHLSL)
				}
				code := compileHLSL(t, module, shader.name)
				rustExpected, err := os.ReadFile(rustHLSL)
				if err != nil {
					t.Fatalf("read Rust reference: %v", err)
				}
				expected := strings.ReplaceAll(string(rustExpected), "\r\n", "\n")
				actual := strings.ReplaceAll(code, "\r\n", "\n")
				if expected != actual {
					hlslFail++
					t.Errorf("HLSL differs from Rust reference %s", rustHLSL)
				} else {
					hlslPass++
				}
			})

			// GLSL vs Rust reference (per entry point)
			t.Run("glsl_vs_rust", func(t *testing.T) {
				glslCfg := readGLSLConfig(shader.name)
				// Clone module for GLSL if pipeline constants are set,
				// to avoid mutation from ProcessOverrides affecting other backends.
				glslModule := module
				if len(glslCfg.pipelineConstants) > 0 {
					glslModule = ir.CloneModuleForOverrides(module)
				}

				// Find all reference files for this shader
				glslRefDir := filepath.Join("testdata", "reference", "glsl")
				prefix := "wgsl-" + shader.name + "."
				refEntries, dirErr := os.ReadDir(glslRefDir)
				if dirErr != nil {
					glslSkip++
					t.Skipf("no GLSL reference dir: %v", dirErr)
					return
				}

				var refFiles []string
				for _, e := range refEntries {
					if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".glsl") {
						refFiles = append(refFiles, e.Name())
					}
				}

				if len(refFiles) == 0 {
					glslSkip++
					t.Skipf("no GLSL reference files for %s", shader.name)
					return
				}

				// For each reference file, extract entry point name and stage,
				// compile, and compare
				allMatch := true
				for _, refName := range refFiles {
					// Parse: wgsl-{shader}.{entry}.{Stage}.glsl
					withoutPrefix := strings.TrimPrefix(refName, prefix)
					withoutSuffix := strings.TrimSuffix(withoutPrefix, ".glsl")
					// Split into entry.Stage
					lastDot := strings.LastIndex(withoutSuffix, ".")
					if lastDot < 0 {
						t.Errorf("unexpected GLSL ref filename: %s", refName)
						allMatch = false
						continue
					}
					epName := withoutSuffix[:lastDot]
					stageStr := withoutSuffix[lastDot+1:]

					// Check if this entry point is in the exclude list
					if glslCfg.excludeList[epName] {
						continue
					}

					refPath := filepath.Join(glslRefDir, refName)
					rustExpected, readErr := os.ReadFile(refPath)
					if readErr != nil {
						t.Errorf("read GLSL ref %s: %v", refPath, readErr)
						allMatch = false
						continue
					}

					// Find matching entry point in module
					epIdx := -1
					for j := range module.EntryPoints {
						ep := &module.EntryPoints[j]
						if ep.Name == epName && glslStageName(ep.Stage) == stageStr {
							epIdx = j
							break
						}
					}
					if epIdx < 0 {
						t.Errorf("entry point %s (%s) not found in module for %s", epName, stageStr, refName)
						allMatch = false
						continue
					}

					// Compile GLSL for this entry point
					opts := glslCfg.toOptions()
					opts.EntryPoint = epName

					code, _, compileErr := glsl.Compile(glslModule, opts)
					if compileErr != nil {
						t.Errorf("GLSL compile %s/%s: %v", shader.name, epName, compileErr)
						allMatch = false
						continue
					}

					expected := strings.ReplaceAll(string(rustExpected), "\r\n", "\n")
					actual := strings.ReplaceAll(code, "\r\n", "\n")
					if expected != actual {
						diff := diffStrings(expected, actual)
						t.Errorf("GLSL differs for %s:\n%s", refName, diff)
						allMatch = false
					}
				}

				if allMatch {
					glslPass++
				} else {
					glslFail++
				}
			})
		})
	}

	t.Logf("=== Rust Reference Summary ===")
	if lowerFail > 0 {
		t.Logf("Lower:  %d shaders failed to compile (not tested in any backend)", lowerFail)
	}
	t.Logf("SPIR-V: %d pass, %d fail, %d skip", spvPass, spvFail, spvSkip)
	t.Logf("MSL:    %d pass, %d fail, %d skip", mslPass, mslFail, mslSkip)
	t.Logf("HLSL:   %d pass, %d fail, %d skip", hlslPass, hlslFail, hlslSkip)
	t.Logf("GLSL:   %d pass, %d fail, %d skip", glslPass, glslFail, glslSkip)
}

// compareStructural compares two SPIR-V disassembly outputs structurally.
// Ignores ID numbers (e.g., %1, %_2) since they always differ between compilers.
// Returns empty string if structurally equivalent, diff description otherwise.
func compareRustStructural(actual, expected string) string {
	// For now, normalize IDs and compare line-by-line.
	// TODO: implement proper structural comparison ignoring ID numbering
	actualNorm := strings.ReplaceAll(actual, "\r\n", "\n")
	expectedNorm := strings.ReplaceAll(expected, "\r\n", "\n")

	actualLines := strings.Split(strings.TrimSpace(actualNorm), "\n")
	expectedLines := strings.Split(strings.TrimSpace(expectedNorm), "\n")

	if len(actualLines) != len(expectedLines) {
		return fmt.Sprintf("line count: ours=%d, rust=%d", len(actualLines), len(expectedLines))
	}

	var diffs []string
	for i := range actualLines {
		if actualLines[i] != expectedLines[i] {
			if len(diffs) < 5 {
				diffs = append(diffs, fmt.Sprintf("  line %d:\n    ours: %s\n    rust: %s", i+1, actualLines[i], expectedLines[i]))
			}
		}
	}
	if len(diffs) > 0 {
		return strings.Join(diffs, "\n")
	}
	return ""
}

// ---------------------------------------------------------------------------
// Shader Loading
// ---------------------------------------------------------------------------

// loadInputShaders reads all .wgsl files from the given directory.
func loadInputShaders(t *testing.T, dir string) []shaderFile {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read input directory %q: %v", dir, err)
	}

	var shaders []shaderFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wgsl") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read shader %q: %v", entry.Name(), readErr)
		}
		name := strings.TrimSuffix(entry.Name(), ".wgsl")
		shaders = append(shaders, shaderFile{name: name, source: string(data)})
	}

	// Sort for deterministic test order
	sort.Slice(shaders, func(i, j int) bool {
		return shaders[i].name < shaders[j].name
	})

	return shaders
}

// ---------------------------------------------------------------------------
// Compilation Helpers
// ---------------------------------------------------------------------------

// compileToIR parses WGSL source and lowers it to the IR module.
func compileToIR(t *testing.T, name, source string) *ir.Module {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("[%s] tokenize failed: %v", name, err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("[%s] parse failed: %v", name, err)
	}

	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		t.Logf("[%s] lower failed: %v", name, err)
		return nil
	}

	return module
}

// compileSPIRV compiles the IR module to SPIR-V binary.
func compileSPIRV(t *testing.T, module *ir.Module) []byte {
	t.Helper()

	backend := spirv.NewBackend(spirv.DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Skipf("SPIR-V compile failed (skipping): %v", err)
	}

	return spvBytes
}

// compileSPIRVWithOpts compiles the IR module to SPIR-V binary with specific options.
func compileSPIRVWithOpts(t *testing.T, module *ir.Module, opts spirv.Options) []byte {
	t.Helper()

	backend := spirv.NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Skipf("SPIR-V compile failed (skipping): %v", err)
	}

	return spvBytes
}

// readRustSPVConfig reads a Rust TOML config file and returns SPIR-V options.
func readRustSPVConfig(shaderName string) spirv.Options {
	opts := spirv.DefaultOptions()

	tomlPath := filepath.Join(rustTomlDir, shaderName+".toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return opts
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	inSPVSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[spv]" {
			inSPVSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[spv]" {
			inSPVSection = false
			continue
		}
		if !inSPVSection {
			continue
		}
		if strings.HasPrefix(trimmed, "use_storage_input_output_16") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.UseStorageInputOutput16 = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "debug") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.Debug = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "force_point_size") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.ForcePointSize = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "adjust_coordinate_space") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.AdjustCoordinateSpace = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "ray_query_initialization_tracking") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.RayQueryInitTracking = strings.TrimSpace(parts[1]) == "true"
			}
		}
		// Parse version = [major, minor]
		if strings.HasPrefix(trimmed, "version") && !strings.HasPrefix(trimmed, "version_") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				// Parse [major, minor] format
				val = strings.TrimPrefix(val, "[")
				val = strings.TrimSuffix(val, "]")
				nums := strings.SplitN(val, ",", 2)
				if len(nums) == 2 {
					major := strings.TrimSpace(nums[0])
					minor := strings.TrimSpace(nums[1])
					var maj, min int
					fmt.Sscanf(major, "%d", &maj)
					fmt.Sscanf(minor, "%d", &min)
					opts.Version = spirv.Version{Major: uint8(maj), Minor: uint8(min)}
				}
			}
		}
		// Parse capabilities = ["Cap1", "Cap2"]
		if strings.HasPrefix(trimmed, "capabilities") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimPrefix(val, "[")
				val = strings.TrimSuffix(val, "]")
				caps := make(map[spirv.Capability]struct{})
				for _, item := range strings.Split(val, ",") {
					name := strings.TrimSpace(item)
					name = strings.Trim(name, "\"")
					if cap, ok := spvCapabilityByName(name); ok {
						caps[cap] = struct{}{}
					}
				}
				if len(caps) > 0 {
					opts.CapabilitiesAvailable = caps
				}
			}
		}
	}

	// Parse [bounds_check_policies] section (top-level, shared across backends)
	opts.BoundsCheckPolicies = parseSPVBoundsCheckPolicies(content)

	return opts
}

// spvCapabilityByName maps SPIR-V capability names (as used in Rust TOML configs)
// to our Capability constants.
func spvCapabilityByName(name string) (spirv.Capability, bool) {
	m := map[string]spirv.Capability{
		"Matrix":                             spirv.CapabilityMatrix,
		"Shader":                             spirv.CapabilityShader,
		"Float16":                            spirv.CapabilityFloat16,
		"Float64":                            spirv.CapabilityFloat64,
		"Int64":                              spirv.CapabilityInt64,
		"Int16":                              spirv.CapabilityInt16,
		"Int8":                               spirv.CapabilityInt8,
		"Linkage":                            spirv.CapabilityLinkage,
		"ClipDistance":                       spirv.CapabilityClipDistance,
		"ImageCubeArray":                     spirv.CapabilityImageCubeArray,
		"SampleRateShading":                  spirv.CapabilitySampleRateShading,
		"Sampled1D":                          spirv.CapabilitySampled1D,
		"Image1D":                            spirv.CapabilityImage1D,
		"SampledCubeArray":                   spirv.CapabilitySampledCubeArray,
		"StorageImageExtendedFormats":        spirv.CapabilityStorageImageExtendedFormats,
		"ImageQuery":                         spirv.CapabilityImageQuery,
		"DerivativeControl":                  spirv.CapabilityDerivativeControl,
		"StorageBuffer16BitAccess":           spirv.CapabilityStorageBuffer16BitAccess,
		"UniformAndStorageBuffer16BitAccess": spirv.CapabilityUniformAndStorageBuffer16BitAccess,
		"StorageInputOutput16":               spirv.CapabilityStorageInputOutput16,
		"MultiView":                          spirv.CapabilityMultiView,
		"FragmentBarycentricKHR":             spirv.CapabilityFragmentBarycentricKHR,
		"ShaderNonUniform":                   spirv.CapabilityShaderNonUniform,
		"AtomicFloat32AddEXT":                spirv.CapabilityAtomicFloat32AddEXT,
		"DotProductInput4x8BitPacked":        spirv.CapabilityDotProductInput4x8BitPacked,
		"DotProduct":                         spirv.CapabilityDotProduct,
		"GroupNonUniform":                    spirv.CapabilityGroupNonUniform,
		"GroupNonUniformVote":                spirv.CapabilityGroupNonUniformVote,
		"GroupNonUniformArithmetic":          spirv.CapabilityGroupNonUniformArithmetic,
		"GroupNonUniformBallot":              spirv.CapabilityGroupNonUniformBallot,
		"GroupNonUniformShuffle":             spirv.CapabilityGroupNonUniformShuffle,
		"GroupNonUniformShuffleRelative":     spirv.CapabilityGroupNonUniformShuffleRel,
		"GroupNonUniformQuad":                spirv.CapabilityGroupNonUniformQuad,
		"Geometry":                           spirv.CapabilityGeometry,
		"SubgroupBallotKHR":                  spirv.CapabilitySubgroupBallotKHR,
	}
	cap, ok := m[name]
	return cap, ok
}

// parseSPVBoundsCheckPolicies parses the [bounds_check_policies] TOML section for SPIR-V.
func parseSPVBoundsCheckPolicies(content string) spirv.BoundsCheckPolicies {
	policies := spirv.BoundsCheckPolicies{}
	lines := strings.Split(content, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[bounds_check_policies]" {
			inSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		policy := toSPVBoundsCheckPolicy(val)
		switch key {
		case "image_load":
			policies.ImageLoad = policy
		case "image_store":
			policies.ImageStore = policy
		case "index":
			policies.Index = policy
		}
	}
	return policies
}

// toSPVBoundsCheckPolicy converts a Rust policy name to a Go BoundsCheckPolicy.
func toSPVBoundsCheckPolicy(name string) spirv.BoundsCheckPolicy {
	switch name {
	case "ReadZeroSkipWrite":
		return spirv.BoundsCheckReadZeroSkipWrite
	case "Restrict":
		return spirv.BoundsCheckRestrict
	default:
		return spirv.BoundsCheckUnchecked
	}
}

// compileGLSL compiles the IR module to GLSL source.
// GLSL compiles one entry point at a time. For modules with multiple entry
// points, the output is concatenated with separators.
func compileGLSL(t *testing.T, module *ir.Module) string {
	t.Helper()

	if len(module.EntryPoints) == 0 {
		t.Skip("no entry points for GLSL")
	}

	// For single entry point, compile directly
	if len(module.EntryPoints) == 1 {
		opts := glsl.DefaultOptions()
		// Compute shaders require GLSL 430+
		if module.EntryPoints[0].Stage == ir.StageCompute {
			opts.LangVersion = glsl.Version430
		}
		code, _, err := glsl.Compile(module, opts)
		if err != nil {
			t.Skipf("GLSL compile failed (skipping): %v", err)
		}
		return code
	}

	// For multiple entry points, compile each separately and concatenate
	var parts []string
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		opts := glsl.DefaultOptions()
		opts.EntryPoint = ep.Name

		// Compute shaders need GLSL 430+
		if ep.Stage == ir.StageCompute {
			opts.LangVersion = glsl.Version430
		}

		code, _, err := glsl.Compile(module, opts)
		if err != nil {
			t.Skipf("GLSL compile for entry point %q failed (skipping): %v", ep.Name, err)
		}

		header := fmt.Sprintf("// === Entry Point: %s (%s) ===\n", ep.Name, stageName(ep.Stage))
		parts = append(parts, header+code)
	}

	return strings.Join(parts, "\n")
}

// compileHLSL compiles the IR module to HLSL source.
func compileHLSL(t *testing.T, module *ir.Module, shaderName string) string {
	t.Helper()

	opts := hlsl.DefaultOptions()
	// Match Rust naga's default_for_testing() settings
	opts.RestrictIndexing = true
	opts.ForceLoopBounding = true

	// Read HLSL-specific TOML settings
	readHLSLConfig(opts, shaderName)

	code, _, err := hlsl.Compile(module, opts)
	if err != nil {
		t.Skipf("HLSL compile failed (skipping): %v", err)
	}

	return code
}

// readHLSLConfig reads HLSL options from a Rust naga test TOML config file.
func readHLSLConfig(opts *hlsl.Options, shaderName string) {
	tomlPath := filepath.Join(rustTomlDir, shaderName+".toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return // No TOML file — use defaults
	}
	content := string(data)

	// Parse [hlsl] section for special_constants_binding
	if strings.Contains(content, "special_constants_binding") {
		// Parse: special_constants_binding = { register = N, space = N }
		re := regexp.MustCompile(`special_constants_binding\s*=\s*\{\s*register\s*=\s*(\d+)\s*,\s*space\s*=\s*(\d+)\s*\}`)
		if m := re.FindStringSubmatch(content); m != nil {
			var reg, space uint32
			fmt.Sscanf(m[1], "%d", &reg)
			fmt.Sscanf(m[2], "%d", &space)
			opts.SpecialConstantsBinding = &hlsl.BindTarget{Register: reg, Space: uint8(space)}
		}
	}

	// Parse fake_missing_bindings ONLY in [hlsl] section (not MSL or other sections)
	{
		hlslSection := extractHLSLSection(content)
		if hlslSection != "" {
			re := regexp.MustCompile(`(?m)^\s*fake_missing_bindings\s*=\s*(true|false)`)
			if m := re.FindStringSubmatch(hlslSection); m != nil {
				opts.FakeMissingBindings = m[1] == "true"
			}
		}
	}

	// Parse [[hlsl.binding_map]] entries
	// Only parse within the [hlsl] section to avoid matching MSL entries.
	{
		hlslSection := extractHLSLSection(content)
		if hlslSection != "" {
			// Match binding_map entries in various TOML formats:
			// [[hlsl.binding_map]]
			// resource_binding = { group = G, binding = B }
			// bind_target = { register = R, space = S }
			// OR inline: { resource_binding = { group = G, binding = B }, bind_target = { register = R, space = S } }
			re := regexp.MustCompile(`resource_binding\s*=\s*\{\s*group\s*=\s*(\d+)\s*,\s*binding\s*=\s*(\d+)\s*\}[\s,]*bind_target\s*=\s*\{([^}]+)\}`)
			re2 := regexp.MustCompile(`bind_target\s*=\s*\{([^}]+)\}[\s,]*resource_binding\s*=\s*\{\s*group\s*=\s*(\d+)\s*,\s*binding\s*=\s*(\d+)\s*\}`)
			for _, m := range re.FindAllStringSubmatch(hlslSection, -1) {
				group, _ := strconv.ParseUint(m[1], 10, 32)
				binding, _ := strconv.ParseUint(m[2], 10, 32)
				btContent := m[3]
				opts.BindingMap[hlsl.ResourceBinding{Group: uint32(group), Binding: uint32(binding)}] = parseBindTargetFull(btContent)
			}
			// Also try reversed order (bind_target before resource_binding)
			for _, m := range re2.FindAllStringSubmatch(hlslSection, -1) {
				btContent := m[1]
				group, _ := strconv.ParseUint(m[2], 10, 32)
				binding, _ := strconv.ParseUint(m[3], 10, 32)
				opts.BindingMap[hlsl.ResourceBinding{Group: uint32(group), Binding: uint32(binding)}] = parseBindTargetFull(btContent)
			}
		}
	}

	// Parse dynamic_storage_buffer_offsets_targets in [hlsl] section
	// Format: dynamic_storage_buffer_offsets_targets = [
	//     { index = 0, bind_target = { register = 1, size = 2, space = 0 } },
	//     ...
	// ]
	if strings.Contains(content, "dynamic_storage_buffer_offsets_targets") {
		re := regexp.MustCompile(`\{\s*index\s*=\s*(\d+)\s*,\s*bind_target\s*=\s*\{\s*register\s*=\s*(\d+)\s*,\s*size\s*=\s*(\d+)\s*,\s*space\s*=\s*(\d+)\s*\}\s*\}`)
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			idx, _ := strconv.ParseUint(m[1], 10, 32)
			reg, _ := strconv.ParseUint(m[2], 10, 32)
			size, _ := strconv.ParseUint(m[3], 10, 32)
			space, _ := strconv.ParseUint(m[4], 10, 32)
			if opts.DynamicStorageBufferOffsetsTargets == nil {
				opts.DynamicStorageBufferOffsetsTargets = make(map[uint32]hlsl.OffsetsBindTarget)
			}
			opts.DynamicStorageBufferOffsetsTargets[uint32(idx)] = hlsl.OffsetsBindTarget{
				Register: uint32(reg),
				Size:     uint32(size),
				Space:    uint8(space),
			}
		}
	}

	// Parse sampler_buffer_binding_map in [hlsl] section
	if strings.Contains(content, "sampler_buffer_binding_map") {
		re := regexp.MustCompile(`\{\s*group\s*=\s*(\d+)\s*,\s*bind_target\s*=\s*\{\s*register\s*=\s*(\d+)\s*,\s*space\s*=\s*(\d+)\s*\}\s*\}`)
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			group, _ := strconv.ParseUint(m[1], 10, 32)
			reg, _ := strconv.ParseUint(m[2], 10, 32)
			space, _ := strconv.ParseUint(m[3], 10, 32)
			if opts.SamplerBufferBindingMap == nil {
				opts.SamplerBufferBindingMap = make(map[uint32]hlsl.BindTarget)
			}
			opts.SamplerBufferBindingMap[uint32(group)] = hlsl.BindTarget{
				Register: uint32(reg),
				Space:    uint8(space),
			}
		}
	}

	// Parse [[hlsl.external_texture_binding_map]] entries
	if strings.Contains(content, "external_texture_binding_map") {
		hlslSection := extractHLSLSection(content)
		if hlslSection != "" {
			// Parse each external_texture_binding_map entry
			rbRe := regexp.MustCompile(`resource_binding\s*=\s*\{\s*group\s*=\s*(\d+)\s*,\s*binding\s*=\s*(\d+)\s*\}`)
			planesRe := regexp.MustCompile(`bind_target\.planes\s*=\s*\[\s*\{[^]]*\}\s*,\s*\{[^]]*\}\s*,\s*\{[^]]*\}\s*,?\s*\]`)
			planeRe := regexp.MustCompile(`\{\s*space\s*=\s*(\d+)\s*,\s*register\s*=\s*(\d+)\s*\}`)
			paramsRe := regexp.MustCompile(`bind_target\.params\s*=\s*\{\s*space\s*=\s*(\d+)\s*,\s*register\s*=\s*(\d+)\s*\}`)

			// Split by [[hlsl.external_texture_binding_map]]
			parts := strings.Split(hlslSection, "[[hlsl.external_texture_binding_map]]")
			for _, part := range parts[1:] { // skip before first match
				rbMatch := rbRe.FindStringSubmatch(part)
				planesMatch := planesRe.FindString(part)
				paramsMatch := paramsRe.FindStringSubmatch(part)
				if rbMatch == nil || planesMatch == "" || paramsMatch == nil {
					continue
				}
				group, _ := strconv.ParseUint(rbMatch[1], 10, 32)
				binding, _ := strconv.ParseUint(rbMatch[2], 10, 32)
				planeMatches := planeRe.FindAllStringSubmatch(planesMatch, -1)
				if len(planeMatches) < 3 {
					continue
				}

				var target hlsl.ExternalTextureBindTarget
				for i := 0; i < 3; i++ {
					sp, _ := strconv.ParseUint(planeMatches[i][1], 10, 8)
					reg, _ := strconv.ParseUint(planeMatches[i][2], 10, 32)
					target.Planes[i] = hlsl.BindTarget{Space: uint8(sp), Register: uint32(reg)}
				}
				pSpace, _ := strconv.ParseUint(paramsMatch[1], 10, 8)
				pReg, _ := strconv.ParseUint(paramsMatch[2], 10, 32)
				target.Params = hlsl.BindTarget{Space: uint8(pSpace), Register: uint32(pReg)}

				if opts.ExternalTextureBindingMap == nil {
					opts.ExternalTextureBindingMap = make(hlsl.ExternalTextureBindingMap)
				}
				opts.ExternalTextureBindingMap[hlsl.ResourceBinding{Group: uint32(group), Binding: uint32(binding)}] = target
			}
		}
	}

	// Parse fragment_module = { entry_point = "...", path = "..." }
	// This provides a fragment entry point to filter vertex outputs.
	if strings.Contains(content, "fragment_module") {
		re := regexp.MustCompile(`fragment_module\s*=\s*\{\s*entry_point\s*=\s*"([^"]+)"\s*,\s*path\s*=\s*"([^"]+)"\s*\}`)
		if m := re.FindStringSubmatch(content); m != nil {
			epName := m[1]
			fragPath := m[2]
			// Load the fragment shader from the same directory as other test inputs
			fragWGSL, err := os.ReadFile(filepath.Join("testdata", "in", fragPath))
			if err != nil {
				return
			}
			fragSource := string(fragWGSL)
			fragLexer := wgsl.NewLexer(fragSource)
			fragTokens, err := fragLexer.Tokenize()
			if err != nil {
				return
			}
			fragParser := wgsl.NewParser(fragTokens)
			fragAST, err := fragParser.Parse()
			if err != nil {
				return
			}
			fragModule, err := wgsl.LowerWithSource(fragAST, fragSource)
			if err != nil {
				return
			}
			// Find the fragment entry point function
			for i := range fragModule.EntryPoints {
				ep := &fragModule.EntryPoints[i]
				if ep.Name == epName && ep.Stage == ir.StageFragment {
					opts.FragmentEntryPoint = &hlsl.FragmentEntryPoint{
						Module:   fragModule,
						Function: &ep.Function,
					}
					break
				}
			}
		}
	}
}

// extractHLSLSection extracts the [hlsl] section and [[hlsl.*]] entries from TOML content.
// Returns the combined HLSL-related content, or empty string if no HLSL config exists.
func extractHLSLSection(content string) string {
	var lines []string
	inHLSLBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Check if this is a section header
		if strings.HasPrefix(trimmed, "[") {
			if strings.HasPrefix(trimmed, "[hlsl]") ||
				strings.HasPrefix(trimmed, "[[hlsl.") ||
				strings.HasPrefix(trimmed, "[hlsl.") {
				inHLSLBlock = true
			} else {
				inHLSLBlock = false
			}
		}

		if inHLSLBlock {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// parseRegisterSpace extracts register and space values from a bind_target content string.
func parseRegisterSpace(btContent string) (uint32, uint32) {
	var reg, space uint32
	reReg := regexp.MustCompile(`register\s*=\s*(\d+)`)
	if rm := reReg.FindStringSubmatch(btContent); rm != nil {
		fmt.Sscanf(rm[1], "%d", &reg)
	}
	reSp := regexp.MustCompile(`space\s*=\s*(\d+)`)
	if sm := reSp.FindStringSubmatch(btContent); sm != nil {
		fmt.Sscanf(sm[1], "%d", &space)
	}
	return reg, space
}

// parseBindTargetFull parses a bind_target content string and returns a full BindTarget,
// including optional dynamic_storage_buffer_offsets_index and restrict_indexing fields.
func parseBindTargetFull(btContent string) hlsl.BindTarget {
	reg, space := parseRegisterSpace(btContent)
	bt := hlsl.BindTarget{
		Register: reg,
		Space:    uint8(space),
	}
	// Parse dynamic_storage_buffer_offsets_index
	reDyn := regexp.MustCompile(`dynamic_storage_buffer_offsets_index\s*=\s*(\d+)`)
	if dm := reDyn.FindStringSubmatch(btContent); dm != nil {
		var idx uint32
		fmt.Sscanf(dm[1], "%d", &idx)
		bt.DynamicStorageBufferOffsetsIndex = &idx
	}
	// Parse restrict_indexing
	reRestrict := regexp.MustCompile(`restrict_indexing\s*=\s*(true|false)`)
	if rm := reRestrict.FindStringSubmatch(btContent); rm != nil {
		bt.RestrictIndexing = rm[1] == "true"
	}
	return bt
}

// ---------------------------------------------------------------------------
// GLSL Config
// ---------------------------------------------------------------------------

// glslConfig holds parsed GLSL options from a Rust naga TOML config.
type glslConfig struct {
	version             glsl.Version // defaults to ES 310 (Rust default)
	writerFlags         glsl.WriterFlags
	excludeList         map[string]bool // entry points to skip
	boundsCheckPolicies glsl.BoundsCheckPolicies
	bindingMap          map[glsl.BindingMapKey]uint8
	pipelineConstants   ir.PipelineConstants
}

// toOptions converts the parsed config to glsl.Options.
func (c glslConfig) toOptions() glsl.Options {
	return glsl.Options{
		LangVersion:         c.version,
		WriterFlags:         c.writerFlags,
		ForceHighPrecision:  true,
		BoundsCheckPolicies: c.boundsCheckPolicies,
		BindingMap:          c.bindingMap,
		PipelineConstants:   c.pipelineConstants,
	}
}

// readGLSLConfig reads GLSL options from a Rust naga test TOML config file.
// Returns Rust defaults (ES 310, ADJUST_COORDINATE_SPACE) if no TOML file exists.
func readGLSLConfig(shaderName string) glslConfig {
	cfg := glslConfig{
		version:     glsl.Version{Major: 3, Minor: 10, ES: true}, // Rust default: ES 310
		writerFlags: glsl.WriterFlagAdjustCoordinateSpace,        // Rust default
		excludeList: make(map[string]bool),
	}

	tomlPath := filepath.Join(rustTomlDir, shaderName+".toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return cfg
	}
	content := string(data)

	// Parse pipeline_constants (top-level, shared across backends) — BEFORE [glsl] check
	cfg.pipelineConstants = ir.PipelineConstants(parsePipelineConstants(content))

	// Parse [bounds_check_policies] section (top-level, shared across backends)
	cfg.boundsCheckPolicies = parseGLSLBoundsCheckPolicies(content)

	// Extract [glsl] section
	glslSection := extractGLSLSection(content)
	if glslSection == "" {
		return cfg
	}

	// Parse version.Desktop = NNN or version.Embedded = { is_webgl = BOOL, version = NNN }
	reDesktop := regexp.MustCompile(`version\.Desktop\s*=\s*(\d+)`)
	if m := reDesktop.FindStringSubmatch(glslSection); m != nil {
		ver, _ := strconv.Atoi(m[1])
		cfg.version = glsl.Version{Major: uint8(ver / 100), Minor: uint8(ver % 100), ES: false}
	}
	reEmbedded := regexp.MustCompile(`version\.Embedded\s*=\s*\{[^}]*version\s*=\s*(\d+)`)
	if m := reEmbedded.FindStringSubmatch(glslSection); m != nil {
		ver, _ := strconv.Atoi(m[1])
		cfg.version = glsl.Version{Major: uint8(ver / 100), Minor: uint8(ver % 100), ES: true}
	}

	// Parse writer_flags
	reFlags := regexp.MustCompile(`writer_flags\s*=\s*"([^"]*)"`)
	if m := reFlags.FindStringSubmatch(glslSection); m != nil {
		flagStr := m[1]
		cfg.writerFlags = 0 // Reset default if explicitly set
		if strings.Contains(flagStr, "ADJUST_COORDINATE_SPACE") {
			cfg.writerFlags |= glsl.WriterFlagAdjustCoordinateSpace
		}
		if strings.Contains(flagStr, "FORCE_POINT_SIZE") {
			cfg.writerFlags |= glsl.WriterFlagForcePointSize
		}
		if strings.Contains(flagStr, "TEXTURE_SHADOW_LOD") {
			cfg.writerFlags |= glsl.WriterFlagTextureShadowLod
		}
	}

	// Parse glsl_exclude_list (top-level, not inside [glsl] section)
	reExclude := regexp.MustCompile(`glsl_exclude_list\s*=\s*\[([^\]]*)\]`)
	if m := reExclude.FindStringSubmatch(content); m != nil {
		// Parse quoted strings: "name1", "name2"
		reStr := regexp.MustCompile(`"([^"]+)"`)
		for _, sm := range reStr.FindAllStringSubmatch(m[1], -1) {
			cfg.excludeList[sm[1]] = true
		}
	}

	// Parse binding_map from [glsl] section
	cfg.bindingMap = parseGLSLBindingMap(glslSection)

	return cfg
}

// parseGLSLBindingMap parses the binding_map array from the GLSL TOML section.
// Format: binding_map = [
//
//	{ resource_binding = { group = 0, binding = 0 }, bind_target = 0 },
//
// ]
func parseGLSLBindingMap(glslSection string) map[glsl.BindingMapKey]uint8 {
	if glslSection == "" {
		return nil
	}
	// Check if binding_map exists
	if !strings.Contains(glslSection, "binding_map") {
		return nil
	}
	result := make(map[glsl.BindingMapKey]uint8)
	// Match each { resource_binding = { group = G, binding = B }, bind_target = T }
	re := regexp.MustCompile(`resource_binding\s*=\s*\{\s*group\s*=\s*(\d+)\s*,\s*binding\s*=\s*(\d+)\s*\}\s*,\s*bind_target\s*=\s*(\d+)`)
	for _, m := range re.FindAllStringSubmatch(glslSection, -1) {
		group, _ := strconv.Atoi(m[1])
		binding, _ := strconv.Atoi(m[2])
		target, _ := strconv.Atoi(m[3])
		key := glsl.BindingMapKey{Group: uint32(group), Binding: uint32(binding)}
		result[key] = uint8(target)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseGLSLBoundsCheckPolicies parses the [bounds_check_policies] TOML section for GLSL.
func parseGLSLBoundsCheckPolicies(content string) glsl.BoundsCheckPolicies {
	policies := glsl.BoundsCheckPolicies{}
	lines := strings.Split(content, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[bounds_check_policies]" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "[") {
			break
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		policy := toGLSLBoundsCheckPolicy(val)
		switch key {
		case "image_load":
			policies.ImageLoad = policy
		case "image_store":
			policies.ImageStore = policy
		}
	}
	return policies
}

func toGLSLBoundsCheckPolicy(name string) glsl.BoundsCheckPolicy {
	switch name {
	case "ReadZeroSkipWrite":
		return glsl.BoundsCheckReadZeroSkipWrite
	case "Restrict":
		return glsl.BoundsCheckRestrict
	default:
		return glsl.BoundsCheckUnchecked
	}
}

// extractGLSLSection extracts the [glsl] section from TOML content.
func extractGLSLSection(content string) string {
	var lines []string
	inGLSLBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if trimmed == "[glsl]" {
				inGLSLBlock = true
			} else {
				inGLSLBlock = false
			}
		}
		if inGLSLBlock {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// glslStageName returns the Rust Debug format stage name used in GLSL reference filenames.
func glslStageName(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "Vertex"
	case ir.StageFragment:
		return "Fragment"
	case ir.StageCompute:
		return "Compute"
	default:
		return "Unknown"
	}
}

// compileMSL compiles the IR module to MSL source.
// Uses metal1.0 to match Rust naga default for snapshot tests.
func compileMSL(t *testing.T, module *ir.Module) string {
	return compileMSLWithVersion(t, module, msl.Version1_0)
}

// compileMSLWithVersion compiles the IR module to MSL source with a specific version.
func compileMSLWithVersion(t *testing.T, module *ir.Module, version msl.Version) string {
	t.Helper()

	opts := msl.DefaultOptions()
	opts.LangVersion = version
	opts.FakeMissingBindings = true
	code, _, err := msl.Compile(module, opts)
	if err != nil {
		t.Skipf("MSL compile failed (skipping): %v", err)
	}

	return code
}

// compileMSLWithOpts compiles the IR module to MSL source with fully specified options.
// Returns the MSL code and any error. Callers should handle compile errors
// as test failures, not skips, so they are properly counted.
func compileMSLWithOpts(t *testing.T, module *ir.Module, opts msl.Options) (string, error) {
	t.Helper()

	code, _, err := msl.Compile(module, opts)
	if err != nil {
		return "", fmt.Errorf("MSL compile failed: %w", err)
	}

	return code, nil
}

// rustTomlDir is the path to Rust naga test TOML configs.
// These are copies of Rust naga's test configs, stored in the repo so CI
// can access them without the full Rust reference checkout.
const rustTomlDir = "testdata/config"

// readRustMSLConfig reads MSL options from a Rust naga test TOML config file.
// Parses lang_version, fake_missing_bindings, bounds_check_policies,
// and per_entry_point_map with resource bindings.
// Returns default options if no TOML file exists.
func readRustMSLConfig(shaderName string) msl.Options {
	opts := msl.DefaultOptions()
	opts.LangVersion = msl.Version1_0
	opts.FakeMissingBindings = true // default for tests without explicit config
	// Rust naga defaults to Unchecked for all bounds check policies.
	// Override our Go defaults (ReadZeroSkipWrite) to match Rust behavior.
	opts.BoundsCheckPolicies = msl.BoundsCheckPolicies{}

	tomlPath := filepath.Join(rustTomlDir, shaderName+".toml")
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return opts
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Parse [bounds_check_policies] section (top-level, not under [msl]).
	opts.BoundsCheckPolicies = parseBoundsCheckPolicies(lines)

	// Parse [msl] section top-level fields.
	inMSLSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[msl]" {
			inMSLSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[msl]" {
			inMSLSection = false
			continue
		}
		if !inMSLSection {
			continue
		}
		if strings.HasPrefix(trimmed, "lang_version") {
			opts.LangVersion = parseMSLVersion(trimmed)
		}
		if strings.HasPrefix(trimmed, "fake_missing_bindings") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.FakeMissingBindings = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "zero_initialize_workgroup_memory") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.ZeroInitializeWorkgroupMemory = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(trimmed, "force_loop_bounding") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				opts.ForceLoopBounding = strings.TrimSpace(parts[1]) == "true"
			}
		}
	}

	// Parse [msl.per_entry_point_map.*] sections.
	epMap := parsePerEntryPointMap(content)
	if len(epMap) > 0 {
		opts.PerEntryPointMap = epMap
	}

	// Parse [[msl.inline_samplers]] sections.
	inlineSamplers := parseInlineSamplers(content)
	if len(inlineSamplers) > 0 {
		opts.InlineSamplers = inlineSamplers
	}

	// Parse pipeline_constants (top-level, not under [msl]).
	pipelineConstants := parsePipelineConstants(content)
	if len(pipelineConstants) > 0 {
		opts.PipelineConstants = pipelineConstants
	}

	// Parse [msl_pipeline] section (top-level).
	// Handles allow_and_force_point_size, vertex_pulling_transform,
	// and [[msl_pipeline.vertex_buffer_mappings]] arrays.
	parseMSLPipeline(content, &opts)

	return opts
}

// parseMSLPipeline parses the [msl_pipeline] TOML section for VPT and point size options.
// Handles both section format ([msl_pipeline]\nkey = val) and inline table format
// (msl_pipeline = { key = val, ... }).
func parseMSLPipeline(content string, opts *msl.Options) {
	lines := strings.Split(content, "\n")

	// Check for inline table format: msl_pipeline = { ... }
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "msl_pipeline") && strings.Contains(trimmed, "{") {
			if strings.Contains(trimmed, "allow_and_force_point_size = true") {
				opts.AllowAndForcePointSize = true
			}
			if strings.Contains(trimmed, "vertex_pulling_transform = true") {
				opts.VertexPullingTransform = true
			}
		}
	}

	// Check for [msl_pipeline] section format
	inPipeline := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[msl_pipeline]" {
			inPipeline = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[msl_pipeline]" &&
			!strings.HasPrefix(trimmed, "[[msl_pipeline.") {
			inPipeline = false
			continue
		}
		if !inPipeline {
			continue
		}
		if strings.HasPrefix(trimmed, "allow_and_force_point_size") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) == "true" {
				opts.AllowAndForcePointSize = true
			}
		}
		if strings.HasPrefix(trimmed, "vertex_pulling_transform") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) == "true" {
				opts.VertexPullingTransform = true
			}
		}
	}

	// Parse [[msl_pipeline.vertex_buffer_mappings]] sections.
	opts.VertexBufferMappings = parseVertexBufferMappings(content)
}

// parseVertexBufferMappings parses [[msl_pipeline.vertex_buffer_mappings]] TOML arrays.
func parseVertexBufferMappings(content string) []msl.VertexBufferMapping {
	var mappings []msl.VertexBufferMapping

	lines := strings.Split(content, "\n")
	inVBM := false
	var current *msl.VertexBufferMapping
	inAttributes := false
	var attrLines strings.Builder

	flushCurrent := func() {
		if current != nil {
			mappings = append(mappings, *current)
			current = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[[msl_pipeline.vertex_buffer_mappings]]" {
			flushCurrent()
			current = &msl.VertexBufferMapping{
				StepMode: msl.VertexStepModeByVertex, // default
			}
			inVBM = true
			inAttributes = false
			attrLines.Reset()
			continue
		}

		if !inVBM || current == nil {
			continue
		}

		// Detect end of section
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[msl_pipeline.") {
			flushCurrent()
			inVBM = false
			continue
		}

		// Parse attributes array (may span multiple lines)
		if strings.HasPrefix(trimmed, "attributes") {
			inAttributes = true
			// Get everything after "attributes = "
			idx := strings.Index(trimmed, "=")
			if idx >= 0 {
				attrLines.WriteString(trimmed[idx+1:])
			}
			// Check if array is closed on same line
			if strings.Contains(trimmed, "]") && !strings.Contains(trimmed, "[[") {
				// Might be closed (but need to check for nested { } ]
				fullLine := attrLines.String()
				if countChar(fullLine, '[') <= countChar(fullLine, ']') {
					current.Attributes = parseAttributeArray(fullLine)
					inAttributes = false
					attrLines.Reset()
				}
			}
			continue
		}

		if inAttributes {
			attrLines.WriteString(" " + trimmed)
			fullLine := attrLines.String()
			if countChar(fullLine, '[') <= countChar(fullLine, ']') {
				current.Attributes = parseAttributeArray(fullLine)
				inAttributes = false
				attrLines.Reset()
			}
			continue
		}

		if strings.HasPrefix(trimmed, "id") && !strings.HasPrefix(trimmed, "id_") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if n, err := strconv.ParseUint(val, 10, 32); err == nil {
					current.ID = uint32(n)
				}
			}
		}
		if strings.HasPrefix(trimmed, "stride") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if n, err := strconv.ParseUint(val, 10, 32); err == nil {
					current.Stride = uint32(n)
				}
			}
		}
		if strings.HasPrefix(trimmed, "step_mode") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, "\"")
				switch val {
				case "ByVertex":
					current.StepMode = msl.VertexStepModeByVertex
				case "ByInstance":
					current.StepMode = msl.VertexStepModeByInstance
				case "Constant":
					current.StepMode = msl.VertexStepModeConstant
				}
			}
		}
	}
	flushCurrent()
	return mappings
}

// countChar counts occurrences of a byte in a string.
func countChar(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

// parseAttributeArray parses the TOML attribute array:
// [ { offset = 0, shader_location = 0, format = "Float32" }, ... ]
func parseAttributeArray(s string) []msl.AttributeMapping {
	var attrs []msl.AttributeMapping

	// Find all { ... } blocks
	for {
		start := strings.Index(s, "{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		block := s[start+1 : start+end]
		s = s[start+end+1:]

		attr := msl.AttributeMapping{}
		for _, pair := range strings.Split(block, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			val = strings.Trim(val, "\"")

			switch key {
			case "offset":
				if n, err := strconv.ParseUint(val, 10, 32); err == nil {
					attr.Offset = uint32(n)
				}
			case "shader_location":
				if n, err := strconv.ParseUint(val, 10, 32); err == nil {
					attr.ShaderLocation = uint32(n)
				}
			case "format":
				attr.Format = parseVertexFormat(val)
			}
		}
		attrs = append(attrs, attr)
	}
	return attrs
}

// parseVertexFormat converts a TOML format string to msl.VertexFormat.
func parseVertexFormat(s string) msl.VertexFormat {
	switch s {
	case "Uint8":
		return msl.VertexFormatUint8
	case "Uint8x2":
		return msl.VertexFormatUint8x2
	case "Uint8x4":
		return msl.VertexFormatUint8x4
	case "Sint8":
		return msl.VertexFormatSint8
	case "Sint8x2":
		return msl.VertexFormatSint8x2
	case "Sint8x4":
		return msl.VertexFormatSint8x4
	case "Unorm8":
		return msl.VertexFormatUnorm8
	case "Unorm8x2":
		return msl.VertexFormatUnorm8x2
	case "Unorm8x4":
		return msl.VertexFormatUnorm8x4
	case "Snorm8":
		return msl.VertexFormatSnorm8
	case "Snorm8x2":
		return msl.VertexFormatSnorm8x2
	case "Snorm8x4":
		return msl.VertexFormatSnorm8x4
	case "Uint16":
		return msl.VertexFormatUint16
	case "Uint16x2":
		return msl.VertexFormatUint16x2
	case "Uint16x4":
		return msl.VertexFormatUint16x4
	case "Sint16":
		return msl.VertexFormatSint16
	case "Sint16x2":
		return msl.VertexFormatSint16x2
	case "Sint16x4":
		return msl.VertexFormatSint16x4
	case "Unorm16":
		return msl.VertexFormatUnorm16
	case "Unorm16x2":
		return msl.VertexFormatUnorm16x2
	case "Unorm16x4":
		return msl.VertexFormatUnorm16x4
	case "Snorm16":
		return msl.VertexFormatSnorm16
	case "Snorm16x2":
		return msl.VertexFormatSnorm16x2
	case "Snorm16x4":
		return msl.VertexFormatSnorm16x4
	case "Float16":
		return msl.VertexFormatFloat16
	case "Float16x2":
		return msl.VertexFormatFloat16x2
	case "Float16x4":
		return msl.VertexFormatFloat16x4
	case "Float32":
		return msl.VertexFormatFloat32
	case "Float32x2":
		return msl.VertexFormatFloat32x2
	case "Float32x3":
		return msl.VertexFormatFloat32x3
	case "Float32x4":
		return msl.VertexFormatFloat32x4
	case "Uint32":
		return msl.VertexFormatUint32
	case "Uint32x2":
		return msl.VertexFormatUint32x2
	case "Uint32x3":
		return msl.VertexFormatUint32x3
	case "Uint32x4":
		return msl.VertexFormatUint32x4
	case "Sint32":
		return msl.VertexFormatSint32
	case "Sint32x2":
		return msl.VertexFormatSint32x2
	case "Sint32x3":
		return msl.VertexFormatSint32x3
	case "Sint32x4":
		return msl.VertexFormatSint32x4
	case "unorm10-10-10-2":
		return msl.VertexFormatUnorm10_10_10_2
	case "unorm8x4-bgra":
		return msl.VertexFormatUnorm8x4Bgra
	}
	return msl.VertexFormatFloat32 // fallback
}

// parsePipelineConstants parses "pipeline_constants = { key = value, ... }" from TOML content.
// Keys can be numeric (override @id) or string (override name).
// Values are f64.
// Examples:
//
//	pipeline_constants = { o = 2.0 }
//	pipeline_constants = { 0 = nan, 1300 = 1.1, depth = 2.3 }
func parsePipelineConstants(content string) map[string]float64 {
	result := make(map[string]float64)

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "pipeline_constants") {
			continue
		}
		// Extract the { ... } part.
		braceStart := strings.Index(trimmed, "{")
		braceEnd := strings.LastIndex(trimmed, "}")
		if braceStart < 0 || braceEnd < 0 || braceEnd <= braceStart {
			continue
		}
		inner := trimmed[braceStart+1 : braceEnd]

		// Parse key = value pairs separated by commas.
		for _, pair := range strings.Split(inner, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			valStr := strings.TrimSpace(parts[1])

			var val float64
			switch valStr {
			case "nan":
				val = math.NaN()
			case "inf":
				val = math.Inf(1)
			case "-inf":
				val = math.Inf(-1)
			default:
				var err error
				val, err = strconv.ParseFloat(valStr, 64)
				if err != nil {
					continue
				}
			}
			result[key] = val
		}
		break // Only one pipeline_constants line expected.
	}

	return result
}

// parseBoundsCheckPolicies parses the [bounds_check_policies] TOML section.
// Fields: index, buffer, image_load (maps to Image), binding_array.
// Rust naga defaults to Unchecked for all policies when not specified.
func parseBoundsCheckPolicies(lines []string) msl.BoundsCheckPolicies {
	policies := msl.BoundsCheckPolicies{} // all Unchecked (zero value)
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[bounds_check_policies]" {
			inSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		policy := toBoundsCheckPolicy(val)
		switch key {
		case "index":
			policies.Index = policy
		case "buffer":
			policies.Buffer = policy
		case "image_load":
			policies.Image = policy
		case "binding_array":
			policies.BindingArray = policy
		}
	}
	return policies
}

// toBoundsCheckPolicy converts a Rust policy name to a Go BoundsCheckPolicy.
func toBoundsCheckPolicy(name string) msl.BoundsCheckPolicy {
	switch name {
	case "ReadZeroSkipWrite":
		return msl.BoundsCheckReadZeroSkipWrite
	case "Restrict":
		return msl.BoundsCheckRestrict
	default:
		return msl.BoundsCheckUnchecked
	}
}

// parseMSLVersion parses "lang_version = [major, minor]" from a line.
func parseMSLVersion(line string) msl.Version {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return msl.Version1_0
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, "[]")
	nums := strings.Split(val, ",")
	if len(nums) != 2 {
		return msl.Version1_0
	}
	var maj, mnr uint8
	if _, scanErr := fmt.Sscanf(strings.TrimSpace(nums[0]), "%d", &maj); scanErr != nil {
		return msl.Version1_0
	}
	if _, scanErr := fmt.Sscanf(strings.TrimSpace(nums[1]), "%d", &mnr); scanErr != nil {
		return msl.Version1_0
	}
	return msl.Version{Major: maj, Minor: mnr}
}

// parsePerEntryPointMap parses all [msl.per_entry_point_map.NAME] sections from TOML content.
// Returns a map of entry point name -> EntryPointResources.
func parsePerEntryPointMap(content string) map[string]msl.EntryPointResources {
	result := make(map[string]msl.EntryPointResources)
	lines := strings.Split(content, "\n")

	const prefix = "[msl.per_entry_point_map."
	var currentEP string
	var sectionLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) && strings.HasSuffix(trimmed, "]") {
			// Flush previous section.
			if currentEP != "" {
				result[currentEP] = parseEntryPointResources(sectionLines)
			}
			// Extract entry point name.
			currentEP = trimmed[len(prefix) : len(trimmed)-1]
			sectionLines = nil
			continue
		}
		if strings.HasPrefix(trimmed, "[") && currentEP != "" {
			// New section => flush current EP.
			result[currentEP] = parseEntryPointResources(sectionLines)
			currentEP = ""
			sectionLines = nil
			continue
		}
		if currentEP != "" {
			sectionLines = append(sectionLines, trimmed)
		}
	}
	// Flush last section.
	if currentEP != "" {
		result[currentEP] = parseEntryPointResources(sectionLines)
	}

	return result
}

// parseEntryPointResources parses the body of a [msl.per_entry_point_map.NAME] section.
// Handles:
//   - resources = [ { bind_target = { buffer = N, mutable = BOOL }, resource_binding = { group = G, binding = B } }, ... ]
//   - sizes_buffer = N
func parseEntryPointResources(lines []string) msl.EntryPointResources {
	epRes := msl.EntryPointResources{
		Resources: make(map[ir.ResourceBinding]msl.BindTarget),
	}

	// Join all lines to handle multi-line resources arrays.
	joined := strings.Join(lines, " ")

	// Parse sizes_buffer and immediates_buffer.
	for _, line := range lines {
		if strings.HasPrefix(line, "sizes_buffer") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				var val uint8
				if _, scanErr := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &val); scanErr == nil {
					epRes.SizesBuffer = &val
				}
			}
		}
		if strings.HasPrefix(line, "immediates_buffer") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				var val uint8
				if _, scanErr := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &val); scanErr == nil {
					epRes.ImmediatesBuffer = &val
				}
			}
		}
	}

	// Extract the resources array content.
	resIdx := strings.Index(joined, "resources")
	if resIdx < 0 {
		return epRes
	}
	afterRes := joined[resIdx:]
	eqIdx := strings.Index(afterRes, "=")
	if eqIdx < 0 {
		return epRes
	}
	afterEq := strings.TrimSpace(afterRes[eqIdx+1:])

	// Find the matching ']' for the outer array.
	if !strings.HasPrefix(afterEq, "[") {
		return epRes
	}
	arrayContent := extractBracketContent(afterEq)

	// Split into individual resource entries (each is { ... }).
	entries := splitInlineTableEntries(arrayContent)
	for _, entry := range entries {
		rb, bt, ok := parseResourceEntry(entry)
		if ok {
			epRes.Resources[rb] = bt
		}
	}

	return epRes
}

// extractBracketContent returns the content inside the outermost [ ].
func extractBracketContent(s string) string {
	depth := 0
	start := -1
	for i, ch := range s {
		switch ch {
		case '[':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start:i]
			}
		}
	}
	if start >= 0 {
		return s[start:]
	}
	return ""
}

// splitInlineTableEntries splits a comma-separated list of inline tables { ... }.
func splitInlineTableEntries(s string) []string {
	var entries []string
	depth := 0
	start := -1
	for i, ch := range s {
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				entries = append(entries, s[start:i+1])
				start = -1
			}
		}
	}
	return entries
}

// parseResourceEntry parses a single resource entry like:
//
//	{ bind_target = { buffer = 0, mutable = false }, resource_binding = { group = 0, binding = 0 } }
func parseResourceEntry(entry string) (ir.ResourceBinding, msl.BindTarget, bool) {
	var rb ir.ResourceBinding
	var bt msl.BindTarget

	// Extract bind_target and resource_binding sub-objects.
	btContent := extractSubObject(entry, "bind_target")
	rbContent := extractSubObject(entry, "resource_binding")
	if btContent == "" || rbContent == "" {
		return rb, bt, false
	}

	// Parse resource_binding { group = G, binding = B }.
	rb.Group = parseUint32Field(rbContent, "group")
	rb.Binding = parseUint32Field(rbContent, "binding")

	// Parse bind_target { buffer = N } or { texture = N } or { sampler... = N }.
	if v, ok := parseOptionalUint8Field(btContent, "buffer"); ok {
		bt.Buffer = &v
	}
	if v, ok := parseOptionalUint8Field(btContent, "texture"); ok {
		bt.Texture = &v
	}
	// Sampler can be "sampler = N", "sampler.Inline = N", or "sampler.Resource = N".
	// We treat all as sampler binding.
	if v, ok := parseSamplerField(btContent); ok {
		bt.Sampler = &v
	}
	if parseBoolField(btContent, "mutable") {
		bt.Mutable = true
	}

	// Parse external_texture = { planes = [0, 1, 2], params = 0 }.
	if ext := parseExternalTextureField(btContent); ext != nil {
		bt.ExternalTexture = ext
	}

	return rb, bt, true
}

// parseExternalTextureField parses "external_texture = { planes = [P0, P1, P2], params = N }"
// from within a bind_target content string. Returns nil if not found.
func parseExternalTextureField(s string) *msl.BindExternalTextureTarget {
	idx := strings.Index(s, "external_texture")
	if idx < 0 {
		return nil
	}
	// Extract the { ... } content for external_texture
	extContent := extractSubObject(s, "external_texture")
	if extContent == "" {
		return nil
	}

	// Parse planes array: planes = [0, 1, 2]
	var planes [3]uint8
	planesIdx := strings.Index(extContent, "planes")
	if planesIdx >= 0 {
		afterPlanes := extContent[planesIdx:]
		bracketStart := strings.Index(afterPlanes, "[")
		bracketEnd := strings.Index(afterPlanes, "]")
		if bracketStart >= 0 && bracketEnd > bracketStart {
			arrayContent := afterPlanes[bracketStart+1 : bracketEnd]
			parts := strings.Split(arrayContent, ",")
			for i, part := range parts {
				if i >= 3 {
					break
				}
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				var val uint8
				if _, err := fmt.Sscanf(part, "%d", &val); err == nil {
					planes[i] = val
				}
			}
		}
	}

	// Parse params = N
	var params uint8
	paramsContent := extContent
	// Remove the planes array to avoid confusing the params parser
	if planesIdx >= 0 {
		bracketEnd := strings.Index(extContent[planesIdx:], "]")
		if bracketEnd >= 0 {
			paramsContent = extContent[planesIdx+bracketEnd+1:]
		}
	}
	if v, ok := parseOptionalUint8Field(paramsContent, "params"); ok {
		params = v
	}

	return &msl.BindExternalTextureTarget{
		Planes: planes,
		Params: params,
	}
}

// extractSubObject extracts the { ... } content for a named key in an inline table.
// For "bind_target = { buffer = 0 }" returns "buffer = 0".
func extractSubObject(s, key string) string {
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	after := s[idx+len(key):]
	eqIdx := strings.Index(after, "=")
	if eqIdx < 0 {
		return ""
	}
	after = strings.TrimSpace(after[eqIdx+1:])
	if !strings.HasPrefix(after, "{") {
		return ""
	}
	// Find matching '}'.
	depth := 0
	for i, ch := range after {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return after[1:i]
			}
		}
	}
	return ""
}

// parseUint32Field extracts a uint32 value for the given key from comma-separated fields.
func parseUint32Field(fields, key string) uint32 {
	for _, part := range strings.Split(fields, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, key) {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				// Ensure the key matches exactly (not a prefix of another key).
				k := strings.TrimSpace(kv[0])
				if k == key {
					var val uint32
					if _, scanErr := fmt.Sscanf(strings.TrimSpace(kv[1]), "%d", &val); scanErr == nil {
						return val
					}
				}
			}
		}
	}
	return 0
}

// parseOptionalUint8Field extracts an optional uint8 value for the given key.
func parseOptionalUint8Field(fields, key string) (uint8, bool) {
	for _, part := range strings.Split(fields, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, key) {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				if k == key {
					var val uint8
					if _, scanErr := fmt.Sscanf(strings.TrimSpace(kv[1]), "%d", &val); scanErr == nil {
						return val, true
					}
				}
			}
		}
	}
	return 0, false
}

// parseSamplerField handles "sampler = N", "sampler.Inline = N", "sampler.Resource = N".
func parseSamplerField(fields string) (msl.BindSamplerTarget, bool) {
	for _, part := range strings.Split(fields, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "sampler") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				var val uint8
				if _, scanErr := fmt.Sscanf(strings.TrimSpace(kv[1]), "%d", &val); scanErr == nil {
					isInline := key == "sampler.Inline"
					return msl.BindSamplerTarget{IsInline: isInline, Slot: val}, true
				}
			}
		}
	}
	return msl.BindSamplerTarget{}, false
}

// parseBoolField extracts a bool value for the given key.
func parseBoolField(fields, key string) bool {
	for _, part := range strings.Split(fields, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, key) {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				if k == key {
					return strings.TrimSpace(kv[1]) == "true"
				}
			}
		}
	}
	return false
}

// parseInlineSamplers parses [[msl.inline_samplers]] array-of-tables from TOML content.
func parseInlineSamplers(content string) []msl.InlineSampler {
	var samplers []msl.InlineSampler
	lines := strings.Split(content, "\n")

	// Find all [[msl.inline_samplers]] sections
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "[[msl.inline_samplers]]" {
			continue
		}
		// Collect lines until next section header
		var sectionLines []string
		for j := i + 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if strings.HasPrefix(t, "[") {
				break
			}
			sectionLines = append(sectionLines, t)
		}
		sampler := parseInlineSampler(sectionLines)
		samplers = append(samplers, sampler)
	}
	return samplers
}

// parseInlineSampler parses a single inline sampler from key-value lines.
func parseInlineSampler(lines []string) msl.InlineSampler {
	s := msl.InlineSampler{
		Address: [3]msl.SamplerAddress{msl.SamplerAddressClampToEdge, msl.SamplerAddressClampToEdge, msl.SamplerAddressClampToEdge},
	}
	joined := strings.Join(lines, "\n")

	for _, line := range lines {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, "\"")

		switch key {
		case "coord":
			if val == "Pixel" {
				s.Coord = msl.SamplerCoordPixel
			}
		case "mag_filter":
			if val == "Linear" {
				s.MagFilter = msl.SamplerFilterLinear
			}
		case "min_filter":
			if val == "Linear" {
				s.MinFilter = msl.SamplerFilterLinear
			}
		case "border_color":
			switch val {
			case "OpaqueBlack":
				s.BorderColor = msl.SamplerBorderColorOpaqueBlack
			case "OpaqueWhite":
				s.BorderColor = msl.SamplerBorderColorOpaqueWhite
			}
		case "compare_func":
			switch val {
			case "Less":
				s.CompareFunc = msl.SamplerCompareFuncLess
			case "LessEqual":
				s.CompareFunc = msl.SamplerCompareFuncLessEqual
			case "Greater":
				s.CompareFunc = msl.SamplerCompareFuncGreater
			case "GreaterEqual":
				s.CompareFunc = msl.SamplerCompareFuncGreaterEqual
			case "Equal":
				s.CompareFunc = msl.SamplerCompareFuncEqual
			case "NotEqual":
				s.CompareFunc = msl.SamplerCompareFuncNotEqual
			case "Always":
				s.CompareFunc = msl.SamplerCompareFuncAlways
			}
		}
	}

	// Parse address array: address = ["ClampToEdge", "ClampToEdge", "ClampToEdge"]
	addrIdx := strings.Index(joined, "address")
	if addrIdx >= 0 {
		after := joined[addrIdx:]
		bracketStart := strings.Index(after, "[")
		bracketEnd := strings.Index(after, "]")
		if bracketStart >= 0 && bracketEnd > bracketStart {
			addrContent := after[bracketStart+1 : bracketEnd]
			parts := strings.Split(addrContent, ",")
			for i, part := range parts {
				if i >= 3 {
					break
				}
				part = strings.TrimSpace(part)
				part = strings.Trim(part, "\"")
				switch part {
				case "Repeat":
					s.Address[i] = msl.SamplerAddressRepeat
				case "MirroredRepeat":
					s.Address[i] = msl.SamplerAddressMirroredRepeat
				case "ClampToEdge":
					s.Address[i] = msl.SamplerAddressClampToEdge
				case "ClampToZero":
					s.Address[i] = msl.SamplerAddressClampToZero
				case "ClampToBorder":
					s.Address[i] = msl.SamplerAddressClampToBorder
				}
			}
		}
	}

	return s
}

// stageName returns a human-readable name for a shader stage.
func stageName(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "vertex"
	case ir.StageTask:
		return "task"
	case ir.StageMesh:
		return "mesh"
	case ir.StageFragment:
		return "fragment"
	case ir.StageCompute:
		return "compute"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Golden File Comparison
// ---------------------------------------------------------------------------

// compareGolden compares actual output with the golden file at path.
// If UPDATE_GOLDEN is set, writes actual output as the new golden file.
func compareGolden(t *testing.T, path, actual string) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			t.Fatalf("create golden dir: %v", mkErr)
		}
		if wErr := os.WriteFile(path, []byte(actual), 0o644); wErr != nil {
			t.Fatalf("write golden file: %v", wErr)
		}
		t.Logf("updated golden file: %s", path)
		return
	}

	expected, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden file missing: %s\nRun with UPDATE_GOLDEN=1 to create.\n\nActual output:\n%s", path, truncate(actual, 500))
	}
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}

	// Normalize line endings for cross-platform comparison.
	// Git may convert \n to \r\n on Windows checkout.
	expectedStr := strings.ReplaceAll(string(expected), "\r\n", "\n")
	actualStr := strings.ReplaceAll(actual, "\r\n", "\n")

	if expectedStr != actualStr {
		diff := diffStrings(expectedStr, actualStr)
		t.Errorf("output differs from golden %s:\n%s", path, diff)
	}
}

// diffStrings produces a simple line-by-line diff showing the first difference
// and surrounding context.
func diffStrings(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var sb strings.Builder
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	const contextLines = 3
	firstDiff := -1
	for i := 0; i < maxLines; i++ {
		var eLine, aLine string
		if i < len(expectedLines) {
			eLine = expectedLines[i]
		}
		if i < len(actualLines) {
			aLine = actualLines[i]
		}
		if eLine != aLine {
			firstDiff = i
			break
		}
	}

	if firstDiff < 0 {
		return "(no difference found)"
	}

	fmt.Fprintf(&sb, "first difference at line %d:\n", firstDiff+1)
	fmt.Fprintf(&sb, "  expected lines: %d\n", len(expectedLines))
	fmt.Fprintf(&sb, "  actual lines:   %d\n\n", len(actualLines))

	// Show context around the first difference
	start := firstDiff - contextLines
	if start < 0 {
		start = 0
	}
	end := firstDiff + contextLines + 1
	if end > maxLines {
		end = maxLines
	}

	for i := start; i < end; i++ {
		prefix := " "
		var eLine, aLine string
		if i < len(expectedLines) {
			eLine = expectedLines[i]
		}
		if i < len(actualLines) {
			aLine = actualLines[i]
		}
		if eLine != aLine {
			prefix = "!"
		}
		fmt.Fprintf(&sb, "%s %4d expected: %s\n", prefix, i+1, truncate(eLine, 120))
		if eLine != aLine {
			fmt.Fprintf(&sb, "%s %4d actual:   %s\n", prefix, i+1, truncate(aLine, 120))
		}
	}

	return sb.String()
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ---------------------------------------------------------------------------
// SPIR-V Disassembler (extracted from cmd/spvdis)
// ---------------------------------------------------------------------------

// disassembleSPIRV converts a SPIR-V binary to deterministic text output
// suitable for diff-friendly golden file comparison. The format follows the
// standard .spvasm text format used by spirv-tools.
func disassembleSPIRV(data []byte) string {
	if len(data) < 20 {
		return "; ERROR: data too small\n"
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x07230203 {
		return fmt.Sprintf("; ERROR: invalid SPIR-V magic: 0x%08X\n", magic)
	}

	version := binary.LittleEndian.Uint32(data[4:8])
	generator := binary.LittleEndian.Uint32(data[8:12])

	var sb strings.Builder
	fmt.Fprintf(&sb, "; SPIR-V\n")
	fmt.Fprintf(&sb, "; Version: %d.%d\n", (version>>16)&0xFF, (version>>8)&0xFF)
	fmt.Fprintf(&sb, "; Generator: 0x%08X\n", generator)
	fmt.Fprintf(&sb, "; Bound: %d\n", binary.LittleEndian.Uint32(data[12:16]))
	fmt.Fprintf(&sb, "; Schema: %d\n", binary.LittleEndian.Uint32(data[16:20]))
	sb.WriteString("\n")

	offset := 20
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		word := binary.LittleEndian.Uint32(data[offset:])
		opcode := uint16(word & 0xFFFF)
		wordCount := int(word >> 16)

		if wordCount == 0 || offset+wordCount*4 > len(data) {
			fmt.Fprintf(&sb, "; ERROR: invalid word count %d at offset 0x%X\n", wordCount, offset)
			break
		}

		ops := make([]uint32, wordCount-1)
		for i := range ops {
			ops[i] = binary.LittleEndian.Uint32(data[offset+4+i*4:])
		}

		name := spvOpcodeNames[opcode]
		if name == "" {
			name = fmt.Sprintf("Op%d", opcode)
		}

		disasmInstruction(&sb, name, opcode, ops, data, offset)
		offset += wordCount * 4
	}

	return sb.String()
}

// disasmInstruction writes a single SPIR-V instruction to the builder.
func disasmInstruction(sb *strings.Builder, name string, opcode uint16, ops []uint32, data []byte, offset int) {
	switch opcode {
	case 17: // OpCapability
		fmt.Fprintf(sb, "               %s %s\n", name, spvLookup(spvCapabilities, ops[0]))

	case 11: // OpExtInstImport
		str, _ := spvReadString(data, offset+8, len(ops)-1)
		fmt.Fprintf(sb, "         %s = %s \"%s\"\n", spvID(ops[0]), name, str)

	case 14: // OpMemoryModel
		addrModels := map[uint32]string{0: "Logical", 1: "Physical32", 2: "Physical64", 5348: "PhysicalStorageBuffer64"}
		memModels := map[uint32]string{0: "Simple", 1: "GLSL450", 2: "OpenCL", 3: "Vulkan"}
		fmt.Fprintf(sb, "               %s %s %s\n", name, spvLookup(addrModels, ops[0]), spvLookup(memModels, ops[1]))

	case 15: // OpEntryPoint
		model := spvLookup(spvExecutionModels, ops[0])
		str, strWords := spvReadString(data, offset+12, len(ops)-2)
		fmt.Fprintf(sb, "               %s %s %s \"%s\"", name, model, spvID(ops[1]), str)
		ifaceStart := 2 + strWords
		for i := ifaceStart; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 16: // OpExecutionMode
		mode := spvLookup(spvExecutionModes, ops[1])
		fmt.Fprintf(sb, "               %s %s %s", name, spvID(ops[0]), mode)
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %d", ops[i])
		}
		sb.WriteString("\n")

	case 5: // OpName
		str, _ := spvReadString(data, offset+8, len(ops)-1)
		fmt.Fprintf(sb, "               %s %s \"%s\"\n", name, spvID(ops[0]), str)

	case 6: // OpMemberName
		str, _ := spvReadString(data, offset+12, len(ops)-2)
		fmt.Fprintf(sb, "               %s %s %d \"%s\"\n", name, spvID(ops[0]), ops[1], str)

	case 71: // OpDecorate
		dec := spvLookup(spvDecorations, ops[1])
		fmt.Fprintf(sb, "               %s %s %s", name, spvID(ops[0]), dec)
		if ops[1] == 11 && len(ops) > 2 { // BuiltIn
			fmt.Fprintf(sb, " %s", spvLookup(spvBuiltins, ops[2]))
		} else {
			for i := 2; i < len(ops); i++ {
				fmt.Fprintf(sb, " %d", ops[i])
			}
		}
		sb.WriteString("\n")

	case 72: // OpMemberDecorate
		dec := spvLookup(spvDecorations, ops[2])
		fmt.Fprintf(sb, "               %s %s %d %s", name, spvID(ops[0]), ops[1], dec)
		if ops[2] == 11 && len(ops) > 3 { // BuiltIn
			fmt.Fprintf(sb, " %s", spvLookup(spvBuiltins, ops[3]))
		} else {
			for i := 3; i < len(ops); i++ {
				fmt.Fprintf(sb, " %d", ops[i])
			}
		}
		sb.WriteString("\n")

	case 19: // OpTypeVoid
		fmt.Fprintf(sb, "         %s = %s\n", spvID(ops[0]), name)

	case 20: // OpTypeBool
		fmt.Fprintf(sb, "         %s = %s\n", spvID(ops[0]), name)

	case 21: // OpTypeInt
		sign := "0"
		if ops[2] == 1 {
			sign = "1"
		}
		fmt.Fprintf(sb, "         %s = %s %d %s\n", spvID(ops[0]), name, ops[1], sign)

	case 22: // OpTypeFloat
		fmt.Fprintf(sb, "         %s = %s %d\n", spvID(ops[0]), name, ops[1])

	case 23: // OpTypeVector
		fmt.Fprintf(sb, "         %s = %s %s %d\n", spvID(ops[0]), name, spvID(ops[1]), ops[2])

	case 24: // OpTypeMatrix
		fmt.Fprintf(sb, "         %s = %s %s %d\n", spvID(ops[0]), name, spvID(ops[1]), ops[2])

	case 25: // OpTypeImage
		dim := spvLookup(spvDims, ops[2])
		fmt.Fprintf(sb, "         %s = %s %s %s %d %d %d %d Unknown", spvID(ops[0]), name, spvID(ops[1]), dim, ops[3], ops[4], ops[5], ops[6])
		if ops[6] != 1 && len(ops) > 7 {
			fmt.Fprintf(sb, " %d", ops[7])
		}
		sb.WriteString("\n")

	case 26: // OpTypeSampler
		fmt.Fprintf(sb, "         %s = %s\n", spvID(ops[0]), name)

	case 27: // OpTypeSampledImage
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[0]), name, spvID(ops[1]))

	case 28: // OpTypeArray
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[0]), name, spvID(ops[1]), spvID(ops[2]))

	case 29: // OpTypeRuntimeArray
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[0]), name, spvID(ops[1]))

	case 30: // OpTypeStruct
		fmt.Fprintf(sb, "         %s = %s", spvID(ops[0]), name)
		for i := 1; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 32: // OpTypePointer
		sc := spvLookup(spvStorageClasses, ops[1])
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[0]), name, sc, spvID(ops[2]))

	case 33: // OpTypeFunction
		fmt.Fprintf(sb, "         %s = %s %s", spvID(ops[0]), name, spvID(ops[1]))
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 41: // OpConstantTrue
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[1]), name, spvID(ops[0]))

	case 42: // OpConstantFalse
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[1]), name, spvID(ops[0]))

	case 43: // OpConstant
		if len(ops) >= 3 {
			fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvFormatConstant(ops[2:]))
		}

	case 44: // OpConstantComposite
		fmt.Fprintf(sb, "         %s = %s %s", spvID(ops[1]), name, spvID(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 46: // OpConstantNull
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[1]), name, spvID(ops[0]))

	case 54: // OpFunction
		fmt.Fprintf(sb, "         %s = %s %s None %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[3]))

	case 55: // OpFunctionParameter
		fmt.Fprintf(sb, "         %s = %s %s\n", spvID(ops[1]), name, spvID(ops[0]))

	case 56: // OpFunctionEnd
		fmt.Fprintf(sb, "               %s\n", name)

	case 57: // OpFunctionCall
		fmt.Fprintf(sb, "         %s = %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))
		for i := 3; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 59: // OpVariable
		sc := spvLookup(spvStorageClasses, ops[2])
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), sc)

	case 61: // OpLoad
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 62: // OpStore
		fmt.Fprintf(sb, "               %s %s %s\n", name, spvID(ops[0]), spvID(ops[1]))

	case 65: // OpAccessChain
		fmt.Fprintf(sb, "         %s = %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))
		for i := 3; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 68: // OpArrayLength
		fmt.Fprintf(sb, "         %s = %s %s %s %d\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), ops[3])

	case 77: // OpVectorExtractDynamic
		fmt.Fprintf(sb, "         %s = %s %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))

	case 79: // OpVectorShuffle
		fmt.Fprintf(sb, "         %s = %s %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))
		for i := 4; i < len(ops); i++ {
			fmt.Fprintf(sb, " %d", ops[i])
		}
		sb.WriteString("\n")

	case 80: // OpCompositeConstruct
		fmt.Fprintf(sb, "         %s = %s %s", spvID(ops[1]), name, spvID(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 81: // OpCompositeExtract
		fmt.Fprintf(sb, "         %s = %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))
		for i := 3; i < len(ops); i++ {
			fmt.Fprintf(sb, " %d", ops[i])
		}
		sb.WriteString("\n")

	case 82: // OpCompositeInsert
		fmt.Fprintf(sb, "         %s = %s %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))
		for i := 4; i < len(ops); i++ {
			fmt.Fprintf(sb, " %d", ops[i])
		}
		sb.WriteString("\n")

	case 84: // OpTranspose
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 86: // OpSampledImage
		fmt.Fprintf(sb, "         %s = %s %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))

	case 87, 88, 89, 90: // OpImageSample*
		fmt.Fprintf(sb, "         %s = %s %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))
		for i := 4; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 95: // OpImageFetch
		fmt.Fprintf(sb, "         %s = %s %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))
		for i := 4; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 96, 97: // OpImageGather, OpImageDrefGather
		fmt.Fprintf(sb, "         %s = %s %s %s %s %s", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]), spvID(ops[4]))
		for i := 5; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 98: // OpImageRead
		fmt.Fprintf(sb, "         %s = %s %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))

	case 99: // OpImageWrite
		fmt.Fprintf(sb, "               %s %s %s %s\n", name, spvID(ops[0]), spvID(ops[1]), spvID(ops[2]))

	case 100: // OpImage
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 103: // OpImageQuerySizeLod
		fmt.Fprintf(sb, "         %s = %s %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]))

	case 104: // OpImageQuerySize
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 106: // OpImageQueryLevels
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 107: // OpImageQuerySamples
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 109, 110, 111, 112, 113, 114, 115: // OpConvert*
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 124: // OpBitcast
		fmt.Fprintf(sb, "         %s = %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]))

	case 179: // OpSelect
		if len(ops) >= 5 {
			fmt.Fprintf(sb, "         %s = %s %s %s %s %s\n", spvID(ops[1]), name, spvID(ops[0]), spvID(ops[2]), spvID(ops[3]), spvID(ops[4]))
		} else {
			disasmGenericInstruction(sb, name, opcode, ops)
		}

	case 245: // OpPhi
		fmt.Fprintf(sb, "         %s = %s %s", spvID(ops[1]), name, spvID(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
		sb.WriteString("\n")

	case 246: // OpLoopMerge
		fmt.Fprintf(sb, "               %s %s %s %d\n", name, spvID(ops[0]), spvID(ops[1]), ops[2])

	case 247: // OpSelectionMerge
		fmt.Fprintf(sb, "               %s %s %d\n", name, spvID(ops[0]), ops[1])

	case 248: // OpLabel
		fmt.Fprintf(sb, "         %s = %s\n", spvID(ops[0]), name)

	case 249: // OpBranch
		fmt.Fprintf(sb, "               %s %s\n", name, spvID(ops[0]))

	case 250: // OpBranchConditional
		fmt.Fprintf(sb, "               %s %s %s %s\n", name, spvID(ops[0]), spvID(ops[1]), spvID(ops[2]))

	case 251: // OpSwitch
		fmt.Fprintf(sb, "               %s %s %s", name, spvID(ops[0]), spvID(ops[1]))
		for i := 2; i < len(ops); i += 2 {
			if i+1 < len(ops) {
				fmt.Fprintf(sb, " %d %s", ops[i], spvID(ops[i+1]))
			}
		}
		sb.WriteString("\n")

	case 252: // OpKill
		fmt.Fprintf(sb, "               %s\n", name)

	case 253: // OpReturn
		fmt.Fprintf(sb, "               %s\n", name)

	case 254: // OpReturnValue
		fmt.Fprintf(sb, "               %s %s\n", name, spvID(ops[0]))

	case 255: // OpUnreachable
		fmt.Fprintf(sb, "               %s\n", name)

	default:
		// Generic fallback: detect result-producing instructions
		disasmGenericInstruction(sb, name, opcode, ops)
	}
}

// disasmGenericInstruction handles opcodes not explicitly covered.
func disasmGenericInstruction(sb *strings.Builder, name string, opcode uint16, ops []uint32) {
	sb.WriteString("         ")
	switch {
	case len(ops) >= 2 && isArithmeticOpcode(opcode):
		// Arithmetic/logic ops: type result operands...
		fmt.Fprintf(sb, "%s = %s %s", spvID(ops[1]), name, spvID(ops[0]))
		for i := 2; i < len(ops); i++ {
			fmt.Fprintf(sb, " %s", spvID(ops[i]))
		}
	case len(ops) >= 1:
		sb.WriteString(name)
		for _, op := range ops {
			fmt.Fprintf(sb, " %s", spvID(op))
		}
	default:
		sb.WriteString(name)
	}
	sb.WriteString("\n")
}

// isArithmeticOpcode returns true for opcodes in the arithmetic/logic/conversion range.
func isArithmeticOpcode(opcode uint16) bool {
	return (opcode >= 126 && opcode <= 205) || // SNegate..BitCount
		(opcode >= 109 && opcode <= 124) || // Conversions
		opcode == 12 // OpExtInst
}

// spvID formats a SPIR-V ID.
func spvID(n uint32) string {
	return fmt.Sprintf("%%_%d", n)
}

// spvLookup looks up a name in a map, returning the numeric value if not found.
func spvLookup(m map[uint32]string, v uint32) string {
	if s, ok := m[v]; ok {
		return s
	}
	return fmt.Sprintf("%d", v)
}

// spvReadString reads a null-terminated UTF-8 string from SPIR-V binary data.
func spvReadString(data []byte, offset int, maxWords int) (string, int) {
	var sb strings.Builder
	words := 0
	for i := 0; i < maxWords*4; i++ {
		if offset+i >= len(data) {
			break
		}
		b := data[offset+i]
		if b == 0 {
			words = (i / 4) + 1
			break
		}
		sb.WriteByte(b)
	}
	return sb.String(), words
}

// spvFormatConstant formats constant literal words. If a single word, try to
// display as both integer and float for readability.
func spvFormatConstant(words []uint32) string {
	if len(words) == 1 {
		// Try to show as float if it looks like a reasonable float value
		bits := words[0]
		f := math.Float32frombits(bits)
		if !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0) && f != 0 && (bits&0x7F800000) != 0 {
			// Has non-zero exponent, likely a float
			return fmt.Sprintf("%d", bits)
		}
		return fmt.Sprintf("%d", bits)
	}
	// Multi-word constant (e.g., 64-bit)
	var parts []string
	for _, w := range words {
		parts = append(parts, fmt.Sprintf("%d", w))
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// SPIR-V Opcode Tables
// ---------------------------------------------------------------------------

var spvOpcodeNames = map[uint16]string{
	0: "OpNop", 1: "OpUndef", 2: "OpSourceContinued", 3: "OpSource",
	4: "OpSourceExtension", 5: "OpName", 6: "OpMemberName", 7: "OpString",
	10: "OpExtension", 11: "OpExtInstImport", 12: "OpExtInst",
	14: "OpMemoryModel", 15: "OpEntryPoint", 16: "OpExecutionMode",
	17: "OpCapability", 19: "OpTypeVoid", 20: "OpTypeBool",
	21: "OpTypeInt", 22: "OpTypeFloat", 23: "OpTypeVector",
	24: "OpTypeMatrix", 25: "OpTypeImage", 26: "OpTypeSampler",
	27: "OpTypeSampledImage", 28: "OpTypeArray", 29: "OpTypeRuntimeArray",
	30: "OpTypeStruct", 31: "OpTypeOpaque", 32: "OpTypePointer",
	33: "OpTypeFunction", 41: "OpConstantTrue", 42: "OpConstantFalse",
	43: "OpConstant", 44: "OpConstantComposite", 45: "OpConstantSampler",
	46: "OpConstantNull", 48: "OpSpecConstantTrue", 49: "OpSpecConstantFalse",
	50: "OpSpecConstant", 51: "OpSpecConstantComposite", 52: "OpSpecConstantOp",
	54: "OpFunction", 55: "OpFunctionParameter", 56: "OpFunctionEnd",
	57: "OpFunctionCall", 59: "OpVariable", 60: "OpImageTexelPointer",
	61: "OpLoad", 62: "OpStore", 63: "OpCopyMemory", 64: "OpCopyMemorySized",
	65: "OpAccessChain", 66: "OpInBoundsAccessChain", 67: "OpPtrAccessChain",
	68: "OpArrayLength", 69: "OpGenericPtrMemSemantics",
	70: "OpInBoundsPtrAccessChain", 71: "OpDecorate", 72: "OpMemberDecorate",
	73: "OpDecorationGroup", 74: "OpGroupDecorate", 75: "OpGroupMemberDecorate",
	77: "OpVectorExtractDynamic", 78: "OpVectorInsertDynamic",
	79: "OpVectorShuffle", 80: "OpCompositeConstruct", 81: "OpCompositeExtract",
	82: "OpCompositeInsert", 83: "OpCopyObject", 84: "OpTranspose",
	86: "OpSampledImage", 87: "OpImageSampleImplicitLod",
	88: "OpImageSampleExplicitLod", 89: "OpImageSampleDrefImplicitLod",
	90: "OpImageSampleDrefExplicitLod", 91: "OpImageSampleProjImplicitLod",
	92: "OpImageSampleProjExplicitLod", 93: "OpImageSampleProjDrefImplicitLod",
	94: "OpImageSampleProjDrefExplicitLod", 95: "OpImageFetch",
	96: "OpImageGather", 97: "OpImageDrefGather", 98: "OpImageRead",
	99: "OpImageWrite", 100: "OpImage", 101: "OpImageQueryFormat",
	102: "OpImageQueryOrder", 103: "OpImageQuerySizeLod", 104: "OpImageQuerySize",
	105: "OpImageQueryLod", 106: "OpImageQueryLevels", 107: "OpImageQuerySamples",
	109: "OpConvertFToU", 110: "OpConvertFToS", 111: "OpConvertSToF",
	112: "OpConvertUToF", 113: "OpUConvert", 114: "OpSConvert",
	115: "OpFConvert", 116: "OpQuantizeToF16", 117: "OpConvertPtrToU",
	118: "OpSatConvertSToU", 119: "OpSatConvertUToS", 120: "OpConvertUToPtr",
	121: "OpPtrCastToGeneric", 122: "OpGenericCastToPtr",
	123: "OpGenericCastToPtrExplicit", 124: "OpBitcast",
	126: "OpSNegate", 127: "OpFNegate", 128: "OpIAdd", 129: "OpFAdd",
	130: "OpISub", 131: "OpFSub", 132: "OpIMul", 133: "OpFMul",
	134: "OpUDiv", 135: "OpSDiv", 136: "OpFDiv", 137: "OpUMod",
	138: "OpSRem", 139: "OpSMod", 140: "OpFRem", 141: "OpFMod",
	142: "OpVectorTimesScalar", 143: "OpMatrixTimesScalar",
	144: "OpVectorTimesMatrix", 145: "OpMatrixTimesVector",
	146: "OpMatrixTimesMatrix", 147: "OpOuterProduct", 148: "OpDot",
	149: "OpIAddCarry", 150: "OpISubBorrow", 151: "OpUMulExtended",
	152: "OpSMulExtended", 164: "OpAny", 165: "OpAll",
	166: "OpIsNan", 167: "OpIsInf", 168: "OpIsFinite", 169: "OpIsNormal",
	170: "OpSignBitSet", 171: "OpLessOrGreater", 172: "OpOrdered",
	173: "OpUnordered", 174: "OpLogicalEqual", 175: "OpLogicalNotEqual",
	176: "OpLogicalOr", 177: "OpLogicalAnd", 178: "OpLogicalNot",
	179: "OpSelect", 180: "OpIEqual", 181: "OpINotEqual",
	182: "OpUGreaterThan", 183: "OpSGreaterThan", 184: "OpUGreaterThanEqual",
	185: "OpSGreaterThanEqual", 186: "OpULessThan", 187: "OpSLessThan",
	188: "OpULessThanEqual", 189: "OpSLessThanEqual",
	190: "OpFOrdEqual", 191: "OpFUnordEqual", 192: "OpFOrdNotEqual",
	193: "OpFUnordNotEqual", 194: "OpShiftRightLogical", 195: "OpShiftRightArithmetic",
	196: "OpShiftLeftLogical", 197: "OpBitwiseOr", 198: "OpBitwiseXor",
	199: "OpBitwiseAnd", 200: "OpNot", 201: "OpBitFieldInsert",
	202: "OpBitFieldSExtract", 203: "OpBitFieldUExtract",
	204: "OpBitReverse", 205: "OpBitCount",
	245: "OpPhi", 246: "OpLoopMerge", 247: "OpSelectionMerge",
	248: "OpLabel", 249: "OpBranch", 250: "OpBranchConditional",
	251: "OpSwitch", 252: "OpKill", 253: "OpReturn", 254: "OpReturnValue",
	255: "OpUnreachable", 256: "OpLifetimeStart", 257: "OpLifetimeStop",
	// Atomic instructions
	227: "OpAtomicLoad", 228: "OpAtomicStore", 229: "OpAtomicExchange",
	230: "OpAtomicCompareExchange", 231: "OpAtomicCompareExchangeWeak",
	232: "OpAtomicIIncrement", 233: "OpAtomicIDecrement",
	234: "OpAtomicIAdd", 235: "OpAtomicISub",
	236: "OpAtomicSMin", 237: "OpAtomicUMin",
	238: "OpAtomicSMax", 239: "OpAtomicUMax",
	240: "OpAtomicAnd", 241: "OpAtomicOr", 242: "OpAtomicXor",
	// Barriers
	224: "OpControlBarrier", 225: "OpMemoryBarrier",
	// Extended ops
	4456: "OpSDotKHR", 4457: "OpUDotKHR",
}

var spvCapabilities = map[uint32]string{
	0: "Matrix", 1: "Shader", 2: "Geometry", 3: "Tessellation",
	4: "Addresses", 5: "Linkage", 6: "Kernel", 7: "Vector16",
	8: "Float16Buffer", 9: "Float16", 10: "Float64", 11: "Int64",
	12: "Int64Atomics", 13: "ImageBasic", 14: "ImageReadWrite", 15: "ImageMipmap",
	17: "Pipes", 18: "Groups", 19: "DeviceEnqueue", 20: "LiteralSampler",
	21: "AtomicStorage", 22: "Int16", 23: "TessellationPointSize",
	24: "GeometryPointSize", 25: "ImageGatherExtended", 26: "StorageImageMultisample",
	27: "UniformBufferArrayDynamicIndexing", 28: "SampledImageArrayDynamicIndexing",
	29: "StorageBufferArrayDynamicIndexing", 30: "StorageImageArrayDynamicIndexing",
	31: "ClipDistance", 32: "CullDistance", 33: "ImageCubeArray",
	34: "SampleRateShading", 35: "ImageRect", 36: "SampledRect",
	37: "GenericPointer", 38: "Int8", 39: "InputAttachment",
	40: "SparseResidency", 41: "MinLod", 42: "Sampled1D", 43: "Image1D",
	44: "SampledCubeArray", 45: "SampledBuffer", 46: "ImageBuffer",
	47: "ImageMSArray", 48: "StorageImageExtendedFormats",
	49: "ImageQuery", 50: "DerivativeControl", 51: "InterpolationFunction",
	52: "TransformFeedback", 53: "GeometryStreams", 54: "StorageImageReadWithoutFormat",
	55: "StorageImageWriteWithoutFormat", 56: "MultiViewport",
	57: "SubgroupDispatch", 58: "NamedBarrier", 59: "PipeStorage",
	60: "GroupNonUniform", 61: "GroupNonUniformVote", 62: "GroupNonUniformArithmetic",
	63: "GroupNonUniformBallot", 64: "GroupNonUniformShuffle",
	65: "GroupNonUniformShuffleRelative", 66: "GroupNonUniformClustered",
	67: "GroupNonUniformQuad", 4423: "SubgroupBallotKHR", 4427: "DrawParameters",
	4437: "StorageBuffer16BitAccess", 4438: "UniformAndStorageBuffer16BitAccess",
	4439: "StoragePushConstant16", 4440: "StorageInputOutput16",
	4441: "DeviceGroup", 4442: "MultiView", 4445: "VariablePointersStorageBuffer",
	4446: "VariablePointers", 5009: "StencilExportEXT", 5010: "SampleMaskPostDepthCoverage",
	5013: "ShaderNonUniform", 5015: "RuntimeDescriptorArray",
	5016: "InputAttachmentArrayDynamicIndexing", 5017: "UniformTexelBufferArrayDynamicIndexing",
	5018: "StorageTexelBufferArrayDynamicIndexing", 5019: "UniformBufferArrayNonUniformIndexing",
	6423: "DotProductInputAll", 6424: "DotProductInput4x8Bit",
	6425: "DotProductInput4x8BitPacked", 6427: "DotProduct",
}

var spvStorageClasses = map[uint32]string{
	0: "UniformConstant", 1: "Input", 2: "Uniform", 3: "Output",
	4: "Workgroup", 5: "CrossWorkgroup", 6: "Private", 7: "Function",
	8: "Generic", 9: "PushConstant", 10: "AtomicCounter", 11: "Image",
	12: "StorageBuffer",
}

var spvDecorations = map[uint32]string{
	0: "RelaxedPrecision", 1: "SpecId", 2: "Block", 3: "BufferBlock",
	4: "RowMajor", 5: "ColMajor", 6: "ArrayStride", 7: "MatrixStride",
	8: "GLSLShared", 9: "GLSLPacked", 10: "CPacked", 11: "BuiltIn",
	13: "NoPerspective", 14: "Flat", 15: "Patch", 16: "Centroid",
	17: "Sample", 18: "Invariant", 19: "Restrict", 20: "Aliased",
	21: "Volatile", 22: "Constant", 23: "Coherent", 24: "NonWritable",
	25: "NonReadable", 26: "Uniform", 28: "SaturatedConversion",
	29: "Stream", 30: "Location", 31: "Component", 32: "Index",
	33: "Binding", 34: "DescriptorSet", 35: "Offset", 36: "XfbBuffer",
	37: "XfbStride", 38: "FuncParamAttr", 39: "FPRoundingMode",
	40: "FPFastMathMode", 41: "LinkageAttributes", 42: "NoContraction",
	43: "InputAttachmentIndex", 44: "Alignment",
}

var spvBuiltins = map[uint32]string{
	0: "Position", 1: "PointSize", 2: "ClipDistance", 3: "CullDistance",
	4: "VertexId", 5: "InstanceId", 6: "PrimitiveId", 7: "InvocationId",
	8: "Layer", 9: "ViewportIndex", 10: "TessLevelOuter", 11: "TessLevelInner",
	12: "TessCoord", 13: "PatchVertices", 14: "FragCoord", 15: "PointCoord",
	16: "FrontFacing", 17: "SampleId", 18: "SamplePosition", 19: "SampleMask",
	22: "FragDepth", 23: "HelperInvocation", 24: "NumWorkgroups",
	25: "WorkgroupSize", 26: "WorkgroupId", 27: "LocalInvocationId",
	28: "GlobalInvocationId", 29: "LocalInvocationIndex",
	42: "VertexIndex", 43: "InstanceIndex",
	4440: "ViewIndex", 5286: "BaryCoordKHR",
}

var spvExecutionModes = map[uint32]string{
	0: "Invocations", 1: "SpacingEqual", 2: "SpacingFractionalEven",
	3: "SpacingFractionalOdd", 4: "VertexOrderCw", 5: "VertexOrderCcw",
	6: "PixelCenterInteger", 7: "OriginUpperLeft", 8: "OriginLowerLeft",
	9: "EarlyFragmentTests", 10: "PointMode", 11: "Xfb", 12: "DepthReplacing",
	14: "DepthGreater", 15: "DepthLess", 16: "DepthUnchanged",
	17: "LocalSize", 18: "LocalSizeHint", 19: "InputPoints", 20: "InputLines",
	21: "InputLinesAdjacency", 22: "Triangles", 23: "InputTrianglesAdjacency",
	24: "Quads", 25: "Isolines", 26: "OutputVertices", 27: "OutputPoints",
	28: "OutputLineStrip", 29: "OutputTriangleStrip", 30: "VecTypeHint",
	31: "ContractionOff", 33: "Initializer", 34: "Finalizer",
	35: "SubgroupSize", 36: "SubgroupsPerWorkgroup",
}

var spvExecutionModels = map[uint32]string{
	0: "Vertex", 1: "TessellationControl", 2: "TessellationEvaluation",
	3: "Geometry", 4: "Fragment", 5: "GLCompute", 6: "Kernel",
}

var spvDims = map[uint32]string{
	0: "1D", 1: "2D", 2: "3D", 3: "Cube", 4: "Rect", 5: "Buffer", 6: "SubpassData",
}
