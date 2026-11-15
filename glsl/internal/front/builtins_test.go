package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// builtinOverload returns the overload of decl whose parameters are exactly
// params, failing the test when the catalog has none.
func builtinOverload(t *testing.T, f *Frontend, decl *functionDeclaration, params ...ir.TypeInner) *overload {
	t.Helper()
	for _, candidate := range decl.overloads {
		if len(candidate.parameters) != len(params) {
			continue
		}
		matches := true
		for i, parameter := range candidate.parameters {
			if !typeInnerEqual(f.typeInner(parameter), params[i]) {
				matches = false
				break
			}
		}
		if matches {
			return candidate
		}
	}
	t.Fatalf("no builtin overload with %d parameter(s) %v", len(params), params)
	return nil
}

// callBuiltin expands ov over args and returns its result.
func callBuiltin(t *testing.T, c *Context, ov *overload, args ...ir.ExpressionHandle) (ir.ExpressionHandle, bool) {
	t.Helper()
	lowered := make([]loweredArg, len(args))
	for i, arg := range args {
		lowered[i] = loweredArg{expr: arg, span: ir.Span{}}
	}
	h, ok, err := ov.macro.call(c, lowered, ir.Span{})
	if err != nil {
		t.Fatalf("builtin call: %v", err)
	}
	return h, ok
}

// dynamicArg returns a value that is not known at compile time, a load of a
// fresh local, so that the expansion under test is not folded away by the
// constant evaluator — Rust's expansions of a runtime argument are not either.
func dynamicArg(t *testing.T, c *Context, ty ir.TypeHandle) ir.ExpressionHandle {
	t.Helper()
	sym := declareLocal(t, c, "arg", ty)
	arena := &hirArena{}
	root := arena.add(hirVariable{Sym: sym}, ir.Span{})
	h, _, err := c.lowerExpect(arena, root, rhsPos)
	if err != nil {
		t.Fatalf("lowerExpect: %v", err)
	}
	return h
}

// mustMath returns the Math expression at h.
func mustMath(t *testing.T, c *Context, h ir.ExpressionHandle) ir.ExprMath {
	t.Helper()
	kind, ok := c.Expression(h).(ir.ExprMath)
	if !ok {
		t.Fatalf("expression %d is %T, want ExprMath", h, c.Expression(h))
	}
	return kind
}

func TestBuiltinAbsF32(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "abs", variationStandard)

	ov := builtinOverload(t, f, decl, scalarF32)
	if ov.kind != overloadMacro || !ov.internal || ov.macro == nil {
		t.Fatalf("overload is not an internal macro: %+v", ov)
	}
	arg := dynamicArg(t, c, f.registerType(scalarF32))
	h, ok := callBuiltin(t, c, ov, arg)
	if !ok {
		t.Fatal("abs reported no value")
	}
	math := mustMath(t, c, h)
	if math.Fun != ir.MathAbs || math.Arg != arg || math.Arg1 != nil {
		t.Fatalf("abs lowered to %+v, want MathAbs over the argument", math)
	}
}

func TestBuiltinModExpandsToFloorForm(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "mod", variationStandard)

	ov := builtinOverload(t, f, decl, scalarF32, scalarF32)
	left := dynamicArg(t, c, f.registerType(scalarF32))
	right := dynamicArg(t, c, f.registerType(scalarF32))
	h, ok := callBuiltin(t, c, ov, left, right)
	if !ok {
		t.Fatal("mod reported no value")
	}

	// x - y * floor(x / y)
	sub, ok := c.Expression(h).(ir.ExprBinary)
	if !ok || sub.Op != ir.BinarySubtract {
		t.Fatalf("mod outer expression is %T (%+v), want BinarySubtract", c.Expression(h), c.Expression(h))
	}
	mult, ok := c.Expression(sub.Right).(ir.ExprBinary)
	if !ok || mult.Op != ir.BinaryMultiply {
		t.Fatalf("mod factor is %T, want BinaryMultiply", c.Expression(sub.Right))
	}
	floor := mustMath(t, c, mult.Left)
	if floor.Fun != ir.MathFloor {
		t.Fatalf("mod floor is %v, want MathFloor", floor.Fun)
	}
	div, ok := c.Expression(floor.Arg).(ir.ExprBinary)
	if !ok || div.Op != ir.BinaryDivide || div.Left != left || div.Right != right {
		t.Fatalf("mod division is %T, want left / right", c.Expression(floor.Arg))
	}
}

