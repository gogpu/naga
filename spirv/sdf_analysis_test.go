package spirv

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// sdfCircleWGSL is the SDF circle compute shader from gg/internal/gpu/shaders/sdf_circle.wgsl.
const sdfCircleWGSL = `
struct Params {
    center_x: f32,
    center_y: f32,
    radius_x: f32,
    radius_y: f32,
    half_stroke_width: f32,
    is_stroked: u32,
    color_r: f32,
    color_g: f32,
    color_b: f32,
    color_a: f32,
    target_width: u32,
    target_height: u32,
}

@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> pixels: array<u32>;

fn sdf_ellipse(px: f32, py: f32, a: f32, b: f32) -> f32 {
    let nx = px / a;
    let ny = py / b;
    let d = length(vec2<f32>(nx, ny)) - 1.0;
    return d * min(a, b);
}

@compute @workgroup_size(8, 8, 1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let x = gid.x;
    let y = gid.y;
    if x >= params.target_width || y >= params.target_height {
        return;
    }

    let px = f32(x) + 0.5 - params.center_x;
    let py = f32(y) + 0.5 - params.center_y;

    let dist = sdf_ellipse(px, py, params.radius_x, params.radius_y);

    var coverage: f32;
    if params.is_stroked != 0u {
        let ring_dist = abs(dist) - params.half_stroke_width;
        coverage = 1.0 - smoothstep(-0.5, 0.5, ring_dist);
    } else {
        coverage = 1.0 - smoothstep(-0.5, 0.5, dist);
    }

    if coverage < 1.0 / 255.0 {
        return;
    }

    let src_a = params.color_a * coverage;
    let src_r = params.color_r * coverage;
    let src_g = params.color_g * coverage;
    let src_b = params.color_b * coverage;

    let idx = y * params.target_width + x;
    let existing = pixels[idx];
    let dst_r = f32(existing & 0xFFu) / 255.0;
    let dst_g = f32((existing >> 8u) & 0xFFu) / 255.0;
    let dst_b = f32((existing >> 16u) & 0xFFu) / 255.0;
    let dst_a = f32((existing >> 24u) & 0xFFu) / 255.0;

    let inv_src_a = 1.0 - src_a;
    let out_r = src_r + dst_r * inv_src_a;
    let out_g = src_g + dst_g * inv_src_a;
    let out_b = src_b + dst_b * inv_src_a;
    let out_a = src_a + dst_a * inv_src_a;

    let ri = u32(clamp(out_r * 255.0 + 0.5, 0.0, 255.0));
    let gi = u32(clamp(out_g * 255.0 + 0.5, 0.0, 255.0));
    let bi = u32(clamp(out_b * 255.0 + 0.5, 0.0, 255.0));
    let ai = u32(clamp(out_a * 255.0 + 0.5, 0.0, 255.0));
    pixels[idx] = ri | (gi << 8u) | (bi << 16u) | (ai << 24u);
}
`

// TestSDFCircleSPIRV compiles the SDF circle shader and dumps a human-readable SPIR-V disassembly.
func TestSDFCircleSPIRV(t *testing.T) {
	// Parse WGSL
	lexer := wgsl.NewLexer(sdfCircleWGSL)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V with debug info for names
	opts := Options{
		Version: Version1_3,
		Debug:   true,
	}
	backend := NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled SDF circle shader: %d bytes (%d words)", len(spirvBytes), len(spirvBytes)/4)

	// Write binary .spv file
	if err := os.MkdirAll("../tmp", 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	if err := os.WriteFile("../tmp/sdf_circle.spv", spirvBytes, 0o644); err != nil {
		t.Fatalf("write spv: %v", err)
	}
	t.Logf("Wrote SPIR-V binary to tmp/sdf_circle.spv")

	// Disassemble and dump
	disasm := disassembleSPIRV(spirvBytes)
	if err := os.WriteFile("../tmp/sdf_circle_disasm.txt", []byte(disasm), 0o644); err != nil {
		t.Fatalf("write disasm: %v", err)
	}
	t.Logf("Wrote SPIR-V disassembly to tmp/sdf_circle_disasm.txt")

	// Print the full disassembly
	t.Log("\n=== SPIR-V DISASSEMBLY ===\n")
	t.Log(disasm)

	// Analyze for potential bugs
	analyzeSDFSPIRV(t, spirvBytes)
}

// disassembleSPIRV produces a human-readable text representation of SPIR-V bytecode.
func disassembleSPIRV(data []byte) string {
	if len(data) < 20 || len(data)%4 != 0 {
		return "ERROR: invalid SPIR-V binary"
	}

	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}

	var sb strings.Builder

	// Header
	sb.WriteString("; SPIR-V\n")
	fmt.Fprintf(&sb, "; Magic:     0x%08X\n", words[0])
	fmt.Fprintf(&sb, "; Version:   %d.%d\n", (words[1]>>16)&0xFF, (words[1]>>8)&0xFF)
	fmt.Fprintf(&sb, "; Generator: 0x%08X\n", words[2])
	fmt.Fprintf(&sb, "; Bound:     %d\n", words[3])
	fmt.Fprintf(&sb, "; Schema:    %d\n", words[4])
	sb.WriteString("\n")

	// Collect names for pretty printing
	names := make(map[uint32]string)
	memberNames := make(map[uint32]map[uint32]string)

	// First pass: collect names
	offset := 5
	for offset < len(words) {
		wordCount := int(words[offset] >> 16)
		opcode := OpCode(words[offset] & 0xFFFF)
		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}
		if opcode == OpName && wordCount >= 3 {
			id := words[offset+1]
			name := decodeString(words[offset+2 : offset+wordCount])
			names[id] = name
		}
		if opcode == OpMemberName && wordCount >= 4 {
			structID := words[offset+1]
			memberIdx := words[offset+2]
			name := decodeString(words[offset+3 : offset+wordCount])
			if memberNames[structID] == nil {
				memberNames[structID] = make(map[uint32]string)
			}
			memberNames[structID][memberIdx] = name
		}
		offset += wordCount
	}

	// Second pass: disassemble
	offset = 5
	for offset < len(words) {
		wordCount := int(words[offset] >> 16)
		opcode := OpCode(words[offset] & 0xFFFF)
		if wordCount == 0 || offset+wordCount > len(words) {
			fmt.Fprintf(&sb, "; ERROR: invalid instruction at word %d\n", offset)
			break
		}

		instrWords := words[offset : offset+wordCount]
		line := formatInstruction(opcode, instrWords, names, memberNames)
		fmt.Fprintf(&sb, "  %4d: %s\n", offset, line)

		offset += wordCount
	}

	return sb.String()
}

