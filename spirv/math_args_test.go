package spirv

import (
	"math"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// TestMathBuiltinArgumentOrder verifies that GLSL.std.450 extended instructions
// receive operands in the correct order. WGSL and GLSL.std.450 use matching
// argument orders for all builtin math functions.
//
// WGSL signatures vs GLSL.std.450 expected order:
//
//	smoothstep(low, high, x) -> SmoothStep(edge0, edge1, x) - MATCH
//	clamp(e, low, high)      -> FClamp(x, minVal, maxVal)   - MATCH
//	step(edge, x)            -> Step(edge, x)                - MATCH
//	mix(e1, e2, e3)          -> FMix(x, y, a)                - MATCH
//	fma(e1, e2, e3)          -> Fma(a, b, c)                 - MATCH
func TestMathBuiltinArgumentOrder(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		glslInst  uint32    // Expected GLSL.std.450 instruction number
		numArgs   int       // Expected number of operands
		argValues []float32 // Expected operand values in order
	}{
		{
			name: "smoothstep(low=0.2, high=0.8, x=0.5)",
			source: `
@fragment
fn main(@location(0) dummy: f32) -> @location(0) vec4<f32> {
    var low: f32 = 0.2;
    var high: f32 = 0.8;
    var x: f32 = 0.5;
    var result: f32 = smoothstep(low, high, x);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst:  GLSLstd450SmoothStep,
			numArgs:   3,
			argValues: []float32{0.2, 0.8, 0.5},
		},
		{
			name: "clamp(e=0.3, low=0.1, high=0.9)",
			source: `
@fragment
fn main(@location(0) dummy: f32) -> @location(0) vec4<f32> {
    var e: f32 = 0.3;
    var low: f32 = 0.1;
    var high: f32 = 0.9;
    var result: f32 = clamp(e, low, high);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst:  GLSLstd450FClamp,
			numArgs:   3,
			argValues: []float32{0.3, 0.1, 0.9},
		},
		{
			name: "step(edge=0.4, x=0.6)",
			source: `
@fragment
fn main(@location(0) dummy: f32) -> @location(0) vec4<f32> {
    var edge_val: f32 = 0.4;
    var x: f32 = 0.6;
    var result: f32 = step(edge_val, x);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst:  GLSLstd450Step,
			numArgs:   2,
			argValues: []float32{0.4, 0.6},
		},
		{
			name: "mix(e1=0.2, e2=0.7, e3=0.5)",
			source: `
@fragment
fn main(@location(0) dummy: f32) -> @location(0) vec4<f32> {
    var e1: f32 = 0.2;
    var e2: f32 = 0.7;
    var e3: f32 = 0.5;
    var result: f32 = mix(e1, e2, e3);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst:  GLSLstd450FMix,
			numArgs:   3,
			argValues: []float32{0.2, 0.7, 0.5},
		},
		{
			name: "fma(a=0.3, b=0.4, c=0.5)",
			source: `
@fragment
fn main(@location(0) dummy: f32) -> @location(0) vec4<f32> {
    var a: f32 = 0.3;
    var b: f32 = 0.4;
    var c: f32 = 0.5;
    var result: f32 = fma(a, b, c);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst:  GLSLstd450Fma,
			numArgs:   3,
			argValues: []float32{0.3, 0.4, 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spirvBytes := compileSPIRV(t, tt.source)
			validateSPIRVBinary(t, spirvBytes)

			// Parse SPIR-V to find the ExtInst and verify operand order
			info := findExtInst(t, spirvBytes, tt.glslInst)
			if info == nil {
				t.Fatalf("ExtInst with GLSL instruction %d not found in SPIR-V binary", tt.glslInst)
			}

			if len(info.operandIDs) != tt.numArgs {
				t.Fatalf("Expected %d operands, got %d", tt.numArgs, len(info.operandIDs))
			}

			// Build a map of SPIR-V ID -> float32 constant value
			constants := parseFloat32Constants(spirvBytes)

			// Build a map of SPIR-V ID -> the constant value it loads from
			// (OpLoad of an OpVariable that was OpStore'd with a constant)
			stores := parseStoreLoadChain(spirvBytes, constants)

			// Verify each operand matches the expected value
			for i, expectedVal := range tt.argValues {
				operandID := info.operandIDs[i]
				actualVal, found := resolveOperandValue(operandID, constants, stores)
				if !found {
					t.Logf("Operand %d (ID=%d): could not resolve to constant (may be runtime expression)", i, operandID)
					continue
				}
				if !floatEqual(actualVal, expectedVal) {
					t.Errorf("Operand %d: expected value %v, got %v (ID=%d) - ARGUMENT ORDER IS WRONG",
						i, expectedVal, actualVal, operandID)
				} else {
					t.Logf("Operand %d: value %v matches expected %v (ID=%d)", i, actualVal, expectedVal, operandID)
				}
			}
		})
	}
}

// TestMathBuiltinCompilation verifies that all five functions compile without error.
func TestMathBuiltinCompilation(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "smoothstep",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = smoothstep(0.0, 1.0, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
		},
		{
			name: "clamp",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = clamp(v, 0.0, 1.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
		},
		{
			name: "step",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = step(0.5, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
		},
		{
			name: "mix",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = mix(0.0, 1.0, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
		},
		{
			name: "fma",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = fma(v, 2.0, 1.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spirvBytes := compileSPIRV(t, tt.source)
			validateSPIRVBinary(t, spirvBytes)
			t.Logf("Successfully compiled %s: %d bytes", tt.name, len(spirvBytes))
		})
	}
}

// extInstInfo holds parsed information about an OpExtInst instruction.
type extInstInfo struct {
	resultType  uint32
	resultID    uint32
	extSetID    uint32
	instruction uint32
	operandIDs  []uint32
}

// compileSPIRV is a helper that parses WGSL, lowers to IR, and compiles to SPIR-V.
func compileSPIRV(t *testing.T, source string) []byte {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	return spirvBytes
}

// parseSPIRVWords converts a byte slice to uint32 words (little-endian).
func parseSPIRVWords(spirvBytes []byte) []uint32 {
	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}
	return words
}

// findExtInst searches the SPIR-V binary for an OpExtInst with the given GLSL instruction.
func findExtInst(t *testing.T, spirvBytes []byte, targetGLSLInst uint32) *extInstInfo {
	t.Helper()
	words := parseSPIRVWords(spirvBytes)

	offset := 5 // Skip header
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		// OpExtInst format:
		// word 0: wordCount | OpExtInst
		// word 1: ResultType
		// word 2: ResultID
		// word 3: ExtSet (GLSL.std.450 import ID)
		// word 4: Instruction number
		// word 5+: Operands
		if opcode == OpExtInst && wordCount >= 5 {
			instruction := words[offset+4]
			if instruction == targetGLSLInst {
				info := &extInstInfo{
					resultType:  words[offset+1],
					resultID:    words[offset+2],
					extSetID:    words[offset+3],
					instruction: instruction,
				}
				for i := 5; i < wordCount; i++ {
					info.operandIDs = append(info.operandIDs, words[offset+i])
				}
				return info
			}
		}

		offset += wordCount
	}

	return nil
}

// parseFloat32Constants extracts OpConstant float32 values from the SPIR-V binary.
// Returns a map from SPIR-V ID -> float32 value.
func parseFloat32Constants(spirvBytes []byte) map[uint32]float32 {
	words := parseSPIRVWords(spirvBytes)
	constants := make(map[uint32]float32)

	// First pass: find float32 type IDs (OpTypeFloat with width 32)
	floatTypeIDs := make(map[uint32]bool)
	offset := 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		// OpTypeFloat: wordCount=3, resultID, width
		if opcode == OpTypeFloat && wordCount == 3 {
			resultID := words[offset+1]
			width := words[offset+2]
			if width == 32 {
				floatTypeIDs[resultID] = true
			}
		}

		offset += wordCount
	}

	// Second pass: find OpConstant for float32 types
	offset = 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		// OpConstant: wordCount=4 for f32, resultType, resultID, value
		if opcode == OpConstant && wordCount == 4 {
			resultType := words[offset+1]
			resultID := words[offset+2]
			if floatTypeIDs[resultType] {
				bits := words[offset+3]
				constants[resultID] = math.Float32frombits(bits)
			}
		}

		offset += wordCount
	}

	return constants
}

// parseStoreLoadChain traces OpStore/OpLoad chains to resolve which constant
// value an ID ultimately holds. Returns a map from loaded-value ID -> float32 value.
func parseStoreLoadChain(spirvBytes []byte, constants map[uint32]float32) map[uint32]float32 {
	words := parseSPIRVWords(spirvBytes)
	resolved := make(map[uint32]float32)

	// Track: variable ID -> last stored constant value
	varToConstant := make(map[uint32]float32)
	varHasConstant := make(map[uint32]bool)

	// First pass: find OpStore(ptr, value) where value is a known constant
	offset := 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		// OpStore: wordCount=3, pointer, object
		if opcode == OpStore && wordCount >= 3 {
			ptrID := words[offset+1]
			objID := words[offset+2]
			if val, ok := constants[objID]; ok {
				varToConstant[ptrID] = val
				varHasConstant[ptrID] = true
			}
		}

		offset += wordCount
	}

	// Second pass: find OpLoad(resultType, resultID, pointer) where pointer holds a constant
	offset = 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		// OpLoad: wordCount=4, resultType, resultID, pointer
		if opcode == OpLoad && wordCount >= 4 {
			resultID := words[offset+2]
			ptrID := words[offset+3]
			if varHasConstant[ptrID] {
				resolved[resultID] = varToConstant[ptrID]
			}
		}

		offset += wordCount
	}

	return resolved
}

// resolveOperandValue attempts to resolve a SPIR-V ID to its float32 value.
// Checks direct constants first, then store/load chains.
func resolveOperandValue(id uint32, constants map[uint32]float32, stores map[uint32]float32) (float32, bool) {
	if val, ok := constants[id]; ok {
		return val, true
	}
	if val, ok := stores[id]; ok {
		return val, true
	}
	return 0, false
}

// floatEqual compares two float32 values with tolerance.
func floatEqual(a, b float32) bool {
	const epsilon = 1e-6
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// TestMathBuiltinExtInstPresence verifies that the expected GLSL.std.450 instruction
// numbers are emitted for each math builtin function.
func TestMathBuiltinExtInstPresence(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		glslInst uint32
		instName string
	}{
		{
			name: "smoothstep emits SmoothStep(49)",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = smoothstep(0.0, 1.0, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst: GLSLstd450SmoothStep,
			instName: "GLSLstd450SmoothStep",
		},
		{
			name: "clamp emits FClamp(43)",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = clamp(v, 0.0, 1.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst: GLSLstd450FClamp,
			instName: "GLSLstd450FClamp",
		},
		{
			name: "step emits Step(48)",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = step(0.5, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst: GLSLstd450Step,
			instName: "GLSLstd450Step",
		},
		{
			name: "mix emits FMix(46)",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = mix(0.0, 1.0, v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst: GLSLstd450FMix,
			instName: "GLSLstd450FMix",
		},
		{
			name: "fma emits Fma(50)",
			source: `
@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    var result: f32 = fma(v, 2.0, 1.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}`,
			glslInst: GLSLstd450Fma,
			instName: "GLSLstd450Fma",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spirvBytes := compileSPIRV(t, tt.source)
			info := findExtInst(t, spirvBytes, tt.glslInst)
			if info == nil {
				t.Fatalf("Expected %s (opcode %d) in SPIR-V output, but not found", tt.instName, tt.glslInst)
			}
			t.Logf("Found %s with %d operands", tt.instName, len(info.operandIDs))
		})
	}
}
