package front

// This file is Rust naga's front/glsl/builtins.rs: the lazily injected builtin
// overloads and the expansion of a builtin call into IR expressions and
// statements.
//
// A builtin is not a module function in Rust naga, and not one here: an
// overload carries a macroCall, and lowering a call to it runs the expansion
// against the call's already lowered arguments. The overloads are injected on
// first use, name by name, because the variations that would request a
// capability (double, cube-array textures, 2D multisampled arrays) must not be
// requested by a shader that never uses them (builtins.rs:74-100).

import "github.com/gogpu/naga/ir"

// The scalar types the catalog is built from. Rust spells them as constants
// (Scalar::F32, Scalar::I32, Scalar::U32, Scalar::BOOL, Scalar::F64).
var (
	scalarF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	scalarF64  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	scalarI32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	scalarU32  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	scalarBool = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
)

// sizePtr returns a pointer to size, for the optional vector sizes the
// expansions carry (Rust's Option<VectorSize>).
func sizePtr(size ir.VectorSize) *ir.VectorSize { return &size }

// sizeOfBits maps a two-bit dimension code to a vector size: Rust's
// `match bits { 0b00 => None, 0b01 => Bi, 0b10 => Tri, _ => Quad }`, which
// most of the catalog uses to enumerate the scalar and vector variants.
func sizeOfBits(bits int) *ir.VectorSize {
	switch bits {
	case 0b00:
		return nil
	case 0b01:
		return sizePtr(ir.Vec2)
	case 0b10:
		return sizePtr(ir.Vec3)
	default:
		return sizePtr(ir.Vec4)
	}
}

// vectorOf returns the type of size over scalar: Rust's
// `match size { Some(size) => TypeInner::Vector { size, scalar }, None =>
// TypeInner::Scalar(scalar) }`.
func vectorOf(size *ir.VectorSize, scalar ir.ScalarType) ir.TypeInner {
	if size == nil {
		return scalar
	}
	return ir.VectorType{Size: *size, Scalar: scalar}
}

// matrixOf returns the matrix type of rows by columns over scalar. Rust spells
// the fields columns then rows, so callers keep the two apart (builtins.rs:1318-1326).
func matrixOf(rows, columns ir.VectorSize, scalar ir.ScalarType) ir.TypeInner {
	return ir.MatrixType{Columns: columns, Rows: rows, Scalar: scalar}
}

// isFloat64 reports whether s is the 64-bit float type, which is what asks for
// the double capability (builtins.rs:1601-1604).
func isFloat64(s ir.ScalarType) bool { return s.Kind == ir.ScalarFloat && s.Width == 8 }

// dummyStorageFormat is the storage format an image builtin's image type
// carries: the parameter only matches on the format's scalar kind, and the
// declaration's own layout format replaces it (Rust's
// ScalarKind::dummy_storage_format, builtins.rs:17-25).
func dummyStorageFormat(kind ir.ScalarKind) ir.StorageFormat {
	switch kind {
	case ir.ScalarSint:
		return ir.StorageFormatR16Sint
	case ir.ScalarUint:
		return ir.StorageFormatR16Uint
	default:
		return ir.StorageFormatR16Float
	}
}

// makeCoordsArg returns the coordinate or offset argument type of a texture
// builtin: one component is a scalar, more make a vector (builtins.rs:58-72).
func makeCoordsArg(components int, kind ir.ScalarKind) ir.TypeInner {
	s := ir.ScalarType{Kind: kind, Width: 4}
	if components == 1 {
		return s
	}
	switch components {
	case 2:
		return ir.VectorType{Size: ir.Vec2, Scalar: s}
	case 3:
		return ir.VectorType{Size: ir.Vec3, Scalar: s}
	default:
		return ir.VectorType{Size: ir.Vec4, Scalar: s}
	}
}

// imageDimsToCoordsSize is the number of coordinate components an image
// dimension needs, written 1 for a scalar (builtins.rs:2367-2375).
func imageDimsToCoordsSize(dim ir.ImageDimension) int {
	switch dim {
	case ir.Dim1D:
		return 1
	case ir.Dim2D:
		return 2
	default:
		return 3
	}
}

// macroKind selects which expansion a macroCall runs. Rust makes MacroCall an
// enum with a payload per variant (builtins.rs:1543-1583); Go has no sum types,
// so the kind picks the arm and the struct carries every payload field.
type macroKind uint8

const (
	macroSampler macroKind = iota
	macroSamplerShadow
	macroTexture
	macroTextureSize
	macroTextureQueryLevels
	macroImageLoad
	macroImageStore
	macroMath
	macroFindLsbUint
	macroFindMsbUint
	macroBitfieldExtract
	macroBitfieldInsert
	macroRelational
	macroUnary
	macroBinary
	macroMod
	macroSplatted
	macroMixBoolean
	macroClamp
	macroBitCast
	macroDerivate
	macroBarrier
	macroSmoothStep
	// macroMemoryBarrier is a memory barrier (OpMemoryBarrier); macroBarrier is
	// the control barrier with execution sync (OpControlBarrier).
	macroMemoryBarrier
	// macroTextureGather is the textureGather family, which is a sampling
	// operation with the IR's Gather component set.
	macroTextureGather
)

// textureLevelType says where a texture() family call takes its level of
// detail from, Rust naga's TextureLevelType (builtins.rs:1536-1541).
type textureLevelType uint8

const (
	textureLevelNone textureLevelType = iota
	textureLevelLod
	textureLevelGrad
)

// macroCall is one builtin overload's expansion, Rust naga's MacroCall
// (builtins.rs:1543-1583). The fields a kind does not use are zero.
type macroCall struct {
	kind macroKind

	// math is the function of a macroMath or macroSplatted expansion.
	math ir.MathFunction
	// arg is the argument a macroSplatted expansion converts to size.
	arg int
	// size is the vector size a conversion splats to; nil converts nothing.
	size *ir.VectorSize

	// unary, binary, relational and cast are the operator or type of the
	// corresponding expansions.
	unary      ir.UnaryOperator
	binary     ir.BinaryOperator
	relational ir.RelationalFunction
	cast       ir.ScalarKind

	// proj, offset, shadow and level describe a macroTexture expansion.
	proj   bool
	offset bool
	shadow bool
	level  textureLevelType

	// arrayed marks a macroTextureSize expansion of an arrayed image, and
	// multi a macroImageLoad expansion of a multisampled one.
	arrayed bool
	multi   bool

	// flags are the barrier flags of a macroMemoryBarrier expansion;
	// macroBarrier asks for barrierFlags, the one workgroup barrier.
	flags ir.BarrierFlags

	// hasComp marks the textureGather overload that takes the component index
	// as its last argument.
	hasComp bool

	// axis and control describe a macroDerivate expansion.
	axis    ir.DerivativeAxis
	control ir.DerivativeControl
}

// arity is the number of arguments the expansion reads without a bounds check.
// Rust indexes its argument slice directly (builtins.rs:1585-2060); a Go
// library must not panic, so the check turns a wrong table entry into an error.
func (m *macroCall) arity() int {
	switch m.kind {
	case macroSampler, macroSamplerShadow, macroTexture, macroImageLoad, macroBinary, macroMod:
		return 2
	case macroImageStore, macroBitfieldExtract, macroMixBoolean, macroClamp, macroSmoothStep:
		return 3
	case macroBitfieldInsert:
		return 4
	case macroBarrier, macroMemoryBarrier:
		return 0
	case macroTextureGather:
		// The image and the coordinate; the depth reference, the offset and the
		// component follow, each present only when the overload has it.
		n := 2
		if m.shadow {
			n++
		}
		if m.offset {
			n++
		}
		if m.hasComp {
			n++
		}
		return n
	case macroSplatted:
		return max(1, m.arg+1)
	default:
		// Every other expansion reads its first argument only.
		return 1
	}
}

// call expands the builtin call, Rust naga's MacroCall::call
// (builtins.rs:1585-2060). ok is false when the builtin yields no value, which
// is what a statement-level builtin such as imageStore or barrier does.
func (m *macroCall) call(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	if len(args) < m.arity() {
		return 0, false, errInternal("builtin called with too few arguments", span)
	}

	switch m.kind {
	case macroSampler:
		c.samplers[args[0].expr] = args[1].expr
		return args[0].expr, true, nil

	case macroSamplerShadow:
		// A shadow sampler's image must become a depth image before the
		// sampler pair is recorded, as Rust's sampled_to_depth does
		// (builtins.rs:1600-1605). Rust collects the error and keeps going.
		if err := sampledToDepth(c, args[0].expr, span); err != nil {
			c.fe.addError(err)
		}
		c.invalidateTypifier()
		c.samplers[args[0].expr] = args[1].expr
		return args[0].expr, true, nil

	case macroTexture:
		return m.callTexture(c, args, span)

	case macroTextureSize:
		return m.callTextureSize(c, args, span)

	case macroTextureQueryLevels:
		return callAsInt(c, ir.ImageQueryNumLevels{}, args[0].expr)

	case macroImageLoad:
		return m.callImageLoad(c, args, span)

	case macroImageStore:
		return m.callImageStore(c, args, span)

	case macroMath:
		h, err := c.mathExpr(m.math, args)
		return h, true, err

	case macroFindLsbUint, macroFindMsbUint:
		fun := ir.MathFirstTrailingBit
		if m.kind == macroFindMsbUint {
			fun = ir.MathFirstLeadingBit
		}
		res, err := c.mathExpr(fun, args)
		if err != nil {
			return 0, false, err
		}
		h, err := c.convertTo32(res, ir.ScalarSint)
		return h, true, err

	case macroBitfieldInsert:
		arg2, err := c.convertTo32(args[2].expr, ir.ScalarUint)
		if err != nil {
			return 0, false, err
		}
		arg3, err := c.convertTo32(args[3].expr, ir.ScalarUint)
		if err != nil {
			return 0, false, err
		}
		h, err := c.addExpression(ir.ExprMath{
			Fun:  ir.MathInsertBits,
			Arg:  args[0].expr,
			Arg1: exprPtr(args[1].expr),
			Arg2: exprPtr(arg2),
			Arg3: exprPtr(arg3),
		}, ir.Span{})
		return h, true, err

	case macroBitfieldExtract:
		arg1, err := c.convertTo32(args[1].expr, ir.ScalarUint)
		if err != nil {
			return 0, false, err
		}
		arg2, err := c.convertTo32(args[2].expr, ir.ScalarUint)
		if err != nil {
			return 0, false, err
		}
		h, err := c.addExpression(ir.ExprMath{
			Fun:  ir.MathExtractBits,
			Arg:  args[0].expr,
			Arg1: exprPtr(arg1),
			Arg2: exprPtr(arg2),
		}, ir.Span{})
		return h, true, err

	case macroRelational:
		h, err := c.addExpression(ir.ExprRelational{Fun: m.relational, Argument: args[0].expr}, ir.Span{})
		return h, true, err

	case macroUnary:
		h, err := c.addExpression(ir.ExprUnary{Op: m.unary, Expr: args[0].expr}, ir.Span{})
		return h, true, err

	case macroBinary:
		h, err := c.addExpression(ir.ExprBinary{Op: m.binary, Left: args[0].expr, Right: args[1].expr}, ir.Span{})
		return h, true, err

	case macroMod:
		return m.callMod(c, args, span)

	case macroSplatted:
		splatted, err := c.implicitSplat(args[m.arg].expr, span, m.size)
		if err != nil {
			return 0, false, err
		}
		args[m.arg].expr = splatted
		h, err := c.mathExpr(m.math, args)
		return h, true, err

	case macroMixBoolean:
		h, err := c.addExpression(ir.ExprSelect{
			Condition: args[2].expr,
			Accept:    args[1].expr,
			Reject:    args[0].expr,
		}, ir.Span{})
		return h, true, err

	case macroClamp:
		arg1, err := c.implicitSplat(args[1].expr, span, m.size)
		if err != nil {
			return 0, false, err
		}
		arg2, err := c.implicitSplat(args[2].expr, span, m.size)
		if err != nil {
			return 0, false, err
		}
		h, err := c.addExpression(ir.ExprMath{
			Fun:  ir.MathClamp,
			Arg:  args[0].expr,
			Arg1: exprPtr(arg1),
			Arg2: exprPtr(arg2),
		}, ir.Span{})
		return h, true, err

	case macroBitCast:
		h, err := c.addExpression(ir.ExprAs{Expr: args[0].expr, Kind: m.cast}, ir.Span{})
		return h, true, err

	case macroDerivate:
		h, err := c.addExpression(ir.ExprDerivative{Axis: m.axis, Control: m.control, Expr: args[0].expr}, ir.Span{})
		return h, true, err

	case macroBarrier:
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtControlBarrier{Flags: barrierFlags}})
		return 0, false, nil

	case macroMemoryBarrier:
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtMemoryBarrier{Flags: m.flags}})
		return 0, false, nil

	case macroTextureGather:
		return m.callTextureGather(c, args, span)

	case macroSmoothStep:
		arg0, err := c.implicitSplat(args[0].expr, span, m.size)
		if err != nil {
			return 0, false, err
		}
		arg1, err := c.implicitSplat(args[1].expr, span, m.size)
		if err != nil {
			return 0, false, err
		}
		h, err := c.addExpression(ir.ExprMath{
			Fun:  ir.MathSmoothStep,
			Arg:  arg0,
			Arg1: exprPtr(arg1),
			Arg2: exprPtr(args[2].expr),
		}, ir.Span{})
		return h, true, err
	}
	return 0, false, errInternal("unknown builtin expansion", span)
}

