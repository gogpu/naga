package codegen

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helpers for div/mod wrapper tests
// =============================================================================

// countOpFunctions counts the number of OpFunction instructions in SPIR-V binary.
func countOpFunctions(spvBytes []byte) int {
	count := 0
	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)
		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}
		if opcode == uint32(OpFunction) {
			count++
		}
		offset += wordCount * 4
	}
	return count
}

// findOpFunctionCalls finds all OpFunctionCall instructions and returns
// the called function IDs.
func findOpFunctionCalls(spvBytes []byte) []uint32 {
	var calls []uint32
	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)
		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}
		if opcode == uint32(OpFunctionCall) && wordCount >= 4 {
			// OpFunctionCall: result_type, result_id, function_id, args...
			funcID := binary.LittleEndian.Uint32(spvBytes[offset+12:])
			calls = append(calls, funcID)
		}
		offset += wordCount * 4
	}
	return calls
}

// divmodHasOpcode checks if a specific opcode exists in the SPIR-V binary.
func divmodHasOpcode(spvBytes []byte, op OpCode) bool {
	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)
		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}
		if opcode == uint32(op) {
			return true
		}
		offset += wordCount * 4
	}
	return false
}

// divmodCountOpcode counts occurrences of a specific opcode.
func divmodCountOpcode(spvBytes []byte, op OpCode) int {
	count := 0
	offset := 20
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := int(word >> 16)
		if wordCount == 0 || offset+wordCount*4 > len(spvBytes) {
			break
		}
		if opcode == uint32(op) {
			count++
		}
		offset += wordCount * 4
	}
	return count
}

// =============================================================================
// Test: u32 division generates a wrapper function
// =============================================================================

func TestWrappedDivU32(t *testing.T) {
	// Simple compute shader: result = a / b (u32)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						// [0] literal 10u
						{Kind: ir.Literal{Value: ir.LiteralU32(10)}},
						// [1] literal 3u
						{Kind: ir.Literal{Value: ir.LiteralU32(3)}},
						// [2] binary divide
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 0, Right: 1}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have 2 OpFunction: the wrapper + the entry point
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 2 {
		t.Errorf("expected 2 OpFunction (wrapper + entry), got %d", funcCount)
	}

	// Should have OpFunctionCall (calling the wrapper)
	calls := findOpFunctionCalls(spvBytes)
	if len(calls) == 0 {
		t.Error("expected at least 1 OpFunctionCall for wrapped division")
	}

	// Should have OpIEqual (zero check in wrapper)
	if !divmodHasOpcode(spvBytes, OpIEqual) {
		t.Error("expected OpIEqual in wrapper for zero check")
	}

	// Should have OpSelect (selecting safe divisor)
	if !divmodHasOpcode(spvBytes, OpSelect) {
		t.Error("expected OpSelect in wrapper")
	}

	// Unsigned div should NOT have OpLogicalAnd (no overflow check needed)
	if divmodHasOpcode(spvBytes, OpLogicalAnd) {
		t.Error("unsigned div wrapper should not have OpLogicalAnd (overflow check)")
	}
}

// =============================================================================
// Test: i32 division generates wrapper with overflow protection
// =============================================================================

func TestWrappedDivI32(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(10)}},
						{Kind: ir.Literal{Value: ir.LiteralI32(3)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 0, Right: 1}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have 2 OpFunction
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 2 {
		t.Errorf("expected 2 OpFunction (wrapper + entry), got %d", funcCount)
	}

	// Signed div SHOULD have OpLogicalAnd (overflow check: lhs==INT_MIN && rhs==-1)
	if !divmodHasOpcode(spvBytes, OpLogicalAnd) {
		t.Error("signed div wrapper should have OpLogicalAnd for overflow check")
	}

	// Should have OpLogicalOr (combining zero check and overflow check)
	if !divmodHasOpcode(spvBytes, OpLogicalOr) {
		t.Error("signed div wrapper should have OpLogicalOr")
	}

	// Should have OpIEqual (at least 3: rhs==0, lhs==INT_MIN, rhs==-1)
	ieqCount := divmodCountOpcode(spvBytes, OpIEqual)
	if ieqCount < 3 {
		t.Errorf("expected at least 3 OpIEqual in signed div wrapper, got %d", ieqCount)
	}
}

// =============================================================================
// Test: i32 modulo generates a wrapper computing lhs - (lhs / rhs) * rhs
// =============================================================================

// OpSRem is undefined for negative operands in the Vulkan environment without
// VK_KHR_maintenance8, so the wrapper avoids it (Rust naga, gfx-rs/wgpu#8191).
func TestWrappedModI32AvoidsOpSRem(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(10)}},
						{Kind: ir.Literal{Value: ir.LiteralI32(3)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryModulo, Left: 0, Right: 1}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	if divmodHasOpcode(spvBytes, OpSRem) || divmodHasOpcode(spvBytes, OpSMod) {
		t.Error("signed modulo wrapper must not use OpSRem or OpSMod")
	}
	for name, op := range map[string]OpCode{"OpSDiv": OpSDiv, "OpIMul": OpIMul, "OpISub": OpISub} {
		if !divmodHasOpcode(spvBytes, op) {
			t.Errorf("signed modulo wrapper has no %s (lhs - (lhs / rhs) * rhs)", name)
		}
	}
}

