package front

// This file implements GLSL's atomic memory functions: the atomicAdd family on
// shared variables and buffer-block members (GLSL 4.60 §8.11) and the
// imageAtomic* family on storage images (GLSL 4.60 §8.12). Rust naga's GLSL
// frontend has neither, so glslangValidator is the reference for the SPIR-V
// shape and the semantics, and the IR itself comes from the WGSL and SPIR-V
// paths: ir.StmtAtomic for a variable, ir.StmtImageAtomic for a texel.
//
// The two families cannot go through the generic builtin machinery: a GLSL
// atomic takes an lvalue, and lowering that lvalue in an Rhs position (which is
// how the overload comparison sees an argument) would emit a load of a variable
// that is about to become atomic — a spurious atomic read in every backend.
// The atomic call path therefore lowers its first argument in Lhs position and
// matches the rest by hand; atomicBuiltinFunction is the catalog both the path
// and the snapshot matrix's coverage test read.

import (
	"sort"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// atomicBuiltin is one GLSL atomic function: the IR operation, which family it
// belongs to, and whether it takes a compare operand as well as a data one.
// ir.AtomicExchange carries its compare operand, so the table cannot express
// the two forms by the IR value alone.
type atomicBuiltin struct {
	fun ir.AtomicFunction
	// image marks the imageAtomic* family, which works on a texel.
	image bool
	// compare marks atomicCompSwap and imageAtomicCompSwap, whose signature is
	// (lvalue or image, coordinate?, compare, data).
	compare bool
}

// atomicBuiltinFunction returns the atomic builtin name denotes. The switch is
// the catalog: the snapshot matrix's coverage test reads it out of this source
// (glsl_atomics_matrix_test.go), so a name added here fails the test until a
// row drives it.
func atomicBuiltinFunction(name string) (atomicBuiltin, bool) {
	switch name {
	case "atomicAdd":
		return atomicBuiltin{fun: ir.AtomicAdd{}}, true
	case "atomicMin":
		return atomicBuiltin{fun: ir.AtomicMin{}}, true
	case "atomicMax":
		return atomicBuiltin{fun: ir.AtomicMax{}}, true
	case "atomicAnd":
		return atomicBuiltin{fun: ir.AtomicAnd{}}, true
	case "atomicOr":
		return atomicBuiltin{fun: ir.AtomicInclusiveOr{}}, true
	case "atomicXor":
		return atomicBuiltin{fun: ir.AtomicExclusiveOr{}}, true
	case "atomicExchange":
		return atomicBuiltin{fun: ir.AtomicExchange{}}, true
	case "atomicCompSwap":
		return atomicBuiltin{fun: ir.AtomicExchange{}, compare: true}, true

	case "imageAtomicAdd":
		return atomicBuiltin{fun: ir.AtomicAdd{}, image: true}, true
	case "imageAtomicMin":
		return atomicBuiltin{fun: ir.AtomicMin{}, image: true}, true
	case "imageAtomicMax":
		return atomicBuiltin{fun: ir.AtomicMax{}, image: true}, true
	case "imageAtomicAnd":
		return atomicBuiltin{fun: ir.AtomicAnd{}, image: true}, true
	case "imageAtomicOr":
		return atomicBuiltin{fun: ir.AtomicInclusiveOr{}, image: true}, true
	case "imageAtomicXor":
		return atomicBuiltin{fun: ir.AtomicExclusiveOr{}, image: true}, true
	case "imageAtomicExchange":
		return atomicBuiltin{fun: ir.AtomicExchange{}, image: true}, true
	case "imageAtomicCompSwap":
		return atomicBuiltin{fun: ir.AtomicExchange{}, image: true, compare: true}, true
	}
	return atomicBuiltin{}, false
}

// glslAtomicLValueText is glslang's diagnostic for an atomic operand that is
// not a shared variable or a buffer-block member (verified with
// glslangValidator 11:16.2.0: `uint c = 0u; atomicAdd(c, 1u);` and an atomic on
// a uniform-block member both print it). GLSL 4.60 §8.11 restricts every
// atomic memory function to those two storage classes.
const glslAtomicLValueText = "Only l-values corresponding to shader block storage or shared variables " +
	"can be used with atomic memory functions"

// glslAtomicImageFormatText is glslang's diagnostic for an image atomic on an
// image whose format is not r32i or r32ui (verified with glslangValidator: an
// rgba8ui image prints "only supported on image with format r32i or r32ui").
const glslAtomicImageFormatText = "only supported on image with format r32i or r32ui"

// atomicVectorComponentText refuses an atomic on a vector component. glslang
// accepts atomicAdd(v.x, 1u) on a shared uvec4 (verified), but ir.StmtAtomic's
// pointer must point to an ir.AtomicType and a vector has no atomic element
// type, so the IR cannot express the operand without a new type.
const atomicVectorComponentText = "a vector component is not a valid atomic operand: " +
	"ir.StmtAtomic requires a pointer to an atomic scalar, and a vector has no atomic element type " +
	"(card P1-GLSLIN-015)"

// atomicCounterText refuses the atomic-counter functions and the atomic_uint
// type: the IR has no atomic-counter resource class, so the frontend cannot
// represent them without inventing a model (card P1-GLSLIN-015). Vulkan GLSL
// has no atomic counters at all — glslang refuses atomic_uint under #version
// 450 — which is why the memoryBarrierAtomicCounter refusal already names the
// same gap.
const atomicCounterText = "atomic counters are not implemented: the IR has no atomic-counter resource class, " +
	"so `layout(binding) uniform atomic_uint` and its atomicCounter* functions cannot be represented " +
	"(card P1-GLSLIN-015)"

// atomicCounterFunction reports whether name is one of the GLSL atomic-counter
// functions (GLSL 4.60 §8.10).
func atomicCounterFunction(name string) bool {
	switch name {
	case "atomicCounter", "atomicCounterIncrement", "atomicCounterDecrement",
		"atomicCounterAdd", "atomicCounterSubtract", "atomicCounterMin", "atomicCounterMax",
		"atomicCounterAnd", "atomicCounterOr", "atomicCounterXor",
		"atomicCounterExchange", "atomicCounterCompSwap":
		return true
	}
	return false
}

// unsupportedTypeText is the message an unknown type name gets when the type is
// one the frontend deliberately does not model, replacing Rust's bare
// "Unknown type" text. atomic_uint is the only such type today.
func unsupportedTypeText(name string) string {
	if name == "atomic_uint" {
		return atomicCounterText
	}
	return ""
}

// atomicCall lowers a call of an atomic builtin. The dialect gate runs first,
// as the generic call path checks builtinRefusal before overload resolution: a
// GL 3.30 or ES 3.00 source meets the version rule rather than a type or lvalue
// complaint.
func (f *Frontend) atomicCall(c *Context, e *hirArena, name string, b atomicBuiltin, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	if reason := f.dialect.builtinRefusal(name); reason != "" {
		return 0, false, errMessage(span, reason)
	}
	if b.image {
		return f.imageAtomicCall(c, e, name, b, rawArgs, span)
	}
	return f.memoryAtomicCall(c, e, name, b, rawArgs, span)
}

// memoryAtomicCall lowers an atomic memory function on a shared variable or a
// buffer-block member. The result is the expression holding the value the
// operation replaced, ir.ExprAtomicResult; a compare-swap yields the struct's
// old_value member, which surfaces only that value (GLSL's atomicCompSwap
// returns the old value alone).
func (f *Frontend) memoryAtomicCall(c *Context, e *hirArena, name string, b atomicBuiltin, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	want := 2
	if b.compare {
		want = 3
	}
	if len(rawArgs) != want {
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}

	pointer, pointerSpan, err := c.lowerExpect(e, rawArgs[0], lhsPos)
	if err != nil {
		return 0, false, err
	}
	scalar, err := f.atomicTargetScalar(c, pointer, pointerSpan, name)
	if err != nil {
		return 0, false, err
	}

	var compare, value ir.ExpressionHandle
	if b.compare {
		// atomicCompSwap(mem, compare, data).
		cmpRaw, cmpSpan, cerr := c.lowerExpect(e, rawArgs[1], rhsPos)
		if cerr != nil {
			return 0, false, cerr
		}
		if compare, cerr = atomicOperand(c, name, cmpRaw, cmpSpan, scalar); cerr != nil {
			return 0, false, cerr
		}
		valRaw, valSpan, verr := c.lowerExpect(e, rawArgs[2], rhsPos)
		if verr != nil {
			return 0, false, verr
		}
		if value, verr = atomicOperand(c, name, valRaw, valSpan, scalar); verr != nil {
			return 0, false, verr
		}
	} else {
		valRaw, valSpan, verr := c.lowerExpect(e, rawArgs[1], rhsPos)
		if verr != nil {
			return 0, false, verr
		}
		if value, verr = atomicOperand(c, name, valRaw, valSpan, scalar); verr != nil {
			return 0, false, verr
		}
	}

	// The value's expressions belong to the range before the statement; the
	// atomic result is produced by the statement and sits outside every range,
	// as Rust naga's interrupt_emitter arranges it.
	c.emitEnd()
	if b.compare {
		result, rerr := c.addExpression(ir.ExprAtomicResult{
			Ty:         f.atomicCompareExchangeResultType(scalar),
			Comparison: true,
		}, span)
		if rerr != nil {
			return 0, false, rerr
		}
		c.body = append(c.body, ir.Statement{Kind: ir.StmtAtomic{
			Pointer: pointer,
			Fun:     ir.AtomicExchange{Compare: exprPtr(compare)},
			Value:   value,
			Result:  &result,
		}})
		c.emitStart()
		old, oerr := c.addExpression(ir.ExprAccessIndex{Base: result, Index: 0}, span)
		return old, true, oerr
	}

	result, rerr := c.addExpression(ir.ExprAtomicResult{Ty: f.registerType(scalar)}, span)
	if rerr != nil {
		return 0, false, rerr
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtAtomic{
		Pointer: pointer,
		Fun:     b.fun,
		Value:   value,
		Result:  &result,
	}})
	c.emitStart()
	return result, true, nil
}