// barrierFlags is what GLSL's barrier() asks for: a workgroup barrier, and
// nothing else.
//
// It is deliberately not Rust naga's Barrier::all() (front/glsl/builtins.rs:2039).
// All four bits make the SPIR-V writer order at Subgroup execution scope, widen
// every memory class to Device scope through the storage bit, raise the module
// to SPIR-V 1.3 and declare CapabilitySubgroupBarrier — none of which a
// workgroup barrier needs, and the version requirement makes an ordinary
// #version 450 compute shader fail on a SPIR-V 1.0 device. glslang compiles the
// same call to
//
//	OpControlBarrier %uint_2 %uint_2 %uint_264
//
// that is Workgroup execution scope, Workgroup memory scope and
// AcquireRelease|WorkgroupMemory, which is exactly what the writer produces for
// BarrierWorkGroup alone. The GLSL spec (§8.16) gives barrier() no scope
// qualifier at all, so the workgroup is the whole of what it promises.
const barrierFlags = ir.BarrierWorkGroup

// The flags of the memory-barrier functions of GLSL 4.50 §8.16, chosen so that
// the SPIR-V writer's BarrierFlags mapping reproduces glslang's scope and
// memory semantics (glslangValidator 1.4.341, #version 450 compute probes):
//
//	memoryBarrier()       OpMemoryBarrier Device     {Uniform, WorkGroup, Image}+AcquireRelease
//	memoryBarrierBuffer() OpMemoryBarrier Device     {Uniform}+AcquireRelease
//	memoryBarrierShared() OpMemoryBarrier Device     {WorkGroup}+AcquireRelease
//	memoryBarrierImage()  OpMemoryBarrier Device     {Image}+AcquireRelease
//	groupMemoryBarrier()  OpMemoryBarrier Workgroup  {Uniform, WorkGroup, Image}+AcquireRelease
//
// glslang adds AtomicCounterMemory and orders at Device scope for the four
// memoryBarrier* functions. Neither is expressible here: ir.BarrierFlags has
// no atomic-counter bit, and the SPIR-V writer ties UniformMemory (which also
// covers StorageBuffer) to BarrierStorage, which in turn raises the memory
// scope to Device. Each constant below is therefore the closest set of flags
// that never under-synchronizes; where it over-synchronizes the comment says so.
const (
	// memoryBarrierAllMemory is memoryBarrier(): every memory class the IR
	// names, at Device scope. glslang also orders atomic counters; the IR has
	// no such bit and Vulkan GLSL has no atomic counters anyway (§8.16).
	memoryBarrierAllMemory = ir.BarrierStorage | ir.BarrierWorkGroup | ir.BarrierTexture
	// memoryBarrierBufferMemory is memoryBarrierBuffer(): Uniform and
	// StorageBuffer accesses, Device scope. Exactly glslang's semantics.
	memoryBarrierBufferMemory = ir.BarrierStorage
	// memoryBarrierSharedMemory is memoryBarrierShared(): WorkGroup storage,
	// Workgroup scope. glslang orders at Device scope, but workgroup memory is
	// only visible inside its workgroup, so the narrower scope synchronizes
	// every access the function is meant to cover.
	memoryBarrierSharedMemory = ir.BarrierWorkGroup
	// memoryBarrierImageMemory is memoryBarrierImage(): Image accesses. The
	// BarrierStorage bit is what makes the writer order at Device scope, which
	// an image needs because storage images are shared device-wide; it also
	// adds UniformMemory, so the barrier is wider than glslang's, never narrower.
	memoryBarrierImageMemory = ir.BarrierStorage | ir.BarrierTexture
	// groupMemoryBarrierMemory is groupMemoryBarrier(): the same memory classes
	// as memoryBarrier() over Workgroup scope. The IR cannot say "Uniform at
	// Workgroup scope", so this emits the Device-scope form. The narrower
	// alternative (WorkGroup|Texture) would drop the Uniform/StorageBuffer class
	// from the barrier and silently under-synchronize; over-synchronizing is
	// always safe (Vulkan §7.3 Memory Model), under-synchronizing is not.
	groupMemoryBarrierMemory = ir.BarrierStorage | ir.BarrierWorkGroup | ir.BarrierTexture
)

// Stage note: a barrier statement is valid only in compute-like stages.
// glslang accepts memoryBarrier, memoryBarrierBuffer and memoryBarrierImage in
// a fragment shader and so does this frontend, but valid.Validate then refuses
// the module ("Uses operations forbidden at this stage") — the same rule as
// Rust naga's validator (valid/function.rs, the COMPUTE_LIKE stage mask), so
// the refusal is parity, not a frontend decision.

// memoryBarrierFlags is the flag set of one GLSL memory-barrier function.
func memoryBarrierFlags(name string) ir.BarrierFlags {
	switch name {
	case "memoryBarrier":
		return memoryBarrierAllMemory
	case "memoryBarrierBuffer":
		return memoryBarrierBufferMemory
	case "memoryBarrierShared":
		return memoryBarrierSharedMemory
	case "memoryBarrierImage":
		return memoryBarrierImageMemory
	case "groupMemoryBarrier":
		return groupMemoryBarrierMemory
	default:
		return 0
	}
}

// refusedBuiltin explains why a GLSL builtin this frontend does not implement
// is refused, and is empty for every name it implements. A refused builtin is
// an error at the call rather than an unknown-function diagnostic, so the
// author learns what the limitation is.
func refusedBuiltin(name string) string {
	switch name {
	case "memoryBarrierAtomicCounter":
		return "memoryBarrierAtomicCounter is not supported: atomic counters do not exist in Vulkan GLSL " +
			"(glslang refuses atomic_uint under #version 450 with 'atomic counter types' : not allowed when using GLSL for Vulkan), " +
			"and its SPIR-V AtomicCounterMemory semantics bit has no ir.BarrierFlags counterpart, so the barrier's memory class cannot be expressed"
	case "textureGatherOffsets":
		return "textureGatherOffsets is not supported: it gathers with four independent offsets, " +
			"but ir.ExprImageSample carries one constant Offset and the SPIR-V writer emits ConstOffset, not ConstOffsets"
	default:
		// The atomic-counter functions are refused here too, so a call reports
		// the missing resource class rather than an unknown-function text
		// (atomic counters, atomics.go).
		if atomicCounterFunction(name) {
			return atomicCounterText
		}
		return ""
	}
}

// exprPtr returns a pointer to h, for the optional operands of an expression.
func exprPtr(h ir.ExpressionHandle) *ir.ExpressionHandle { return &h }

// mathExpr appends a Math expression over the arguments it is given, Rust's
// `Expression::Math { arg, arg1: args.get(1), .. }` (builtins.rs:1834-1843).
func (c *Context) mathExpr(fun ir.MathFunction, args []loweredArg) (ir.ExpressionHandle, *frontError) {
	kind := ir.ExprMath{Fun: fun, Arg: args[0].expr}
	if len(args) > 1 {
		kind.Arg1 = exprPtr(args[1].expr)
	}
	if len(args) > 2 {
		kind.Arg2 = exprPtr(args[2].expr)
	}
	if len(args) > 3 {
		kind.Arg3 = exprPtr(args[3].expr)
	}
	return c.addExpression(kind, ir.Span{})
}

// convertTo32 converts h to a 32-bit scalar of kind, Rust's
// `Expression::As { expr, kind, convert: Some(4) }` (builtins.rs:1860-1867).
func (c *Context) convertTo32(h ir.ExpressionHandle, kind ir.ScalarKind) (ir.ExpressionHandle, *frontError) {
	width := uint8(4)
	return c.addExpression(ir.ExprAs{Expr: h, Kind: kind, Convert: &width}, ir.Span{})
}

// callAsInt appends an image query followed by its conversion to a signed
// 32-bit integer, the shape every texture query result takes
// (builtins.rs:1775-1801).
func callAsInt(c *Context, query ir.ImageQuery, image ir.ExpressionHandle) (ir.ExpressionHandle, bool, *frontError) {
	expr, err := c.addExpression(ir.ExprImageQuery{Image: image, Query: query}, ir.Span{})
	if err != nil {
		return 0, false, err
	}
	h, err := c.convertTo32(expr, ir.ScalarSint)
	return h, true, err
}

// callTexture lowers a texture() family call: the projection divisor, the
// coordinate split and the level of detail, then the sample itself
// (builtins.rs:1606-1717).
func (m *macroCall) callTexture(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	coords := args[1].expr

	if m.proj {
		size, ok, err := c.vectorSizeOf(coords, span)
		if err != nil {
			return 0, false, err
		}
		if !ok {
			return 0, false, errInternal("projected texture coordinate is not a vector", span)
		}
		right, err := c.addExpression(ir.ExprAccessIndex{Base: coords, Index: uint32(size) - 1}, ir.Span{})
		if err != nil {
			return 0, false, err
		}
		var left ir.ExpressionHandle
		if size == ir.Vec2 {
			left, err = c.addExpression(ir.ExprAccessIndex{Base: coords, Index: 0}, ir.Span{})
		} else {
			// The divisor is the last component, splatted to the vector the
			// remaining components are resized to (builtins.rs:1635-1644).
			resize := ir.Vec3
			if size == ir.Vec3 {
				resize = ir.Vec2
			}
			right, err = c.addExpression(ir.ExprSplat{Size: resize, Value: right}, ir.Span{})
			if err == nil {
				left, err = c.vectorResize(resize, coords, ir.Span{})
			}
		}
		if err != nil {
			return 0, false, err
		}
		coords, err = c.addExpression(ir.ExprBinary{Op: ir.BinaryDivide, Left: left, Right: right}, ir.Span{})
		if err != nil {
			return 0, false, err
		}
	}

	var extra *ir.ExpressionHandle
	if len(args) > 2 {
		extra = exprPtr(args[2].expr)
	}
	comps, err := c.fe.coordinateComponents(c, args[0].expr, coords, extra, span)
	if err != nil {
		return 0, false, err
	}

	numArgs := 2
	if comps.usedExtra {
		numArgs++
	}

	var level ir.SampleLevel
	switch m.level {
	case textureLevelNone:
		level = ir.SampleLevelAuto{}
	case textureLevelLod:
		numArgs++
		if m.shadow {
			// GLSL lets a shadow sampler take a LOD Rust's IR cannot express;
			// Rust assumes level zero (builtins.rs:1668-1678).
			level = ir.SampleLevelZero{}
		} else {
			level = ir.SampleLevelExact{Level: args[2].expr}
		}
	case textureLevelGrad:
		numArgs += 2
		if m.shadow {
			// Likewise for the gradients of a shadow sampler
			// (builtins.rs:1680-1696).
			level = ir.SampleLevelZero{}
		} else {
			level = ir.SampleLevelGradient{X: args[2].expr, Y: args[3].expr}
		}
	}

	var offset *ir.ExpressionHandle
	if m.offset {
		// GLSL 4.50 §8.9 requires a texture offset to be a constant
		// expression, and ir.ExprImageSample.Offset is one: a run-time offset
		// would be written as the ConstOffset operand of an OpImageSample*
		// instruction, and spirv-val rejects a non-constant one. glslang is
		// laxer and accepts the source, so the check is ours, as it is for
		// textureGatherOffset.
		// The check is the resolving one: a const-qualified local is a
		// constant expression (GLSL 4.50 §4.3.3), and the offset stored in the
		// IR must be a value the backends can read as a constant.
		offsetExpr, ok := c.eval.ConstExprWithConstLocals(args[numArgs].expr)
		if !ok {
			return 0, false, errMessage(span, "the offset argument of a texture sampling call must be a constant expression")
		}
		offset = exprPtr(offsetExpr)
		numArgs++
	}

	if m.level == textureLevelNone && numArgs < len(args) {
		level = ir.SampleLevelBias{Bias: args[numArgs].expr}
	}

	return textureCall(c, args[0].expr, level, comps, offset, nil, span)
}