func TestBuiltinMixBooleanSelects(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "mix", variationStandard)

	ov := builtinOverload(t, f, decl, scalarF32, scalarF32, scalarBool)
	if ov.macro.kind != macroMixBoolean {
		t.Fatalf("boolean mix expansion is %v, want macroMixBoolean", ov.macro.kind)
	}
	reject := dynamicArg(t, c, f.registerType(scalarF32))
	accept := dynamicArg(t, c, f.registerType(scalarF32))
	condition := dynamicArg(t, c, f.registerType(scalarBool))
	h, _ := callBuiltin(t, c, ov, reject, accept, condition)

	sel, ok := c.Expression(h).(ir.ExprSelect)
	if !ok {
		t.Fatalf("mix lowered to %T, want ExprSelect", c.Expression(h))
	}
	if sel.Condition != condition || sel.Accept != accept || sel.Reject != reject {
		t.Fatalf("select is %+v, want the condition third and the values in GLSL order", sel)
	}
}

func TestBuiltinClampSplatsBounds(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "clamp", variationStandard)

	vector := ir.VectorType{Size: ir.Vec4, Scalar: scalarF32}
	ov := builtinOverload(t, f, decl, vector, scalarF32, scalarF32)
	value := dynamicArg(t, c, f.registerType(vector))
	low := dynamicArg(t, c, f.registerType(scalarF32))
	high := dynamicArg(t, c, f.registerType(scalarF32))
	h, _ := callBuiltin(t, c, ov, value, low, high)

	math := mustMath(t, c, h)
	if math.Fun != ir.MathClamp || math.Arg != value || math.Arg1 == nil || math.Arg2 == nil {
		t.Fatalf("clamp lowered to %+v, want MathClamp with both bounds", math)
	}
	// A scalar bound of a vector clamp is splatted to the vector's size.
	splat, ok := c.Expression(*math.Arg1).(ir.ExprSplat)
	if !ok || splat.Size != ir.Vec4 {
		t.Fatalf("clamp lower bound is %T, want a Vec4 splat", c.Expression(*math.Arg1))
	}
}

func TestBuiltinFindLsbUintConvertsResult(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "findLSB", variationStandard)

	ov := builtinOverload(t, f, decl, scalarU32)
	if ov.macro.kind != macroFindLsbUint {
		t.Fatalf("uint findLSB expansion is %v, want macroFindLsbUint", ov.macro.kind)
	}
	arg := dynamicArg(t, c, f.registerType(scalarU32))
	h, _ := callBuiltin(t, c, ov, arg)

	cast, ok := c.Expression(h).(ir.ExprAs)
	if !ok || cast.Kind != ir.ScalarSint || cast.Convert == nil || *cast.Convert != 4 {
		t.Fatalf("findLSB lowered to %T (%+v), want a conversion to sint", c.Expression(h), c.Expression(h))
	}
	math := mustMath(t, c, cast.Expr)
	if math.Fun != ir.MathFirstTrailingBit {
		t.Fatalf("findLSB is %v, want MathFirstTrailingBit", math.Fun)
	}
}

func TestBuiltinFloatBitsToIntBitcasts(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "floatBitsToInt", variationStandard)

	ov := builtinOverload(t, f, decl, scalarF32)
	arg := dynamicArg(t, c, f.registerType(scalarF32))
	h, _ := callBuiltin(t, c, ov, arg)

	cast, ok := c.Expression(h).(ir.ExprAs)
	if !ok || cast.Kind != ir.ScalarSint {
		t.Fatalf("floatBitsToInt lowered to %T, want a conversion to sint", c.Expression(h))
	}
	// A bit cast keeps the bit pattern, so it carries no width conversion.
	if cast.Convert != nil {
		t.Fatalf("floatBitsToInt converts the width to %d, want a plain bit cast", *cast.Convert)
	}
}

// declareImageGlobal adds a handle-space global of ty and returns the
// expression naming it.
func declareImageGlobal(t *testing.T, f *Frontend, c *Context, name string, ty ir.TypeHandle) ir.ExpressionHandle {
	t.Helper()
	h := ir.GlobalVariableHandle(len(f.mod.GlobalVariables))
	f.mod.GlobalVariables = append(f.mod.GlobalVariables, ir.GlobalVariable{
		Name:  name,
		Space: ir.SpaceHandle,
		Type:  ty,
	})
	return mustExpr(t, c, ir.ExprGlobalVariable{Variable: h})
}

