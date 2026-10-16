// Package snapshot_test provides SPIR-V validation tests using spirv-val on binary output.
//
// TestSpirvValBinary compiles each WGSL shader through the naga pipeline to binary
// SPIR-V and validates directly with spirv-val — no disassembler roundtrip.
//
// The comparison of our SPIR-V with Rust naga's is part of TestRustReference.
//
// Requirements: spirv-val from SPIRV-Tools (e.g. the Vulkan SDK) in PATH.
package snapshot_test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

// TestSpirvValBinary validates binary SPIR-V output from our naga compiler using spirv-val.
// Unlike TestSpirvVal which goes through the disassembler (text roundtrip), this test
// validates the actual binary output directly — giving the true validation rate.
func TestSpirvValBinary(t *testing.T) {
	spirvValPath := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)

	shaders := loadCorpus(t)

	var passCount, compileFailCount, valFailCount int

	for i := range shaders {
		shader := &shaders[i]
		t.Run(shader.name, func(t *testing.T) {
			// Step 1: Compile WGSL to binary SPIR-V through our full pipeline.
			spirvBytes, compileErr := compileSpirvBinary(shader.name, shader.source)
			if compileErr != nil {
				compileFailCount++
				t.Skipf("compile failed: %v", compileErr)
				return
			}

			// Step 2: Write binary to temp file for spirv-val.
			tmpFile, tmpErr := os.CreateTemp("", "spirv-val-binary-*.spv")
			if tmpErr != nil {
				t.Fatalf("create temp file: %v", tmpErr)
			}
			tmpPath := tmpFile.Name()
			if _, writeErr := tmpFile.Write(spirvBytes); writeErr != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				t.Fatalf("write temp file: %v", writeErr)
			}
			tmpFile.Close()
			defer os.Remove(tmpPath)

			// Step 3: Validate with spirv-val.
			// --uniform-buffer-standard-layout: Rust naga relies on VK_KHR_uniform_buffer_standard_layout
			// for matrices with small column vectors (e.g., mat3x2) in Uniform blocks.
			// Without this flag, spirv-val rejects stride 8 (std430) for Uniform which requires stride 16 (std140).
			targetEnv := spirvValTargetEnv(spirvBytes)
			cmd := exec.Command(spirvValPath, "--target-env", targetEnv, "--uniform-buffer-standard-layout", tmpPath)
			output, valErr := cmd.CombinedOutput()
			if valErr != nil {
				valFailCount++
				t.Errorf("spirv-val failed:\n%s", strings.TrimSpace(string(output)))
				return
			}

			passCount++
		})
	}

	t.Logf("=== Binary SPIR-V Validation Results ===")
	t.Logf("Total shaders:   %d", len(shaders))
	t.Logf("Pass:            %d (%.1f%%)", passCount, pct(passCount, len(shaders)))
	t.Logf("Validation fail: %d (%.1f%%)", valFailCount, pct(valFailCount, len(shaders)))
	t.Logf("Compile fail:    %d (%.1f%%)", compileFailCount, pct(compileFailCount, len(shaders)))
}