// callTextureGather lowers a textureGather or textureGatherOffset call
// (GLSL 4.50 §8.9): the coordinate split, the separate depth reference of a
// depth image, the constant offset, and the constant component.
func (m *macroCall) callTextureGather(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	comps, err := c.fe.gatherCoordinateComponents(c, args[0].expr, args[1].expr, span)
	if err != nil {
		return 0, false, err
	}

	next := 2
	if m.shadow {
		// A gather of a depth image compares against its own argument, which
		// GLSL places after the coordinate and before the offset.
		comps.depthRef = exprPtr(args[next].expr)
		next++
	}

	var offset *ir.ExpressionHandle
	if m.offset {
		// ir.ExprImageSample.Offset is a const-expression, which is also what
		// GLSL 4.50 §8.9 requires of a gather offset ("must be a constant
		// expression"). glslang is laxer and emits a non-constant Offset
		// operand, so the check is ours.
		// The resolving check: a const-qualified local is a constant
		// expression (GLSL 4.50 §4.3.3), and the stored offset must be a value
		// the backends can read as a constant.
		offsetExpr, ok := c.eval.ConstExprWithConstLocals(args[next].expr)
		if !ok {
			return 0, false, errMessage(span, "the offset argument of textureGatherOffset must be a constant expression")
		}
		offset = exprPtr(offsetExpr)
		next++
	}

	component := ir.SwizzleX
	if m.hasComp {
		component, err = gatherComponent(c, args[next].expr, span)
		if err != nil {
			return 0, false, err
		}
	}

	return textureCall(c, args[0].expr, ir.SampleLevelZero{}, comps, offset, &component, span)
}

// gatherComponent evaluates textureGather's component argument, which GLSL
// 4.50 §8.9 constrains to a constant integral expression in 0 to 3.
func gatherComponent(c *Context, h ir.ExpressionHandle, span ir.Span) (ir.SwizzleComponent, *frontError) {
	// The resolving view: a const-qualified local is a constant expression
	// (GLSL 4.50 §4.3.3), as is a chain through other const locals.
	lit, ok := c.eval.LiteralWithConstLocals(h)
	if !ok {
		// glslang: "must be a compile-time constant: component argument".
		return 0, errMessage(span, "the comp argument of textureGather must be a constant expression")
	}
	var index int64
	switch v := lit.(type) {
	case ir.LiteralI32:
		index = int64(v)
	case ir.LiteralU32:
		index = int64(v)
	default:
		return 0, errMessage(span, "the comp argument of textureGather must be a signed or unsigned integer constant")
	}
	if index < 0 || index > 3 {
		// glslang: "must be 0, 1, 2, or 3: component argument".
		return 0, errSemantic(span, "the comp argument of textureGather is %d, which is outside the component range 0 to 3", index)
	}
	return [...]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleW}[index], nil
}

// callTextureSize lowers textureSize and imageSize, whose arrayed forms build
// a vector of the base size and the layer count (builtins.rs:1719-1783).
func (m *macroCall) callTextureSize(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	query := ir.ImageQuerySize{}
	if len(args) > 1 {
		query.Level = exprPtr(args[1].expr)
	}
	expr, err := c.addExpression(ir.ExprImageQuery{Image: args[0].expr, Query: query}, ir.Span{})
	if err != nil {
		return 0, false, err
	}

	if m.arrayed {
		var size ir.VectorSize
		components := make([]ir.ExpressionHandle, 0, 4)
		inner, terr := c.resolveType(expr, span)
		if terr != nil {
			return 0, false, terr
		}
		vector, ok := inner.(ir.VectorType)
		if ok {
			for index := uint32(0); index < uint32(vector.Size); index++ {
				component, cerr := c.addExpression(ir.ExprAccessIndex{Base: expr, Index: index}, ir.Span{})
				if cerr != nil {
					return 0, false, cerr
				}
				components = append(components, component)
			}
			size = ir.Vec4
			if vector.Size == ir.Vec2 {
				size = ir.Vec3
			}
		} else {
			components = append(components, expr)
			size = ir.Vec2
		}

		layers, lerr := c.addExpression(ir.ExprImageQuery{Image: args[0].expr, Query: ir.ImageQueryNumLayers{}}, ir.Span{})
		if lerr != nil {
			return 0, false, lerr
		}
		components = append(components, layers)

		ty := c.fe.registerType(ir.VectorType{Size: size, Scalar: scalarU32})
		expr, err = c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
		if err != nil {
			return 0, false, err
		}
	}

	h, err := c.convertTo32(expr, ir.ScalarSint)
	return h, true, err
}

// callImageLoad lowers texelFetch and imageLoad: the coordinate split, and the
// texel's sample index or mip level (builtins.rs:1802-1819).
func (m *macroCall) callImageLoad(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	comps, err := c.fe.coordinateComponents(c, args[0].expr, args[1].expr, nil, span)
	if err != nil {
		return 0, false, err
	}
	kind := ir.ExprImageLoad{
		Image:      args[0].expr,
		Coordinate: comps.coordinate,
		ArrayIndex: comps.arrayIndex,
	}
	if len(args) > 2 {
		if m.multi {
			kind.Sample = exprPtr(args[2].expr)
		} else {
			kind.Level = exprPtr(args[2].expr)
		}
	}
	h, err := c.addExpression(kind, ir.Span{})
	return h, true, err
}

// callImageStore lowers imageStore, a statement rather than a value
// (builtins.rs:1820-1833).
func (m *macroCall) callImageStore(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	comps, err := c.fe.coordinateComponents(c, args[0].expr, args[1].expr, nil, span)
	if err != nil {
		return 0, false, err
	}
	c.emitRestart()
	c.body = append(c.body, ir.Statement{Kind: ir.StmtImageStore{
		Image:      args[0].expr,
		Coordinate: comps.coordinate,
		ArrayIndex: comps.arrayIndex,
		Value:      args[2].expr,
	}})
	return 0, false, nil
}

// callMod lowers mod as x - y * floor(x / y), the expansion Rust uses to keep
// GLSL's sign rules (builtins.rs:1943-1982).
func (m *macroCall) callMod(c *Context, args []loweredArg, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	divisor, err := c.implicitSplat(args[1].expr, span, m.size)
	if err != nil {
		return 0, false, err
	}
	div, err := c.addExpression(ir.ExprBinary{Op: ir.BinaryDivide, Left: args[0].expr, Right: divisor}, ir.Span{})
	if err != nil {
		return 0, false, err
	}
	floored, err := c.addExpression(ir.ExprMath{Fun: ir.MathFloor, Arg: div}, ir.Span{})
	if err != nil {
		return 0, false, err
	}
	mult, err := c.addExpression(ir.ExprBinary{Op: ir.BinaryMultiply, Left: floored, Right: divisor}, ir.Span{})
	if err != nil {
		return 0, false, err
	}
	h, err := c.addExpression(ir.ExprBinary{Op: ir.BinarySubtract, Left: args[0].expr, Right: mult}, ir.Span{})
	return h, true, err
}

// vectorSizeOf returns the vector size of expr's type, and false when it is
// not a vector.
func (c *Context) vectorSizeOf(expr ir.ExpressionHandle, span ir.Span) (ir.VectorSize, bool, *frontError) {
	inner, err := c.resolveType(expr, span)
	if err != nil {
		return 0, false, err
	}
	vector, ok := inner.(ir.VectorType)
	return vector.Size, ok, nil
}

// coordComponents is the result of splitting a texture call's coordinate
// argument, Rust naga's CoordComponents (builtins.rs:2100-2109).
type coordComponents struct {
	coordinate ir.ExpressionHandle
	depthRef   *ir.ExpressionHandle
	arrayIndex *ir.ExpressionHandle
	usedExtra  bool
}

// coordinateComponents splits a texture call's coordinate argument into the
// image coordinate, the array index and the depth reference, Rust naga's
// Frontend::coordinate_components (builtins.rs:2111-2209). extra is the
// separate depth argument a cube-array shadow call takes.
func (f *Frontend) coordinateComponents(c *Context, image, coord ir.ExpressionHandle, extra *ir.ExpressionHandle, span ir.Span) (coordComponents, *frontError) {
	inner, err := c.resolveType(image, span)
	if err != nil {
		return coordComponents{}, err
	}
	img, ok := inner.(ir.ImageType)
	if !ok {
		// Rust records the error and treats the argument as the coordinate
		// (builtins.rs:2195-2207).
		f.addError(errMessage(span, "Type is not an image"))
		return coordComponents{coordinate: coord}, nil
	}

	var imageSize *ir.VectorSize
	switch img.Dim {
	case ir.Dim2D:
		imageSize = sizePtr(ir.Vec2)
	case ir.Dim3D, ir.DimCube:
		imageSize = sizePtr(ir.Vec3)
	}

	var coordSize *ir.VectorSize
	coordInner, err := c.resolveType(coord, span)
	if err != nil {
		return coordComponents{}, err
	}
	if vector, ok := coordInner.(ir.VectorType); ok {
		coordSize = sizePtr(vector.Size)
	}

	shadow := img.Class == ir.ImageClassDepth
	storage := img.Class == ir.ImageClassStorage

	coordinate := coord
	switch {
	case imageSize != nil && coordSize != nil && *imageSize != *coordSize:
		// A wider coordinate is cut down to the image's own size: GLSL lets a
		// call pass extra components the image ignores.
		coordinate, err = c.vectorResize(*imageSize, coord, ir.Span{})
	case imageSize == nil && coordSize != nil:
		coordinate, err = c.addExpression(ir.ExprAccessIndex{Base: coord, Index: 0}, ir.Span{})
	}
	if err != nil {
		return coordComponents{}, err
	}

	coordIndex := uint32(1)
	if imageSize != nil {
		coordIndex = uint32(*imageSize)
	}

	var arrayIndex *ir.ExpressionHandle
	// A cubemap array's layer is part of the coordinate vector of a storage
	// image, not a separate index (builtins.rs:2160-2170).
	if img.Arrayed && !(storage && img.Dim == ir.DimCube) {
		index := coordIndex
		coordIndex++
		h, aerr := c.addExpression(ir.ExprAccessIndex{Base: coord, Index: index}, ir.Span{})
		if aerr != nil {
			return coordComponents{}, aerr
		}
		arrayIndex = exprPtr(h)
	}

	usedExtra := false
	var depthRef *ir.ExpressionHandle
	if shadow {
		if coordIndex == 4 {
			usedExtra = true
			depthRef = extra
		} else {
			h, derr := c.addExpression(ir.ExprAccessIndex{Base: coord, Index: coordIndex}, ir.Span{})
			if derr != nil {
				return coordComponents{}, derr
			}
			depthRef = exprPtr(h)
		}
	}

	return coordComponents{
		coordinate: coordinate,
		depthRef:   depthRef,
		arrayIndex: arrayIndex,
		usedExtra:  usedExtra,
	}, nil
}

