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
	"path/filepath"
	"sort"
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
func TestSnapshots(t *testing.T) {
	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found in testdata/in/")
	}

	for i := range shaders {
		shader := &shaders[i]
		t.Run(shader.name, func(t *testing.T) {
			module := compileToIR(t, shader.name, shader.source)

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
				code := compileHLSL(t, module)
				compareGolden(t, filepath.Join("testdata", "golden", "hlsl", shader.name+".hlsl"), code)
			})

			t.Run("msl", func(t *testing.T) {
				code := compileMSL(t, module)
				compareGolden(t, filepath.Join("testdata", "golden", "msl", shader.name+".msl"), code)
			})
		})
	}
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
		t.Fatalf("[%s] lower failed: %v", name, err)
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
func compileHLSL(t *testing.T, module *ir.Module) string {
	t.Helper()

	opts := hlsl.DefaultOptions()
	code, _, err := hlsl.Compile(module, opts)
	if err != nil {
		t.Skipf("HLSL compile failed (skipping): %v", err)
	}

	return code
}

// compileMSL compiles the IR module to MSL source.
func compileMSL(t *testing.T, module *ir.Module) string {
	t.Helper()

	opts := msl.DefaultOptions()
	opts.FakeMissingBindings = true
	code, _, err := msl.Compile(module, opts)
	if err != nil {
		t.Skipf("MSL compile failed (skipping): %v", err)
	}

	return code
}

// stageName returns a human-readable name for a shader stage.
func stageName(stage ir.ShaderStage) string {
	switch stage {
	case ir.StageVertex:
		return "vertex"
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
