package lower

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// Type constructor expressions, ported from Rust naga's
// front/wgsl/lower/construction.rs. Every constructor form (`vec3<f32>(...)`,
// `vec3(...)`, `vec3f(...)`, `f32(...)`, `S(...)`, `array(...)`) is lowered
// here: the arguments are lowered as possibly-abstract values, converted with
// WGSL's automatic conversions, and the result is appended through the
// constant evaluator.

// constructorKind is the head of a constructor expression (Rust Constructor).
type constructorKind int

const (
	// ctorType: a known type, like vec3<f32> or a struct.
	ctorType constructorKind = iota
	// ctorPartialVector: vecN(...) with the component type inferred.
	ctorPartialVector
	// ctorPartialMatrix: matCxR(...) with the component type inferred.
	ctorPartialMatrix
	// ctorPartialArray: array(...) with element type and size inferred.
	ctorPartialArray
)

type constructor struct {
	kind          constructorKind
	ty            ir.TypeHandle // ctorType
	size          ir.VectorSize // ctorPartialVector
	columns, rows ir.VectorSize // ctorPartialMatrix
}

// WGSL type names used by the constructor and diagnostic code.
const (
	vecTypePrefix = "vec"  // vecN
	matTypePrefix = "mat"  // matCxR
	boolTypeName  = "bool" // the boolean scalar
)

// partialConstructor classifies a constructor name without template
// parameters: vecN, matCxR and array are partial; anything else is not.
func partialConstructor(name string) (constructor, bool) {
	switch {
	case len(name) == 4 && name[:3] == vecTypePrefix && name[3] >= '2' && name[3] <= '4':
		return constructor{kind: ctorPartialVector, size: ir.VectorSize(name[3] - '0')}, true
	case len(name) == 6 && name[:3] == matTypePrefix && name[4] == 'x' &&
		name[3] >= '2' && name[3] <= '4' && name[5] >= '2' && name[5] <= '4':
		return constructor{kind: ctorPartialMatrix, columns: ir.VectorSize(name[3] - '0'), rows: ir.VectorSize(name[5] - '0')}, true
	case name == "array":
		return constructor{kind: ctorPartialArray}, true
	}
	return constructor{}, false
}

// construct lowers a constructor of ctor with the AST arguments args (Rust
// Lowerer::construct).
func (l *Lowerer) construct(ctor constructor, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	comps := make([]ir.ExpressionHandle, len(args))
	for i, arg := range args {
		h, err := l.lowerExpression(arg, target)
		if err != nil {
			return 0, err
		}
		comps[i] = h
	}
	kind, err := l.constructKind(ctor, comps)
	if err != nil {
		return 0, err
	}
	if h, ok := kind.(ir.ExpressionHandle); ok {
		return h, nil // trivial conversion: the argument itself
	}
	return l.addExpression(ir.Expression{Kind: kind.(ir.ExpressionKind)})
}

// constructKind builds the expression for ctor applied to the lowered
// arguments comps. It returns either the expression kind to append or, for a
// constructor that is the identity on its argument, that argument's handle.
func (l *Lowerer) constructKind(ctor constructor, comps []ir.ExpressionHandle) (any, error) {
	var target ir.TypeInner
	if ctor.kind == ctorType {
		target = l.module.Types[ctor.ty].Inner
	}
	switch len(comps) {
	case 0:
		return l.constructZero(ctor, target)
	case 1:
		if k, ok, err := l.constructOne(ctor, target, comps[0]); ok || err != nil {
			return k, err
		}
	}
	return l.constructMany(ctor, target, comps)
}

func (l *Lowerer) constructZero(ctor constructor, target ir.TypeInner) (any, error) {
	switch ctor.kind {
	case ctorType:
		if !l.isConstructible(target) {
			return nil, l.notConstructible(ctor.ty)
		}
		return ir.ExprZeroValue{Type: ctor.ty}, nil
	case ctorPartialVector:
		// vec2(), vec3(), vec4() are vectors of AbstractInt (WGSL spec).
		ty := l.registerType("", ir.VectorType{Size: ctor.size, Scalar: scalarAbstractInt})
		return ir.ExprZeroValue{Type: ty}, nil
	}
	return nil, fmt.Errorf("type can't be inferred")
}