// gatherCoordinateComponents splits a textureGather coordinate argument. A
// gather of a depth image takes the depth reference as its own argument, so
// the coordinate splits the same way for a depth image and a sampled one: the
// vector is cut to the image's own size and, for an arrayed image, its last
// component is the layer index.
func (f *Frontend) gatherCoordinateComponents(c *Context, image, coord ir.ExpressionHandle, span ir.Span) (coordComponents, *frontError) {
	inner, err := c.resolveType(image, span)
	if err != nil {
		return coordComponents{}, err
	}
	img, ok := inner.(ir.ImageType)
	if !ok {
		f.addError(errMessage(span, "Type is not an image"))
		return coordComponents{coordinate: coord}, nil
	}

	// The gather overloads exist for a 2D and a cube image only.
	var imageSize ir.VectorSize
	switch img.Dim {
	case ir.Dim2D:
		imageSize = ir.Vec2
	case ir.DimCube:
		imageSize = ir.Vec3
	default:
		return coordComponents{}, errInternal("textureGather of an image dimension it has no form for", span)
	}

	coordinate := coord
	coordInner, err := c.resolveType(coord, span)
	if err != nil {
		return coordComponents{}, err
	}
	if vector, isVector := coordInner.(ir.VectorType); isVector && vector.Size != imageSize {
		coordinate, err = c.vectorResize(imageSize, coord, ir.Span{})
		if err != nil {
			return coordComponents{}, err
		}
	}

	var arrayIndex *ir.ExpressionHandle
	if img.Arrayed {
		layer, aerr := c.addExpression(ir.ExprAccessIndex{Base: coord, Index: uint32(imageSize)}, ir.Span{})
		if aerr != nil {
			return coordComponents{}, aerr
		}
		arrayIndex = exprPtr(layer)
	}

	return coordComponents{coordinate: coordinate, arrayIndex: arrayIndex}, nil
}

// textureCall appends the sample itself, using the sampler of the combined
// sampler the image came from, Rust naga's texture_call
// (builtins.rs:2063-2098). gather is the component a textureGather reads, and
// nil for every other call.
func textureCall(c *Context, image ir.ExpressionHandle, level ir.SampleLevel, comps coordComponents, offset *ir.ExpressionHandle, gather *ir.SwizzleComponent, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	sampler, ok := c.samplers[image]
	if !ok {
		// A sampled image that no combined sampler declaration created has no
		// sampler to pair with it, which is Rust's "Bad call"
		// (builtins.rs:2092-2097).
		return 0, false, errMessage(span, "Bad call")
	}

	if comps.arrayIndex != nil {
		converted, err := c.conversion(*comps.arrayIndex, span, scalarI32)
		if err != nil {
			return 0, false, err
		}
		comps.arrayIndex = exprPtr(converted)
	}

	h, err := c.addExpression(ir.ExprImageSample{
		Image:      image,
		Sampler:    sampler,
		Gather:     gather,
		Coordinate: comps.coordinate,
		ArrayIndex: comps.arrayIndex,
		Offset:     offset,
		Level:      level,
		DepthRef:   comps.depthRef,
	}, span)
	return h, true, err
}

// addBuiltin builds the overload of one builtin signature, Rust naga's
// Module::add_builtin (builtins.rs:27-56). Every parameter is an in parameter
// the expansion reads by value.
func (f *Frontend) addBuiltinOverload(args []ir.TypeInner, macro *macroCall) *overload {
	parameters := make([]ir.TypeHandle, 0, len(args))
	parametersInfo := make([]parameterInfo, 0, len(args))
	for _, arg := range args {
		parameters = append(parameters, f.registerType(arg))
		parametersInfo = append(parametersInfo, parameterInfo{qualifier: paramIn})
	}
	return &overload{
		parameters:     parameters,
		parametersInfo: parametersInfo,
		kind:           overloadMacro,
		macro:          macro,
		defined:        true,
		internal:       true,
	}
}

// injectBuiltin adds the builtin overloads of name to its declaration, Rust
// naga's inject_builtin (builtins.rs:74-459). Only the variations the name has
// not seen yet are generated, so a capability is requested by a shader that
// uses it and no other.
func (f *Frontend) injectBuiltin(decl *functionDeclaration, name string, variations builtinVariations) {
	if reason := refusedBuiltin(name); reason != "" {
		// The call is always an error, so no overload is injected and the
		// reason is recorded first: it is the message the author sees, and the
		// caller's later "Unknown function" stays a note (glsl/parse.go
		// publicError).
		f.addError(errMessage(ir.Span{}, reason))
		return
	}
	if len(decl.overloads) == 0 {
		// The creation call. Rust asks for the standard variation exactly
		// once, when its map entry for the name is created
		// (functions.rs:535-539); the Go caller creates the declaration and
		// passes it here in two steps, so an empty overload set is what marks
		// the first injection. The declaration may already record the bit, and
		// the request must survive the subtraction below either way.
		variations |= variationStandard
		decl.variations &^= variationStandard
	}
	variations &^= decl.variations
	decl.variations |= variations

	if variations.has(variationStandard) {
		f.injectStandardBuiltins(decl, name)
	}
	if variations.has(variationDouble) {
		f.injectDoubleBuiltins(decl, name)
	}
	f.injectTextureBuiltin(decl, name, variations)
}

// injectTextureBuiltin adds the texture, image and texture-query overloads of
// name, the texture family of inject_builtin (builtins.rs:102-458). ok is false
// when name is not part of the family.
func (f *Frontend) injectTextureBuiltin(decl *functionDeclaration, name string, variations builtinVariations) bool {
	switch name {
	case "texture", "textureGrad", "textureGradOffset", "textureLod", "textureLodOffset",
		"textureOffset", "textureProj", "textureProjGrad", "textureProjGradOffset",
		"textureProjLod", "textureProjLodOffset", "textureProjOffset":
		f.injectTextureSampleBuiltins(decl, name, variations)
	case "textureSize":
		f.injectTextureSizeBuiltins(decl, variations)
	case "textureGather", "textureGatherOffset":
		f.injectTextureGatherBuiltins(decl, name, variations)
	case "textureQueryLevels":
		f.injectTextureQueryLevelsBuiltins(decl, variations)
	case "texelFetch", "texelFetchOffset":
		f.injectTexelFetchBuiltins(decl, name, variations)
	case "imageSize":
		f.injectImageSizeBuiltins(decl, variations)
	case "imageLoad":
		f.injectImageLoadBuiltins(decl, variations)
	case "imageStore":
		f.injectImageStoreBuiltins(decl, variations)
	default:
		return false
	}
	return true
}

// textureSampleShape is the coordinate handling of one texture() family
// member: whether the coordinate vector carries the projection divisor, whether
// the call takes an offset, and where its level comes from
// (builtins.rs:120-146).
func textureSampleShape(name string) (proj, offset bool, level textureLevelType) {
	switch name {
	case "texture":
		return false, false, textureLevelNone
	case "textureGrad":
		return false, false, textureLevelGrad
	case "textureGradOffset":
		return false, true, textureLevelGrad
	case "textureLod":
		return false, false, textureLevelLod
	case "textureLodOffset":
		return false, true, textureLevelLod
	case "textureOffset":
		return false, true, textureLevelNone
	case "textureProj":
		return true, false, textureLevelNone
	case "textureProjGrad":
		return true, false, textureLevelGrad
	case "textureProjGradOffset":
		return true, true, textureLevelGrad
	case "textureProjLod":
		return true, false, textureLevelLod
	case "textureProjLodOffset":
		return true, true, textureLevelLod
	case "textureProjOffset":
		return true, true, textureLevelNone
	default:
		return false, false, textureLevelNone
	}
}

// textureSampleVariant is one texture() overload the generator asks for.
type textureSampleVariant struct {
	kind    ir.ScalarKind
	dim     ir.ImageDimension
	arrayed bool
	multi   bool
	shadow  bool

	proj   bool
	offset bool
	level  textureLevelType
	// variant asks for the four-component projected form instead of the
	// dimension-plus-one one, and bias for the optional bias argument.
	variant bool
	bias    bool
}

// injectTextureSampleBuiltins adds the texture() family overloads
// (builtins.rs:115-267). A shadow variant compares against a depth image, and
// the four-variant bits add the projected and biased forms GLSL has.
func (f *Frontend) injectTextureSampleBuiltins(decl *functionDeclaration, name string, variations builtinVariations) {
	proj, offset, level := textureSampleShape(name)
	f.textureArgsGenerator(textureArgsShadow|textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) {
		// One expansion serves every projected and biased variant of this
		// image shape; the parameters are what differ between them.
		macro := &macroCall{kind: macroTexture, proj: proj, offset: offset, shadow: shadow, level: level}
		for bits := 0; bits <= 0b11; bits++ {
			v := textureSampleVariant{
				kind:    kind,
				dim:     dim,
				arrayed: arrayed,
				multi:   multi,
				shadow:  shadow,
				proj:    proj,
				offset:  offset,
				level:   level,
				variant: bits&0b1 != 0,
				bias:    bits&0b10 != 0,
			}
			args, ok := textureSampleArgs(v)
			if !ok {
				continue
			}
			decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
		}
	})
}

// textureSampleArgs returns the parameter types of one texture() variant, and
// false for a form GLSL does not have (builtins.rs:156-262).
func textureSampleArgs(v textureSampleVariant) ([]ir.TypeInner, bool) {
	grad := v.level == textureLevelGrad
	lod := v.level == textureLevelLod

	// Proj is only combined with the biased form, and a shadow sampler takes
	// no bias, no LOD and no offset on a cube map.
	if v.variant && !(v.proj && !v.shadow) {
		return nil, false
	}
	if v.bias && v.level != textureLevelNone {
		return nil, false
	}
	if v.proj && (v.arrayed || v.dim == ir.DimCube) {
		return nil, false
	}
	if v.dim == ir.DimCube && v.offset {
		return nil, false
	}
	if (lod || v.bias) && v.arrayed && v.shadow && v.dim == ir.Dim2D {
		return nil, false
	}
	if v.bias && v.shadow {
		return nil, false
	}

	class := ir.ImageClassSampled
	if v.shadow {
		class = ir.ImageClassDepth
	}
	image := ir.ImageType{
		Dim:          v.dim,
		Arrayed:      v.arrayed,
		Class:        class,
		Multisampled: v.multi,
		SampledKind:  v.kind,
	}

	fromDim := imageDimsToCoordsSize(v.dim)
	numCoords := fromDim
	switch {
	case v.shadow && v.proj:
		numCoords = 4
	case v.dim == ir.Dim1D && v.shadow:
		numCoords = 3
	case v.shadow:
		numCoords++
	case v.proj:
		if v.variant && numCoords == 4 {
			// The normal form already has four components.
			return nil, false
		}
		if v.variant {
			numCoords = 4
		} else {
			numCoords++
		}
	}
	if !(v.dim == ir.Dim1D && v.shadow) && v.arrayed {
		numCoords++
	}

	// texture(gsamplerCubeArrayShadow) takes its depth reference as a separate
	// argument, and has no other forms (builtins.rs:223-232).
	if numCoords >= 5 && (lod || grad || v.offset || v.proj || v.bias) {
		return nil, false
	}

	args := make([]ir.TypeInner, 0, 6)
	args = append(args, image, makeCoordsArg(numCoords, ir.ScalarFloat))
	if numCoords == 5 {
		args = append(args, scalarF32)
	}
	switch v.level {
	case textureLevelLod:
		args = append(args, scalarF32)
	case textureLevelGrad:
		args = append(args, makeCoordsArg(fromDim, ir.ScalarFloat), makeCoordsArg(fromDim, ir.ScalarFloat))
	}
	if v.offset {
		args = append(args, makeCoordsArg(fromDim, ir.ScalarSint))
	}
	if v.bias {
		args = append(args, scalarF32)
	}
	return args, true
}

