// Package typifier types the expressions of an IR function: Rust naga's
// Typifier (front/mod.rs) with the rules of proc::ResolveContext::resolve
// (proc/typifier.rs). The validator's callers, every backend and the WGSL
// frontend type expressions with it; no stage keeps types of its own in the
// IR.
//
// A Typifier holds one TypeResolution per expression of one function, in
// arena order. Grow types the expressions it does not have yet, each from
// the resolutions of its operands, which it already holds: one linear pass,
// memoized, with no recursion.
package typifier

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Typifier holds the type of every expression of a function up to Len.
// The zero value is empty and ready to use.
type Typifier struct {
	resolutions []ir.TypeResolution
}

// Reset empties t, for a new function.
func (t *Typifier) Reset() {
	t.resolutions = t.resolutions[:0]
}

// Len returns the number of expressions t has typed: the first Len
// expressions of the function.
func (t *Typifier) Len() int {
	return len(t.resolutions)
}

// Get returns the type of expression h, which Grow has typed.
func (t *Typifier) Get(h ir.ExpressionHandle) ir.TypeResolution {
	return t.resolutions[h]
}

// Inner returns the type of expression h as a TypeInner of m.
func (t *Typifier) Inner(m *ir.Module, h ir.ExpressionHandle) ir.TypeInner {
	return t.resolutions[h].Inner(m.Types)
}

// Grow types every expression of fn below upTo that t has not typed yet,
// in arena order. A pass that appends expressions to fn calls it again to
// type them, as Rust's Typifier::grow.
//
// The IR a frontend makes refers only to earlier expressions. The DXIL
// backend's passes also refer forward: an inlined call's result loads a
// later return slot, an alias stands for a later phi, a phi merges values of
// a loop's later iteration. Grow types such an expression once the
// expression it reads is typed, after the expressions that follow it; a phi
// takes the type of the first incoming value that is typed, since every
// incoming value has the phi's type.
//
// On an error t keeps what it had before the call.
func (t *Typifier) Grow(m *ir.Module, fn *ir.Function, upTo ir.ExpressionHandle) error {
	n := int(upTo)
	if n > len(fn.Expressions) {
		return fmt.Errorf("typifier: expression [%d] out of range (%d expressions)", n-1, len(fn.Expressions))
	}
	start := len(t.resolutions)
	if n <= start {
		return nil
	}
	t.resolutions = append(t.resolutions, make([]ir.TypeResolution, n-start)...)
	r := resolver{module: m, fn: fn, t: t}
	var pending []ir.ExpressionHandle
	for i := start; i < n; i++ {
		h := ir.ExpressionHandle(i) //nolint:gosec // i < len(fn.Expressions), which fits a 32-bit handle by IR design
		res, ok, err := r.resolve(h)
		if err != nil {
			t.resolutions = t.resolutions[:start]
			return fmt.Errorf("typifier: expression [%d]: %w", h, err)
		}
		if ok {
			t.resolutions[h] = res
		} else {
			pending = append(pending, h)
		}
	}
	for len(pending) > 0 {
		left := pending[:0]
		for _, h := range pending {
			res, ok, err := r.resolve(h)
			if err != nil {
				t.resolutions = t.resolutions[:start]
				return fmt.Errorf("typifier: expression [%d]: %w", h, err)
			}
			if ok {
				t.resolutions[h] = res
			} else {
				left = append(left, h)
			}
		}
		if len(left) == len(pending) {
			t.resolutions = t.resolutions[:start]
			return fmt.Errorf("typifier: expression [%d] reads an expression that cannot be typed before it", left[0])
		}
		pending = left
	}
	return nil
}

// Invalidate types expression h, which t has typed, again from what fn's
// arena holds now: for a pass that rewrote h in place (Rust's
// Typifier::invalidate). The expressions h reads keep their types.
func (t *Typifier) Invalidate(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) error {
	if int(h) >= len(t.resolutions) {
		return fmt.Errorf("typifier: expression [%d] is not typed yet", h)
	}
	r := resolver{module: m, fn: fn, t: t}
	res, ok, err := r.resolve(h)
	if err != nil {
		return fmt.Errorf("typifier: expression [%d]: %w", h, err)
	}
	if !ok {
		return fmt.Errorf("typifier: expression [%d] reads an expression that is not typed", h)
	}
	t.resolutions[h] = res
	return nil
}