// decodeString decodes a SPIR-V string literal from words.
func decodeString(words []uint32) string {
	var bytes []byte
	for _, w := range words {
		b0 := byte(w & 0xFF)
		b1 := byte((w >> 8) & 0xFF)
		b2 := byte((w >> 16) & 0xFF)
		b3 := byte((w >> 24) & 0xFF)
		if b0 == 0 {
			break
		}
		bytes = append(bytes, b0)
		if b1 == 0 {
			break
		}
		bytes = append(bytes, b1)
		if b2 == 0 {
			break
		}
		bytes = append(bytes, b2)
		if b3 == 0 {
			break
		}
		bytes = append(bytes, b3)
	}
	return string(bytes)
}

// idStr formats an ID with its name if known.
func idStr(id uint32, names map[uint32]string) string {
	if name, ok := names[id]; ok && name != "" {
		return fmt.Sprintf("%%%d(%s)", id, name)
	}
	return fmt.Sprintf("%%%d", id)
}

// formatInstruction formats a single SPIR-V instruction for display.
func formatInstruction(opcode OpCode, words []uint32, names map[uint32]string, memberNames map[uint32]map[uint32]string) string {
	opName := opcodeName(opcode)

	switch opcode {
	case OpCapability:
		if len(words) >= 2 {
			return fmt.Sprintf("%s %s", opName, capabilityName(words[1]))
		}
	case OpExtInstImport:
		if len(words) >= 3 {
			name := decodeString(words[2:])
			return fmt.Sprintf("%s %s = %q", opName, idStr(words[1], names), name)
		}
	case OpMemoryModel:
		if len(words) >= 3 {
			return fmt.Sprintf("%s %s %s", opName, addressingModelName(words[1]), memoryModelName(words[2]))
		}
	case OpEntryPoint:
		if len(words) >= 4 {
			model := executionModelName(words[1])
			funcID := idStr(words[2], names)
			name := decodeString(words[3:])
			// Count interface vars at end
			nameWords := (len(name) + 4) / 4 // string takes ceil((len+1)/4) words
			ifaceStart := 3 + nameWords
			var ifaces []string
			for i := ifaceStart; i < len(words); i++ {
				ifaces = append(ifaces, idStr(words[i], names))
			}
			return fmt.Sprintf("%s %s %s %q %s", opName, model, funcID, name, strings.Join(ifaces, " "))
		}
	case OpExecutionMode:
		if len(words) >= 3 {
			funcID := idStr(words[1], names)
			mode := executionModeName(words[2])
			var extras []string
			for i := 3; i < len(words); i++ {
				extras = append(extras, fmt.Sprintf("%d", words[i]))
			}
			if len(extras) > 0 {
				return fmt.Sprintf("%s %s %s %s", opName, funcID, mode, strings.Join(extras, " "))
			}
			return fmt.Sprintf("%s %s %s", opName, funcID, mode)
		}
	case OpName:
		if len(words) >= 3 {
			name := decodeString(words[2:])
			return fmt.Sprintf("%s %s %q", opName, idStr(words[1], names), name)
		}
	case OpMemberName:
		if len(words) >= 4 {
			name := decodeString(words[3:])
			return fmt.Sprintf("%s %s %d %q", opName, idStr(words[1], names), words[2], name)
		}
	case OpDecorate:
		if len(words) >= 3 {
			target := idStr(words[1], names)
			dec := decorationName(words[2])
			var extras []string
			for i := 3; i < len(words); i++ {
				extras = append(extras, fmt.Sprintf("%d", words[i]))
			}
			if len(extras) > 0 {
				return fmt.Sprintf("%s %s %s %s", opName, target, dec, strings.Join(extras, " "))
			}
			return fmt.Sprintf("%s %s %s", opName, target, dec)
		}
	case OpMemberDecorate:
		if len(words) >= 4 {
			structID := idStr(words[1], names)
			memberIdx := words[2]
			dec := decorationName(words[3])
			var extras []string
			for i := 4; i < len(words); i++ {
				extras = append(extras, fmt.Sprintf("%d", words[i]))
			}
			memberStr := fmt.Sprintf("%d", memberIdx)
			if mn, ok := memberNames[words[1]]; ok {
				if name, ok2 := mn[memberIdx]; ok2 {
					memberStr = fmt.Sprintf("%d(%s)", memberIdx, name)
				}
			}
			if len(extras) > 0 {
				return fmt.Sprintf("%s %s %s %s %s", opName, structID, memberStr, dec, strings.Join(extras, " "))
			}
			return fmt.Sprintf("%s %s %s %s", opName, structID, memberStr, dec)
		}
	case OpTypeVoid:
		return fmt.Sprintf("%s %s", opName, idStr(words[1], names))
	case OpTypeBool:
		return fmt.Sprintf("%s %s", opName, idStr(words[1], names))
	case OpTypeInt:
		if len(words) >= 4 {
			sign := "unsigned"
			if words[3] == 1 {
				sign = "signed"
			}
			return fmt.Sprintf("%s %s %d %s", opName, idStr(words[1], names), words[2], sign)
		}
	case OpTypeFloat:
		if len(words) >= 3 {
			return fmt.Sprintf("%s %s %d", opName, idStr(words[1], names), words[2])
		}
	case OpTypeVector:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s %d", opName, idStr(words[1], names), idStr(words[2], names), words[3])
		}
	case OpTypeMatrix:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s %d", opName, idStr(words[1], names), idStr(words[2], names), words[3])
		}
	case OpTypeArray:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s %s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names))
		}
	case OpTypeRuntimeArray:
		if len(words) >= 3 {
			return fmt.Sprintf("%s %s %s", opName, idStr(words[1], names), idStr(words[2], names))
		}
	case OpTypeStruct:
		var members []string
		for i := 2; i < len(words); i++ {
			members = append(members, idStr(words[i], names))
		}
		return fmt.Sprintf("%s %s { %s }", opName, idStr(words[1], names), strings.Join(members, ", "))
	case OpTypePointer:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s %s", opName, idStr(words[1], names), storageClassName(words[2]), idStr(words[3], names))
		}
	case OpTypeFunction:
		var params []string
		for i := 3; i < len(words); i++ {
			params = append(params, idStr(words[i], names))
		}
		if len(params) > 0 {
			return fmt.Sprintf("%s %s %s (%s)", opName, idStr(words[1], names), idStr(words[2], names), strings.Join(params, ", "))
		}
		return fmt.Sprintf("%s %s %s", opName, idStr(words[1], names), idStr(words[2], names))
	case OpConstant:
		if len(words) >= 4 {
			typeID := words[1]
			resultID := words[2]
			val := words[3]
			return fmt.Sprintf("%s %s %s = %d (0x%08X, f32=%v)", opName, idStr(typeID, names), idStr(resultID, names), val, val, math.Float32frombits(val))
		}
	case OpConstantComposite:
		var constituents []string
		for i := 3; i < len(words); i++ {
			constituents = append(constituents, idStr(words[i], names))
		}
		return fmt.Sprintf("%s %s %s = { %s }", opName, idStr(words[1], names), idStr(words[2], names), strings.Join(constituents, ", "))
	case OpConstantNull:
		if len(words) >= 3 {
			return fmt.Sprintf("%s %s %s", opName, idStr(words[1], names), idStr(words[2], names))
		}
	case OpVariable:
		if len(words) >= 4 {
			sc := storageClassName(words[3])
			return fmt.Sprintf("%s %s %s %s", opName, idStr(words[1], names), idStr(words[2], names), sc)
		}
	case OpFunction:
		if len(words) >= 5 {
			return fmt.Sprintf("%s %s %s %s %s", opName, idStr(words[1], names), idStr(words[2], names), funcControlName(words[3]), idStr(words[4], names))
		}
	case OpFunctionParameter:
		if len(words) >= 3 {
			return fmt.Sprintf("%s %s %s", opName, idStr(words[1], names), idStr(words[2], names))
		}
	case OpFunctionEnd:
		return opName
	case OpFunctionCall:
		var args []string
		for i := 4; i < len(words); i++ {
			args = append(args, idStr(words[i], names))
		}
		return fmt.Sprintf("%s %s %s = call %s(%s)", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), strings.Join(args, ", "))
	case OpLabel:
		return fmt.Sprintf("%s %s", opName, idStr(words[1], names))
	case OpBranch:
		return fmt.Sprintf("%s %s", opName, idStr(words[1], names))
	case OpBranchConditional:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s true:%s false:%s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names))
		}
	case OpSelectionMerge:
		if len(words) >= 3 {
			return fmt.Sprintf("%s merge:%s control:%d", opName, idStr(words[1], names), words[2])
		}
	case OpLoopMerge:
		if len(words) >= 4 {
			return fmt.Sprintf("%s merge:%s continue:%s control:%d", opName, idStr(words[1], names), idStr(words[2], names), words[3])
		}
	case OpReturn:
		return opName
	case OpReturnValue:
		if len(words) >= 2 {
			return fmt.Sprintf("%s %s", opName, idStr(words[1], names))
		}
	case OpLoad:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s = load %s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names))
		}
	case OpStore:
		if len(words) >= 3 {
			return fmt.Sprintf("OpStore *%s = %s", idStr(words[1], names), idStr(words[2], names))
		}
	case OpAccessChain:
		var indices []string
		for i := 4; i < len(words); i++ {
			indices = append(indices, idStr(words[i], names))
		}
		return fmt.Sprintf("%s %s %s = base:%s [%s]", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), strings.Join(indices, ", "))
	case OpCompositeConstruct:
		var constituents []string
		for i := 3; i < len(words); i++ {
			constituents = append(constituents, idStr(words[i], names))
		}
		return fmt.Sprintf("%s %s %s = { %s }", opName, idStr(words[1], names), idStr(words[2], names), strings.Join(constituents, ", "))
	case OpCompositeExtract:
		var indices []string
		for i := 4; i < len(words); i++ {
			indices = append(indices, fmt.Sprintf("%d", words[i]))
		}
		return fmt.Sprintf("%s %s %s = %s . [%s]", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), strings.Join(indices, ", "))
	case OpVectorExtractDynamic:
		if len(words) >= 5 {
			return fmt.Sprintf("%s %s %s = %s [%s]", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), idStr(words[4], names))
		}
	case OpExtInst:
		if len(words) >= 5 {
			extSet := idStr(words[3], names)
			extOpcode := words[4]
			var operands []string
			for i := 5; i < len(words); i++ {
				operands = append(operands, idStr(words[i], names))
			}
			return fmt.Sprintf("%s %s %s = %s op:%d(%s) (%s)", opName, idStr(words[1], names), idStr(words[2], names), extSet, extOpcode, glslOpName(extOpcode), strings.Join(operands, ", "))
		}
	case OpConvertFToU, OpConvertFToS, OpConvertUToF, OpConvertSToF, OpBitcast:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s = %s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names))
		}
	case OpFNegate, OpSNegate, OpNot:
		if len(words) >= 4 {
			return fmt.Sprintf("%s %s %s = %s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names))
		}
	case OpFAdd, OpFSub, OpFMul, OpFDiv, OpFMod,
		OpIAdd, OpISub, OpIMul, OpSDiv, OpUDiv, OpSMod, OpUMod,
		OpShiftLeftLogical, OpShiftRightLogical, OpShiftRightArithmetic,
		OpBitwiseOr, OpBitwiseAnd, OpBitwiseXor,
		OpLogicalOr, OpLogicalAnd,
		OpFOrdEqual, OpFOrdNotEqual, OpFOrdLessThan, OpFOrdGreaterThan,
		OpFOrdLessThanEqual, OpFOrdGreaterThanEqual,
		OpIEqual, OpINotEqual,
		OpULessThan, OpULessThanEqual, OpUGreaterThan, OpUGreaterThanEqual,
		OpSLessThan, OpSLessThanEqual, OpSGreaterThan, OpSGreaterThanEqual:
		if len(words) >= 5 {
			return fmt.Sprintf("%s %s %s = %s, %s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), idStr(words[4], names))
		}
	case OpSelect:
		if len(words) >= 6 {
			return fmt.Sprintf("%s %s %s = cond:%s true:%s false:%s", opName, idStr(words[1], names), idStr(words[2], names), idStr(words[3], names), idStr(words[4], names), idStr(words[5], names))
		}
	}

	// Fallback: raw word dump
	var rawWords []string
	for i := 1; i < len(words); i++ {
		rawWords = append(rawWords, fmt.Sprintf("0x%08X", words[i]))
	}
	return fmt.Sprintf("%s [%s]", opName, strings.Join(rawWords, " "))
}