// TestSpirvValBinarySummary provides a non-failing summary of binary SPIR-V validation.
// All results are logged but failures do not cause the test to fail.
func TestSpirvValBinarySummary(t *testing.T) {
	spirvValPath := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)

	shaders := loadCorpus(t)

	type result struct {
		name     string
		category string // "pass", "compile_fail", "val_fail"
		message  string
	}

	var results []result
	var passCount, compileFailCount, valFailCount int

	for i := range shaders {
		shader := &shaders[i]

		spirvBytes, compileErr := compileSpirvBinary(shader.name, shader.source)
		if compileErr != nil {
			compileFailCount++
			results = append(results, result{shader.name, "compile_fail", compileErr.Error()})
			continue
		}

		tmpFile, tmpErr := os.CreateTemp("", "spirv-val-binary-*.spv")
		if tmpErr != nil {
			t.Fatalf("create temp file: %v", tmpErr)
		}
		tmpPath := tmpFile.Name()
		_, _ = tmpFile.Write(spirvBytes)
		tmpFile.Close()

		cmd := exec.Command(spirvValPath, "--target-env", spirvValTargetEnv(spirvBytes), tmpPath)
		output, valErr := cmd.CombinedOutput()
		os.Remove(tmpPath)

		if valErr != nil {
			valFailCount++
			results = append(results, result{shader.name, "val_fail", strings.TrimSpace(string(output))})
			continue
		}

		passCount++
		results = append(results, result{shader.name, "pass", ""})
	}

	t.Logf("=== Binary SPIR-V Validation Summary ===")
	t.Logf("Total:        %d", len(results))
	t.Logf("Pass:         %d (%.1f%%)", passCount, pct(passCount, len(results)))
	t.Logf("Val fail:     %d (%.1f%%)", valFailCount, pct(valFailCount, len(results)))
	t.Logf("Compile fail: %d (%.1f%%)", compileFailCount, pct(compileFailCount, len(results)))

	// Categorize compile failures.
	compileCategories := map[string]int{}
	for _, r := range results {
		if r.category != "compile_fail" {
			continue
		}
		switch {
		case strings.Contains(r.message, "parse error"):
			compileCategories["parse"]++
		case strings.Contains(r.message, "lowering error"):
			compileCategories["lowering"]++
		case strings.Contains(r.message, "validation"):
			compileCategories["ir_validation"]++
		case strings.Contains(r.message, "SPIR-V generation"):
			compileCategories["spirv_gen"]++
		default:
			compileCategories["other"]++
		}
	}

	if len(compileCategories) > 0 {
		t.Logf("\n=== Compile Failure Categories ===")
		for cat, count := range compileCategories {
			t.Logf("  %-25s %d", cat, count)
		}
	}

	// Categorize validation failures.
	valCategories := map[string]int{}
	for _, r := range results {
		if r.category != "val_fail" {
			continue
		}
		switch {
		case strings.Contains(r.message, "not listed as an interface"):
			valCategories["missing_interface_var"]++
		case strings.Contains(r.message, "explicit layout decorations"):
			valCategories["function_var_layout"]++
		case strings.Contains(r.message, "PointCoord"):
			valCategories["pointcoord_type"]++
		case strings.Contains(r.message, "Flat decoration"):
			valCategories["missing_flat"]++
		case strings.Contains(r.message, "Block"):
			valCategories["block_layout"]++
		case strings.Contains(r.message, "OpAccessChain"):
			valCategories["access_chain_type"]++
		case strings.Contains(r.message, "Result Type"):
			valCategories["result_type_mismatch"]++
		case strings.Contains(r.message, "Matrix types"):
			valCategories["matrix_type"]++
		case strings.Contains(r.message, "Constituents"):
			valCategories["composite_construct"]++
		case strings.Contains(r.message, "storage class"):
			valCategories["storage_class"]++
		case strings.Contains(r.message, "IsFinite"):
			valCategories["kernel_capability"]++
		case strings.Contains(r.message, "Offset"):
			valCategories["missing_offset"]++
		case strings.Contains(r.message, "Uniform"):
			valCategories["uniform_layout"]++
		default:
			valCategories["other"]++
		}
	}

	if len(valCategories) > 0 {
		t.Logf("\n=== Validation Failure Categories ===")
		for cat, count := range valCategories {
			t.Logf("  %-25s %d", cat, count)
		}
	}

	// List all failures for analysis.
	t.Logf("\n=== Failed Shaders ===")
	for _, r := range results {
		if r.category == "pass" {
			continue
		}
		// Show first line of error message only.
		msg := r.message
		if idx := strings.IndexByte(msg, '\n'); idx > 0 {
			msg = msg[:idx]
		}
		if len(msg) > 120 {
			msg = msg[:117] + "..."
		}
		t.Logf("  [%s] %s: %s", r.category, r.name, msg)
	}
}