func TestBuiltinTextureSamplesWithSampler(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "texture", variationStandard)

	imageTy := f.registerType(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})
	samplerTy := f.registerType(ir.SamplerType{})
	coordsTy := f.registerType(ir.VectorType{Size: ir.Vec2, Scalar: scalarF32})

	image := declareImageGlobal(t, f, c, "tex", imageTy)
	sampler := declareImageGlobal(t, f, c, "tex_sampler", samplerTy)
	c.samplers[image] = sampler

	coords := dynamicArg(t, c, coordsTy)
	ov := builtinOverload(t, f, decl, f.typeInner(imageTy), f.typeInner(coordsTy))
	h, ok := callBuiltin(t, c, ov, image, coords)
	if !ok {
		t.Fatal("texture reported no value")
	}

	sample, ok := c.Expression(h).(ir.ExprImageSample)
	if !ok {
		t.Fatalf("texture lowered to %T, want ExprImageSample", c.Expression(h))
	}
	if sample.Image != image || sample.Sampler != sampler || sample.Coordinate != coords {
		t.Fatalf("sample is %+v, want the image, its sampler and the coordinate", sample)
	}
	if _, ok := sample.Level.(ir.SampleLevelAuto); !ok {
		t.Fatalf("level is %T, want SampleLevelAuto", sample.Level)
	}
	if sample.ArrayIndex != nil || sample.DepthRef != nil || sample.Offset != nil {
		t.Fatalf("sample has unexpected components: %+v", sample)
	}
}

func TestBuiltinTextureArrayIndexIsSint(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "texture", variationStandard)

	imageTy := f.registerType(ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})
	samplerTy := f.registerType(ir.SamplerType{})
	coordsTy := f.registerType(ir.VectorType{Size: ir.Vec3, Scalar: scalarF32})

	image := declareImageGlobal(t, f, c, "tex", imageTy)
	sampler := declareImageGlobal(t, f, c, "tex_sampler", samplerTy)
	c.samplers[image] = sampler

	coords := dynamicArg(t, c, coordsTy)
	ov := builtinOverload(t, f, decl, f.typeInner(imageTy), f.typeInner(coordsTy))
	h, _ := callBuiltin(t, c, ov, image, coords)

	sample, ok := c.Expression(h).(ir.ExprImageSample)
	if !ok {
		t.Fatalf("texture lowered to %T, want ExprImageSample", c.Expression(h))
	}
	if sample.ArrayIndex == nil {
		t.Fatal("an arrayed texture has no array index")
	}
	// The layer is the coordinate's third component, converted to a signed
	// integer for the IR.
	index, ok := c.Expression(*sample.ArrayIndex).(ir.ExprAs)
	if !ok || index.Kind != ir.ScalarSint {
		t.Fatalf("array index is %T, want a conversion to sint", c.Expression(*sample.ArrayIndex))
	}
	access, ok := c.Expression(index.Expr).(ir.ExprAccessIndex)
	if !ok || access.Index != 2 {
		t.Fatalf("array index reads %T, want component 2", c.Expression(index.Expr))
	}
}

func TestBuiltinTextureWithoutSamplerFails(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "texture", variationStandard)

	imageTy := f.registerType(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})
	coordsTy := f.registerType(ir.VectorType{Size: ir.Vec2, Scalar: scalarF32})

	image := declareImageGlobal(t, f, c, "tex", imageTy)
	coords := dynamicArg(t, c, coordsTy)
	ov := builtinOverload(t, f, decl, f.typeInner(imageTy), f.typeInner(coordsTy))

	_, _, err := ov.macro.call(c, []loweredArg{{expr: image}, {expr: coords}}, ir.Span{})
	if err == nil || err.Message != "Bad call" {
		t.Fatalf("error is %v, want Rust's \"Bad call\"", err)
	}
}

func TestBuiltinImageStoreIsVoid(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "imageStore", variationStandard)

	imageTy := f.registerType(ir.ImageType{
		Dim:           ir.Dim2D,
		Class:         ir.ImageClassStorage,
		SampledKind:   ir.ScalarFloat,
		StorageFormat: ir.StorageFormatR16Float,
		StorageAccess: ir.StorageAccessStore,
	})
	coordsTy := f.registerType(ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}})
	valueTy := f.registerType(ir.VectorType{Size: ir.Vec4, Scalar: scalarF32})

	image := declareImageGlobal(t, f, c, "img", imageTy)
	coords := dynamicArg(t, c, coordsTy)
	value := dynamicArg(t, c, valueTy)
	ov := builtinOverload(t, f, decl, f.typeInner(imageTy), f.typeInner(coordsTy), f.typeInner(valueTy))

	_, ok := callBuiltin(t, c, ov, image, coords, value)
	if ok {
		t.Fatal("imageStore reported a value, it is a statement")
	}
	var store *ir.StmtImageStore
	for _, stmt := range c.body {
		if s, isStore := stmt.Kind.(ir.StmtImageStore); isStore {
			store = &s
		}
	}
	if store == nil {
		t.Fatal("imageStore emitted no image store statement")
	}
	if store.Image != image || store.Coordinate != coords || store.Value != value {
		t.Fatalf("store is %+v, want the image, coordinate and value", store)
	}
}