// analyzeSDFSPIRV searches the SPIR-V for specific patterns relevant to the storage buffer write bug.
func analyzeSDFSPIRV(t *testing.T, data []byte) {
	t.Helper()

	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}

	// Collect names
	names := make(map[uint32]string)
	offset := 5
	for offset < len(words) {
		wc := int(words[offset] >> 16)
		op := OpCode(words[offset] & 0xFFFF)
		if wc == 0 || offset+wc > len(words) {
			break
		}
		if op == OpName && wc >= 3 {
			names[words[offset+1]] = decodeString(words[offset+2 : offset+wc])
		}
		offset += wc
	}

	t.Log("\n=== BUG ANALYSIS: Storage Buffer Write ===\n")

	// Track types to understand pointer chains
	typeInfo := make(map[uint32]string) // id -> description

	// Track all OpStore instructions
	var stores []struct {
		offset    int
		pointerID uint32
		valueID   uint32
	}

	// Track all OpAccessChain instructions
	var accessChains []struct {
		offset   int
		resultID uint32
		baseID   uint32
		indices  []uint32
	}

	// Track BitwiseOr/ShiftLeft for packing
	var bitwiseOrs []struct {
		offset   int
		resultID uint32
		op1, op2 uint32
	}
	var shiftLefts []struct {
		offset   int
		resultID uint32
		base     uint32
		shift    uint32
	}
	var convertFToUs []struct {
		offset   int
		resultID uint32
		operand  uint32
	}

	// Track storage class for variables
	storageVars := make(map[uint32]uint32) // varID -> storageClass

	offset = 5
	for offset < len(words) {
		wc := int(words[offset] >> 16)
		op := OpCode(words[offset] & 0xFFFF)
		if wc == 0 || offset+wc > len(words) {
			break
		}

		switch op {
		case OpTypeVoid:
			typeInfo[words[offset+1]] = "void"
		case OpTypeBool:
			typeInfo[words[offset+1]] = "bool"
		case OpTypeInt:
			sign := "u"
			if words[offset+3] == 1 {
				sign = "i"
			}
			typeInfo[words[offset+1]] = fmt.Sprintf("%s%d", sign, words[offset+2])
		case OpTypeFloat:
			typeInfo[words[offset+1]] = fmt.Sprintf("f%d", words[offset+2])
		case OpTypeVector:
			typeInfo[words[offset+1]] = fmt.Sprintf("vec%d<%s>", words[offset+3], typeInfo[words[offset+2]])
		case OpTypeRuntimeArray:
			typeInfo[words[offset+1]] = fmt.Sprintf("RuntimeArray<%s>", typeInfo[words[offset+2]])
		case OpTypeStruct:
			var members []string
			for i := 2; i < wc; i++ {
				members = append(members, typeInfo[words[offset+i]])
			}
			name := names[words[offset+1]]
			if name == "" {
				name = fmt.Sprintf("struct_%d", words[offset+1])
			}
			typeInfo[words[offset+1]] = fmt.Sprintf("%s{%s}", name, strings.Join(members, ", "))
		case OpTypePointer:
			sc := storageClassName(words[offset+2])
			typeInfo[words[offset+1]] = fmt.Sprintf("ptr<%s, %s>", sc, typeInfo[words[offset+3]])
		case OpTypeFunction:
			typeInfo[words[offset+1]] = "functype"

		case OpVariable:
			if wc >= 4 {
				storageVars[words[offset+2]] = words[offset+3]
			}

		case OpStore:
			stores = append(stores, struct {
				offset    int
				pointerID uint32
				valueID   uint32
			}{offset, words[offset+1], words[offset+2]})

		case OpAccessChain:
			var indices []uint32
			for i := 4; i < wc; i++ {
				indices = append(indices, words[offset+i])
			}
			accessChains = append(accessChains, struct {
				offset   int
				resultID uint32
				baseID   uint32
				indices  []uint32
			}{offset, words[offset+2], words[offset+3], indices})

		case OpBitwiseOr:
			if wc >= 5 {
				bitwiseOrs = append(bitwiseOrs, struct {
					offset   int
					resultID uint32
					op1, op2 uint32
				}{offset, words[offset+2], words[offset+3], words[offset+4]})
			}

		case OpShiftLeftLogical:
			if wc >= 5 {
				shiftLefts = append(shiftLefts, struct {
					offset   int
					resultID uint32
					base     uint32
					shift    uint32
				}{offset, words[offset+2], words[offset+3], words[offset+4]})
			}

		case OpConvertFToU:
			if wc >= 4 {
				convertFToUs = append(convertFToUs, struct {
					offset   int
					resultID uint32
					operand  uint32
				}{offset, words[offset+2], words[offset+3]})
			}
		}

		offset += wc
	}

	// Report findings
	t.Logf("Storage variables (StorageBuffer class=%d):", StorageClassStorageBuffer)
	for varID, sc := range storageVars {
		if sc == uint32(StorageClassStorageBuffer) {
			t.Logf("  %s (storageClass=StorageBuffer)", idStr(varID, names))
		}
	}

	t.Logf("\nAccessChain instructions (%d total):", len(accessChains))
	for _, ac := range accessChains {
		indexStrs := make([]string, len(ac.indices))
		for i, idx := range ac.indices {
			indexStrs[i] = idStr(idx, names)
		}
		t.Logf("  word %d: %s = base:%s [%s]", ac.offset, idStr(ac.resultID, names), idStr(ac.baseID, names), strings.Join(indexStrs, ", "))
	}

	t.Logf("\nOpStore instructions (%d total):", len(stores))
	for _, s := range stores {
		t.Logf("  word %d: *%s = %s", s.offset, idStr(s.pointerID, names), idStr(s.valueID, names))
	}

	t.Logf("\nConvertFToU instructions (%d total):", len(convertFToUs))
	for _, c := range convertFToUs {
		t.Logf("  word %d: %s = ConvertFToU %s", c.offset, idStr(c.resultID, names), idStr(c.operand, names))
	}

	t.Logf("\nShiftLeftLogical instructions (%d total):", len(shiftLefts))
	for _, s := range shiftLefts {
		t.Logf("  word %d: %s = %s << %s", s.offset, idStr(s.resultID, names), idStr(s.base, names), idStr(s.shift, names))
	}

	t.Logf("\nBitwiseOr instructions (%d total):", len(bitwiseOrs))
	for _, b := range bitwiseOrs {
		t.Logf("  word %d: %s = %s | %s", b.offset, idStr(b.resultID, names), idStr(b.op1, names), idStr(b.op2, names))
	}

	// KEY ANALYSIS: Check the final store to pixels[idx]
	// The last OpStore in the function body should be the store to pixels[idx]
	// Its value should trace back through BitwiseOr chain
	if len(stores) > 0 {
		lastStore := stores[len(stores)-1]
		t.Logf("\n=== FINAL STORE (pixels[idx] = packed_color) ===")
		t.Logf("  OpStore at word %d: pointer=%s value=%s", lastStore.offset, idStr(lastStore.pointerID, names), idStr(lastStore.valueID, names))

		// Check if pointer came from an AccessChain into storage buffer
		for _, ac := range accessChains {
			if ac.resultID == lastStore.pointerID {
				t.Logf("  Pointer from AccessChain: base=%s indices=%v", idStr(ac.baseID, names), ac.indices)
				// Check if base is a storage buffer variable
				if sc, ok := storageVars[ac.baseID]; ok {
					t.Logf("  Base variable storage class: %d (%s)", sc, storageClassName(sc))
				}
				// Check number of indices - for wrapped storage buffer, need 2 levels
				if len(ac.indices) == 1 {
					t.Logf("  WARNING: Only 1 index in AccessChain - if storage buffer is wrapped in struct, need 2 indices!")
				}
				break
			}
		}

		// Trace the value back through BitwiseOr chain
		t.Logf("\n  Tracing value %s:", idStr(lastStore.valueID, names))
		traceValueChain(t, lastStore.valueID, bitwiseOrs, shiftLefts, convertFToUs, names)
	}

	// Additional check: verify AccessChain for wrapped storage buffers
	t.Log("\n=== ACCESS CHAIN ANALYSIS FOR STORAGE BUFFER ===")
	for _, ac := range accessChains {
		if sc, ok := storageVars[ac.baseID]; ok && sc == uint32(StorageClassStorageBuffer) {
			indexStrs := make([]string, len(ac.indices))
			for i, idx := range ac.indices {
				indexStrs[i] = idStr(idx, names)
			}
			t.Logf("  StorageBuffer AccessChain at word %d: result=%s base=%s indices=[%s]",
				ac.offset, idStr(ac.resultID, names), idStr(ac.baseID, names), strings.Join(indexStrs, ", "))
		}
	}
}