// injectTextureGatherBuiltins adds textureGather and textureGatherOffset,
// GLSL 4.50 §8.9 Texture Gather Functions. Every signature here was checked
// against glslangValidator (#version 450): a gather exists for 2D, 2D array,
// cube and cube array images (sampled in float, int and uint, and depth), has
// no 1D, 3D or multisampled form, and takes an offset only for a 2D image.
//
// Unlike texture(), a gather of a depth image takes the depth reference as its
// own argument instead of packing it into the coordinate vector, and has no
// component argument (glslang: no matching overloaded function).
func (f *Frontend) injectTextureGatherBuiltins(decl *functionDeclaration, name string, variations builtinVariations) {
	offset := name == "textureGatherOffset"
	f.textureArgsGenerator(textureArgsShadow|textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) {
		// glslang: no matching overloaded function for a 1D, 3D or multisampled
		// image, or for an offset on a cube map.
		if multi || dim == ir.Dim1D || dim == ir.Dim3D || (offset && dim != ir.Dim2D) {
			return
		}
		class := ir.ImageClassSampled
		if shadow {
			class = ir.ImageClassDepth
		}
		image := ir.ImageType{Dim: dim, Arrayed: arrayed, Class: class, SampledKind: kind}
		coordSize := imageDimsToCoordsSize(dim)
		if arrayed {
			coordSize++
		}
		// makeCoordsArg writes one component as a scalar, which is what GLSL
		// spells for a 1D image; every gather dimension here is a vector.
		args := []ir.TypeInner{image, makeCoordsArg(coordSize, ir.ScalarFloat)}

		if shadow {
			// textureGather(gsampler2DShadow, vec2 P, float refZ) and
			// textureGatherOffset(gsampler2DShadow, vec2 P, float refZ, ivec2 offset):
			// the depth reference comes before the offset and there is no comp.
			args = append(args, scalarF32)
			if offset {
				args = append(args, gatherOffsetCoord())
			}
			decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{
				kind:   macroTextureGather,
				shadow: true,
				offset: offset,
			}))
			return
		}

		if offset {
			args = append(args, gatherOffsetCoord())
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{
			kind:   macroTextureGather,
			offset: offset,
		}))
		// The component argument is optional and comes last; without it the
		// gather reads component x.
		withComp := make([]ir.TypeInner, len(args)+1)
		copy(withComp, args)
		withComp[len(args)] = scalarI32
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(withComp, &macroCall{
			kind:    macroTextureGather,
			offset:  offset,
			hasComp: true,
		}))
	})
}

// gatherOffsetCoord is the offset argument of textureGatherOffset: GLSL's
// ivec2 (§8.9).
func gatherOffsetCoord() ir.TypeInner { return makeCoordsArg(2, ir.ScalarSint) }

// injectTextureSizeBuiltins adds textureSize: the image's size at a level, in
// signed integers, with the layer count appended for an arrayed image
// (builtins.rs:268-296).
func (f *Frontend) injectTextureSizeBuiltins(decl *functionDeclaration, variations builtinVariations) {
	f.textureArgsGenerator(textureArgsShadow|textureArgsMulti|textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) {
		class := ir.ImageClassSampled
		if shadow {
			class = ir.ImageClassDepth
		}
		args := []ir.TypeInner{ir.ImageType{
			Dim:          dim,
			Arrayed:      arrayed,
			Class:        class,
			Multisampled: multi,
			SampledKind:  kind,
		}}
		// A multisampled image has no mip levels to ask the size of.
		if !multi {
			args = append(args, scalarI32)
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroTextureSize, arrayed: arrayed}))
	})
}

// injectTextureQueryLevelsBuiltins adds textureQueryLevels
// (builtins.rs:297-316).
func (f *Frontend) injectTextureQueryLevelsBuiltins(decl *functionDeclaration, variations builtinVariations) {
	f.textureArgsGenerator(textureArgsShadow|textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool) {
		class := ir.ImageClassSampled
		if shadow {
			class = ir.ImageClassDepth
		}
		image := ir.ImageType{
			Dim:          dim,
			Arrayed:      arrayed,
			Class:        class,
			Multisampled: multi,
			SampledKind:  kind,
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload([]ir.TypeInner{image}, &macroCall{kind: macroTextureQueryLevels}))
	})
}

// injectTexelFetchBuiltins adds texelFetch and texelFetchOffset, which take a
// signed integer coordinate and a sample or level index (builtins.rs:317-347).
func (f *Frontend) injectTexelFetchBuiltins(decl *functionDeclaration, name string, variations builtinVariations) {
	offset := name == "texelFetchOffset"
	f.textureArgsGenerator(textureArgsMulti|textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, _ bool) {
		// Naga has no cube image fetch.
		if dim == ir.DimCube {
			return
		}
		image := ir.ImageType{
			Dim:          dim,
			Arrayed:      arrayed,
			Class:        ir.ImageClassSampled,
			Multisampled: multi,
			SampledKind:  kind,
		}
		dimValue := imageDimsToCoordsSize(dim)
		coordSize := dimValue
		if arrayed {
			coordSize++
		}
		args := []ir.TypeInner{image, makeCoordsArg(coordSize, ir.ScalarSint), scalarI32}
		if offset {
			args = append(args, makeCoordsArg(dimValue, ir.ScalarSint))
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroImageLoad, multi: multi}))
	})
}

// injectImageSizeBuiltins adds imageSize. The storage format is a placeholder
// the declaration's own layout format replaces, so only the format's scalar
// kind is matched (builtins.rs:348-371).
func (f *Frontend) injectImageSizeBuiltins(decl *functionDeclaration, variations builtinVariations) {
	f.textureArgsGenerator(textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, _, _ bool) {
		if dim == ir.DimCube {
			return
		}
		image := ir.ImageType{
			Dim:           dim,
			Arrayed:       arrayed,
			Class:         ir.ImageClassStorage,
			SampledKind:   kind,
			StorageFormat: dummyStorageFormat(kind),
			// Rust leaves this parameter type's access empty; our IR's empty
			// set is "unset", which validation refuses (ADR-010 E2), and the
			// access does not take part in storage-image overload matching
			// (functions.rs:596-649).
			StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore,
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload([]ir.TypeInner{image}, &macroCall{kind: macroTextureSize, arrayed: arrayed}))
	})
}

// injectImageLoadBuiltins adds imageLoad, which reads a texel of a storage
// image (builtins.rs:372-410).
func (f *Frontend) injectImageLoadBuiltins(decl *functionDeclaration, variations builtinVariations) {
	f.textureArgsGenerator(textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, _, _ bool) {
		if dim == ir.DimCube {
			return
		}
		image := ir.ImageType{
			Dim:           dim,
			Arrayed:       arrayed,
			Class:         ir.ImageClassStorage,
			SampledKind:   kind,
			StorageFormat: dummyStorageFormat(kind),
			StorageAccess: ir.StorageAccessLoad,
		}
		dimValue := imageDimsToCoordsSize(dim)
		coordSize := dimValue
		if arrayed {
			coordSize++
		}
		// A cubemap array's layer is the third component of its coordinate
		// rather than a separate index (builtins.rs:391-398).
		if dim == ir.DimCube && arrayed {
			coordSize = 3
		}
		args := []ir.TypeInner{image, makeCoordsArg(coordSize, ir.ScalarSint)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroImageLoad}))
	})
}

// injectImageStoreBuiltins adds imageStore, a void builtin that writes a texel
// of a storage image (builtins.rs:411-456).
func (f *Frontend) injectImageStoreBuiltins(decl *functionDeclaration, variations builtinVariations) {
	f.textureArgsGenerator(textureArgsOptionsOf(variations), func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, _, _ bool) {
		if dim == ir.DimCube {
			return
		}
		image := ir.ImageType{
			Dim:           dim,
			Arrayed:       arrayed,
			Class:         ir.ImageClassStorage,
			SampledKind:   kind,
			StorageFormat: dummyStorageFormat(kind),
			StorageAccess: ir.StorageAccessStore,
		}
		dimValue := imageDimsToCoordsSize(dim)
		coordSize := dimValue
		if arrayed {
			coordSize++
		}
		if dim == ir.DimCube && arrayed {
			coordSize = 3
		}
		args := []ir.TypeInner{
			image,
			makeCoordsArg(coordSize, ir.ScalarSint),
			ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: kind, Width: 4}},
		}
		overload := f.addBuiltinOverload(args, &macroCall{kind: macroImageStore})
		overload.void = true
		decl.overloads = append(decl.overloads, overload)
	})
}

// injectStandardBuiltins adds the builtins every shader may use, Rust naga's
// inject_standard_builtins (builtins.rs:462-1044).
func (f *Frontend) injectStandardBuiltins(decl *functionDeclaration, name string) {
	if f.injectSamplerBuiltins(decl, name) {
		return
	}

	switch name {
	case "sin", "exp", "exp2", "sinh", "cos", "cosh", "tan", "tanh", "acos", "asin",
		"log", "log2", "radians", "degrees", "asinh", "acosh", "atanh",
		"floatBitsToInt", "floatBitsToUint", "dFdx", "dFdxFine", "dFdxCoarse",
		"dFdy", "dFdyFine", "dFdyCoarse", "fwidth", "fwidthFine", "fwidthCoarse":
		f.injectOneArgFloatBuiltins(decl, name)
	case "intBitsToFloat", "uintBitsToFloat":
		f.injectBitsToFloatBuiltins(decl, name)
	case "pow":
		f.injectPowBuiltins(decl)
	case "abs", "sign":
		f.injectAbsSignBuiltins(decl, name, scalarF32, scalarI32)
	case "bitCount", "bitfieldReverse", "bitfieldExtract", "bitfieldInsert", "findLSB", "findMSB":
		f.injectBitBuiltins(decl, name)
	case "packSnorm4x8", "packUnorm4x8", "packSnorm2x16", "packUnorm2x16", "packHalf2x16":
		f.injectPackBuiltins(decl, name)
	case "unpackSnorm4x8", "unpackUnorm4x8", "unpackSnorm2x16", "unpackUnorm2x16", "unpackHalf2x16":
		f.injectUnpackBuiltins(decl, name)
	case "atan":
		f.injectAtanBuiltins(decl)
	case "all", "any", "not":
		f.injectLogicalBuiltins(decl, name)
	case "lessThan", "greaterThan", "lessThanEqual", "greaterThanEqual":
		f.injectComparisonBuiltins(decl, name, standardComparisonVariants())
	case "equal", "notEqual":
		f.injectComparisonBuiltins(decl, name, equalityComparisonVariants())
	case "min", "max":
		f.injectMinMaxBuiltins(decl, name, scalarF32, scalarI32, scalarU32)
	case "mix":
		f.injectMixBuiltins(decl)
	case "clamp":
		f.injectClampBuiltins(decl)
	case "barrier":
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(nil, &macroCall{kind: macroBarrier}))
	case "memoryBarrier", "memoryBarrierBuffer", "memoryBarrierShared", "memoryBarrierImage", "groupMemoryBarrier":
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(nil, &macroCall{
			kind:  macroMemoryBarrier,
			flags: memoryBarrierFlags(name),
		}))
	default:
		f.injectCommonBuiltins(decl, name, 4)
	}
}