// Function returns a Typifier that has typed every expression of fn.
func Function(m *ir.Module, fn *ir.Function) (*Typifier, error) {
	t := &Typifier{resolutions: make([]ir.TypeResolution, 0, len(fn.Expressions))}
	if err := t.Grow(m, fn, ir.ExpressionHandle(len(fn.Expressions))); err != nil { //nolint:gosec // expression count fits a 32-bit handle by IR design
		return nil, err
	}
	return t, nil
}

// Module holds the typifier of every function and entry point of a module:
// what a backend's Compile types once before it writes the module.
type Module struct {
	functions map[*ir.Function]*Typifier
}

// ForModule types every function and entry point of m.
func ForModule(m *ir.Module) (*Module, error) {
	mt := &Module{functions: make(map[*ir.Function]*Typifier, len(m.Functions)+len(m.EntryPoints))}
	for i := range m.Functions {
		fn := &m.Functions[i]
		t, err := Function(m, fn)
		if err != nil {
			return nil, fmt.Errorf("function %q: %w", fn.Name, err)
		}
		mt.functions[fn] = t
	}
	for i := range m.EntryPoints {
		fn := &m.EntryPoints[i].Function
		t, err := Function(m, fn)
		if err != nil {
			return nil, fmt.Errorf("entry point %q: %w", m.EntryPoints[i].Name, err)
		}
		mt.functions[fn] = t
	}
	return mt, nil
}

// Function returns the typifier of fn, one of the module's functions or
// entry point functions, or nil for any other function.
func (mt *Module) Function(fn *ir.Function) *Typifier {
	if mt == nil {
		return nil
	}
	return mt.functions[fn]
}

// typed reports a resolution Grow has set; the zero TypeResolution stands
// for no type yet.
func typed(res ir.TypeResolution) bool {
	return res.Handle != nil || res.Value != nil
}

// resolver types one expression from the resolutions of its operands.
type resolver struct {
	module *ir.Module
	fn     *ir.Function
	t      *Typifier
}

// errPending is returned by operand for an expression not typed yet.
var errPending = errors.New("operand not typed yet")

// operand returns the resolution of expression h, errPending when Grow has
// not typed it yet.
func (r *resolver) operand(h ir.ExpressionHandle) (ir.TypeResolution, error) {
	if int(h) >= len(r.fn.Expressions) {
		return ir.TypeResolution{}, fmt.Errorf("expression [%d] out of range (%d expressions)", h, len(r.fn.Expressions))
	}
	if int(h) >= len(r.t.resolutions) || !typed(r.t.resolutions[h]) {
		return ir.TypeResolution{}, errPending
	}
	return r.t.resolutions[h], nil
}

// inner returns the TypeInner res stands for, checking its handle.
func (r *resolver) inner(res ir.TypeResolution) (ir.TypeInner, error) {
	if res.Handle != nil {
		if int(*res.Handle) >= len(r.module.Types) {
			return nil, fmt.Errorf("type handle %d out of range", *res.Handle)
		}
		return r.module.Types[*res.Handle].Inner, nil
	}
	return res.Value, nil
}

// resolve types expression h. ok is false when an operand is not typed yet.
func (r *resolver) resolve(h ir.ExpressionHandle) (res ir.TypeResolution, ok bool, err error) {
	res, err = r.resolveKind(r.fn.Expressions[h].Kind)
	if errors.Is(err, errPending) {
		return ir.TypeResolution{}, false, nil
	}
	if err != nil {
		return ir.TypeResolution{}, false, err
	}
	return res, true, nil
}

func handleRes(h ir.TypeHandle) ir.TypeResolution {
	return ir.TypeResolution{Handle: &h}
}

func valueRes(inner ir.TypeInner) ir.TypeResolution {
	return ir.TypeResolution{Value: inner}
}

var (
	u32Scalar  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	i32Scalar  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	f32Scalar  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	boolScalar = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
)

