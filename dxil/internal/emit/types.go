package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// scalarToDXIL maps a naga ScalarType to a DXIL type.
// DXIL types are always scalar — vectors are decomposed by the emitter.
func scalarToDXIL(mod *module.Module, s ir.ScalarType) *module.Type {
	switch s.Kind {
	case ir.ScalarBool:
		return mod.GetIntType(1)
	case ir.ScalarSint:
		return mod.GetIntType(uint(s.Width) * 8)
	case ir.ScalarUint:
		return mod.GetIntType(uint(s.Width) * 8)
	case ir.ScalarFloat:
		return mod.GetFloatType(uint(s.Width) * 8)
	default:
		// Abstract types should have been resolved before reaching the backend.
		return mod.GetIntType(32)
	}
}

// typeToDXIL maps a naga IR TypeInner to a DXIL type.
// Vectors and matrices are scalarized — this returns the element type.
// Callers must handle multi-component types by iterating over components.
//
//nolint:gocognit // dispatch over scalar/vec/mat/array/struct/atomic/pointer kinds
func typeToDXIL(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) (*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToDXIL(mod, t), nil

	case ir.VectorType:
		// DXIL has no native vectors. Return the scalar element type.
		// The caller is responsible for creating N separate values.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.MatrixType:
		// DXIL has no native matrices. Return the scalar element type.
		// Matrices are fully scalarized: columns * rows separate values.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.ArrayType:
		// DXIL allows only ONE array dimension. Flatten nested constant
		// arrays into a single [N*M*... x T] type. Mesa's nir_to_dxil
		// applies the same flattening via its lowering passes; DXC does
		// it via LLVM's SROA + GVN before reaching DXIL emission.
		// Without this, [4 x [4 x i32]] trips the validator's rule
		// 'Only one dimension allowed for array type'.
		//
		// Element flattening also covers VECTOR and MATRIX leaves: DXIL
		// has no native vectors, so `array<vec2<f32>, 3>` must become
		// `[6 x f32]` — 3 array slots × 2 scalar components — not
		// `[3 x f32]`. Our load/store paths already scalarize vectors
		// and track per-component IDs, so the slot count must include
		// every scalar lane. Before this fix, typeToDXIL recursed into
		// `ir.VectorType` and returned just the scalar, dropping the
		// vector width — the resulting alloca was too small, per-lane
		// stores landed on wrong offsets, and the LLVM 3.7 bitcode
		// writer emitted INST_LOAD / INST_STORE records that dxil.dll
		// rejected with HRESULT 0x80aa0009 "Invalid record" at the
		// record-level pre-parse. This was the root cause of the
		// `gogpu/examples/triangle` / `array<vec2<f32>, 3>` failure
		// surfaced by `GOGPU_DX12_DXIL_VALIDATE=1` (BUG-DXIL-011).
		flatSize := uint(1)
		curBase := t
		hasUnsized := false
		for {
			if curBase.Size.Constant == nil {
				hasUnsized = true
				break
			}
			flatSize *= uint(*curBase.Size.Constant)
			inner := irMod.Types[curBase.Base].Inner
			arr, isArr := inner.(ir.ArrayType)
			if !isArr {
				// Vector/matrix leaf: multiply flatSize by the component
				// count and emit an array of the scalar element type.
				// This is NECESSARY but NOT SUFFICIENT — the store/load
				// paths on top of this flattened alloca also need to
				// scale indices by the vector width. See BUG-DXIL-025
				// for the follow-up SROA-style access lowering; before
				// that lands, dynamic `array<vec<T,N>, M>` access from
				// local vars still trips 'Invalid record' at the
				// bitcode level even though the type is now correct.
				switch leaf := inner.(type) {
				case ir.VectorType:
					flatSize *= uint(leaf.Size)
					return mod.GetArrayType(scalarToDXIL(mod, leaf.Scalar), flatSize), nil
				case ir.MatrixType:
					flatSize *= uint(leaf.Columns) * uint(leaf.Rows)
					return mod.GetArrayType(scalarToDXIL(mod, leaf.Scalar), flatSize), nil
				}
				elemTy, err := typeToDXIL(mod, irMod, inner)
				if err != nil {
					return nil, fmt.Errorf("array element type: %w", err)
				}
				return mod.GetArrayType(elemTy, flatSize), nil
			}
			curBase = arr
		}
		// Runtime-sized array (or chain encountered one) — fall back to
		// single-dimension emission with size 0.
		if hasUnsized {
			elemTy, err := typeToDXIL(mod, irMod, irMod.Types[curBase.Base].Inner)
			if err != nil {
				return nil, fmt.Errorf("array element type: %w", err)
			}
			return mod.GetArrayType(elemTy, 0), nil
		}
		// Should not reach here, but keep a safe default.
		return mod.GetArrayType(mod.GetIntType(32), 0), nil

	case ir.StructType:
		elems := make([]*module.Type, len(t.Members))
		for i, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			var err error
			elems[i], err = typeToDXIL(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, fmt.Errorf("struct member %d (%s): %w", i, m.Name, err)
			}
		}
		return mod.GetStructType("", elems), nil

	case ir.AtomicType:
		// Atomic types map to their underlying scalar in DXIL (i32, i64, etc.).
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.PointerType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXIL(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("pointer element type: %w", err)
		}
		return mod.GetPointerType(elemTy), nil

	default:
		return mod.GetIntType(32), nil
	}
}

