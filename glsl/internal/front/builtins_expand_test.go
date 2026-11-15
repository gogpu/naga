package front

// This file tests the expansion half of the builtin catalog: macroCall::call,
// Rust naga's MacroCall::call (builtins.rs:1585-2060). Each expansion turns an
// already resolved overload into IR expressions and statements, so the shapes
// here — the coordinate split, the level of detail, the array index
// conversion, the operand conversions of the bit functions — are the contract
// the backends read.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// expandedBuiltin injects name, picks the overload with params and calls it
// with args.
func expandedBuiltin(t *testing.T, f *Frontend, c *Context, name string, params []ir.TypeInner, args ...ir.ExpressionHandle) (ir.ExpressionHandle, bool) {
	t.Helper()
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, name, variationStandard)
	ov := builtinOverload(t, f, decl, params...)
	return callBuiltin(t, c, ov, args...)
}

// textureContext builds a context holding one sampled image global and the
// sampler paired with it.
func textureContext(t *testing.T, image ir.TypeInner) (*Frontend, *Context, ir.ExpressionHandle) {
	t.Helper()
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	samplerTy := f.registerType(ir.SamplerType{})
	imageExpr := declareImageGlobal(t, f, c, "tex", imageTy)
	samplerExpr := declareImageGlobal(t, f, c, "tex_sampler", samplerTy)
	c.samplers[imageExpr] = samplerExpr
	return f, c, imageExpr
}

// TestBuiltinTextureQueryLevelsConvertsToSint checks that textureQueryLevels
// is an image query followed by a conversion to a signed 32-bit integer, the
// shape Rust naga gives every texture query result (builtins.rs:1775-1801).
func TestBuiltinTextureQueryLevelsConvertsToSint(t *testing.T) {
	image := sampledImage(ir.Dim2D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)

	h, ok := expandedBuiltin(t, f, c, "textureQueryLevels", []ir.TypeInner{image}, imageExpr)
	if !ok {
		t.Fatal("textureQueryLevels reported no value")
	}
	cast := exprAs[ir.ExprAs](t, c, h, "result")
	if cast.Kind != ir.ScalarSint || cast.Convert == nil || *cast.Convert != 4 {
		t.Fatalf("conversion is %+v, want a width-4 conversion to sint", cast)
	}
	query := exprAs[ir.ExprImageQuery](t, c, cast.Expr, "query")
	if query.Image != imageExpr {
		t.Fatalf("query image is %d, want %d", query.Image, imageExpr)
	}
	if _, isLevels := query.Query.(ir.ImageQueryNumLevels); !isLevels {
		t.Fatalf("query is %T, want ImageQueryNumLevels", query.Query)
	}
}

// TestBuiltinTextureSizeAppendsNumLayers checks that an arrayed textureSize
// builds a vector of the base size components and the layer count
// (builtins.rs:1719-1783).
func TestBuiltinTextureSizeAppendsNumLayers(t *testing.T) {
	image := sampledImage(ir.Dim2D, true, false, ir.ScalarFloat)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	imageExpr := declareImageGlobal(t, f, c, "tex", imageTy)
	level := dynamicArg(t, c, f.registerType(scalarI32))

	h, ok := expandedBuiltin(t, f, c, "textureSize", []ir.TypeInner{image, scalarI32}, imageExpr, level)
	if !ok {
		t.Fatal("textureSize reported no value")
	}
	cast := exprAs[ir.ExprAs](t, c, h, "result")
	if cast.Kind != ir.ScalarSint {
		t.Fatalf("result converts to %d, want sint", cast.Kind)
	}
	compose := exprAs[ir.ExprCompose](t, c, cast.Expr, "composed size")
	if len(compose.Components) != 3 {
		t.Fatalf("composed size has %d components, want 3 (x, y, layers)", len(compose.Components))
	}
	layers := exprAs[ir.ExprImageQuery](t, c, compose.Components[2], "layer count")
	if _, isLayers := layers.Query.(ir.ImageQueryNumLayers); !isLayers {
		t.Fatalf("last component is %T, want ImageQueryNumLayers", layers.Query)
	}
	vector, isVector := f.typeInner(compose.Type).(ir.VectorType)
	if !isVector || vector.Size != ir.Vec3 {
		t.Fatalf("composed type is %v, want a vec3", f.typeInner(compose.Type))
	}
}