//nolint:gocyclo,cyclop,funlen // one case per expression kind, as Rust's ResolveContext::resolve
func (r *resolver) resolveKind(kind ir.ExpressionKind) (ir.TypeResolution, error) {
	m := r.module
	switch k := kind.(type) {
	case ir.Literal:
		return k.Type()
	case ir.ExprConstant:
		if int(k.Constant) >= len(m.Constants) {
			return ir.TypeResolution{}, fmt.Errorf("constant %d out of range", k.Constant)
		}
		return handleRes(m.Constants[k.Constant].Type), nil
	case ir.ExprOverride:
		if int(k.Override) >= len(m.Overrides) {
			return ir.TypeResolution{}, fmt.Errorf("override %d out of range", k.Override)
		}
		return handleRes(m.Overrides[k.Override].Ty), nil
	case ir.ExprZeroValue:
		return handleRes(k.Type), nil
	case ir.ExprCompose:
		return handleRes(k.Type), nil
	case ir.ExprAccess:
		return r.access(k.Base, nil)
	case ir.ExprAccessIndex:
		return r.access(k.Base, &k.Index)
	case ir.ExprSplat:
		return r.splat(k)
	case ir.ExprSwizzle:
		return r.swizzle(k)
	case ir.ExprFunctionArgument:
		if int(k.Index) >= len(r.fn.Arguments) {
			return ir.TypeResolution{}, fmt.Errorf("function argument index %d out of range", k.Index)
		}
		return handleRes(r.fn.Arguments[k.Index].Type), nil
	case ir.ExprGlobalVariable:
		if int(k.Variable) >= len(m.GlobalVariables) {
			return ir.TypeResolution{}, fmt.Errorf("global variable %d out of range", k.Variable)
		}
		gv := &m.GlobalVariables[k.Variable]
		if gv.Space == ir.SpaceHandle {
			return handleRes(gv.Type), nil
		}
		// Rust: GlobalVariable -> Pointer { base: var.ty, space: var.space }.
		return valueRes(ir.PointerType{Base: gv.Type, Space: gv.Space}), nil
	case ir.ExprLocalVariable:
		if int(k.Variable) >= len(r.fn.LocalVars) {
			return ir.TypeResolution{}, fmt.Errorf("local variable %d out of range", k.Variable)
		}
		// Rust: LocalVariable -> Pointer { base: var.ty, space: Function }.
		return valueRes(ir.PointerType{Base: r.fn.LocalVars[k.Variable].Type, Space: ir.SpaceFunction}), nil
	case ir.ExprLoad:
		return r.load(k)
	case ir.ExprAlias:
		// The DXIL mem2reg pass's alias stands for its source.
		return r.operand(k.Source)
	case ir.ExprPhi:
		return r.phi(k)
	case ir.ExprImageSample:
		if k.Gather != nil {
			// A gather is a vec4 even for a depth image (Rust).
			return r.imageGather(k.Image)
		}
		return r.imageResult(k.Image, "image sample")
	case ir.ExprImageLoad:
		return r.imageResult(k.Image, "image load")
	case ir.ExprImageQuery:
		return r.imageQuery(k)
	case ir.ExprUnary:
		return r.operand(k.Expr)
	case ir.ExprBinary:
		return r.binary(k)
	case ir.ExprSelect:
		return r.operand(k.Accept)
	case ir.ExprDerivative:
		return r.operand(k.Expr)
	case ir.ExprRelational:
		return r.relational(k)
	case ir.ExprMath:
		return r.math(k)
	case ir.ExprAs:
		return r.as(k)
	case ir.ExprCallResult:
		if int(k.Function) >= len(m.Functions) {
			return ir.TypeResolution{}, fmt.Errorf("function %d out of range", k.Function)
		}
		result := m.Functions[k.Function].Result
		if result == nil {
			return ir.TypeResolution{}, fmt.Errorf("function has no return type")
		}
		return handleRes(result.Type), nil
	case ir.ExprArrayLength:
		return valueRes(u32Scalar), nil
	case ir.ExprAtomicResult:
		return handleRes(k.Ty), nil
	case ir.ExprSubgroupBallotResult:
		return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: u32Scalar}), nil
	case ir.ExprSubgroupOperationResult:
		return handleRes(k.Type), nil
	case ir.ExprWorkGroupUniformLoadResult:
		return handleRes(k.Ty), nil
	case ir.ExprRayQueryProceedResult:
		return valueRes(boolScalar), nil
	case ir.ExprRayQueryGetIntersection:
		for i := range m.Types {
			if m.Types[i].Name == "RayIntersection" {
				return handleRes(ir.TypeHandle(i)), nil
			}
		}
		return ir.TypeResolution{}, fmt.Errorf("RayIntersection type not found in module")
	default:
		return ir.TypeResolution{}, fmt.Errorf("unsupported expression kind: %T", kind)
	}
}