func TestBuiltinSamplerShadowMarksDepth(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "sampler2DShadow", variationStandard)

	sampledTy := f.registerType(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})
	comparisonTy := f.registerType(ir.SamplerType{Comparison: true})

	image := declareImageGlobal(t, f, c, "shadow_map", sampledTy)
	sampler := declareImageGlobal(t, f, c, "shadow_map_sampler", comparisonTy)

	ov := builtinOverload(t, f, decl, f.typeInner(sampledTy), f.typeInner(comparisonTy))
	h, ok := callBuiltin(t, c, ov, image, sampler)
	if !ok {
		t.Fatal("shadow sampler reported no value")
	}
	if h != image {
		t.Fatalf("shadow sampler returned expression %d, want the image %d", h, image)
	}

	// The image global becomes a depth image, and the sampler pair is recorded
	// so that texture() finds it.
	global := f.mod.GlobalVariables[0]
	img, isImage := f.typeInner(global.Type).(ir.ImageType)
	if !isImage || img.Class != ir.ImageClassDepth {
		t.Fatalf("shadow image type is %v, want a depth image", f.typeInner(global.Type))
	}
	recorded, ok := c.samplers[image]
	if !ok || recorded != sampler {
		t.Fatalf("samplers[%d] = %d, want the sampler %d", image, recorded, sampler)
	}
}

func TestBuiltinVariationsAreInjectedOnce(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "ceil", variationStandard)
	first := len(decl.overloads)
	if first == 0 {
		t.Fatal("ceil injected no overloads")
	}
	f.injectBuiltin(decl, "ceil", variationStandard)
	if len(decl.overloads) != first {
		t.Fatalf("second injection added %d overloads, want none", len(decl.overloads)-first)
	}

	// A new variation adds only its own overloads.
	f.injectBuiltin(decl, "ceil", variationStandard|variationDouble)
	if len(decl.overloads) == first {
		t.Fatal("the double variation added no overloads")
	}
	if !decl.variations.has(variationStandard | variationDouble) {
		t.Fatalf("declaration variations are %b, want both bits", decl.variations)
	}
}

func TestBuiltinRequiredVariations(t *testing.T) {
	cubeArray := ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	multiArray := ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	plainArray := ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	double := ir.VectorType{Size: ir.Vec3, Scalar: scalarF64}

	tests := []struct {
		name string
		args []ir.TypeInner
		want builtinVariations
	}{
		{"no arguments", nil, 0},
		{"float argument", []ir.TypeInner{scalarF32}, 0},
		{"double scalar", []ir.TypeInner{scalarF64}, variationDouble},
		{"double vector", []ir.TypeInner{double}, variationDouble},
		{"cube array image", []ir.TypeInner{cubeArray}, variationCubeTexturesArray},
		{"multisampled 2D array image", []ir.TypeInner{multiArray}, variationD2MultiTexturesArray},
		{"arrayed 2D image", []ir.TypeInner{plainArray}, 0},
		{"both image variations", []ir.TypeInner{cubeArray, multiArray}, variationCubeTexturesArray | variationD2MultiTexturesArray},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := builtinRequiredVariations(test.args...); got != test.want {
				t.Fatalf("variations are %b, want %b", got, test.want)
			}
		})
	}
}

func TestBuiltinUnsupportedNameInjectsNothing(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "notABuiltin", variationStandard)
	if len(decl.overloads) != 0 {
		t.Fatalf("injected %d overloads for an unknown name", len(decl.overloads))
	}
}

// TestBuiltinShaderCompiles checks that a builtin call reaches a module through
// the whole frontend: the overloads are injected on first use, resolved against
// the argument types and expanded.
func TestBuiltinShaderCompiles(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"abs", `#version 450
layout(location = 0) in vec4 inColor;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(abs(inColor.x));
}
`},
		{"mod", `#version 450
layout(location = 0) in vec4 inColor;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(mod(inColor.x, 2.0));
}
`},
		{"clamp with splatted bounds", `#version 450
layout(location = 0) in vec4 inColor;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(clamp(inColor.xyz, 0.0, 1.0), 1.0);
}
`},
		{"derivative", `#version 450
layout(location = 0) in vec4 inColor;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(dFdx(inColor.x), 0.0, 0.0, 1.0);
}
`},
		{"findLSB", `#version 450
layout(location = 0) flat in uvec4 inBits;
layout(location = 0) flat out int outBit;
void main() {
    outBit = findLSB(inBits.x);
}
`},
		{"floatBitsToUint", `#version 450
layout(location = 0) in vec4 inColor;
layout(location = 0) flat out uvec4 outBits;
void main() {
    outBits = uvec4(floatBitsToUint(inColor.x));
}
`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := Parse(test.src, Options{Stage: ir.StageFragment}); err != nil {
				t.Fatalf("Parse: %v", err)
			}
		})
	}
}