// constructOne handles the single-argument forms: conversions, identity
// partial constructors and splats. ok is false for a form that is
// handled like several arguments (single-element array, struct, ...).
func (l *Lowerer) constructOne(ctor constructor, target ir.TypeInner, c ir.ExpressionHandle) (any, bool, error) {
	arg := l.exprInner(c)
	switch a := arg.(type) {
	case ir.ScalarType:
		switch t := target.(type) {
		case ir.ScalarType: // scalar conversion
			w := t.Width
			return ir.ExprAs{Expr: c, Kind: t.Kind, Convert: &w}, true, nil
		case ir.VectorType: // splat with an explicit type
			if !automaticallyConvertsTo(a, t.Scalar) {
				return nil, true, fmt.Errorf("wrong type passed as argument #1 to `%s`: expected %s, found %s",
					l.wgslTypeHandle(ctor.ty), wgslScalar(t.Scalar), l.wgslType(arg))
			}
			v, err := l.convertToLeafScalar(c, t.Scalar)
			if err != nil {
				return nil, true, err
			}
			return ir.ExprSplat{Size: t.Size, Value: v}, true, nil
		}
		if ctor.kind == ctorPartialVector {
			return ir.ExprSplat{Size: ctor.size, Value: c}, true, nil
		}
	case ir.VectorType:
		if t, ok := target.(ir.VectorType); ok && t.Size == a.Size {
			w := t.Scalar.Width
			return ir.ExprAs{Expr: c, Kind: t.Scalar.Kind, Convert: &w}, true, nil
		}
		if ctor.kind == ctorPartialVector && ctor.size == a.Size {
			return c, true, nil
		}
	case ir.MatrixType:
		if t, ok := target.(ir.MatrixType); ok && t.Columns == a.Columns && t.Rows == a.Rows {
			w := t.Scalar.Width
			return ir.ExprAs{Expr: c, Kind: t.Scalar.Kind, Convert: &w}, true, nil
		}
		if ctor.kind == ctorPartialMatrix && ctor.columns == a.Columns && ctor.rows == a.Rows {
			return c, true, nil
		}
	}
	return nil, false, nil
}

// constructMany handles every form not handled by constructZero or
// constructOne, one helper per constructor kind (the match arms of Rust
// Lowerer::construct).
func (l *Lowerer) constructMany(ctor constructor, target ir.TypeInner, comps []ir.ExpressionHandle) (any, error) {
	switch ctor.kind {
	case ctorPartialVector:
		return l.constructPartialVector(ctor, comps)
	case ctorPartialMatrix:
		if k, ok, err := l.constructPartialMatrix(ctor, comps); ok || err != nil {
			return k, err
		}
	case ctorPartialArray:
		return l.constructPartialArray(comps)
	case ctorType:
		return l.constructTyped(ctor, target, comps)
	}
	return nil, fmt.Errorf("type can't be inferred")
}

// constructPartialVector builds vecN(...) of the arguments' common scalar.
func (l *Lowerer) constructPartialVector(ctor constructor, comps []ir.ExpressionHandle) (any, error) {
	consensus, bad, ok := l.conversionConsensus(nil, comps)
	if !ok {
		return nil, fmt.Errorf("invalid type for constructor component at index [%d]", bad)
	}
	if err := l.convertSliceToCommonLeafScalar(comps, consensus); err != nil {
		return nil, err
	}
	ty := l.registerType("", ir.VectorType{Size: ctor.size, Scalar: consensus})
	return ir.ExprCompose{Type: ty, Components: comps}, nil
}

// constructPartialMatrix builds matCxR(...) by elements or by columns; ok
// is false for any other argument count.
func (l *Lowerer) constructPartialMatrix(ctor constructor, comps []ir.ExpressionHandle) (any, bool, error) {
	consensus, bad, ok := l.conversionConsensus(&scalarAbstractFloat, comps)
	if !ok {
		return nil, true, fmt.Errorf("invalid type for constructor component at index [%d]", bad)
	}
	switch len(comps) {
	case int(ctor.columns) * int(ctor.rows): // by elements
		if err := l.convertSliceToCommonLeafScalar(comps, consensus); err != nil {
			return nil, true, err
		}
		k, err := l.matrixFromElements(ir.MatrixType{Columns: ctor.columns, Rows: ctor.rows, Scalar: consensus}, comps)
		return k, true, err
	case int(ctor.columns): // by columns
		if err := l.tryAutomaticConversionsSlice(comps, ir.VectorType{Size: ctor.rows, Scalar: consensus}); err != nil {
			return nil, true, err
		}
		ty := l.registerType("", ir.MatrixType{Columns: ctor.columns, Rows: ctor.rows, Scalar: consensus})
		return ir.ExprCompose{Type: ty, Components: comps}, true, nil
	}
	return nil, false, nil
}

// constructPartialArray builds array(...): the element type is the first
// argument's type after the leaf scalars are reconciled.
func (l *Lowerer) constructPartialArray(comps []ir.ExpressionHandle) (any, error) {
	if consensus, _, ok := l.conversionConsensus(nil, comps); ok {
		// Reconciling the leaf scalars is right whenever the array can
		// be well-typed at all; otherwise validation reports it.
		if err := l.convertSliceToCommonLeafScalar(comps, consensus); err != nil {
			return nil, err
		}
	}
	base, err := l.inferTypeFromExpression(comps[0])
	if err != nil {
		return nil, err
	}
	n := index32[uint32](len(comps))
	return ir.ExprCompose{Type: l.registerArrayType(base, ir.ArraySize{Constant: &n}), Components: comps}, nil
}