// injectSamplerBuiltins adds the image and sampler pair an opaque sampler type
// names, Rust naga's sampler arms of inject_standard_builtins
// (builtins.rs:467-549). ok is false when name is not a sampler type.
func (f *Frontend) injectSamplerBuiltins(decl *functionDeclaration, name string) bool {
	if sampler, kind, anykind := anykindSampler(name); anykind {
		if f.injectAnykindSampler(decl, sampler, kind) {
			return true
		}
	}

	var dim ir.ImageDimension
	var arrayed bool
	switch name {
	case "sampler1DShadow", "sampler1DArrayShadow":
		dim = ir.Dim1D
	case "sampler2DShadow", "sampler2DArrayShadow":
		dim = ir.Dim2D
	case "samplerCubeShadow", "samplerCubeArrayShadow":
		dim = ir.DimCube
	default:
		return false
	}
	switch name {
	case "sampler1DArrayShadow", "sampler2DArrayShadow", "samplerCubeArrayShadow":
		arrayed = true
	}

	// A shadow sampler is usable both as its sampled image and as its depth
	// image, and the call picks the one the builtin needs
	// (builtins.rs:531-548).
	for i := 0; i < 2; i++ {
		class := ir.ImageClassSampled
		if i > 0 {
			class = ir.ImageClassDepth
		}
		image := ir.ImageType{Dim: dim, Arrayed: arrayed, Class: class, SampledKind: ir.ScalarFloat}
		args := []ir.TypeInner{image, ir.SamplerType{Comparison: true}}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroSamplerShadow}))
	}
	return true
}

// anykindSampler splits a plain sampler type name into the shape it names and
// the kind of its texels: sampler1D and friends are float, usampler1D uint and
// isampler1D sint (builtins.rs:467-476).
func anykindSampler(name string) (sampler string, kind ir.ScalarKind, ok bool) {
	switch {
	case len(name) >= len("sampler") && name[:len("sampler")] == "sampler":
		return name, ir.ScalarFloat, true
	case len(name) >= len("usampler") && name[:len("usampler")] == "usampler":
		return name[1:], ir.ScalarUint, true
	case len(name) >= len("isampler") && name[:len("isampler")] == "isampler":
		return name[1:], ir.ScalarSint, true
	default:
		return "", 0, false
	}
}

// injectAnykindSampler adds the image and sampler pair of one sampler shape,
// and reports false when the shape is not one GLSL declares
// (builtins.rs:478-511).
func (f *Frontend) injectAnykindSampler(decl *functionDeclaration, sampler string, kind ir.ScalarKind) bool {
	var dim ir.ImageDimension
	var arrayed, multi bool
	switch sampler {
	case "sampler1D":
		dim = ir.Dim1D
	case "sampler1DArray":
		dim, arrayed = ir.Dim1D, true
	case "sampler2D":
		dim = ir.Dim2D
	case "sampler2DArray":
		dim, arrayed = ir.Dim2D, true
	case "sampler2DMS":
		dim, multi = ir.Dim2D, true
	case "sampler2DMSArray":
		dim, arrayed, multi = ir.Dim2D, true, true
	case "sampler3D":
		dim = ir.Dim3D
	case "samplerCube":
		dim = ir.DimCube
	case "samplerCubeArray":
		dim, arrayed = ir.DimCube, true
	default:
		return false
	}
	image := ir.ImageType{
		Dim:          dim,
		Arrayed:      arrayed,
		Class:        ir.ImageClassSampled,
		Multisampled: multi,
		SampledKind:  kind,
	}
	args := []ir.TypeInner{image, ir.SamplerType{}}
	decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroSampler}))
	return true
}

// injectOneArgFloatBuiltins adds the single-argument float builtins of the
// standard variation: the trigonometric and exponential functions, the float
// bit casts and the derivatives (builtins.rs:550-603).
func (f *Frontend) injectOneArgFloatBuiltins(decl *functionDeclaration, name string) {
	for bits := 0; bits < 0b100; bits++ {
		size := sizeOfBits(bits)
		args := []ir.TypeInner{vectorOf(size, scalarF32)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, oneArgFloatMacro(name)))
	}
}

// oneArgFloatMacro is the expansion of one name of the single-argument float
// family (builtins.rs:570-599).
func oneArgFloatMacro(name string) *macroCall {
	switch name {
	case "sin":
		return &macroCall{kind: macroMath, math: ir.MathSin}
	case "exp":
		return &macroCall{kind: macroMath, math: ir.MathExp}
	case "exp2":
		return &macroCall{kind: macroMath, math: ir.MathExp2}
	case "sinh":
		return &macroCall{kind: macroMath, math: ir.MathSinh}
	case "cos":
		return &macroCall{kind: macroMath, math: ir.MathCos}
	case "cosh":
		return &macroCall{kind: macroMath, math: ir.MathCosh}
	case "tan":
		return &macroCall{kind: macroMath, math: ir.MathTan}
	case "tanh":
		return &macroCall{kind: macroMath, math: ir.MathTanh}
	case "acos":
		return &macroCall{kind: macroMath, math: ir.MathAcos}
	case "asin":
		return &macroCall{kind: macroMath, math: ir.MathAsin}
	case "log":
		return &macroCall{kind: macroMath, math: ir.MathLog}
	case "log2":
		return &macroCall{kind: macroMath, math: ir.MathLog2}
	case "asinh":
		return &macroCall{kind: macroMath, math: ir.MathAsinh}
	case "acosh":
		return &macroCall{kind: macroMath, math: ir.MathAcosh}
	case "atanh":
		return &macroCall{kind: macroMath, math: ir.MathAtanh}
	case "radians":
		return &macroCall{kind: macroMath, math: ir.MathRadians}
	case "degrees":
		return &macroCall{kind: macroMath, math: ir.MathDegrees}
	case "floatBitsToInt":
		return &macroCall{kind: macroBitCast, cast: ir.ScalarSint}
	case "floatBitsToUint":
		return &macroCall{kind: macroBitCast, cast: ir.ScalarUint}
	case "dFdxCoarse":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeX, control: ir.DerivativeCoarse}
	case "dFdyCoarse":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeY, control: ir.DerivativeCoarse}
	case "fwidthCoarse":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeWidth, control: ir.DerivativeCoarse}
	case "dFdxFine":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeX, control: ir.DerivativeFine}
	case "dFdyFine":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeY, control: ir.DerivativeFine}
	case "fwidthFine":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeWidth, control: ir.DerivativeFine}
	case "dFdx":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeX, control: ir.DerivativeNone}
	case "dFdy":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeY, control: ir.DerivativeNone}
	case "fwidth":
		return &macroCall{kind: macroDerivate, axis: ir.DerivativeWidth, control: ir.DerivativeNone}
	default:
		return &macroCall{kind: macroMath}
	}
}

// injectBitsToFloatBuiltins adds intBitsToFloat and uintBitsToFloat
// (builtins.rs:604-627).
func (f *Frontend) injectBitsToFloatBuiltins(decl *functionDeclaration, name string) {
	scalar := scalarI32
	if name == "uintBitsToFloat" {
		scalar = scalarU32
	}
	for bits := 0; bits < 0b100; bits++ {
		args := []ir.TypeInner{vectorOf(sizeOfBits(bits), scalar)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroBitCast, cast: ir.ScalarFloat}))
	}
}

// injectPowBuiltins adds pow (builtins.rs:628-649).
func (f *Frontend) injectPowBuiltins(decl *functionDeclaration) {
	for bits := 0; bits < 0b100; bits++ {
		ty := vectorOf(sizeOfBits(bits), scalarF32)
		args := []ir.TypeInner{ty, ty}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: ir.MathPow}))
	}
}

// injectAbsSignBuiltins adds abs and sign for the given scalar kinds: f32 and
// i32 for the standard variation, f64 for the double one
// (builtins.rs:650-680, 1049-1075).
func (f *Frontend) injectAbsSignBuiltins(decl *functionDeclaration, name string, scalars ...ir.ScalarType) {
	math := ir.MathSign
	if name == "abs" {
		math = ir.MathAbs
	}
	for _, scalar := range scalars {
		for bits := 0; bits < 0b100; bits++ {
			args := []ir.TypeInner{vectorOf(sizeOfBits(bits), scalar)}
			decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: math}))
		}
	}
}

// injectBitBuiltins adds the bit manipulation builtins of the standard
// variation (builtins.rs:681-751).
func (f *Frontend) injectBitBuiltins(decl *functionDeclaration, name string) {
	macro := bitBuiltinMacro(name)
	for bits := 0; bits < 0b1000; bits++ {
		scalar := scalarI32
		if bits&0b1 != 0 {
			scalar = scalarU32
		}
		size := sizeOfBits(bits >> 1)
		ty := func() ir.TypeInner { return vectorOf(size, scalar) }

		args := []ir.TypeInner{ty()}
		switch macro.kind {
		case macroBitfieldExtract:
			args = append(args, scalarI32, scalarI32)
		case macroBitfieldInsert:
			args = append(args, ty(), scalarI32, scalarI32)
		}

		injected := macro
		if scalar.Kind == ir.ScalarUint {
			// findLSB and findMSB return a sint in GLSL whatever the argument
			// kind, so the uint variants cast their result
			// (builtins.rs:734-747).
			switch macro.kind {
			case macroMath:
				if macro.math == ir.MathFirstTrailingBit {
					injected = &macroCall{kind: macroFindLsbUint}
				} else if macro.math == ir.MathFirstLeadingBit {
					injected = &macroCall{kind: macroFindMsbUint}
				}
			}
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, injected))
	}
}

// bitBuiltinMacro is the expansion of one bit manipulation builtin before the
// unsigned find variants are specialised (builtins.rs:683-697).
func bitBuiltinMacro(name string) *macroCall {
	switch name {
	case "bitCount":
		return &macroCall{kind: macroMath, math: ir.MathCountOneBits}
	case "bitfieldReverse":
		return &macroCall{kind: macroMath, math: ir.MathReverseBits}
	case "bitfieldExtract":
		return &macroCall{kind: macroBitfieldExtract}
	case "bitfieldInsert":
		return &macroCall{kind: macroBitfieldInsert}
	case "findLSB":
		return &macroCall{kind: macroMath, math: ir.MathFirstTrailingBit}
	case "findMSB":
		return &macroCall{kind: macroMath, math: ir.MathFirstLeadingBit}
	default:
		return &macroCall{kind: macroMath}
	}
}

// packMathFunctions maps a packing builtin to its math function
// (builtins.rs:752-781).
var packMathFunctions = map[string]ir.MathFunction{
	"packSnorm4x8":  ir.MathPack4x8snorm,
	"packUnorm4x8":  ir.MathPack4x8unorm,
	"packSnorm2x16": ir.MathPack2x16snorm,
	"packUnorm2x16": ir.MathPack2x16unorm,
	"packHalf2x16":  ir.MathPack2x16float,
}

// unpackMathFunctions maps an unpacking builtin to its math function
// (builtins.rs:782-798).
var unpackMathFunctions = map[string]ir.MathFunction{
	"unpackSnorm4x8":  ir.MathUnpack4x8snorm,
	"unpackUnorm4x8":  ir.MathUnpack4x8unorm,
	"unpackSnorm2x16": ir.MathUnpack2x16snorm,
	"unpackUnorm2x16": ir.MathUnpack2x16unorm,
	"unpackHalf2x16":  ir.MathUnpack2x16float,
}

// injectPackBuiltins adds one packing builtin, whose argument is the float
// vector it packs (builtins.rs:752-781).
func (f *Frontend) injectPackBuiltins(decl *functionDeclaration, name string) {
	fun, ok := packMathFunctions[name]
	if !ok {
		return
	}
	size := ir.Vec2
	if fun == ir.MathPack4x8snorm || fun == ir.MathPack4x8unorm {
		size = ir.Vec4
	}
	args := []ir.TypeInner{ir.VectorType{Size: size, Scalar: scalarF32}}
	decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: fun}))
}

// injectUnpackBuiltins adds one unpacking builtin, whose argument is the packed
// uint (builtins.rs:782-798).
func (f *Frontend) injectUnpackBuiltins(decl *functionDeclaration, name string) {
	fun, ok := unpackMathFunctions[name]
	if !ok {
		return
	}
	decl.overloads = append(decl.overloads, f.addBuiltinOverload([]ir.TypeInner{scalarU32}, &macroCall{kind: macroMath, math: fun}))
}