// TestBuiltinTextureSizeOfNonVectorImage checks the scalar-image arm of the
// arrayed textureSize: a 1D image's size is one component, so the vector is
// the size and the layer count (builtins.rs:1745-1765).
func TestBuiltinTextureSizeOfNonVectorImage(t *testing.T) {
	image := sampledImage(ir.Dim1D, true, false, ir.ScalarFloat)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	imageExpr := declareImageGlobal(t, f, c, "tex", imageTy)
	level := dynamicArg(t, c, f.registerType(scalarI32))

	h, _ := expandedBuiltin(t, f, c, "textureSize", []ir.TypeInner{image, scalarI32}, imageExpr, level)
	cast := exprAs[ir.ExprAs](t, c, h, "result")
	compose := exprAs[ir.ExprCompose](t, c, cast.Expr, "composed size")
	if len(compose.Components) != 2 {
		t.Fatalf("composed size has %d components, want 2", len(compose.Components))
	}
	vector, isVector := f.typeInner(compose.Type).(ir.VectorType)
	if !isVector || vector.Size != ir.Vec2 {
		t.Fatalf("composed type is %v, want a vec2", f.typeInner(compose.Type))
	}
}

// TestBuiltinTexelFetchSplitsArrayCoordinate checks that texelFetch's
// coordinate is split into the image coordinate and the array index for an
// arrayed image (builtins.rs:2111-2209).
func TestBuiltinTexelFetchSplitsArrayCoordinate(t *testing.T) {
	image := sampledImage(ir.Dim2D, true, false, ir.ScalarFloat)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	imageExpr := declareImageGlobal(t, f, c, "tex", imageTy)
	coordTy := f.registerType(vecOf(ir.Vec3, scalarI32))
	coord := dynamicArg(t, c, coordTy)
	level := dynamicArg(t, c, f.registerType(scalarI32))

	h, ok := expandedBuiltin(t, f, c, "texelFetch", []ir.TypeInner{image, vecOf(ir.Vec3, scalarI32), scalarI32}, imageExpr, coord, level)
	if !ok {
		t.Fatal("texelFetch reported no value")
	}
	load := exprAs[ir.ExprImageLoad](t, c, h, "load")
	if load.ArrayIndex == nil {
		t.Fatal("an arrayed texelFetch has no array index")
	}
	access := exprAs[ir.ExprAccessIndex](t, c, *load.ArrayIndex, "array index")
	if access.Index != 2 {
		t.Fatalf("array index reads component %d, want 2", access.Index)
	}
	// The coordinate is resized from the three-component argument down to the
	// image's own two-component size.
	if load.Coordinate == coord {
		t.Fatal("the coordinate was not resized to the image size")
	}
	if load.Level == nil || *load.Level != level {
		t.Fatal("a mipmapped texelFetch carries no level")
	}
	if load.Sample != nil {
		t.Fatal("a mipmapped texelFetch carries a sample index")
	}
}

// TestBuiltinTexelFetchMultisampledUsesSample checks that a multisampled
// sampled image's texelFetch stores its third argument as the sample index
// rather than the mip level (builtins.rs:1820-1833).
func TestBuiltinTexelFetchMultisampledUsesSample(t *testing.T) {
	image := sampledImage(ir.Dim2D, false, true, ir.ScalarFloat)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	imageExpr := declareImageGlobal(t, f, c, "tex", imageTy)
	coordTy := f.registerType(vecOf(ir.Vec2, scalarI32))
	coord := dynamicArg(t, c, coordTy)
	sample := dynamicArg(t, c, f.registerType(scalarI32))

	h, ok := expandedBuiltin(t, f, c, "texelFetch", []ir.TypeInner{image, vecOf(ir.Vec2, scalarI32), scalarI32}, imageExpr, coord, sample)
	if !ok {
		t.Fatal("texelFetch reported no value")
	}
	load := exprAs[ir.ExprImageLoad](t, c, h, "load")
	if load.Sample == nil || *load.Sample != sample {
		t.Fatal("a multisampled fetch does not carry the sample index")
	}
	if load.Level != nil {
		t.Fatal("a multisampled fetch carries a mip level")
	}
}