// imageAtomicCall lowers an imageAtomic* function on a storage image. The call
// is a statement: ir.StmtImageAtomic has no result expression, so the old value
// GLSL's image atomic returns cannot be represented — using it is refused in
// lowerExpect with statementOnlyAtomicText.
func (f *Frontend) imageAtomicCall(c *Context, e *hirArena, name string, b atomicBuiltin, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	want := 3
	if b.compare {
		want = 4
	}
	if len(rawArgs) != want {
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}

	image, imageSpan, err := c.lowerExpect(e, rawArgs[0], rhsPos)
	if err != nil {
		return 0, false, err
	}
	img, ok, ierr := f.storageImage(c, image, imageSpan)
	if ierr != nil {
		return 0, false, ierr
	}
	if !ok {
		// A sampled texture, or a value that is not an image at all: glslang's
		// "no matching overloaded function found".
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}
	scalar, ok := atomicImageScalar(img.StorageFormat)
	if !ok {
		return 0, false, errSemantic(span, "'%s' : %s", name, glslAtomicImageFormatText)
	}

	coord, coordSpan, err := c.lowerExpect(e, rawArgs[1], rhsPos)
	if err != nil {
		return 0, false, err
	}
	comps, err := f.coordinateComponents(c, image, coord, nil, coordSpan)
	if err != nil {
		return 0, false, err
	}

	var compare, value ir.ExpressionHandle
	if b.compare {
		// imageAtomicCompSwap(image, P, compare, data). The compare operand is
		// part of the operation: ir.AtomicExchange carries it, and dropping it
		// turns the compare-and-swap into an unconditional exchange in every
		// backend without any validator noticing.
		cmpRaw, cmpSpan, cerr := c.lowerExpect(e, rawArgs[2], rhsPos)
		if cerr != nil {
			return 0, false, cerr
		}
		if compare, cerr = atomicOperand(c, name, cmpRaw, cmpSpan, scalar); cerr != nil {
			return 0, false, cerr
		}
		valRaw, valSpan, verr := c.lowerExpect(e, rawArgs[3], rhsPos)
		if verr != nil {
			return 0, false, verr
		}
		if value, verr = atomicOperand(c, name, valRaw, valSpan, scalar); verr != nil {
			return 0, false, verr
		}
	} else {
		valRaw, valSpan, verr := c.lowerExpect(e, rawArgs[2], rhsPos)
		if verr != nil {
			return 0, false, verr
		}
		if value, verr = atomicOperand(c, name, valRaw, valSpan, scalar); verr != nil {
			return 0, false, verr
		}
	}

	c.emitRestart()
	fun := b.fun
	if b.compare {
		fun = ir.AtomicExchange{Compare: exprPtr(compare)}
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtImageAtomic{
		Image:      image,
		Coordinate: comps.coordinate,
		ArrayIndex: comps.arrayIndex,
		Fun:        fun,
		Value:      value,
	}})
	return 0, false, nil
}