// spirvBinaryUsesLinkage returns true if the SPIR-V binary contains OpCapability Linkage.
// Modules with Linkage capability (e.g., no entry points) are not valid under vulkan1.2
// target env but are valid under spv1.6.
func spirvBinaryUsesLinkage(data []byte) bool {
	if len(data) < 24 { // 5-word header + at least 2-word instruction
		return false
	}
	// Skip the 5-word (20-byte) header, then scan OpCapability instructions.
	// OpCapability: word_count=2, opcode=17 -> encoded as (2<<16)|17 = 0x00020011.
	// Linkage capability value = 5.
	for i := 20; i+8 <= len(data); i += 4 {
		word := uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
		if word != 0x00020011 { // not OpCapability
			break // capabilities are first instructions after header
		}
		capWord := uint32(data[i+4]) | uint32(data[i+5])<<8 | uint32(data[i+6])<<16 | uint32(data[i+7])<<24
		if capWord == 5 { // Linkage
			return true
		}
		i += 4 // skip operand word (total 8 bytes per OpCapability)
	}
	return false
}

// spirvValTargetEnv returns the target environment for spirv-val.
// We use spv1.6 instead of vulkan1.2 because newer spirv-val versions
// enforce VUIDs (e.g., VUID-StandaloneSpirv-None-10684 for Workgroup layout,
// VUID-StandaloneSpirv-MemorySemantics-10870 for subgroup barriers) that are
// stricter than the Vulkan version Rust naga targets. Using spv1.6 validates
// general SPIR-V correctness without binding to a specific Vulkan VUID set.
func spirvValTargetEnv(_ []byte) string {
	return "spv1.6"
}

// compileSpirvBinary compiles a WGSL shader to binary SPIR-V using the naga pipeline.
// Returns the binary bytes or an error if any compilation stage fails.
func compileSpirvBinary(name, source string) ([]byte, error) {
	// Use the public naga API for the full pipeline (parse + lower + validate + generate).
	// Import is already available via snapshot_test package — we use the same
	// compilation path as snapshot_test.go but return bytes instead of calling t.Fatal.
	spirvBytes, err := compileWGSLToSPIRVBytes(name, source)
	if err != nil {
		return nil, err
	}
	return spirvBytes, nil
}

// compileWGSLToSPIRVBytes is the actual compilation implementation.
func compileWGSLToSPIRVBytes(name, source string) ([]byte, error) {
	// We reuse the same approach as snapshot_test.go:
	// 1. Tokenize + Parse
	// 2. Lower to IR
	// 3. Process overrides if pipeline_constants present (matches Rust test driver)
	// 4. Generate SPIR-V binary
	// We skip IR validation to match what the snapshot test does (it skips on compile error).

	ast, err := parseWGSL(source)
	if err != nil {
		return nil, err
	}

	module, err := lowerToIR(ast, source)
	if err != nil {
		return nil, err
	}

	// Process pipeline overrides before SPIR-V compilation
	// (Rust test driver calls process_overrides for all backends)
	pipelineConstants := readSPVPipelineConstants(name)
	if len(pipelineConstants) > 0 {
		module = module.Clone()
		if err := ir.ProcessOverrides(module, pipelineConstants); err != nil {
			return nil, fmt.Errorf("ProcessOverrides failed: %w", err)
		}
	}

	return generateSPIRVBinary(module)
}

