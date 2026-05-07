package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
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
	if !hasOpcodeInInstrs(instrs, OpImageTexelPointer) {
		t.Error("expected OpImageTexelPointer for image atomic")
	}
	// The atomic operation itself
	if !hasOpcodeInInstrs(instrs, OpAtomicIAdd) {
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

	if !hasOpcodeInInstrs(instrs, OpAtomicExchange) {
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
	if !hasOpcodeInInstrs(instrs, OpAtomicSMin) {
		t.Error("expected OpAtomicSMin for signed image atomic min")
	}
}