// statementOnlyAtomicText explains why an image atomic call that yielded no
// value cannot be used as one. Without this the author would read the generic
// "Expression returns void", which hides the actual limitation of the IR. A
// function the shader itself declares — a definition or a prototype — is never
// the builtin, as in functionOrConstructorCall.
func (f *Frontend) statementOnlyAtomicText(node *hirNode) string {
	if node == nil {
		return ""
	}
	call, ok := node.Kind.(hirCall)
	if !ok || call.Callee.Kind != calleeFunction {
		return ""
	}
	b, ok := atomicBuiltinFunction(call.Callee.Name)
	if !ok || !b.image || declaredFunction(f.functions[call.Callee.Name]) {
		return ""
	}
	return "'" + call.Callee.Name + "' : the old value is not available: ir.StmtImageAtomic carries " +
		"no result expression, so an image atomic is a statement (card P1-GLSLIN-015)"
}

// storageImage resolves the type of an image atomic's first argument. ok is
// false when the argument is not a storage image, which the caller reports as
// an unknown function, as glslang does.
func (f *Frontend) storageImage(c *Context, image ir.ExpressionHandle, span ir.Span) (ir.ImageType, bool, *frontError) {
	inner, err := c.resolveType(image, span)
	if err != nil {
		return ir.ImageType{}, false, err
	}
	img, ok := inner.(ir.ImageType)
	if !ok || img.Class != ir.ImageClassStorage {
		return ir.ImageType{}, false, nil
	}
	return img, true, nil
}