// access types Access (index nil) and AccessIndex: the element of an array,
// vector, matrix or binding array, the member of a struct, and through a
// pointer a pointer to it.
//
//nolint:gocyclo,cyclop // one case per indexable type, as Rust
func (r *resolver) access(base ir.ExpressionHandle, index *uint32) (ir.TypeResolution, error) {
	baseRes, err := r.operand(base)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(baseRes)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	m := r.module
	switch t := inner.(type) {
	case ir.ArrayType:
		return handleRes(t.Base), nil
	case ir.VectorType:
		return valueRes(t.Scalar), nil
	case ir.MatrixType:
		// A column.
		return valueRes(ir.VectorType{Size: t.Rows, Scalar: t.Scalar}), nil
	case ir.StructType:
		if index == nil {
			return ir.TypeResolution{}, fmt.Errorf("cannot index into type %T", t)
		}
		if int(*index) >= len(t.Members) {
			return ir.TypeResolution{}, fmt.Errorf("struct member index %d out of range", *index)
		}
		return handleRes(t.Members[*index].Type), nil
	case ir.PointerType:
		if int(t.Base) >= len(m.Types) {
			return ir.TypeResolution{}, fmt.Errorf("pointer base type %d out of range", t.Base)
		}
		switch pt := m.Types[t.Base].Inner.(type) {
		case ir.ArrayType:
			return valueRes(ir.PointerType{Base: pt.Base, Space: t.Space}), nil
		case ir.VectorType:
			return valueRes(ir.ValuePointerType{Scalar: pt.Scalar, Space: t.Space}), nil
		case ir.MatrixType:
			rows := pt.Rows
			return valueRes(ir.ValuePointerType{Size: &rows, Scalar: pt.Scalar, Space: t.Space}), nil
		case ir.StructType:
			if index == nil {
				return ir.TypeResolution{}, fmt.Errorf("cannot index through pointer into type %T", pt)
			}
			if int(*index) >= len(pt.Members) {
				return ir.TypeResolution{}, fmt.Errorf("struct member index %d out of range through pointer", *index)
			}
			return valueRes(ir.PointerType{Base: pt.Members[*index].Type, Space: t.Space}), nil
		case ir.BindingArrayType:
			return valueRes(ir.PointerType{Base: pt.Base, Space: t.Space}), nil
		default:
			return ir.TypeResolution{}, fmt.Errorf("cannot index through pointer into type %T", pt)
		}
	case ir.ValuePointerType:
		if t.Size != nil {
			return valueRes(ir.ValuePointerType{Scalar: t.Scalar, Space: t.Space}), nil
		}
		return ir.TypeResolution{}, fmt.Errorf("cannot index into scalar value pointer")
	case ir.BindingArrayType:
		return handleRes(t.Base), nil
	default:
		return ir.TypeResolution{}, fmt.Errorf("cannot index into type %T", t)
	}
}

func (r *resolver) splat(k ir.ExprSplat) (ir.TypeResolution, error) {
	valueType, err := r.operand(k.Value)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(valueType)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	scalar, ok := inner.(ir.ScalarType)
	if !ok {
		return ir.TypeResolution{}, fmt.Errorf("splat value must be scalar, got %T", inner)
	}
	return valueRes(ir.VectorType{Size: k.Size, Scalar: scalar}), nil
}

func (r *resolver) swizzle(k ir.ExprSwizzle) (ir.TypeResolution, error) {
	vectorType, err := r.operand(k.Vector)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(vectorType)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	vec, ok := inner.(ir.VectorType)
	if !ok {
		return ir.TypeResolution{}, fmt.Errorf("swizzle base must be vector, got %T", inner)
	}
	return valueRes(ir.VectorType{Size: k.Size, Scalar: vec.Scalar}), nil
}

func (r *resolver) load(k ir.ExprLoad) (ir.TypeResolution, error) {
	pointerType, err := r.operand(k.Pointer)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(pointerType)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	switch t := inner.(type) {
	case ir.PointerType:
		// A load of an atomic<T> is a T (Rust).
		if int(t.Base) < len(r.module.Types) {
			if at, ok := r.module.Types[t.Base].Inner.(ir.AtomicType); ok {
				return valueRes(at.Scalar), nil
			}
		}
		return handleRes(t.Base), nil
	case ir.ValuePointerType:
		if t.Size != nil {
			return valueRes(ir.VectorType{Size: *t.Size, Scalar: t.Scalar}), nil
		}
		return valueRes(t.Scalar), nil
	case ir.AtomicType:
		return valueRes(t.Scalar), nil
	}
	// A pointer operand that is already a value loads that value.
	return pointerType, nil
}