// TestBuiltinTextureProjDividesCoordinates checks the projection divisor of
// textureProj: for a three-component coordinate the last component is
// splatted to a vector and divided component-wise into the resized
// coordinate (builtins.rs:1620-1650).
func TestBuiltinTextureProjDividesCoordinates(t *testing.T) {
	image := sampledImage(ir.Dim2D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)
	coordTy := f.registerType(vecOf(ir.Vec3, scalarF32))
	coord := dynamicArg(t, c, coordTy)

	h, ok := expandedBuiltin(t, f, c, "textureProj", []ir.TypeInner{image, vecOf(ir.Vec3, scalarF32)}, imageExpr, coord)
	if !ok {
		t.Fatal("textureProj reported no value")
	}
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	divide := exprAs[ir.ExprBinary](t, c, sample.Coordinate, "projected coordinate")
	if divide.Op != ir.BinaryDivide {
		t.Fatalf("projection op is %d, want divide", divide.Op)
	}
	splat := exprAs[ir.ExprSplat](t, c, divide.Right, "divisor")
	if splat.Size != ir.Vec2 {
		t.Fatalf("divisor splats to %d, want vec2", splat.Size)
	}
}

// TestBuiltinTextureProjVec2UsesScalarDivisor checks the two-component arm of
// the projection: the two components are divided without a splat
// (builtins.rs:1628-1634).
func TestBuiltinTextureProjVec2UsesScalarDivisor(t *testing.T) {
	image := sampledImage(ir.Dim1D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)
	coordTy := f.registerType(vecOf(ir.Vec2, scalarF32))
	coord := dynamicArg(t, c, coordTy)

	h, ok := expandedBuiltin(t, f, c, "textureProj", []ir.TypeInner{image, vecOf(ir.Vec2, scalarF32)}, imageExpr, coord)
	if !ok {
		t.Fatal("textureProj reported no value")
	}
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	divide := exprAs[ir.ExprBinary](t, c, sample.Coordinate, "projected coordinate")
	if divide.Op != ir.BinaryDivide {
		t.Fatalf("projection op is %d, want divide", divide.Op)
	}
	left := exprAs[ir.ExprAccessIndex](t, c, divide.Left, "projected value")
	right := exprAs[ir.ExprAccessIndex](t, c, divide.Right, "divisor")
	if left.Index != 0 || right.Index != 1 {
		t.Fatalf("projection reads components %d and %d, want 0 and 1", left.Index, right.Index)
	}
}

// TestBuiltinTextureBiasIsAutoLevelBias checks that a fourth argument of the
// plain texture() form is the automatic-level bias (builtins.rs:1712-1717).
func TestBuiltinTextureBiasIsAutoLevelBias(t *testing.T) {
	image := sampledImage(ir.Dim2D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)
	coordTy := f.registerType(vecOf(ir.Vec2, scalarF32))
	coord := dynamicArg(t, c, coordTy)
	bias := dynamicArg(t, c, f.registerType(scalarF32))

	h, _ := expandedBuiltin(t, f, c, "texture", []ir.TypeInner{image, vecOf(ir.Vec2, scalarF32), scalarF32}, imageExpr, coord, bias)
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	level, isBias := sample.Level.(ir.SampleLevelBias)
	if !isBias || level.Bias != bias {
		t.Fatalf("level is %T, want a bias over the fourth argument", sample.Level)
	}
}