func traceValueChain(t *testing.T, valueID uint32,
	bitwiseOrs []struct {
		offset   int
		resultID uint32
		op1, op2 uint32
	},
	shiftLefts []struct {
		offset   int
		resultID uint32
		base     uint32
		shift    uint32
	},
	convertFToUs []struct {
		offset   int
		resultID uint32
		operand  uint32
	},
	names map[uint32]string) {
	// Check if it's a BitwiseOr result
	for _, bor := range bitwiseOrs {
		if bor.resultID == valueID {
			t.Logf("    %s = BitwiseOr(%s, %s)", idStr(bor.resultID, names), idStr(bor.op1, names), idStr(bor.op2, names))
			traceValueChain(t, bor.op1, bitwiseOrs, shiftLefts, convertFToUs, names)
			traceValueChain(t, bor.op2, bitwiseOrs, shiftLefts, convertFToUs, names)
			return
		}
	}

	// Check if it's a ShiftLeft result
	for _, shl := range shiftLefts {
		if shl.resultID == valueID {
			t.Logf("    %s = ShiftLeft(%s, %s)", idStr(shl.resultID, names), idStr(shl.base, names), idStr(shl.shift, names))
			return
		}
	}

	// Check if it's a ConvertFToU result
	for _, cfu := range convertFToUs {
		if cfu.resultID == valueID {
			t.Logf("    %s = ConvertFToU(%s)", idStr(cfu.resultID, names), idStr(cfu.operand, names))
			return
		}
	}

	t.Logf("    %s = (other expression, not in tracked set)", idStr(valueID, names))
}