func (r *resolver) phi(k ir.ExprPhi) (ir.TypeResolution, error) {
	if len(k.Incoming) == 0 {
		return ir.TypeResolution{}, fmt.Errorf("ExprPhi with zero incomings")
	}
	for _, in := range k.Incoming {
		res, err := r.operand(in.Value)
		if errors.Is(err, errPending) {
			continue
		}
		return res, err
	}
	return ir.TypeResolution{}, errPending
}

// imageOperand returns the image type of an image operand.
func (r *resolver) imageOperand(image ir.ExpressionHandle, context string) (ir.ImageType, error) {
	imageType, err := r.operand(image)
	if err != nil {
		return ir.ImageType{}, err
	}
	inner, err := r.inner(imageType)
	if err != nil {
		return ir.ImageType{}, err
	}
	img, ok := inner.(ir.ImageType)
	if !ok {
		return ir.ImageType{}, fmt.Errorf("%s requires image type, got %T", context, inner)
	}
	return img, nil
}

// imageGather is a vec4 of the image's sampled kind, f32 for a depth image.
func (r *resolver) imageGather(image ir.ExpressionHandle) (ir.TypeResolution, error) {
	img, err := r.imageOperand(image, "image gather")
	if err != nil {
		return ir.TypeResolution{}, err
	}
	scalar := f32Scalar
	if img.Class == ir.ImageClassSampled {
		scalar = ir.ScalarType{Kind: img.SampledKind, Width: 4}
	}
	return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: scalar}), nil
}

// imageResult is the texel of an image sample or load: f32 for a depth
// image, else a vec4 of the image's texel kind.
func (r *resolver) imageResult(image ir.ExpressionHandle, context string) (ir.TypeResolution, error) {
	img, err := r.imageOperand(image, context)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	if img.Class == ir.ImageClassDepth {
		return valueRes(f32Scalar), nil
	}
	scalar := f32Scalar
	switch img.Class {
	case ir.ImageClassStorage:
		scalar.Kind = img.StorageFormat.ScalarKind()
		if img.StorageFormat == ir.StorageFormatR64Uint || img.StorageFormat == ir.StorageFormatR64Sint {
			scalar.Width = 8
		}
	case ir.ImageClassSampled:
		// Rust ImageClass::Sampled { kind }.
		scalar.Kind = img.SampledKind
	}
	return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: scalar}), nil
}

func (r *resolver) imageQuery(k ir.ExprImageQuery) (ir.TypeResolution, error) {
	switch k.Query.(type) {
	case ir.ImageQuerySize:
		// The spatial size, without the layer count of an arrayed image
		// (Rust's image_query_size_result_type): u32 in 1D, vec3 in 3D, else
		// vec2.
		imgType, err := r.operand(k.Image)
		if err != nil {
			return ir.TypeResolution{}, err
		}
		dim := ir.Dim2D
		if it, ok := resolutionInner(r.module, imgType).(ir.ImageType); ok {
			dim = it.Dim
		}
		switch dim {
		case ir.Dim1D:
			return valueRes(u32Scalar), nil
		case ir.Dim3D:
			return valueRes(ir.VectorType{Size: ir.Vec3, Scalar: u32Scalar}), nil
		default:
			return valueRes(ir.VectorType{Size: ir.Vec2, Scalar: u32Scalar}), nil
		}
	case ir.ImageQueryNumLevels, ir.ImageQueryNumLayers, ir.ImageQueryNumSamples:
		return valueRes(u32Scalar), nil
	default:
		return ir.TypeResolution{}, fmt.Errorf("unknown image query type: %T", k.Query)
	}
}

// resolutionInner is res's TypeInner, or nil for a handle out of range.
func resolutionInner(m *ir.Module, res ir.TypeResolution) ir.TypeInner {
	if res.Handle != nil {
		if int(*res.Handle) < len(m.Types) {
			return m.Types[*res.Handle].Inner
		}
		return nil
	}
	return res.Value
}