// TestBuiltinShadowLodUsesLevelZero checks that a shadow sampler's explicit
// LOD is lowered as level zero: the IR cannot express it
// (builtins.rs:1668-1678). The depth reference is the coordinate's last
// component.
func TestBuiltinShadowLodUsesLevelZero(t *testing.T) {
	image := depthImage(ir.Dim2D, false)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	samplerTy := f.registerType(ir.SamplerType{Comparison: true})
	imageExpr := declareImageGlobal(t, f, c, "depth_map", imageTy)
	samplerExpr := declareImageGlobal(t, f, c, "depth_map_sampler", samplerTy)
	c.samplers[imageExpr] = samplerExpr
	coordTy := f.registerType(vecOf(ir.Vec3, scalarF32))
	coord := dynamicArg(t, c, coordTy)
	lod := dynamicArg(t, c, f.registerType(scalarF32))

	params := []ir.TypeInner{image, vecOf(ir.Vec3, scalarF32), scalarF32}
	h, _ := expandedBuiltin(t, f, c, "textureLod", params, imageExpr, coord, lod)
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	if _, isZero := sample.Level.(ir.SampleLevelZero); !isZero {
		t.Fatalf("level is %T, want SampleLevelZero for a shadow sampler", sample.Level)
	}
	if sample.DepthRef == nil {
		t.Fatal("the shadow sample has no depth reference")
	}
	depth := exprAs[ir.ExprAccessIndex](t, c, *sample.DepthRef, "depth reference")
	if depth.Index != 2 {
		t.Fatalf("depth reference reads component %d, want 2", depth.Index)
	}
}

// TestBuiltinShadowGradientUsesLevelZero checks the gradient arm of the same
// rule for a shadow sampler (builtins.rs:1680-1696).
func TestBuiltinShadowGradientUsesLevelZero(t *testing.T) {
	image := depthImage(ir.Dim2D, false)
	f, c := newTestContext(t)
	imageTy := f.registerType(image)
	samplerTy := f.registerType(ir.SamplerType{Comparison: true})
	imageExpr := declareImageGlobal(t, f, c, "depth_map", imageTy)
	samplerExpr := declareImageGlobal(t, f, c, "depth_map_sampler", samplerTy)
	c.samplers[imageExpr] = samplerExpr
	coordTy := f.registerType(vecOf(ir.Vec3, scalarF32))
	coord := dynamicArg(t, c, coordTy)
	gradTy := f.registerType(vecOf(ir.Vec2, scalarF32))
	dx := dynamicArg(t, c, gradTy)
	dy := dynamicArg(t, c, gradTy)

	params := []ir.TypeInner{image, vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec2, scalarF32), vecOf(ir.Vec2, scalarF32)}
	h, _ := expandedBuiltin(t, f, c, "textureGrad", params, imageExpr, coord, dx, dy)
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	if _, isZero := sample.Level.(ir.SampleLevelZero); !isZero {
		t.Fatalf("level is %T, want SampleLevelZero for a shadow sampler", sample.Level)
	}
	if sample.Offset != nil {
		t.Fatal("a gradient sample without an offset carries one")
	}
}

// TestBuiltinTextureGradCarriesBothGradients checks that a non-shadow
// textureGrad keeps its two gradients (builtins.rs:1680-1696).
func TestBuiltinTextureGradCarriesBothGradients(t *testing.T) {
	image := sampledImage(ir.Dim3D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)
	coordTy := f.registerType(vecOf(ir.Vec3, scalarF32))
	coord := dynamicArg(t, c, coordTy)
	dx := dynamicArg(t, c, coordTy)
	dy := dynamicArg(t, c, coordTy)

	params := []ir.TypeInner{image, vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32), vecOf(ir.Vec3, scalarF32)}
	h, _ := expandedBuiltin(t, f, c, "textureGrad", params, imageExpr, coord, dx, dy)
	sample := exprAs[ir.ExprImageSample](t, c, h, "sample")
	grad, isGrad := sample.Level.(ir.SampleLevelGradient)
	if !isGrad {
		t.Fatalf("level is %T, want SampleLevelGradient", sample.Level)
	}
	if grad.X != dx || grad.Y != dy {
		t.Fatalf("gradients are (%d, %d), want (%d, %d)", grad.X, grad.Y, dx, dy)
	}
}