// Helper name functions for SPIR-V constants

func opcodeName(op OpCode) string {
	switch op {
	case OpNop:
		return "OpNop"
	case OpSource:
		return "OpSource"
	case OpString:
		return "OpString"
	case OpName:
		return "OpName"
	case OpMemberName:
		return "OpMemberName"
	case OpExtInstImport:
		return "OpExtInstImport"
	case OpExtInst:
		return "OpExtInst"
	case OpMemoryModel:
		return "OpMemoryModel"
	case OpEntryPoint:
		return "OpEntryPoint"
	case OpExecutionMode:
		return "OpExecutionMode"
	case OpCapability:
		return "OpCapability"
	case OpTypeVoid:
		return "OpTypeVoid"
	case OpTypeBool:
		return "OpTypeBool"
	case OpTypeInt:
		return "OpTypeInt"
	case OpTypeFloat:
		return "OpTypeFloat"
	case OpTypeVector:
		return "OpTypeVector"
	case OpTypeMatrix:
		return "OpTypeMatrix"
	case OpTypeArray:
		return "OpTypeArray"
	case OpTypeRuntimeArray:
		return "OpTypeRuntimeArray"
	case OpTypeStruct:
		return "OpTypeStruct"
	case OpTypePointer:
		return "OpTypePointer"
	case OpTypeFunction:
		return "OpTypeFunction"
	case OpConstant:
		return "OpConstant"
	case OpConstantComposite:
		return "OpConstantComposite"
	case OpConstantNull:
		return "OpConstantNull"
	case OpFunction:
		return "OpFunction"
	case OpFunctionParameter:
		return "OpFunctionParameter"
	case OpFunctionEnd:
		return "OpFunctionEnd"
	case OpFunctionCall:
		return "OpFunctionCall"
	case OpVariable:
		return "OpVariable"
	case OpLoad:
		return "OpLoad"
	case OpStore:
		return "OpStore"
	case OpAccessChain:
		return "OpAccessChain"
	case OpDecorate:
		return "OpDecorate"
	case OpMemberDecorate:
		return "OpMemberDecorate"
	case OpLabel:
		return "OpLabel"
	case OpBranch:
		return "OpBranch"
	case OpBranchConditional:
		return "OpBranchConditional"
	case OpSelectionMerge:
		return "OpSelectionMerge"
	case OpLoopMerge:
		return "OpLoopMerge"
	case OpReturn:
		return "OpReturn"
	case OpReturnValue:
		return "OpReturnValue"
	case OpUnreachable:
		return "OpUnreachable"
	case OpKill:
		return "OpKill"
	case OpCompositeConstruct:
		return "OpCompositeConstruct"
	case OpCompositeExtract:
		return "OpCompositeExtract"
	case OpVectorExtractDynamic:
		return "OpVectorExtractDynamic"
	case OpVectorShuffle:
		return "OpVectorShuffle"
	case OpFNegate:
		return "OpFNegate"
	case OpSNegate:
		return "OpSNegate"
	case OpFAdd:
		return "OpFAdd"
	case OpFSub:
		return "OpFSub"
	case OpFMul:
		return "OpFMul"
	case OpFDiv:
		return "OpFDiv"
	case OpFMod:
		return "OpFMod"
	case OpIAdd:
		return "OpIAdd"
	case OpISub:
		return "OpISub"
	case OpIMul:
		return "OpIMul"
	case OpSDiv:
		return "OpSDiv"
	case OpUDiv:
		return "OpUDiv"
	case OpSMod:
		return "OpSMod"
	case OpUMod:
		return "OpUMod"
	case OpConvertFToU:
		return "OpConvertFToU"
	case OpConvertFToS:
		return "OpConvertFToS"
	case OpConvertUToF:
		return "OpConvertUToF"
	case OpConvertSToF:
		return "OpConvertSToF"
	case OpBitcast:
		return "OpBitcast"
	case OpShiftLeftLogical:
		return "OpShiftLeftLogical"
	case OpShiftRightLogical:
		return "OpShiftRightLogical"
	case OpShiftRightArithmetic:
		return "OpShiftRightArithmetic"
	case OpBitwiseOr:
		return "OpBitwiseOr"
	case OpBitwiseAnd:
		return "OpBitwiseAnd"
	case OpBitwiseXor:
		return "OpBitwiseXor"
	case OpLogicalOr:
		return "OpLogicalOr"
	case OpLogicalAnd:
		return "OpLogicalAnd"
	case OpLogicalNot:
		return "OpLogicalNot"
	case OpNot:
		return "OpNot"
	case OpSelect:
		return "OpSelect"
	case OpFOrdEqual:
		return "OpFOrdEqual"
	case OpFOrdNotEqual:
		return "OpFOrdNotEqual"
	case OpFOrdLessThan:
		return "OpFOrdLessThan"
	case OpFOrdGreaterThan:
		return "OpFOrdGreaterThan"
	case OpFOrdLessThanEqual:
		return "OpFOrdLessThanEqual"
	case OpFOrdGreaterThanEqual:
		return "OpFOrdGreaterThanEqual"
	case OpIEqual:
		return "OpIEqual"
	case OpINotEqual:
		return "OpINotEqual"
	case OpULessThan:
		return "OpULessThan"
	case OpULessThanEqual:
		return "OpULessThanEqual"
	case OpUGreaterThan:
		return "OpUGreaterThan"
	case OpUGreaterThanEqual:
		return "OpUGreaterThanEqual"
	case OpSLessThan:
		return "OpSLessThan"
	case OpSLessThanEqual:
		return "OpSLessThanEqual"
	case OpSGreaterThan:
		return "OpSGreaterThan"
	case OpSGreaterThanEqual:
		return "OpSGreaterThanEqual"
	case OpSwitch:
		return "OpSwitch"
	case OpAtomicLoad:
		return "OpAtomicLoad"
	case OpAtomicStore:
		return "OpAtomicStore"
	case OpAtomicIAdd:
		return "OpAtomicIAdd"
	case OpAtomicCompareExch:
		return "OpAtomicCompareExch"
	default:
		return fmt.Sprintf("Op(%d)", op)
	}
}