// typeToDXILFull maps a naga IR TypeInner to a DXIL type WITHOUT scalarizing
// vectors. Used for struct allocas and GEP source element types where the
// full composite type is needed.
func typeToDXILFull(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) (*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToDXIL(mod, t), nil

	case ir.VectorType:
		// Keep as scalar — DXIL has no vectors, but struct members that are vectors
		// are represented as multiple scalar fields or as the scalar type.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.MatrixType:
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.ArrayType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXILFull(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("array element type: %w", err)
		}
		if t.Size.Constant != nil {
			return mod.GetArrayType(elemTy, uint(*t.Size.Constant)), nil
		}
		return mod.GetArrayType(elemTy, 0), nil

	case ir.StructType:
		// Flatten struct members: vectors expand to N scalar fields,
		// nested structs recurse. This ensures every element in the DXIL
		// struct is a scalar type, matching DXIL's scalarized model.
		var elems []*module.Type
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			memberElems, err := flattenStructMember(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, fmt.Errorf("struct member %s: %w", m.Name, err)
			}
			elems = append(elems, memberElems...)
		}
		return mod.GetStructType("", elems), nil

	case ir.AtomicType:
		// Atomic types map to their underlying scalar in DXIL (i32, i64, etc.).
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.PointerType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXILFull(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("pointer element type: %w", err)
		}
		return mod.GetPointerType(elemTy), nil

	default:
		return mod.GetIntType(32), nil
	}
}

// flatMemberOffset computes the flat field index of a struct member at the
// given IR member index. This sums the total scalar component counts of all
// preceding members (vectors count as N, nested structs recursively flatten).
func flatMemberOffset(irMod *ir.Module, st ir.StructType, memberIndex int) int {
	offset := 0
	for i := 0; i < memberIndex && i < len(st.Members); i++ {
		memberTy := irMod.Types[st.Members[i].Type]
		offset += totalScalarCount(irMod, memberTy.Inner)
	}
	return offset
}

// totalScalarCount returns the total number of scalar values in a type,
// recursively flattening vectors, matrices, arrays, and nested structs.
func totalScalarCount(irMod *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		// Arrays are kept as array types in the DXIL struct (not flattened),
		// so they count as 1 element for flat offset computation.
		return 1
	case ir.StructType:
		total := 0
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			total += totalScalarCount(irMod, memberTy.Inner)
		}
		return total
	default:
		return 1
	}
}

// flattenStructMember expands a struct member type into scalar DXIL types.
// Vectors become N scalars, arrays expand element-wise, nested structs recurse,
// scalars return as-is.
func flattenStructMember(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) ([]*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return []*module.Type{scalarToDXIL(mod, t)}, nil
	case ir.VectorType:
		s := scalarToDXIL(mod, t.Scalar)
		elems := make([]*module.Type, t.Size)
		for i := range elems {
			elems[i] = s
		}
		return elems, nil
	case ir.MatrixType:
		s := scalarToDXIL(mod, t.Scalar)
		count := int(t.Columns) * int(t.Rows)
		elems := make([]*module.Type, count)
		for i := range elems {
			elems[i] = s
		}
		return elems, nil
	case ir.ArrayType:
		// Arrays are kept as array types within the struct — NOT flattened.
		// Only vectors/matrices are scalarized.
		arrayTy, err := typeToDXIL(mod, irMod, inner)
		if err != nil {
			return nil, fmt.Errorf("array member: %w", err)
		}
		return []*module.Type{arrayTy}, nil
	case ir.StructType:
		var elems []*module.Type
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			sub, err := flattenStructMember(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, err
			}
			elems = append(elems, sub...)
		}
		return elems, nil
	case ir.AtomicType:
		return []*module.Type{scalarToDXIL(mod, t.Scalar)}, nil
	default:
		return []*module.Type{mod.GetIntType(32)}, nil
	}
}