// TestBuiltinBarrierEmitsControlBarrier checks that barrier() is a statement,
// not a value, and that it asks for every barrier bit Rust's Barrier::all
// carries (builtins.rs:2039).
func TestBuiltinBarrierEmitsControlBarrier(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "barrier", variationStandard)
	ov := builtinOverload(t, f, decl)

	_, ok, err := ov.macro.call(c, nil, ir.Span{})
	if err != nil {
		t.Fatalf("barrier: %v", err)
	}
	if ok {
		t.Fatal("barrier reported a value, it is a statement")
	}
	var barrier *ir.StmtControlBarrier
	for i := range c.body {
		if b, isBarrier := c.body[i].Kind.(ir.StmtControlBarrier); isBarrier {
			barrier = &b
		}
	}
	if barrier == nil {
		t.Fatal("barrier emitted no control barrier statement")
	}
	if barrier.Flags != barrierAll {
		t.Fatalf("barrier flags are %b, want %b", barrier.Flags, barrierAll)
	}
}

// TestBuiltinBitfieldExtractConvertsOperands checks that bitfieldExtract
// converts its offset and count to 32-bit unsigned integers before the IR
// expression, which is what Rust's convert_to_32 does (builtins.rs:1860-1875).
func TestBuiltinBitfieldExtractConvertsOperands(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "bitfieldExtract", variationStandard)
	ov := builtinOverload(t, f, decl, scalarI32, scalarI32, scalarI32)

	value := dynamicArg(t, c, f.registerType(scalarI32))
	offset := dynamicArg(t, c, f.registerType(scalarI32))
	count := dynamicArg(t, c, f.registerType(scalarI32))
	h, ok := callBuiltin(t, c, ov, value, offset, count)
	if !ok {
		t.Fatal("bitfieldExtract reported no value")
	}
	math := exprAs[ir.ExprMath](t, c, h, "extract")
	if math.Fun != ir.MathExtractBits {
		t.Fatalf("math is %d, want ExtractBits", math.Fun)
	}
	if math.Arg != value {
		t.Fatalf("value is %d, want %d", math.Arg, value)
	}
	for _, operand := range []*ir.ExpressionHandle{math.Arg1, math.Arg2} {
		if operand == nil {
			t.Fatal("an extract operand is missing")
		}
		cast := exprAs[ir.ExprAs](t, c, *operand, "converted operand")
		if cast.Kind != ir.ScalarUint || cast.Convert == nil || *cast.Convert != 4 {
			t.Fatalf("converted operand is %+v, want a width-4 conversion to uint", cast)
		}
	}
}

// TestBuiltinBitfieldInsertConvertsOperands covers the four-argument insert
// builtin: its offset and count are converted the same way
// (builtins.rs:1877-1896).
func TestBuiltinBitfieldInsertConvertsOperands(t *testing.T) {
	f, c := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "bitfieldInsert", variationStandard)
	ov := builtinOverload(t, f, decl, scalarU32, scalarU32, scalarI32, scalarI32)

	base := dynamicArg(t, c, f.registerType(scalarU32))
	insert := dynamicArg(t, c, f.registerType(scalarU32))
	offset := dynamicArg(t, c, f.registerType(scalarI32))
	count := dynamicArg(t, c, f.registerType(scalarI32))
	h, ok := callBuiltin(t, c, ov, base, insert, offset, count)
	if !ok {
		t.Fatal("bitfieldInsert reported no value")
	}
	math := exprAs[ir.ExprMath](t, c, h, "insert")
	if math.Fun != ir.MathInsertBits {
		t.Fatalf("math is %d, want InsertBits", math.Fun)
	}
	if math.Arg != base || math.Arg1 == nil || *math.Arg1 != insert {
		t.Fatal("insert does not carry its base and inserted values")
	}
}