// atomicImageScalar returns the scalar type an image atomic's value has, from
// the image's storage format: r32ui holds uint, r32i int. Every other format is
// refused, as glslang refuses it (GLSL 4.60 §8.12 requires an integer image
// whose format is r32i or r32ui).
func atomicImageScalar(format ir.StorageFormat) (ir.ScalarType, bool) {
	switch format {
	case ir.StorageFormatR32Uint:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, true
	case ir.StorageFormatR32Sint:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, true
	}
	return ir.ScalarType{}, false
}

// atomicOperand converts one value operand of an atomic call to the atomic's
// scalar type and checks that it has it. GLSL's implicit conversion applies
// (glslang accepts atomicAdd(uint_mem, 1) but not atomicAdd(int_mem, 1u)), and
// anything else is the call's "no matching overload".
func atomicOperand(c *Context, name string, h ir.ExpressionHandle, span ir.Span, scalar ir.ScalarType) (ir.ExpressionHandle, *frontError) {
	converted, err := c.implicitConversion(h, span, scalar)
	if err != nil {
		return 0, err
	}
	ty, err := c.resolveType(converted, span)
	if err != nil {
		return 0, err
	}
	if !typeInnerEqual(ty, ir.TypeInner(scalar)) {
		return 0, errSemantic(span, "Unknown function '%s'", name)
	}
	return converted, nil
}

// atomicTargetScalar records the atomic access pointer makes and returns the
// scalar type of the variable member it addresses. The pointee is checked
// first: a value that is not a 32-bit int or uint is the call's "no matching
// overload", as glslang reports it (an image, an aggregate, a 16- or 64-bit
// integer). The walk then enforces GLSL's storage rule, and a vector component
// — a valid glslang operand the IR cannot express — gets its own message.
func (f *Frontend) atomicTargetScalar(c *Context, pointer ir.ExpressionHandle, span ir.Span, name string) (ir.ScalarType, *frontError) {
	pointerTy, err := c.resolveType(pointer, span)
	if err != nil {
		return ir.ScalarType{}, err
	}
	var scalar ir.ScalarType
	switch ptr := pointerTy.(type) {
	case ir.PointerType:
		s, ok := f.typeInner(ptr.Base).(ir.ScalarType)
		if !ok {
			return ir.ScalarType{}, errSemantic(span, "Unknown function '%s'", name)
		}
		scalar = s
	case ir.ValuePointerType:
		// A vector or matrix component: a value pointer rather than a typed
		// pointer. The walk below explains why it cannot be an atomic operand.
		scalar = ptr.Scalar
	default:
		return ir.ScalarType{}, errSemantic(span, "Unknown function '%s'", name)
	}
	if scalar.Width != 4 || (scalar.Kind != ir.ScalarSint && scalar.Kind != ir.ScalarUint) {
		return ir.ScalarType{}, errSemantic(span, "Unknown function '%s'", name)
	}
	if _, rerr := f.recordAtomicAccess(c, pointer, span); rerr != nil {
		return ir.ScalarType{}, rerr
	}
	return scalar, nil
}