// =============================================================================
// Test: f32 division does NOT generate wrappers
// =============================================================================

func TestFloatDivNoWrapper(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(10.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 0, Right: 1}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have only 1 OpFunction (the entry point, no wrapper)
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 1 {
		t.Errorf("expected 1 OpFunction (no wrapper for float div), got %d", funcCount)
	}

	// Should NOT have OpFunctionCall
	calls := findOpFunctionCalls(spvBytes)
	if len(calls) != 0 {
		t.Errorf("expected no OpFunctionCall for float div, got %d", len(calls))
	}

	// Should have OpFDiv directly
	if !divmodHasOpcode(spvBytes, OpFDiv) {
		t.Error("float division should use OpFDiv directly")
	}
}

// =============================================================================
// Test: wrappers are de-duplicated (two u32 divides share one wrapper)
// =============================================================================

func TestWrappedDivDeduplicated(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralU32(10)}},
						{Kind: ir.Literal{Value: ir.LiteralU32(3)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 0, Right: 1}},
						// Second division with same types
						{Kind: ir.Literal{Value: ir.LiteralU32(20)}},
						{Kind: ir.Literal{Value: ir.LiteralU32(7)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 3, Right: 4}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have 2 OpFunction: ONE wrapper + entry point (not 3)
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 2 {
		t.Errorf("expected 2 OpFunction (1 deduped wrapper + entry), got %d", funcCount)
	}

	// Should have 2 OpFunctionCall (both divides call the same wrapper)
	calls := findOpFunctionCalls(spvBytes)
	if len(calls) != 2 {
		t.Errorf("expected 2 OpFunctionCall, got %d", len(calls))
	}

	// Both calls should reference the same wrapper function ID
	if len(calls) == 2 && calls[0] != calls[1] {
		t.Errorf("expected both calls to same wrapper, got IDs %d and %d", calls[0], calls[1])
	}
}

// =============================================================================
// Test: vec2<i32> division generates wrapper (vector types)
// =============================================================================

func TestWrappedDivVec2I32(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                 // [0] i32
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}, // [1] vec2<i32>
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(10)}},
						{Kind: ir.Literal{Value: ir.LiteralI32(20)}},
						{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
						{Kind: ir.Literal{Value: ir.LiteralI32(3)}},
						{Kind: ir.Literal{Value: ir.LiteralI32(4)}},
						{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{3, 4}}},
						{Kind: ir.ExprBinary{Op: ir.BinaryDivide, Left: 2, Right: 5}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
					},
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have 2 OpFunction: vec2<i32> wrapper + entry point
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 2 {
		t.Errorf("expected 2 OpFunction (wrapper + entry), got %d", funcCount)
	}

	// Signed vec div should have OpLogicalAnd (overflow check applies per component)
	if !divmodHasOpcode(spvBytes, OpLogicalAnd) {
		t.Error("signed vec2<i32> div wrapper should have OpLogicalAnd for overflow check")
	}

	// Should have OpFunctionCall
	calls := findOpFunctionCalls(spvBytes)
	if len(calls) == 0 {
		t.Error("expected OpFunctionCall for wrapped vec2<i32> division")
	}
}

// =============================================================================
// Test: u32 modulo generates wrapper
// =============================================================================

func TestWrappedModU32(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralU32(10)}},
						{Kind: ir.Literal{Value: ir.LiteralU32(3)}},
						{Kind: ir.ExprBinary{Op: ir.BinaryModulo, Left: 0, Right: 1}},
					},
					Body: ir.Block{
						ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					},
					// No Result — entry point is void in SPIR-V
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	spvBytes := compileModule(t, module)

	// Should have wrapper
	funcCount := countOpFunctions(spvBytes)
	if funcCount != 2 {
		t.Errorf("expected 2 OpFunction (wrapper + entry), got %d", funcCount)
	}

	// Should use OpUMod in the wrapper
	if !divmodHasOpcode(spvBytes, OpUMod) {
		t.Error("u32 modulo wrapper should use OpUMod")
	}
}

// =============================================================================
// Test: f32 modulo is OpFRem (truncated), not OpFMod (floored)
// =============================================================================

// WGSL defines e1 % e2 on floats as e1 - e2 * trunc(e1 / e2), whose sign
// follows e1 (OpFRem). OpFMod follows e2: -1.5 % 1.0 gave 0.5, not -0.5.
func TestFloatModUsesOpFRem(t *testing.T) {
	spvBytes := compileWGSL(t, `
@group(0) @binding(0) var<storage, read_write> d: array<f32>;

@compute @workgroup_size(1)
fn main() {
    let v = vec2(d[0], d[1]);
    d[2] = d[0] % d[1];
    d[3] = (v % v.yx).x;
    d[4] = (v % d[1]).y;
}
`)
	if n := divmodCountOpcode(spvBytes, OpFRem); n != 3 {
		t.Errorf("OpFRem count = %d, want 3 (scalar, vector, vector by scalar)", n)
	}
	if divmodHasOpcode(spvBytes, OpFMod) {
		t.Error("float modulo must not use OpFMod")
	}
}