// TestBuiltinMacroRejectsTooFewArguments checks the arity guard: a Go library
// must not panic on a table entry that reads more arguments than the call
// carries (builtins.go's arity).
func TestBuiltinMacroRejectsTooFewArguments(t *testing.T) {
	_, c := newTestContext(t)
	macro := &macroCall{kind: macroBinary, binary: ir.BinaryAdd}
	_, _, err := macro.call(c, nil, ir.Span{})
	if err == nil || err.Message != "Internal error: builtin called with too few arguments" {
		t.Fatalf("error is %v, want the arity failure", err)
	}
}

// TestBuiltinUnknownMacroKindFails checks the final arm of the expansion
// dispatch: a kind outside the catalog is an internal error, never a silent
// zero expression.
func TestBuiltinUnknownMacroKindFails(t *testing.T) {
	_, c := newTestContext(t)
	macro := &macroCall{kind: macroKind(250)}
	_, _, err := macro.call(c, []loweredArg{{expr: 0}}, ir.Span{})
	if err == nil || err.Message != "Internal error: unknown builtin expansion" {
		t.Fatalf("error is %v, want the unknown-expansion failure", err)
	}
}

// TestBuiltinCoordinateComponentsOfNonImageRecordsError checks the fallback of
// coordinateComponents: a non-image argument is reported and treated as the
// coordinate itself, as Rust does (builtins.rs:2195-2207).
func TestBuiltinCoordinateComponentsOfNonImageRecordsError(t *testing.T) {
	f, c := newTestContext(t)
	notImage := dynamicArg(t, c, f.registerType(scalarF32))
	coord := dynamicArg(t, c, f.registerType(vecOf(ir.Vec2, scalarF32)))

	comps, err := f.coordinateComponents(c, notImage, coord, nil, ir.Span{})
	if err != nil {
		t.Fatalf("coordinateComponents: %v", err)
	}
	if comps.coordinate != coord {
		t.Fatalf("coordinate is %d, want the argument %d", comps.coordinate, coord)
	}
	if len(f.errors) == 0 || f.errors[0].Message != "Type is not an image" {
		t.Fatalf("errors are %+v, want \"Type is not an image\"", f.errors)
	}
}

// TestBuiltinCoordinateComponentsResizesWiderCoordinate checks that a
// coordinate wider than the image's own size is cut down: GLSL lets a call
// pass components the image ignores (builtins.rs:2135-2150).
func TestBuiltinCoordinateComponentsResizesWiderCoordinate(t *testing.T) {
	f, c := newTestContext(t)
	image := declareImageGlobal(t, f, c, "tex", f.registerType(sampledImage(ir.Dim3D, false, false, ir.ScalarFloat)))
	coord := dynamicArg(t, c, f.registerType(vecOf(ir.Vec4, scalarF32)))

	comps, err := f.coordinateComponents(c, image, coord, nil, ir.Span{})
	if err != nil {
		t.Fatalf("coordinateComponents: %v", err)
	}
	if comps.coordinate == coord {
		t.Fatal("the wider coordinate was not resized to the image size")
	}
	if comps.arrayIndex != nil || comps.depthRef != nil {
		t.Fatal("a 3D image produced an index or a depth reference")
	}
}

// TestBuiltinStorageCubeArrayTakesLayerFromCoordinate checks that a storage
// cubemap array's layer is the coordinate's fourth component rather than a
// separate index (builtins.rs:2160-2170).
func TestBuiltinStorageCubeArrayTakesLayerFromCoordinate(t *testing.T) {
	f, c := newTestContext(t)
	image := declareImageGlobal(t, f, c, "img",
		f.registerType(storageImage(ir.DimCube, true, ir.ScalarFloat, ir.StorageAccessLoad)))
	coord := dynamicArg(t, c, f.registerType(vecOf(ir.Vec4, scalarI32)))

	comps, err := f.coordinateComponents(c, image, coord, nil, ir.Span{})
	if err != nil {
		t.Fatalf("coordinateComponents: %v", err)
	}
	if comps.arrayIndex != nil {
		t.Fatal("a storage cubemap array produced a separate array index")
	}
}