// atomicAccess records what a module accesses atomically: the globals, and the
// struct fields of those globals' types, that have to become Atomic. It is the
// GLSL frontend's counterpart of the SPIR-V frontend's atomic_upgrade::Upgrades
// (spirv/internal/front/atomic.go): a GLSL variable is declared int or uint, and
// the IR's atomic statements address a pointer to ir.AtomicType, so the accessed
// types are upgraded once every function is lowered.
type atomicAccess struct {
	globals map[ir.GlobalVariableHandle]ir.Span
	fields  map[ir.TypeHandle]map[uint32]struct{}
}

// recordGlobal marks a global variable as atomically accessed, keeping the span
// of the access that asked for the upgrade so a later refusal can point at it.
func (a *atomicAccess) recordGlobal(handle ir.GlobalVariableHandle, span ir.Span) {
	if _, seen := a.globals[handle]; !seen {
		a.globals[handle] = span
	}
}

// recordField marks one member of a struct type as atomically accessed.
func (a *atomicAccess) recordField(handle ir.TypeHandle, index uint32) {
	fields, ok := a.fields[handle]
	if !ok {
		fields = make(map[uint32]struct{})
		a.fields[handle] = fields
	}
	fields[index] = struct{}{}
}

// recordAtomicAccess walks the pointer expression down to the module-scope
// variable it addresses, recording every variable and struct member on the way,
// and returns the type of the value the expression addresses. The walk is the
// SPIR-V frontend's record_atomic_access with GLSL's storage rule: only a shared
// variable or a buffer-block member is a valid atomic operand.
func (f *Frontend) recordAtomicAccess(c *Context, handle ir.ExpressionHandle, span ir.Span) (ir.TypeHandle, *frontError) {
	switch kind := c.Expression(handle).(type) {
	case ir.ExprAccess:
		ty, err := f.recordAtomicAccess(c, kind.Base, span)
		if err != nil {
			return 0, err
		}
		array, ok := f.typeInner(ty).(ir.ArrayType)
		if !ok {
			return 0, errMessage(span, glslAtomicLValueText)
		}
		return array.Base, nil
	case ir.ExprAccessIndex:
		ty, err := f.recordAtomicAccess(c, kind.Base, span)
		if err != nil {
			return 0, err
		}
		switch t := f.typeInner(ty).(type) {
		case ir.StructType:
			if int(kind.Index) >= len(t.Members) {
				return 0, errInternal("atomic access index is outside its struct", span)
			}
			f.recordAtomicField(ty, kind.Index)
			return t.Members[kind.Index].Type, nil
		case ir.ArrayType:
			return t.Base, nil
		case ir.VectorType:
			// glslang accepts atomicAdd(v.x, 1u) on a shared uvec4, but the IR
			// has no atomic vector component: ir.StmtAtomic's pointer must point
			// to an ir.AtomicType, and a vector has no atomic element type.
			return 0, errMessage(span, atomicVectorComponentText)
		}
		return 0, errMessage(span, glslAtomicLValueText)
	case ir.ExprGlobalVariable:
		if int(kind.Variable) >= len(f.mod.GlobalVariables) {
			return 0, errInternal("atomic access names a global variable that does not exist", span)
		}
		global := f.mod.GlobalVariables[kind.Variable]
		if global.Space != ir.SpaceStorage && global.Space != ir.SpaceWorkGroup {
			return 0, errMessage(span, glslAtomicLValueText)
		}
		if f.atomicUpgrades.globals == nil {
			f.atomicUpgrades.globals = map[ir.GlobalVariableHandle]ir.Span{}
		}
		f.atomicUpgrades.recordGlobal(kind.Variable, span)
		return global.Type, nil
	}
	return 0, errMessage(span, glslAtomicLValueText)
}

// recordAtomicField marks one struct member, creating the field map on first
// use.
func (f *Frontend) recordAtomicField(handle ir.TypeHandle, index uint32) {
	if f.atomicUpgrades.fields == nil {
		f.atomicUpgrades.fields = map[ir.TypeHandle]map[uint32]struct{}{}
	}
	f.atomicUpgrades.recordField(handle, index)
}

