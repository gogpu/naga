package front

// This file is Rust naga's front/glsl/offset.rs: the std140 and std430 member
// offsets, array strides and struct spans of interface blocks and struct
// definitions.

import "github.com/gogpu/naga/ir"

// minUniformAlignment is the base alignment std140 rounds every array and
// matrix to: the alignment of a vec4 in basic machine units (OpenGL spec
// §7.6.2.2 rule 4), Rust's Alignment::MIN_UNIFORM.
const minUniformAlignment = 16

// typeAlignSpan is a member's type with the alignment and size the layout
// requires: the handle may be a new array or struct type whose stride or
// member offsets differ from the original, Rust naga's TypeAlignSpan
// (offset.rs:27-36).
type typeAlignSpan struct {
	ty    ir.TypeHandle
	align uint32
	span  uint32
}

// calculateOffset returns the type, alignment and size of a struct member
// under layout, following the OpenGL spec's std140 and std430 rules (OpenGL
// spec §7.6.2.2), Rust naga's calculate_offset (offset.rs:43-190).
func (f *Frontend) calculateOffset(ty ir.TypeHandle, span ir.Span, layout structLayout) typeAlignSpan {
	switch t := f.mod.Types[ty].Inner.(type) {
	// 1. A scalar consuming N basic machine units has base alignment N.
	case ir.ScalarType:
		width := layoutScalarWidth(t)
		return typeAlignSpan{ty: ty, align: width, span: width}

	// 2. A two- or four-component vector has base alignment 2N or 4N.
	// 3. A three-component vector has base alignment 4N.
	case ir.VectorType:
		width := layoutScalarWidth(t.Scalar)
		align := alignmentOf(t.Size) * width
		return typeAlignSpan{ty: ty, align: align, span: uint32(t.Size) * width}

	// 4. An array of scalars or vectors takes the base alignment of its
	// element, and under std140 is rounded up to the alignment of a vec4.
	case ir.ArrayType:
		info := f.calculateOffset(t.Base, span, layout)
		var align, stride uint32
		if layout == structLayoutStd430 {
			align = info.align
			stride = roundUp(info.align, info.span)
		} else {
			align = max(info.align, minUniformAlignment)
			stride = roundUp(align, info.span)
		}
		var count uint32 = 1
		if t.Size.Constant != nil {
			count = *t.Size.Constant
		}
		// A size given by an override cannot come from a GLSL declaration;
		// Rust panics on it (offset.rs:89), we count one element.
		arrayTy := f.registerNamedType(f.mod.Types[ty].Name,
			ir.ArrayType{Base: info.ty, Size: t.Size, Stride: stride})
		return typeAlignSpan{ty: arrayTy, align: align, span: count * stride}

	// 5. A column-major matrix is stored as an array of its column vectors.
	case ir.MatrixType:
		align := alignmentOf(t.Rows) * uint32(t.Scalar.Width)
		if layout != structLayoutStd430 {
			align = max(align, minUniformAlignment)
		}
		if layout == structLayoutStd140 {
			// The IR's matrix stride cannot express these under std140:
			// f16 matrices and matrices of two rows, as Rust refuses them
			// (offset.rs:125-144).
			if t.Scalar.Kind == ir.ScalarFloat && t.Scalar.Width == 2 {
				f.addError(errSemantic(span, "unsupported matrix of the form f16matCxR (in this case f16mat%dx%d) in std140 block layout.", t.Columns, t.Rows))
			}
			if t.Rows == ir.Vec2 {
				f.addError(errSemantic(span, "unsupported matrix of the form matCx2 (in this case mat%dx2) in std140 block layout.", t.Columns))
			}
		}
		return typeAlignSpan{ty: ty, align: align, span: align * uint32(t.Columns)}

	case ir.StructType:
		members := make([]ir.StructMember, len(t.Members))
		copy(members, t.Members)
		var total, align uint32
		align = 1
		for i := range members {
			info := f.calculateOffset(members[i].Type, span, layout)
			total = roundUp(info.align, total)
			align = max(info.align, align)
			members[i].Type = info.ty
			members[i].Offset = total
			total += info.span
		}
		// Rule 9: a struct's base alignment, as a member of a block, is the
		// largest alignment of its own members rounded up to the alignment of a
		// vec4 under std140 (std430 omits the rounding). glslangValidator lays
		// `struct S { float a; }; S s; float after_s;` out with after_s at 16,
		// and the same block of `struct T { vec2 p; float q; };` with a following
		// member at 16, not at 12: without the rounding the member's offset is
		// four bytes past where every driver reads it. Rust naga's
		// calculate_offset keeps the natural alignment here.
		if layout != structLayoutStd430 {
			align = max(align, minUniformAlignment)
		}
		total = roundUp(align, total)
		structTy := f.registerNamedType(f.mod.Types[ty].Name,
			ir.StructType{Members: members, Span: total})
		return typeAlignSpan{ty: structTy, align: align, span: total}

	default:
		f.addError(errMessage(span, "Invalid struct member type"))
		return typeAlignSpan{ty: ty, align: 1, span: 0}
	}
}

// alignmentOf is the base alignment factor of a vector size:
// Alignment::from in Rust (offset.rs:66-68): 2 for a two-component vector and
// 4 for three and four components.
func alignmentOf(size ir.VectorSize) uint32 {
	switch size {
	case ir.Vec2:
		return 2
	default:
		return 4
	}
}

// layoutScalarWidth is the size of a scalar in the std140 and std430 layouts:
// its IR width in bytes for a numeric scalar, and the four basic machine units
// of a word for a bool.
//
// The IR gives a bool a width of 1, the size the validator and the layouter use
// for it, but a uniform block stores it in a word: glslangValidator lays the
// members of `bool a; bool b;` out at 0 and 4, a bvec2 at 8 bytes with alignment
// 8 and a bvec3 at 12 bytes with alignment 16, and a shader that wrote such a
// member at byte 0 alone would disagree with every driver. Rust naga's
// calculate_offset takes the IR width here and puts a bool member at byte 0
// with the next member at byte 1; the reference compiler's layout decides,
// because the buffer is filled by a caller who reads these offsets.
func layoutScalarWidth(scalar ir.ScalarType) uint32 {
	if scalar.Kind == ir.ScalarBool {
		return 4
	}
	return uint32(scalar.Width)
}

// roundUp rounds v up to a multiple of align, Rust's Alignment::round_up.
func roundUp(align, v uint32) uint32 {
	if align == 0 {
		return v
	}
	return (v + align - 1) / align * align
}