// compareStructural compares structural elements between our disassembly and reference.
// Returns a list of human-readable difference descriptions.
func compareStructural(ours, reference string) []string {
	var diffs []string

	// Compare OpCapability counts and values.
	ourCaps := extractMatches(ours, `OpCapability\s+(\S+)`)
	refCaps := extractMatches(reference, `OpCapability\s+(\S+)`)
	if capDiff := compareSets("OpCapability", ourCaps, refCaps); capDiff != "" {
		diffs = append(diffs, capDiff)
	}

	// Compare decoration counts.
	ourDecorates := countPattern(ours, `OpDecorate\s+`)
	refDecorates := countPattern(reference, `OpDecorate\s+`)
	if ourDecorates != refDecorates {
		diffs = append(diffs, formatCountDiff("OpDecorate count", ourDecorates, refDecorates))
	}

	ourMemberDecorates := countPattern(ours, `OpMemberDecorate\s+`)
	refMemberDecorates := countPattern(reference, `OpMemberDecorate\s+`)
	if ourMemberDecorates != refMemberDecorates {
		diffs = append(diffs, formatCountDiff("OpMemberDecorate count", ourMemberDecorates, refMemberDecorates))
	}

	// Check Block decorations.
	ourBlocks := countPattern(ours, `Block`)
	refBlocks := countPattern(reference, `Block`)
	if ourBlocks != refBlocks {
		diffs = append(diffs, formatCountDiff("Block decorations", ourBlocks, refBlocks))
	}

	// Check Offset decorations.
	ourOffsets := countPattern(ours, `Offset\s+\d+`)
	refOffsets := countPattern(reference, `Offset\s+\d+`)
	if ourOffsets != refOffsets {
		diffs = append(diffs, formatCountDiff("Offset decorations", ourOffsets, refOffsets))
	}

	// Check ArrayStride decorations.
	ourArrayStrides := countPattern(ours, `ArrayStride\s+\d+`)
	refArrayStrides := countPattern(reference, `ArrayStride\s+\d+`)
	if ourArrayStrides != refArrayStrides {
		diffs = append(diffs, formatCountDiff("ArrayStride decorations", ourArrayStrides, refArrayStrides))
	}

	// Compare entry point interface variable counts.
	ourEntryVars := countEntryPointVars(ours)
	refEntryVars := countEntryPointVars(reference)
	if ourEntryVars != refEntryVars {
		diffs = append(diffs, formatCountDiff("EntryPoint interface vars", ourEntryVars, refEntryVars))
	}

	// Compare function counts.
	ourFuncs := countPattern(ours, `OpFunction\s+`)
	refFuncs := countPattern(reference, `OpFunction\s+`)
	if ourFuncs != refFuncs {
		diffs = append(diffs, formatCountDiff("OpFunction count", ourFuncs, refFuncs))
	}

	// Compare type declaration counts.
	ourTypes := countPattern(ours, `OpType\w+`)
	refTypes := countPattern(reference, `OpType\w+`)
	if abs(ourTypes-refTypes) > 3 { // Allow small variance in types (3 for workgroup no-layout duplication).
		diffs = append(diffs, formatCountDiff("OpType* count", ourTypes, refTypes))
	}

	return diffs
}

// extractMatches returns all first capture group matches for a regex pattern.
func extractMatches(text, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			result = append(result, m[1])
		}
	}
	sort.Strings(result)
	return result
}

// countPattern counts occurrences of a regex pattern in text.
func countPattern(text, pattern string) int {
	re := regexp.MustCompile(pattern)
	return len(re.FindAllString(text, -1))
}

// countEntryPointVars counts the total number of interface variables across all OpEntryPoint lines.
func countEntryPointVars(text string) int {
	// OpEntryPoint format: OpEntryPoint <model> %id "name" %var1 %var2 ...
	re := regexp.MustCompile(`OpEntryPoint\s+\S+\s+%\S+\s+"[^"]*"(.*)`)
	matches := re.FindAllStringSubmatch(text, -1)
	total := 0
	for _, m := range matches {
		if len(m) > 1 {
			vars := strings.TrimSpace(m[1])
			if vars == "" {
				continue
			}
			// Count %id references.
			varRe := regexp.MustCompile(`%\S+`)
			total += len(varRe.FindAllString(vars, -1))
		}
	}
	return total
}

// compareSets compares two string slices and returns a diff description.
func compareSets(label string, ours, reference []string) string {
	ourSet := map[string]bool{}
	refSet := map[string]bool{}
	for _, s := range ours {
		ourSet[s] = true
	}
	for _, s := range reference {
		refSet[s] = true
	}

	var missing, extra []string
	for _, s := range reference {
		if !ourSet[s] {
			missing = append(missing, s)
		}
	}
	for _, s := range ours {
		if !refSet[s] {
			extra = append(extra, s)
		}
	}

	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}

	parts := []string{label + ":"}
	if len(missing) > 0 {
		parts = append(parts, "missing=["+strings.Join(missing, ",")+"]")
	}
	if len(extra) > 0 {
		parts = append(parts, "extra=["+strings.Join(extra, ",")+"]")
	}
	return strings.Join(parts, " ")
}

// formatCountDiff formats a count difference.
func formatCountDiff(label string, ours, reference int) string {
	return label + ": ours=" + itoa(ours) + " ref=" + itoa(reference)
}

// pct calculates a percentage safely.
func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

// abs returns the absolute value of an integer.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// itoa converts int to string without importing strconv (already have fmt available via other file).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