// injectAtanBuiltins adds atan and atan2: bit 0 picks between them and bits 1
// to 2 the dimension (builtins.rs:799-830).
func (f *Frontend) injectAtanBuiltins(decl *functionDeclaration) {
	for bits := 0; bits < 0b1000; bits++ {
		fun := ir.MathAtan
		if bits&0b1 != 0 {
			fun = ir.MathAtan2
		}
		ty := vectorOf(sizeOfBits(bits>>1), scalarF32)
		args := []ir.TypeInner{ty}
		if fun == ir.MathAtan2 {
			args = append(args, ty)
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: fun}))
	}
}

// injectLogicalBuiltins adds all, any and not, which take a boolean vector
// (builtins.rs:831-855).
func (f *Frontend) injectLogicalBuiltins(decl *functionDeclaration, name string) {
	sizes := [...]ir.VectorSize{ir.Vec2, ir.Vec3, ir.Vec4}
	for bits := 0; bits < len(sizes); bits++ {
		args := []ir.TypeInner{ir.VectorType{Size: sizes[bits], Scalar: scalarBool}}
		var macro *macroCall
		switch name {
		case "all":
			macro = &macroCall{kind: macroRelational, relational: ir.RelationalAll}
		case "any":
			macro = &macroCall{kind: macroRelational, relational: ir.RelationalAny}
		default:
			macro = &macroCall{kind: macroUnary, unary: ir.UnaryLogicalNot}
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
	}
}

// comparisonVariant is one vector comparison overload: the vector size and the
// scalar type of its components.
type comparisonVariant struct {
	size   ir.VectorSize
	scalar ir.ScalarType
}

// standardComparisonVariants are the vector comparisons of the standard
// variation: float, int and uint at every size (builtins.rs:857-883).
func standardComparisonVariants() []comparisonVariant {
	variants := make([]comparisonVariant, 0, 9)
	for _, scalar := range []ir.ScalarType{scalarF32, scalarI32, scalarU32} {
		for _, size := range []ir.VectorSize{ir.Vec2, ir.Vec3, ir.Vec4} {
			variants = append(variants, comparisonVariant{size: size, scalar: scalar})
		}
	}
	return variants
}

// equalityComparisonVariants adds the boolean vectors equal and notEqual also
// compare (builtins.rs:884-912).
func equalityComparisonVariants() []comparisonVariant {
	return append(standardComparisonVariants(), comparisonVariant{size: ir.Vec2, scalar: scalarBool},
		comparisonVariant{size: ir.Vec3, scalar: scalarBool}, comparisonVariant{size: ir.Vec4, scalar: scalarBool})
}

// doubleComparisonVariants are the vector comparisons of the double variation
// (builtins.rs:1188-1213).
func doubleComparisonVariants() []comparisonVariant {
	return []comparisonVariant{
		{size: ir.Vec2, scalar: scalarF64},
		{size: ir.Vec3, scalar: scalarF64},
		{size: ir.Vec4, scalar: scalarF64},
	}
}

// comparisonOperator maps a vector comparison builtin to its operator
// (builtins.rs:873-879, 904-908, 1201-1208).
func comparisonOperator(name string) ir.BinaryOperator {
	switch name {
	case "lessThan":
		return ir.BinaryLess
	case "greaterThan":
		return ir.BinaryGreater
	case "lessThanEqual":
		return ir.BinaryLessEqual
	case "greaterThanEqual":
		return ir.BinaryGreaterEqual
	case "equal":
		return ir.BinaryEqual
	default:
		return ir.BinaryNotEqual
	}
}

// injectComparisonBuiltins adds a vector comparison builtin over the given
// variants (builtins.rs:856-912).
func (f *Frontend) injectComparisonBuiltins(decl *functionDeclaration, name string, variants []comparisonVariant) {
	op := comparisonOperator(name)
	for _, variant := range variants {
		ty := ir.VectorType{Size: variant.size, Scalar: variant.scalar}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload([]ir.TypeInner{ty, ty}, &macroCall{kind: macroBinary, binary: op}))
	}
}

// splatShapes are the seven argument shapes the two-argument builtins that mix
// a vector with a splattable scalar use: (size, second size)
// (builtins.rs:924-932, 1080-1088, 1358-1366).
var splatShapes = [7]struct {
	size       *ir.VectorSize
	secondSize *ir.VectorSize
}{
	{size: nil, secondSize: nil},
	{size: sizePtr(ir.Vec2), secondSize: nil},
	{size: sizePtr(ir.Vec3), secondSize: nil},
	{size: sizePtr(ir.Vec4), secondSize: nil},
	{size: sizePtr(ir.Vec2), secondSize: sizePtr(ir.Vec2)},
	{size: sizePtr(ir.Vec3), secondSize: sizePtr(ir.Vec3)},
	{size: sizePtr(ir.Vec4), secondSize: sizePtr(ir.Vec4)},
}

// injectMinMaxBuiltins adds min and max for the given scalar kinds
// (builtins.rs:913-953, 1076-1110).
func (f *Frontend) injectMinMaxBuiltins(decl *functionDeclaration, name string, scalars ...ir.ScalarType) {
	fun := ir.MathMax
	if name == "min" {
		fun = ir.MathMin
	}
	for _, scalar := range scalars {
		for _, shape := range splatShapes {
			args := []ir.TypeInner{vectorOf(shape.size, scalar), vectorOf(shape.secondSize, scalar)}
			macro := &macroCall{kind: macroSplatted, math: fun, size: shape.size, arg: 1}
			decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
		}
	}
}

// mixGroup is one mix family: the scalar of its values and how its third
// argument is shaped (builtins.rs:968-974, 1126-1130).
type mixGroup struct {
	scalar   ir.ScalarType
	splatted bool
	boolean  bool
}

// addMixOverloads adds one mix family for every vector size it covers
// (builtins.rs:976-996).
func (f *Frontend) addMixOverloads(decl *functionDeclaration, sizes []*ir.VectorSize, group mixGroup) {
	for _, size := range sizes {
		third := vectorOf(size, group.scalar)
		switch {
		case group.boolean:
			third = vectorOf(size, scalarBool)
		case !group.splatted:
			third = group.scalar
		}
		args := []ir.TypeInner{vectorOf(size, group.scalar), vectorOf(size, group.scalar), third}
		macro := &macroCall{kind: macroSplatted, math: ir.MathMix, size: size, arg: 2}
		if group.boolean {
			macro = &macroCall{kind: macroMixBoolean}
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
	}
}

// injectMixBuiltins adds the standard mix overloads: the boolean form for int,
// uint and float, the scalar-weight form and the vector-weight form
// (builtins.rs:954-998).
func (f *Frontend) injectMixBuiltins(decl *functionDeclaration) {
	sizes := []*ir.VectorSize{sizePtr(ir.Vec2), sizePtr(ir.Vec3), sizePtr(ir.Vec4), nil}
	vectorSizes := sizes[:3]
	for _, group := range []mixGroup{
		{scalar: scalarI32, boolean: true},
		{scalar: scalarU32, boolean: true},
		{scalar: scalarF32, boolean: true},
		{scalar: scalarF32},
	} {
		f.addMixOverloads(decl, sizes, group)
	}
	f.addMixOverloads(decl, vectorSizes, mixGroup{scalar: scalarF32, splatted: true})
}

// injectDoubleMixBuiltins adds the double mix overloads. The sizes come in a
// different order than the standard ones, so the table is its own
// (builtins.rs:1111-1154).
func (f *Frontend) injectDoubleMixBuiltins(decl *functionDeclaration) {
	sizes := []*ir.VectorSize{sizePtr(ir.Vec4), sizePtr(ir.Vec2), sizePtr(ir.Vec3), nil}
	f.addMixOverloads(decl, sizes, mixGroup{scalar: scalarF64})
	f.addMixOverloads(decl, sizes, mixGroup{scalar: scalarF64, boolean: true})
	f.addMixOverloads(decl, sizes[:3], mixGroup{scalar: scalarF64, splatted: true})
}

// addClampOverloads adds the clamp overloads of one family
// (builtins.rs:1022-1035).
func (f *Frontend) addClampOverloads(decl *functionDeclaration, scalar ir.ScalarType, sizes []*ir.VectorSize, splatted bool) {
	for _, size := range sizes {
		limit := vectorOf(size, scalar)
		if splatted {
			limit = scalar
		}
		args := []ir.TypeInner{vectorOf(size, scalar), limit, limit}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroClamp, size: size}))
	}
}

// injectClampBuiltins adds the standard clamp overloads: float, int and uint at
// every size, with vector and splatted scalar bounds
// (builtins.rs:999-1037).
func (f *Frontend) injectClampBuiltins(decl *functionDeclaration) {
	for _, scalar := range []ir.ScalarType{scalarF32, scalarI32, scalarU32} {
		for _, size := range []*ir.VectorSize{sizePtr(ir.Vec2), sizePtr(ir.Vec3), sizePtr(ir.Vec4), nil} {
			f.addClampOverloads(decl, scalar, []*ir.VectorSize{size}, false)
			if size != nil {
				f.addClampOverloads(decl, scalar, []*ir.VectorSize{size}, true)
			}
		}
	}
}

// injectDoubleClampBuiltins adds the double clamp overloads
// (builtins.rs:1155-1187).
func (f *Frontend) injectDoubleClampBuiltins(decl *functionDeclaration) {
	sizes := []*ir.VectorSize{sizePtr(ir.Vec2), sizePtr(ir.Vec3), sizePtr(ir.Vec4)}
	f.addClampOverloads(decl, scalarF64, sizes, true)
	f.addClampOverloads(decl, scalarF64, []*ir.VectorSize{sizePtr(ir.Vec2), sizePtr(ir.Vec3), sizePtr(ir.Vec4), nil}, false)
}

// injectDoubleBuiltins adds the builtins that need the double capability, Rust
// naga's inject_double_builtin (builtins.rs:1047-1217).
func (f *Frontend) injectDoubleBuiltins(decl *functionDeclaration, name string) {
	switch name {
	case "abs", "sign":
		f.injectAbsSignBuiltins(decl, name, scalarF64)
	case "min", "max":
		f.injectMinMaxBuiltins(decl, name, scalarF64)
	case "mix":
		f.injectDoubleMixBuiltins(decl)
	case "clamp":
		f.injectDoubleClampBuiltins(decl)
	case "lessThan", "greaterThan", "lessThanEqual", "greaterThanEqual", "equal", "notEqual":
		f.injectComparisonBuiltins(decl, name, doubleComparisonVariants())
	default:
		f.injectCommonBuiltins(decl, name, 8)
	}
}

// injectCommonBuiltins adds the builtins that come in a float and a double
// variant, Rust naga's inject_common_builtin (builtins.rs:1220-1534).
func (f *Frontend) injectCommonBuiltins(decl *functionDeclaration, name string, floatWidth uint8) {
	floatScalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: floatWidth}
	switch name {
	case "ceil", "round", "roundEven", "floor", "fract", "trunc", "sqrt", "inversesqrt",
		"normalize", "length", "isinf", "isnan":
		f.injectOneArgCommonBuiltins(decl, name, floatScalar)
	case "dot", "reflect", "distance", "ldexp":
		f.injectDotBuiltins(decl, name, floatScalar)
	case "transpose":
		f.injectTransposeBuiltins(decl, floatScalar)
	case "inverse", "determinant":
		f.injectInverseBuiltins(decl, name, floatScalar)
	case "mod", "step":
		f.injectModStepBuiltins(decl, name, floatScalar)
	case "cross":
		cross := ir.VectorType{Size: ir.Vec3, Scalar: floatScalar}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload([]ir.TypeInner{cross, cross},
			&macroCall{kind: macroMath, math: ir.MathCross}))
	case "outerProduct":
		f.injectOuterProductBuiltins(decl, floatScalar)
	case "faceforward", "fma":
		f.injectTernaryBuiltins(decl, name, floatScalar)
	case "refract":
		f.injectRefractBuiltins(decl, floatScalar)
	case "smoothstep":
		f.injectSmoothStepBuiltins(decl, floatScalar)
	}
}