func capabilityName(c uint32) string {
	switch Capability(c) {
	case CapabilityShader:
		return "Shader"
	case CapabilityFloat16:
		return "Float16"
	case CapabilityFloat64:
		return "Float64"
	case CapabilityInt64:
		return "Int64"
	default:
		return fmt.Sprintf("Cap(%d)", c)
	}
}

func addressingModelName(m uint32) string {
	switch AddressingModel(m) {
	case AddressingModelLogical:
		return "Logical"
	case AddressingModelPhysical32:
		return "Physical32"
	case AddressingModelPhysical64:
		return "Physical64"
	default:
		return fmt.Sprintf("Addressing(%d)", m)
	}
}

func memoryModelName(m uint32) string {
	switch MemoryModel(m) {
	case MemoryModelSimple:
		return "Simple"
	case MemoryModelGLSL450:
		return "GLSL450"
	case MemoryModelVulkan:
		return "Vulkan"
	default:
		return fmt.Sprintf("Memory(%d)", m)
	}
}

func executionModelName(m uint32) string {
	switch ExecutionModel(m) {
	case ExecutionModelVertex:
		return "Vertex"
	case ExecutionModelFragment:
		return "Fragment"
	case ExecutionModelGLCompute:
		return "GLCompute"
	default:
		return fmt.Sprintf("Model(%d)", m)
	}
}