// elemByteSize returns the size of a type in bytes, following WGSL layout rules
// for matrices (column stride = round_up(rows * scalarW, 16-byte alignment for matCx≥3))
// and arrays (uses ArrayType.Stride if set). Used by UAV index computation in
// dxil/internal/emit/resources.go to convert element indices to byte offsets per
// the DXIL spec for RWRawBuffer (`coord0 in bytes`, see DXIL.rst BufferStore section).
//
// References:
//   - DXC HLOperationLower.cpp:4721 — `EltSize = OP->GetAllocSizeForType(EltTy)` for raw buffers.
//   - WGSL spec §10.3.3 — matrix layout rules (matCxR with R=3 gets stride 16 not 12).
//   - DXIL.rst:1789 — BufferStore coord0 semantics by resource kind.
func elemByteSize(irMod *ir.Module, inner ir.TypeInner) uint32 {
	switch t := inner.(type) {
	case ir.ScalarType:
		return uint32(t.Width)
	case ir.VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	case ir.MatrixType:
		// Each column is a vector of `Rows` scalars. WGSL aligns matCxR with R=3
		// to 16-byte column stride (matches HLSL `float3` 16-byte alignment).
		colStride := uint32(t.Rows) * uint32(t.Scalar.Width)
		if t.Rows == 3 && t.Scalar.Width == 4 {
			colStride = 16
		}
		return uint32(t.Columns) * colStride
	case ir.ArrayType:
		if t.Stride > 0 {
			elemCount := uint32(1)
			if t.Size.Constant != nil {
				elemCount = *t.Size.Constant
			}
			return elemCount * t.Stride
		}
		// No stride set (runtime-sized): use element type
		elemCount := uint32(1)
		if t.Size.Constant != nil {
			elemCount = *t.Size.Constant
		}
		return elemCount * elemByteSize(irMod, irMod.Types[t.Base].Inner)
	case ir.StructType:
		return t.Span
	case ir.AtomicType:
		return uint32(t.Scalar.Width)
	default:
		return 4
	}
}

// matrixColumnByteStride returns the byte stride between consecutive columns
// of a matrix per WGSL alignment rules. For matCxR with R=3 (e.g., mat3x3),
// columns are padded to 16 bytes per HLSL `float3` alignment; otherwise
// the stride equals R*scalarW.
func matrixColumnByteStride(m ir.MatrixType) uint32 {
	stride := uint32(m.Rows) * uint32(m.Scalar.Width)
	if m.Rows == 3 && m.Scalar.Width == 4 {
		stride = 16
	}
	return stride
}

// cbvComponentCount returns the total number of scalar components for a type
// in the context of CBV (constant buffer) loads. Unlike componentCount, this
// recursively expands structs and arrays to count all scalar values.
func cbvComponentCount(irMod *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		elemCount := 0
		if t.Size.Constant != nil {
			elemCount = int(*t.Size.Constant)
		}
		if elemCount == 0 {
			return 1
		}
		return elemCount * cbvComponentCount(irMod, irMod.Types[t.Base].Inner)
	case ir.StructType:
		total := 0
		for _, m := range t.Members {
			total += cbvComponentCount(irMod, irMod.Types[m.Type].Inner)
		}
		return total
	default:
		return 1
	}
}

// componentCount returns the number of scalar components for a type.
// isScalarizableType returns true if the type can be scalarized for DXIL
// function parameters (scalars, vectors, matrices). Returns false for
// arrays, structs, pointers, and other complex types that need special
// calling convention handling not yet implemented.
func isScalarizableType(inner ir.TypeInner) bool {
	switch inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		return true
	default:
		return false
	}
}

func componentCount(inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	default:
		return 1
	}
}

// scalarOfType returns the scalar type underlying any type.
// For scalars, returns the scalar directly. For vectors/matrices,
// returns the element scalar.
func scalarOfType(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.AtomicType:
		return t.Scalar, true
	default:
		return ir.ScalarType{}, false
	}
}

// isFloatType returns true if the type is a floating-point type.
func isFloatType(inner ir.TypeInner) bool {
	s, ok := scalarOfType(inner)
	if !ok {
		return false
	}
	return s.Kind == ir.ScalarFloat
}

// isSignedInt returns true if the type is a signed integer type.
func isSignedInt(inner ir.TypeInner) bool {
	s, ok := scalarOfType(inner)
	if !ok {
		return false
	}
	return s.Kind == ir.ScalarSint
}
