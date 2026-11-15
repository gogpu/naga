package front

// This file resolves the types GLSL writes to IR type handles: a builtin type
// name, a declared struct, an inline struct definition, and the array
// specifiers around them. Rust naga does this while parsing
// (parser/types.rs:19-134); here it is part of lowering, which owns the
// module's type arena.

import (
	"errors"

	"github.com/gogpu/naga/ir"
)

// resolveTypeSpec resolves a written type to a module type handle. layout is
// the struct layout an inline struct definition is laid out with: Rust naga
// uses std140 for a plain struct definition (parser/types.rs:86-92) and the
// block's own layout for an interface block.
func (f *Frontend) resolveTypeSpec(spec *typeSpec, layout structLayout) (ir.TypeHandle, *frontError) {
	var ty ir.TypeHandle
	switch {
	case spec.Builtin != nil:
		if spec.Builtin.Combined {
			// A combined sampler is not an IR type: it is a pair of globals
			// that only a module-scope uniform declaration creates.
			return 0, errMessage(spec.Span, "combined image sampler types are only supported for module-scope uniform variables")
		}
		ty = f.registerType(spec.Builtin.Inner)
	case spec.StructDef != nil:
		var err *frontError
		ty, err = f.defineStruct(spec.StructDef, layout)
		if err != nil {
			return 0, err
		}
	case spec.Name != "":
		h, ok := f.types[spec.Name]
		if !ok {
			return 0, errUnknownType(spec.Name, spec.Span)
		}
		ty = h
	default:
		return 0, errInternal("empty type specification", spec.Span)
	}
	return f.resolveTypeArrays(ty, spec.Arrays)
}

// resolveTypeArrays applies the array specifiers written after a type to it,
// Rust naga's parse_array_specifier (parser/types.rs:19-73).
func (f *Frontend) resolveTypeArrays(ty ir.TypeHandle, arrays []arraySpec) (ir.TypeHandle, *frontError) {
	for _, spec := range arrays {
		var size ir.ArraySize
		if spec.Size != nil {
			count, err := f.constU32(spec.Size)
			if err != nil {
				return 0, err
			}
			if count == 0 {
				return 0, errMessage(spec.Span, "Array size must be greater than zero")
			}
			size.Constant = &count
		}
		if err := f.layouter.Update(f.mod); err != nil {
			// The array is being built from types that exist, so a cycle
			// cannot come from here; a zero stride keeps the lowering going
			// and validation reports the type.
			ty = f.registerType(ir.ArrayType{Base: ty, Size: size})
			continue
		}
		layout := f.layouter.Layout(ty)
		stride := roundUp(max(layout.Alignment, 1), layout.Size)
		ty = f.registerType(ir.ArrayType{Base: ty, Size: size, Stride: stride})
	}
	return ty, nil
}

// defineStruct registers an inline struct definition, Rust naga's struct
// branch of parse_type (parser/types.rs:84-106).
func (f *Frontend) defineStruct(def *structDef, layout structLayout) (ir.TypeHandle, *frontError) {
	inner, err := f.layoutStructMembers(def.Members, layout)
	if err != nil {
		return 0, err
	}
	ty := f.registerNamedType(def.Name, inner)
	f.types[def.Name] = ty
	return ty, nil
}

// layoutStructMembers resolves the members of a struct definition or an
// interface block and lays them out, Rust naga's parse_struct_declaration_list
// (parser/declarations.rs:625-685): every declarator of a member declaration
// is placed on its own.
func (f *Frontend) layoutStructMembers(members []structMember, layout structLayout) (ir.StructType, *frontError) {
	out := make([]ir.StructMember, 0, len(members))
	var span, align uint32
	align = 1
	for _, member := range members {
		ty, err := f.resolveTypeSpec(&member.Typ, layout)
		if err != nil {
			return ir.StructType{}, err
		}
		ty, err = f.resolveTypeArrays(ty, member.Arrays)
		if err != nil {
			return ir.StructType{}, err
		}
		info := f.calculateOffset(ty, member.Span, layout)
		span = roundUp(info.align, span)
		align = max(info.align, align)
		out = append(out, ir.StructMember{Name: member.Name, Type: info.ty, Offset: span})
		span += info.span
	}
	span = roundUp(align, span)
	return ir.StructType{Members: out, Span: span}, nil
}

// errNegativeConstant is the reason a constant expression is not a uint: it
// is a negative signed value, which Rust reports as an overflow
// (parser.rs:216-221).
var errNegativeConstant = errors.New("negative constant")

// constU32 evaluates a constant expression to a uint32, Rust naga's
// parse_uint_constant (parser.rs:203-229). The expression is lowered in the
// module's shared const context, where only module-scope constants and
// overrides resolve, as GLSL requires of a constant expression.
func (f *Frontend) constU32(e *exprs) (uint32, *frontError) {
	c, err := f.constContext()
	if err != nil {
		return 0, err
	}
	h, span, err := c.lowerExpect(&e.Arena, e.Root, rhsPos)
	if err != nil {
		return 0, err
	}
	lit, ok := c.eval.Literal(h)
	if !ok {
		return 0, errMessage(span, "Expected a uint constant")
	}
	v, convErr := u32FromLiteral(lit)
	if convErr != nil {
		if errors.Is(convErr, errNegativeConstant) {
			return 0, errMessage(span, "int constant overflows")
		}
		return 0, errMessage(span, "Expected a uint constant")
	}
	return v, nil
}

// u32FromLiteral converts a literal to uint32 as Rust's TryFrom<Literal> for
// u32 does (proc/mod.rs:242-254), reporting a negative value apart from a
// value of another type.
func u32FromLiteral(lit ir.LiteralValue) (uint32, error) {
	switch v := lit.(type) {
	case ir.LiteralU16:
		return uint32(v), nil
	case ir.LiteralU32:
		return uint32(v), nil
	case ir.LiteralI16:
		if v < 0 {
			return 0, errNegativeConstant
		}
		return uint32(v), nil
	case ir.LiteralI32:
		if v < 0 {
			return 0, errNegativeConstant
		}
		return uint32(v), nil
	default:
		return 0, errors.New("not an integer constant")
	}
}