func executionModeName(m uint32) string {
	switch ExecutionMode(m) {
	case ExecutionModeOriginUpperLeft:
		return "OriginUpperLeft"
	case ExecutionModeLocalSize:
		return "LocalSize"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
}

func storageClassName(sc uint32) string {
	switch StorageClass(sc) {
	case StorageClassUniformConstant:
		return "UniformConstant"
	case StorageClassInput:
		return "Input"
	case StorageClassUniform:
		return "Uniform"
	case StorageClassOutput:
		return "Output"
	case StorageClassWorkgroup:
		return "Workgroup"
	case StorageClassPrivate:
		return "Private"
	case StorageClassFunction:
		return "Function"
	case StorageClassPushConstant:
		return "PushConstant"
	case StorageClassStorageBuffer:
		return "StorageBuffer"
	default:
		return fmt.Sprintf("SC(%d)", sc)
	}
}

func decorationName(dec uint32) string {
	switch Decoration(dec) {
	case DecorationBlock:
		return "Block"
	case DecorationArrayStride:
		return "ArrayStride"
	case DecorationMatrixStride:
		return "MatrixStride"
	case DecorationBuiltIn:
		return "BuiltIn"
	case DecorationLocation:
		return "Location"
	case DecorationBinding:
		return "Binding"
	case DecorationDescriptorSet:
		return "DescriptorSet"
	case DecorationOffset:
		return "Offset"
	case DecorationColMajor:
		return "ColMajor"
	case DecorationRowMajor:
		return "RowMajor"
	default:
		return fmt.Sprintf("Dec(%d)", dec)
	}
}

func funcControlName(fc uint32) string {
	if fc == 0 {
		return "None"
	}
	return fmt.Sprintf("FuncCtrl(%d)", fc)
}

func glslOpName(op uint32) string {
	// GLSL.std.450 extended instruction opcodes
	switch op {
	case 1:
		return "Round"
	case 4:
		return "FAbs"
	case 5:
		return "SAbs"
	case 8:
		return "Floor"
	case 9:
		return "Ceil"
	case 10:
		return "Fract"
	case 13:
		return "Sin"
	case 14:
		return "Cos"
	case 15:
		return "Tan"
	case 28:
		return "Pow"
	case 31:
		return "Sqrt"
	case 32:
		return "InverseSqrt"
	case 37:
		return "FMin"
	case 38:
		return "UMin"
	case 39:
		return "SMin"
	case 40:
		return "FMax"
	case 41:
		return "UMax"
	case 42:
		return "SMax"
	case 43:
		return "FClamp"
	case 44:
		return "UClamp"
	case 45:
		return "SClamp"
	case 46:
		return "FMix"
	case 48:
		return "Step"
	case 49:
		return "SmoothStep"
	case 66:
		return "Length"
	case 68:
		return "Normalize"
	case 69:
		return "FaceForward"
	case 70:
		return "Reflect"
	case 71:
		return "Cross"
	default:
		return fmt.Sprintf("GLSL(%d)", op)
	}
}