func (r *resolver) binary(k ir.ExprBinary) (ir.TypeResolution, error) {
	left, err := r.operand(k.Left)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	switch k.Op {
	case ir.BinaryEqual, ir.BinaryNotEqual, ir.BinaryLess, ir.BinaryLessEqual, ir.BinaryGreater, ir.BinaryGreaterEqual:
		inner, err := r.inner(left)
		if err != nil {
			return ir.TypeResolution{}, err
		}
		if vec, ok := inner.(ir.VectorType); ok {
			return valueRes(ir.VectorType{Size: vec.Size, Scalar: boolScalar}), nil
		}
		return valueRes(boolScalar), nil
	case ir.BinaryLogicalAnd, ir.BinaryLogicalOr:
		return valueRes(boolScalar), nil
	}
	right, err := r.operand(k.Right)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	leftInner := resolutionInner(r.module, left)
	rightInner := resolutionInner(r.module, right)
	if k.Op == ir.BinaryMultiply {
		return mulResult(left, right, leftInner, rightInner), nil
	}
	// A scalar and a vector operand make a vector (WGSL splats the scalar).
	_, leftIsScalar := leftInner.(ir.ScalarType)
	_, rightIsVec := rightInner.(ir.VectorType)
	if leftIsScalar && rightIsVec {
		return right, nil
	}
	return left, nil
}

// mulResult types a multiplication: scalar*vec is a vec, scalar*mat a mat,
// mat*vec a vec of the rows, vec*mat a vec of the columns and matCxR *
// matKxC a matKxR (WGSL).
func mulResult(left, right ir.TypeResolution, leftInner, rightInner ir.TypeInner) ir.TypeResolution {
	_, leftIsScalar := leftInner.(ir.ScalarType)
	_, rightIsScalar := rightInner.(ir.ScalarType)
	_, leftIsVec := leftInner.(ir.VectorType)
	_, rightIsVec := rightInner.(ir.VectorType)
	leftMat, leftIsMat := leftInner.(ir.MatrixType)
	rightMat, rightIsMat := rightInner.(ir.MatrixType)
	switch {
	case leftIsScalar && (rightIsVec || rightIsMat):
		return right
	case (leftIsVec || leftIsMat) && rightIsScalar:
		return left
	case leftIsMat && rightIsVec:
		return valueRes(ir.VectorType{Size: leftMat.Rows, Scalar: leftMat.Scalar})
	case leftIsVec && rightIsMat:
		return valueRes(ir.VectorType{Size: rightMat.Columns, Scalar: rightMat.Scalar})
	case leftIsMat && rightIsMat:
		return valueRes(ir.MatrixType{Columns: rightMat.Columns, Rows: leftMat.Rows, Scalar: leftMat.Scalar})
	default:
		return left
	}
}

func (r *resolver) relational(k ir.ExprRelational) (ir.TypeResolution, error) {
	arg, err := r.operand(k.Argument)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(arg)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	// isnan and isinf keep a vector's size; all and any make one bool.
	if vec, ok := inner.(ir.VectorType); ok && (k.Fun == ir.RelationalIsNan || k.Fun == ir.RelationalIsInf) {
		return valueRes(ir.VectorType{Size: vec.Size, Scalar: boolScalar}), nil
	}
	return valueRes(boolScalar), nil
}

