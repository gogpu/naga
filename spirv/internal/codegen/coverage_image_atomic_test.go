package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// ---------------------------------------------------------------------------
// Image atomic operations — 0% coverage for emitImageAtomic,
// resolveImageGlobalVar, emitI32Constant, emitU32Constant.
// These require storage textures with atomic access — not reachable from WGSL.
// ---------------------------------------------------------------------------

// TestImageAtomicAdd exercises emitImageAtomic with AtomicAdd on a storage
// texture. Covers resolveImageGlobalVar, emitI32Constant, emitU32Constant,
// and the entire image atomic code path including OpImageTexelPointer.
func TestImageAtomicAdd(t *testing.T) {
	r32uintHandle := ir.TypeHandle(0)
	vec2i32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			// type 0: storage texture r32uint
			{Name: "tex_r32uint", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Arrayed:       false,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Uint,
			}},
			// type 1: u32
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// type 2: vec2<i32> for coordinates
			{Name: "vec2i32", Inner: ir.VectorType{
				Size:   2,
				Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "img",
				Type:    r32uintHandle,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						// expr 0: GlobalVariable -> img
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						// expr 1: coordinate vec2<i32>(0, 0)
						{Kind: ir.ExprZeroValue{Type: vec2i32Handle}},
						// expr 2: value 1u
						{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
						{Kind: ir.StmtImageAtomic{
							Image:      0,
							Coordinate: 1,
							ArrayIndex: nil,
							Fun:        ir.AtomicAdd{},
							Value:      2,
						}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	// OpImageTexelPointer should be emitted to get a pointer to the texel
	if !hasOpcodeInInstrs(instrs, spvenums.OpImageTexelPointer) {
		t.Error("expected OpImageTexelPointer for image atomic")
	}
	// The atomic operation itself
	if !hasOpcodeInInstrs(instrs, spvenums.OpAtomicIAdd) {
		t.Error("expected OpAtomicIAdd for image atomic add")
	}
}

// TestImageAtomicExchange exercises a different atomic operation on a storage
// texture — AtomicExchange — to cover that opcode selection path.
func TestImageAtomicExchange(t *testing.T) {
	r32uintHandle := ir.TypeHandle(0)
	vec2i32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "tex_r32uint", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Arrayed:       false,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Uint,
			}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "vec2i32", Inner: ir.VectorType{
				Size:   2,
				Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "img",
				Type:    r32uintHandle,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.ExprZeroValue{Type: vec2i32Handle}},
						{Kind: ir.Literal{Value: ir.LiteralU32(99)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
						{Kind: ir.StmtImageAtomic{
							Image:      0,
							Coordinate: 1,
							Fun:        ir.AtomicExchange{},
							Value:      2,
						}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if !hasOpcodeInInstrs(instrs, spvenums.OpAtomicExchange) {
		t.Error("expected OpAtomicExchange for image atomic exchange")
	}
}

// TestImageAtomicSignedMin exercises image atomic on a signed integer storage
// texture (r32sint) to cover the signed path in atomicOpcode.
func TestImageAtomicSignedMin(t *testing.T) {
	r32sintHandle := ir.TypeHandle(0)
	vec2i32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "tex_r32sint", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Arrayed:       false,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Sint,
			}},
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "vec2i32", Inner: ir.VectorType{
				Size:   2,
				Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "img",
				Type:    r32sintHandle,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.ExprZeroValue{Type: vec2i32Handle}},
						{Kind: ir.Literal{Value: ir.LiteralI32(-5)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
						{Kind: ir.StmtImageAtomic{
							Image:      0,
							Coordinate: 1,
							Fun:        ir.AtomicMin{},
							Value:      2,
						}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	// Signed min uses OpAtomicSMin
	if !hasOpcodeInInstrs(instrs, spvenums.OpAtomicSMin) {
		t.Error("expected OpAtomicSMin for signed image atomic min")
	}
}

// TestImageAtomicCompareExchange pins the compare form of an image atomic:
// OpAtomicCompareExchange with the texel pointer, the scope, the two memory
// semantics operands, the value and the comparator. Emitting OpAtomicExchange
// instead is a valid instruction that silently drops the comparison, which no
// validator reports, so the operands have to be checked by value.
func TestImageAtomicCompareExchange(t *testing.T) {
	r32uintHandle := ir.TypeHandle(0)
	vec2i32Handle := ir.TypeHandle(2)
	compare := ir.ExpressionHandle(3)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "tex_r32uint", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Arrayed:       false,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Uint,
			}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "vec2i32", Inner: ir.VectorType{
				Size:   2,
				Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "img",
				Type:    r32uintHandle,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
						{Kind: ir.ExprZeroValue{Type: vec2i32Handle}},
						{Kind: ir.Literal{Value: ir.LiteralU32(2)}}, // value
						{Kind: ir.Literal{Value: ir.LiteralU32(1)}}, // comparator
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 4}}},
						{Kind: ir.StmtImageAtomic{
							Image:      0,
							Coordinate: 1,
							Fun:        ir.AtomicExchange{Compare: &compare},
							Value:      2,
						}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(mod)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	assertValidSPIRV(t, spvBytes)
	instrs := decodeSPIRVInstructions(spvBytes)

	if hasOpcodeInInstrs(instrs, spvenums.OpAtomicExchange) {
		t.Error("emitted OpAtomicExchange for a compare-exchange: the comparator is lost")
	}

	// OpConstant: Opcode ResultType Result Value.
	constants := make(map[uint32]uint32)
	for _, inst := range instrs {
		if inst.opcode == spvenums.OpConstant && len(inst.words) >= 4 {
			constants[inst.words[2]] = inst.words[3]
		}
	}

	var cas *spirvInstruction
	for i := range instrs {
		if instrs[i].opcode == spvenums.OpAtomicCompareExchange {
			cas = &instrs[i]
			break
		}
	}
	if cas == nil {
		t.Fatal("no OpAtomicCompareExchange for an image compare-exchange")
	}
	// Opcode, ResultType, Result, Pointer, Scope, Equal semantics, Unequal
	// semantics, Value, Comparator.
	if len(cas.words) != 9 {
		t.Fatalf("OpAtomicCompareExchange has %d words, want 9 (opcode word plus Result Type, Result, "+
			"Pointer, Scope, Equal and Unequal semantics, Value, Comparator)", len(cas.words))
	}
	valueID, compareID := cas.words[7], cas.words[8]
	if got, ok := constants[valueID]; !ok || got != 2 {
		t.Errorf("value operand resolves to %d (a constant: %v), want 2", got, ok)
	}
	if got, ok := constants[compareID]; !ok || got != 1 {
		t.Errorf("comparator operand resolves to %d (a constant: %v), want 1", got, ok)
	}
}