// commonOneArgMacro maps a single-argument common builtin to its expansion
// (builtins.rs:1251-1264).
func commonOneArgMacro(name string) *macroCall {
	switch name {
	case "ceil":
		return &macroCall{kind: macroMath, math: ir.MathCeil}
	case "round", "roundEven":
		return &macroCall{kind: macroMath, math: ir.MathRound}
	case "floor":
		return &macroCall{kind: macroMath, math: ir.MathFloor}
	case "fract":
		return &macroCall{kind: macroMath, math: ir.MathFract}
	case "trunc":
		return &macroCall{kind: macroMath, math: ir.MathTrunc}
	case "sqrt":
		return &macroCall{kind: macroMath, math: ir.MathSqrt}
	case "inversesqrt":
		return &macroCall{kind: macroMath, math: ir.MathInverseSqrt}
	case "normalize":
		return &macroCall{kind: macroMath, math: ir.MathNormalize}
	case "length":
		return &macroCall{kind: macroMath, math: ir.MathLength}
	case "isinf":
		return &macroCall{kind: macroRelational, relational: ir.RelationalIsInf}
	case "isnan":
		return &macroCall{kind: macroRelational, relational: ir.RelationalIsNan}
	default:
		return &macroCall{kind: macroMath}
	}
}

// injectOneArgCommonBuiltins adds the single-argument builtins of the common
// family (builtins.rs:1231-1268).
func (f *Frontend) injectOneArgCommonBuiltins(decl *functionDeclaration, name string, floatScalar ir.ScalarType) {
	macro := commonOneArgMacro(name)
	for bits := 0; bits < 0b100; bits++ {
		args := []ir.TypeInner{vectorOf(sizeOfBits(bits), floatScalar)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
	}
}

// injectDotBuiltins adds dot, reflect, distance and ldexp, whose second
// argument is a vector of exponents for ldexp (builtins.rs:1269-1301).
func (f *Frontend) injectDotBuiltins(decl *functionDeclaration, name string, floatScalar ir.ScalarType) {
	fun := ir.MathDot
	switch name {
	case "reflect":
		fun = ir.MathReflect
	case "distance":
		fun = ir.MathDistance
	case "ldexp":
		fun = ir.MathLdexp
	}
	second := floatScalar
	if fun == ir.MathLdexp {
		second = scalarI32
	}
	for bits := 0; bits < 0b100; bits++ {
		size := sizeOfBits(bits)
		args := []ir.TypeInner{vectorOf(size, floatScalar), vectorOf(size, second)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: fun}))
	}
}

// matrixShapes are the nine non-square and square matrix shapes the matrix
// builtins cover, as (rows, columns) (builtins.rs:1305-1316).
var matrixShapes = [9]struct {
	rows    ir.VectorSize
	columns ir.VectorSize
}{
	{rows: ir.Vec2, columns: ir.Vec2},
	{rows: ir.Vec2, columns: ir.Vec3},
	{rows: ir.Vec2, columns: ir.Vec4},
	{rows: ir.Vec3, columns: ir.Vec2},
	{rows: ir.Vec3, columns: ir.Vec3},
	{rows: ir.Vec3, columns: ir.Vec4},
	{rows: ir.Vec4, columns: ir.Vec2},
	{rows: ir.Vec4, columns: ir.Vec3},
	{rows: ir.Vec4, columns: ir.Vec4},
}

// injectTransposeBuiltins adds transpose (builtins.rs:1302-1327).
func (f *Frontend) injectTransposeBuiltins(decl *functionDeclaration, floatScalar ir.ScalarType) {
	for _, shape := range matrixShapes {
		args := []ir.TypeInner{matrixOf(shape.rows, shape.columns, floatScalar)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: ir.MathTranspose}))
	}
}

// injectInverseBuiltins adds inverse and determinant, which take a square
// matrix (builtins.rs:1328-1353).
func (f *Frontend) injectInverseBuiltins(decl *functionDeclaration, name string, floatScalar ir.ScalarType) {
	fun := ir.MathInverse
	if name == "determinant" {
		fun = ir.MathDeterminant
	}
	for _, size := range []ir.VectorSize{ir.Vec2, ir.Vec3, ir.Vec4} {
		args := []ir.TypeInner{matrixOf(size, size, floatScalar)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: fun}))
	}
}

// injectModStepBuiltins adds mod and step. step's edge is its first argument,
// which is the scalar that gets splatted (builtins.rs:1354-1394).
func (f *Frontend) injectModStepBuiltins(decl *functionDeclaration, name string, floatScalar ir.ScalarType) {
	step := name == "step"
	for _, shape := range splatShapes {
		// Rust swaps the two argument shapes for step, so that the edge comes
		// first (builtins.rs:1371-1384).
		first, second := shape.size, shape.secondSize
		if step {
			first, second = shape.secondSize, shape.size
		}
		args := []ir.TypeInner{vectorOf(first, floatScalar), vectorOf(second, floatScalar)}
		macro := &macroCall{kind: macroMod, size: shape.size}
		if step {
			macro = &macroCall{kind: macroSplatted, math: ir.MathStep, size: shape.size, arg: 0}
		}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, macro))
	}
}

// injectOuterProductBuiltins adds outerProduct, which takes two vectors of any
// two sizes (builtins.rs:1413-1444).
func (f *Frontend) injectOuterProductBuiltins(decl *functionDeclaration, floatScalar ir.ScalarType) {
	for _, first := range []ir.VectorSize{ir.Vec2, ir.Vec3, ir.Vec4} {
		for _, second := range []ir.VectorSize{ir.Vec2, ir.Vec3, ir.Vec4} {
			args := []ir.TypeInner{
				ir.VectorType{Size: first, Scalar: floatScalar},
				ir.VectorType{Size: second, Scalar: floatScalar},
			}
			decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: ir.MathOuter}))
		}
	}
}

// injectTernaryBuiltins adds faceforward and fma, whose three arguments share
// one type (builtins.rs:1445-1473).
func (f *Frontend) injectTernaryBuiltins(decl *functionDeclaration, name string, floatScalar ir.ScalarType) {
	fun := ir.MathFaceForward
	if name == "fma" {
		fun = ir.MathFma
	}
	for bits := 0; bits < 0b100; bits++ {
		ty := vectorOf(sizeOfBits(bits), floatScalar)
		args := []ir.TypeInner{ty, ty, ty}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: fun}))
	}
}

// injectRefractBuiltins adds refract, whose index of refraction is always a
// 32-bit float whatever the vector's width (builtins.rs:1474-1497).
func (f *Frontend) injectRefractBuiltins(decl *functionDeclaration, floatScalar ir.ScalarType) {
	for bits := 0; bits < 0b100; bits++ {
		ty := vectorOf(sizeOfBits(bits), floatScalar)
		args := []ir.TypeInner{ty, ty, scalarF32}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroMath, math: ir.MathRefract}))
	}
}

// injectSmoothStepBuiltins adds smoothstep, whose first two arguments are the
// ones a scalar edge is splatted into (builtins.rs:1498-1530).
func (f *Frontend) injectSmoothStepBuiltins(decl *functionDeclaration, floatScalar ir.ScalarType) {
	for bits := 0; bits < 0b1000; bits++ {
		splatted := bits&0b1 == 0b1
		size := sizeOfBits(bits >> 1)
		if splatted && size == nil {
			// Splatting a scalar into a scalar is the same overload again.
			continue
		}
		edge := vectorOf(size, floatScalar)
		if splatted {
			edge = floatScalar
		}
		args := []ir.TypeInner{edge, edge, vectorOf(size, floatScalar)}
		decl.overloads = append(decl.overloads, f.addBuiltinOverload(args, &macroCall{kind: macroSmoothStep, size: size}))
	}
}

// textureArgsOptions selects which image shapes a texture builtin is generated
// for, Rust naga's TextureArgsOptions (builtins.rs:2281-2295).
type textureArgsOptions uint32

const (
	// textureArgsMulti generates the multisampled image variants.
	textureArgsMulti textureArgsOptions = 1 << iota
	// textureArgsShadow generates the depth image variants.
	textureArgsShadow
	// textureArgsStandard generates the plain image variants.
	textureArgsStandard
	// textureArgsCubeArray generates the cube array variants.
	textureArgsCubeArray
	// textureArgsD2MultiArray generates the 2D multisampled array variants.
	textureArgsD2MultiArray
)

// textureArgsOptionsOf maps the builtin variations to the generator options
// they stand for, Rust naga's From<BuiltinVariations> for TextureArgsOptions
// (builtins.rs:2297-2311).
func textureArgsOptionsOf(variations builtinVariations) textureArgsOptions {
	var options textureArgsOptions
	if variations.has(variationStandard) {
		options |= textureArgsStandard
	}
	if variations.has(variationCubeTexturesArray) {
		options |= textureArgsCubeArray
	}
	if variations.has(variationD2MultiTexturesArray) {
		options |= textureArgsD2MultiArray
	}
	return options
}

// textureArgsFunc receives one image shape the generator produces.
type textureArgsFunc func(kind ir.ScalarKind, dim ir.ImageDimension, arrayed, multi, shadow bool)

// textureArgsGenerator calls f with every image shape the options select, Rust
// naga's texture_args_generator (builtins.rs:2313-2365).
func (f *Frontend) textureArgsGenerator(options textureArgsOptions, fn textureArgsFunc) {
	for _, kind := range [...]ir.ScalarKind{ir.ScalarFloat, ir.ScalarUint, ir.ScalarSint} {
		for _, dim := range [...]ir.ImageDimension{ir.Dim1D, ir.Dim2D, ir.Dim3D, ir.DimCube} {
			for _, arrayed := range [...]bool{false, true} {
				if dim == ir.DimCube && arrayed {
					if !options.has(textureArgsCubeArray) {
						continue
					}
				} else if dim == ir.Dim2D && options.has(textureArgsMulti) && arrayed && options.has(textureArgsD2MultiArray) {
					// The multisampled 2D array image the standard variation
					// does not request on its own.
					fn(kind, dim, arrayed, true, false)
				} else if !options.has(textureArgsStandard) {
					continue
				}

				fn(kind, dim, arrayed, false, false)

				// A 3D image is never arrayed and never a shadow image, so the
				// loop breaks before either branch below.
				if dim == ir.Dim3D {
					break
				}

				if dim == ir.Dim2D && options.has(textureArgsMulti) && !arrayed {
					fn(kind, dim, arrayed, true, false)
				}

				if kind == ir.ScalarFloat && options.has(textureArgsShadow) {
					fn(kind, dim, arrayed, false, true)
				}
			}
		}
	}
}

// has reports whether every bit of other is set in o.
func (o textureArgsOptions) has(other textureArgsOptions) bool { return o&other == other }

// builtinRequiredVariations reports the non-standard builtin variations the
// argument types need, Rust naga's builtin_required_variations
// (functions.rs:1591-1624): a double argument asks for the double overloads, a
// cube array image for the cube array ones, and a multisampled 2D array image
// for the 2D multisampled array ones.
func builtinRequiredVariations(types ...ir.TypeInner) builtinVariations {
	var variations builtinVariations
	for _, ty := range types {
		switch t := ty.(type) {
		case ir.ScalarType:
			if isFloat64(t) {
				variations |= variationDouble
			}
		case ir.VectorType:
			if isFloat64(t.Scalar) {
				variations |= variationDouble
			}
		case ir.MatrixType:
			if isFloat64(t.Scalar) {
				variations |= variationDouble
			}
		case ir.ValuePointerType:
			if isFloat64(t.Scalar) {
				variations |= variationDouble
			}
		case ir.ImageType:
			if t.Dim == ir.DimCube && t.Arrayed {
				variations |= variationCubeTexturesArray
			}
			if t.Dim == ir.Dim2D && t.Arrayed && imageMultisampled(t) {
				variations |= variationD2MultiTexturesArray
			}
		}
	}
	return variations
}

// imageMultisampled reports whether an image carries more than one sample per
// texel, Rust naga's ImageClass::is_multisampled: only the sampled and depth
// classes can (builtins.rs:1615).
func imageMultisampled(image ir.ImageType) bool {
	switch image.Class {
	case ir.ImageClassSampled, ir.ImageClassDepth:
		return image.Multisampled
	default:
		return false
	}
}