// TestBuiltinSampledToDepthOfFunctionArgument checks that a shadow sampler
// passing an image through a function argument rewrites the argument's type
// and marks the parameter as carrying a depth image
// (builtins.rs:2213-2284).
func TestBuiltinSampledToDepthOfFunctionArgument(t *testing.T) {
	f, c := newTestContext(t)
	imageTy := f.registerType(sampledImage(ir.Dim2D, false, false, ir.ScalarFloat))
	sym := &glslSymbol{Kind: symParam, Name: "tex"}
	if err := c.addFunctionArg(sym, imageTy, paramIn, ir.Span{}); err != nil {
		t.Fatalf("addFunctionArg: %v", err)
	}
	argument := mustExpr(t, c, ir.ExprFunctionArgument{Index: 0})
	sampler := declareImageGlobal(t, f, c, "tex_sampler", f.registerType(ir.SamplerType{Comparison: true}))

	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "sampler2DShadow", variationStandard)
	ov := builtinOverload(t, f, decl, sampledImage(ir.Dim2D, false, false, ir.ScalarFloat), ir.SamplerType{Comparison: true})
	if _, _, err := ov.macro.call(c, []loweredArg{{expr: argument}, {expr: sampler}}, ir.Span{}); err != nil {
		t.Fatalf("shadow sampler: %v", err)
	}
	if !c.parametersInfo[0].depth {
		t.Fatal("the parameter is not marked as carrying a depth image")
	}
	img, isImage := f.typeInner(c.parameters[0]).(ir.ImageType)
	if !isImage || img.Class != ir.ImageClassDepth {
		t.Fatalf("parameter type is %v, want a depth image", f.typeInner(c.parameters[0]))
	}
	// The argument's own type changed too, so the IR validator sees the call
	// as passing a depth image.
	argImg, isImage := f.typeInner(c.arguments[0].Type).(ir.ImageType)
	if !isImage || argImg.Class != ir.ImageClassDepth {
		t.Fatalf("argument type is %v, want a depth image", f.typeInner(c.arguments[0].Type))
	}
}

// TestBuiltinSampledToDepthOfNonTextureRecordsError checks the error path of
// sampled_to_depth: an expression that is neither a global nor a function
// argument is reported, and the expansion keeps going, as Rust does
// (builtins.rs:2280-2284 collects the error).
func TestBuiltinSampledToDepthOfNonTextureRecordsError(t *testing.T) {
	f, c := newTestContext(t)
	literal := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(0)})
	sampler := declareImageGlobal(t, f, c, "tex_sampler", f.registerType(ir.SamplerType{Comparison: true}))

	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "sampler2DShadow", variationStandard)
	ov := builtinOverload(t, f, decl, sampledImage(ir.Dim2D, false, false, ir.ScalarFloat), ir.SamplerType{Comparison: true})
	h, ok, err := ov.macro.call(c, []loweredArg{{expr: literal}, {expr: sampler}}, ir.Span{})
	if err != nil {
		t.Fatalf("shadow sampler: %v", err)
	}
	if !ok || h != literal {
		t.Fatal("the expansion did not continue past the error")
	}
	found := false
	for _, e := range f.errors {
		if e.Message == "Not a valid texture expression" {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors are %+v, want the texture-expression failure", f.errors)
	}
}

// TestBuiltinProjectionOfScalarFails checks the projection guard: a scalar
// coordinate has no projection divisor (builtins.go's callTexture).
func TestBuiltinProjectionOfScalarFails(t *testing.T) {
	f, c := newTestContext(t)
	scalar := dynamicArg(t, c, f.registerType(scalarF32))
	macro := &macroCall{kind: macroTexture, proj: true}
	_, _, err := macro.call(c, []loweredArg{{expr: 0}, {expr: scalar}}, ir.Span{})
	if err == nil || err.Message != "Internal error: projected texture coordinate is not a vector" {
		t.Fatalf("error is %v, want the projection failure", err)
	}
}