// upgradeAtomics rewrites the types of the globals the module accesses
// atomically: every scalar leaf on the path of an atomic access becomes an
// atomic, the IR's counterpart of Rust naga's Module::upgrade_atomics
// (spirv/internal/front/atomic.go). It runs once every function is lowered, so
// every function sees the same upgraded types.
func (f *Frontend) upgradeAtomics() *frontError {
	if len(f.atomicUpgrades.globals) == 0 {
		return nil
	}
	handles := make([]ir.GlobalVariableHandle, 0, len(f.atomicUpgrades.globals))
	for handle := range f.atomicUpgrades.globals {
		handles = append(handles, handle)
	}
	sort.Slice(handles, func(i, j int) bool { return handles[i] < handles[j] })

	upgraded := make(map[ir.TypeHandle]ir.TypeHandle)
	for _, handle := range handles {
		if int(handle) >= len(f.mod.GlobalVariables) {
			continue
		}
		global := &f.mod.GlobalVariables[handle]
		if global.InitExpr != nil {
			return errMessage(f.atomicUpgrades.globals[handle],
				"the variable has an initializer, and it is used with atomic memory functions: "+
					"an atomic variable cannot have an initializer (shared and buffer-block variables "+
					"take none in GLSL)")
		}
		newType, err := f.upgradeAtomicType(global.Type, upgraded)
		if err != nil {
			return err
		}
		global.Type = newType
	}
	return nil
}

// upgradeAtomicType returns the type equivalent to ty with its scalar leaves
// upgraded where the module accesses them atomically.
func (f *Frontend) upgradeAtomicType(ty ir.TypeHandle, upgraded map[ir.TypeHandle]ir.TypeHandle) (ir.TypeHandle, *frontError) {
	if newType, ok := upgraded[ty]; ok {
		return newType, nil
	}
	if int(ty) >= len(f.mod.Types) {
		return 0, errInternal("atomic upgrade of a type that does not exist", ir.Span{})
	}
	original := f.mod.Types[ty]
	var inner ir.TypeInner
	switch t := original.Inner.(type) {
	case ir.ScalarType:
		inner = ir.AtomicType{Scalar: t}
	case ir.ArrayType:
		base, err := f.upgradeAtomicType(t.Base, upgraded)
		if err != nil {
			return 0, err
		}
		t.Base = base
		inner = t
	case ir.StructType:
		fields, ok := f.atomicUpgrades.fields[ty]
		if !ok {
			// No member is accessed atomically; nothing to rewrite.
			return ty, nil
		}
		members := append([]ir.StructMember(nil), t.Members...)
		for index := range fields {
			if int(index) >= len(members) {
				return 0, errInternal("atomic upgrade of a struct member that does not exist", ir.Span{})
			}
			base, err := f.upgradeAtomicType(members[index].Type, upgraded)
			if err != nil {
				return 0, err
			}
			members[index].Type = base
		}
		t.Members = members
		inner = t
	case ir.BindingArrayType:
		base, err := f.upgradeAtomicType(t.Base, upgraded)
		if err != nil {
			return 0, err
		}
		t.Base = base
		inner = t
	default:
		return ty, nil
	}
	newType := f.registry.GetOrCreateNamed(original.Name, original.NamePresent, inner)
	f.mod.Types = f.registry.GetTypes()
	upgraded[ty] = newType
	return newType, nil
}

// atomicCompareExchangeResultType returns the predeclared
// __atomic_compare_exchange_result<T> struct type for a scalar, as the WGSL and
// SPIR-V paths build it: the value the operation replaced, then whether it
// exchanged. GLSL's atomicCompSwap surfaces only the first member.
func (f *Frontend) atomicCompareExchangeResultType(scalar ir.ScalarType) ir.TypeHandle {
	kindName := "Float"
	switch scalar.Kind {
	case ir.ScalarUint:
		kindName = "Uint"
	case ir.ScalarSint:
		kindName = "Sint"
	}
	name := "__atomic_compare_exchange_result<" + kindName + "," + strconv.Itoa(int(scalar.Width)) + ">"
	for index, ty := range f.mod.Types {
		if ty.Name == name {
			return ir.TypeHandle(index)
		}
	}
	boolType := f.registerType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	scalarType := f.registerType(scalar)
	return f.registerNamedType(name, ir.StructType{
		Members: []ir.StructMember{
			{Name: "old_value", Type: scalarType, Offset: 0},
			{Name: "exchanged", Type: boolType, Offset: uint32(scalar.Width)},
		},
		Span: uint32(scalar.Width) * 2,
	})
}