// constructTyped builds a constructor of a known type from its arguments.
func (l *Lowerer) constructTyped(ctor constructor, target ir.TypeInner, comps []ir.ExpressionHandle) (any, error) {
	if len(comps) == 1 {
		switch target.(type) {
		case ir.ScalarType, ir.VectorType, ir.MatrixType:
			// One argument that no conversion arm accepted.
			return nil, fmt.Errorf("cannot cast a %s to a %s", l.wgslType(l.exprInner(comps[0])), l.wgslTypeHandle(ctor.ty))
		}
	}
	switch t := target.(type) {
	case ir.VectorType:
		if err := l.tryAutomaticConversionsForVector(comps, t.Scalar); err != nil {
			return nil, err
		}
		return ir.ExprCompose{Type: ctor.ty, Components: comps}, nil
	case ir.MatrixType:
		switch len(comps) {
		case int(t.Columns) * int(t.Rows):
			if err := l.tryAutomaticConversionsSlice(comps, t.Scalar); err != nil {
				return nil, err
			}
			return l.matrixFromElements(t, comps)
		case int(t.Columns):
			if err := l.tryAutomaticConversionsSlice(comps, ir.VectorType{Size: t.Rows, Scalar: t.Scalar}); err != nil {
				return nil, err
			}
			return ir.ExprCompose{Type: ctor.ty, Components: comps}, nil
		}
	case ir.ArrayType:
		if !l.isConstructible(t) {
			break
		}
		if err := l.tryAutomaticConversionsSlice(comps, l.module.Types[t.Base].Inner); err != nil {
			return nil, err
		}
		return ir.ExprCompose{Type: ctor.ty, Components: comps}, nil
	case ir.StructType:
		if !l.isConstructible(t) {
			break
		}
		if err := l.convertStructMembers(t, comps); err != nil {
			return nil, err
		}
		return ir.ExprCompose{Type: ctor.ty, Components: comps}, nil
	case ir.ScalarType:
		return nil, fmt.Errorf("unexpected components")
	}
	return nil, l.notConstructible(ctor.ty)
}

// convertStructMembers converts each argument of a struct constructor to
// the type of its member; extra arguments are left to validation.
func (l *Lowerer) convertStructMembers(t ir.StructType, comps []ir.ExpressionHandle) error {
	for i := range comps {
		if i >= len(t.Members) {
			break
		}
		c, err := l.tryAutomaticConversions(comps[i], l.module.Types[t.Members[i].Type].Inner)
		if err != nil {
			return err
		}
		comps[i] = c
	}
	return nil
}

// matrixFromElements groups the column-major scalar elements of a matrix
// into column vectors (Rust appends each column Compose, then the matrix).
func (l *Lowerer) matrixFromElements(mt ir.MatrixType, elems []ir.ExpressionHandle) (any, error) {
	colTy := l.registerType("", ir.VectorType{Size: mt.Rows, Scalar: mt.Scalar})
	rows := int(mt.Rows)
	cols := make([]ir.ExpressionHandle, mt.Columns)
	for c := range cols {
		h, err := l.addExpression(ir.Expression{Kind: ir.ExprCompose{Type: colTy, Components: elems[c*rows : (c+1)*rows]}})
		if err != nil {
			return nil, err
		}
		cols[c] = h
	}
	return ir.ExprCompose{Type: l.registerType("", mt), Components: cols}, nil
}

func (l *Lowerer) notConstructible(ty ir.TypeHandle) error {
	return fmt.Errorf("type `%s` is not constructible", l.wgslTypeHandle(ty))
}

// isConstructible reports a constructible type (Rust
// TypeInner::is_constructible): scalars, vectors, matrices, fixed-size
// arrays of constructible types and structs of constructible members.
func (l *Lowerer) isConstructible(inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		return true
	case ir.ArrayType:
		return t.Size.Constant != nil && l.isConstructible(l.module.Types[t.Base].Inner)
	case ir.StructType:
		for _, m := range t.Members {
			if !l.isConstructible(l.module.Types[m.Type].Inner) {
				return false
			}
		}
		return true
	}
	return false
}

// lowerShortAliasConstructor handles constructor calls using short type aliases
// (e.g., vec3f(1.0, 2.0, 3.0) which expands to vec3<f32>(1.0, 2.0, 3.0)).
func (l *Lowerer) lowerShortAliasConstructor(alias shortTypeAlias, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	typeHandle, err := l.resolveNamedType(&parser.NamedType{
		Name:       alias.baseName,
		TypeParams: []parser.Type{&parser.NamedType{Name: alias.scalarName}},
	})
	if err != nil {
		return 0, fmt.Errorf("short type alias %s<%s>: %w", alias.baseName, alias.scalarName, err)
	}
	return l.construct(constructor{kind: ctorType, ty: typeHandle}, args, target)
}

// lowerTypeConstructorCall handles a constructor call for a named type (struct, vector, matrix,
// scalar, or type alias). E.g., VertexOutput(pos, uv), FVec3(0.0), Mat(1.0, 2.0, 3.0, 4.0).
func (l *Lowerer) lowerTypeConstructorCall(typeHandle ir.TypeHandle, args []parser.Expr, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	return l.construct(constructor{kind: ctorType, ty: typeHandle}, args, target)
}