//nolint:gocyclo,cyclop // one case per math function family, as Rust
func (r *resolver) math(k ir.ExprMath) (ir.TypeResolution, error) {
	arg, err := r.operand(k.Arg)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	m := r.module
	switch k.Fun {
	case ir.MathDot4I8Packed:
		return valueRes(i32Scalar), nil
	case ir.MathDot4U8Packed:
		return valueRes(u32Scalar), nil
	case ir.MathDot:
		inner, err := r.inner(arg)
		if err != nil {
			return ir.TypeResolution{}, err
		}
		if vec, ok := inner.(ir.VectorType); ok {
			return valueRes(vec.Scalar), nil
		}
		return arg, nil
	case ir.MathLength, ir.MathDistance:
		switch t := resolutionInner(m, arg).(type) {
		case ir.VectorType:
			return valueRes(t.Scalar), nil
		case ir.ScalarType:
			return valueRes(t), nil
		}
		return arg, nil
	case ir.MathDeterminant:
		if mt, ok := resolutionInner(m, arg).(ir.MatrixType); ok {
			return valueRes(mt.Scalar), nil
		}
		return arg, nil
	case ir.MathTranspose:
		if mt, ok := resolutionInner(m, arg).(ir.MatrixType); ok {
			return valueRes(ir.MatrixType{Columns: mt.Rows, Rows: mt.Columns, Scalar: mt.Scalar}), nil
		}
		return arg, nil
	case ir.MathOuter:
		// outer(a: vecR, b: vecC) is matCxR.
		a, aok := resolutionInner(m, arg).(ir.VectorType)
		if aok && k.Arg1 != nil {
			bRes, err := r.operand(*k.Arg1)
			if err != nil {
				return ir.TypeResolution{}, err
			}
			if b, ok := resolutionInner(m, bRes).(ir.VectorType); ok {
				return valueRes(ir.MatrixType{Columns: b.Size, Rows: a.Size, Scalar: a.Scalar}), nil
			}
		}
		return arg, nil
	case ir.MathUnpack4xI8:
		return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: i32Scalar}), nil
	case ir.MathUnpack4xU8:
		return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: u32Scalar}), nil
	case ir.MathPack4xI8, ir.MathPack4xU8, ir.MathPack4xI8Clamp, ir.MathPack4xU8Clamp,
		ir.MathPack4x8snorm, ir.MathPack4x8unorm,
		ir.MathPack2x16snorm, ir.MathPack2x16unorm, ir.MathPack2x16float:
		return valueRes(u32Scalar), nil
	case ir.MathUnpack4x8snorm, ir.MathUnpack4x8unorm:
		return valueRes(ir.VectorType{Size: ir.Vec4, Scalar: f32Scalar}), nil
	case ir.MathUnpack2x16snorm, ir.MathUnpack2x16unorm, ir.MathUnpack2x16float:
		return valueRes(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar}), nil
	case ir.MathModf:
		// The predeclared __modf_result_* struct of the argument's type.
		if h, ok := findNamedType(m, resultStructName(m, "__modf_result_", arg)); ok {
			return handleRes(h), nil
		}
		return arg, nil
	case ir.MathFrexp:
		if h, ok := findNamedType(m, resultStructName(m, "__frexp_result_", arg)); ok {
			return handleRes(h), nil
		}
		return arg, nil
	default:
		// The other functions, the bit counts among them, keep the
		// argument's type.
		return arg, nil
	}
}

// resultStructName is the name of the modf or frexp result struct for an
// argument of type arg: prefix, then vecN_ and the float width for a
// vector, f32 for anything else.
func resultStructName(m *ir.Module, prefix string, arg ir.TypeResolution) string {
	t, ok := resolutionInner(m, arg).(ir.VectorType)
	if !ok {
		return prefix + "f32"
	}
	size := ""
	switch t.Size {
	case ir.Vec2:
		size = "vec2"
	case ir.Vec3:
		size = "vec3"
	case ir.Vec4:
		size = "vec4"
	}
	width := "f32"
	switch t.Scalar.Width {
	case 8:
		width = "f64"
	case 2:
		width = "f16"
	}
	return prefix + size + "_" + width
}

// findNamedType returns the first type of m named name.
func findNamedType(m *ir.Module, name string) (ir.TypeHandle, bool) {
	for i := range m.Types {
		if m.Types[i].Name == name {
			return ir.TypeHandle(i), true
		}
	}
	return 0, false
}

func (r *resolver) as(k ir.ExprAs) (ir.TypeResolution, error) {
	exprType, err := r.operand(k.Expr)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	inner, err := r.inner(exprType)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	if k.Convert != nil {
		target := ir.ScalarType{Kind: k.Kind, Width: *k.Convert}
		switch t := inner.(type) {
		case ir.VectorType:
			return valueRes(ir.VectorType{Size: t.Size, Scalar: target}), nil
		case ir.MatrixType:
			return valueRes(ir.MatrixType{Columns: t.Columns, Rows: t.Rows, Scalar: target}), nil
		}
		return valueRes(target), nil
	}
	// A bitcast keeps the width and the vector size.
	switch t := inner.(type) {
	case ir.ScalarType:
		return valueRes(ir.ScalarType{Kind: k.Kind, Width: t.Width}), nil
	case ir.VectorType:
		return valueRes(ir.VectorType{Size: t.Size, Scalar: ir.ScalarType{Kind: k.Kind, Width: t.Scalar.Width}}), nil
	}
	return exprType, nil
}
