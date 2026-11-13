package front

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// imageModule assembles a fragment module with one image and one sampler, both
// separate UniformConstant variables, and the body the caller gives. Ids:
// 1 the function, 10 void, 11 fn void(), 13 f32, 14 vec2<f32>, 15 vec4<f32>,
// 16 the image type, 17 the sampler type, 18 the sampled image type, 19/20
// their UniformConstant pointers, 21/22 the variables, 23 the f32 constant 1.0,
// 24 the coordinate vec2(1.0), 25 i32, 26 the i32 constant 2.
func imageModule(t *testing.T, body ...[]uint32) *ir.Module {
	t.Helper()
	module, _, err := parseWords(t, Options{}, imageModuleGroups(body...)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return module
}

// imageModuleGroups builds the module's instruction groups without parsing
// them, so a test can parse a body that must fail.
func imageModuleGroups(body ...[]uint32) [][]uint32 {
	return imageVariantGroups(instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0), nil, body...)
}

// imageVariantGroups is imageModuleGroups with another OpTypeImage for id 16
// (2D sampled float by default) and extra type, constant and decoration groups
// before the function.
func imageVariantGroups(imageType []uint32, extra [][]uint32, body ...[]uint32) [][]uint32 {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelFragment, 21, 22),
		instWords(spvenums.OpDecorate, 21, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 21, uint32(spvenums.DecorationBinding), 0),
		instWords(spvenums.OpDecorate, 22, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 22, uint32(spvenums.DecorationBinding), 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeFloat, 13, 32),
		instWords(spvenums.OpTypeVector, 14, 13, 2),
		instWords(spvenums.OpTypeVector, 15, 13, 4),
		imageType,
		instWords(spvenums.OpTypeSampler, 17),
		instWords(spvenums.OpTypeSampledImage, 18, 16),
		instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassUniformConstant), 16),
		instWords(spvenums.OpTypePointer, 20, uint32(spvenums.StorageClassUniformConstant), 17),
		instWords(spvenums.OpTypeInt, 25, 32, 1),
		instWords(spvenums.OpTypeVector, 27, 25, 2),
		instWords(spvenums.OpConstant, 13, 23, 0x3f800000),
		instWords(spvenums.OpConstantComposite, 14, 24, 23, 23),
		instWords(spvenums.OpConstant, 25, 26, 2),
	}
	groups = append(groups, extra...)
	groups = append(groups,
		instWords(spvenums.OpVariable, 19, 21, uint32(spvenums.StorageClassUniformConstant)),
		instWords(spvenums.OpVariable, 20, 22, uint32(spvenums.StorageClassUniformConstant)),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		// Both handles are loaded into the expressions the sampling
		// instructions take.
		instWords(spvenums.OpLoad, 16, 30, 21),
		instWords(spvenums.OpLoad, 17, 31, 22),
		instWords(spvenums.OpSampledImage, 18, 32, 30, 31),
	)
	groups = append(groups, body...)
	groups = append(groups,
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
	return groups
}

// parseImageModule parses imageModule's module and returns the parsed function.
func parseImageModule(t *testing.T, body ...[]uint32) *ir.Function {
	t.Helper()
	module := imageModule(t, body...)
	if len(module.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(module.Functions))
	}
	return &module.Functions[0]
}

// TestImageSampleShape: OpImageSampleImplicitLod reads into an ImageSample
// expression whose image and sampler are the loaded handles and whose level is
// automatic.
func TestImageSampleShape(t *testing.T) {
	fn := parseImageModule(t,
		instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24),
	)
	sample, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageSample)
		return ok
	})
	if !ok {
		t.Fatal("no ImageSample expression")
	}
	kind := sample.(ir.ExprImageSample)
	if kind.Gather != nil || kind.DepthRef != nil || kind.Offset != nil || kind.ArrayIndex != nil {
		t.Errorf("sample = %#v, want a plain sample", kind)
	}
	if _, ok := kind.Level.(ir.SampleLevelAuto); !ok {
		t.Errorf("level = %T, want automatic", kind.Level)
	}
}

// TestImageGatherComponent: the component operand of OpImageGather selects the
// gather swizzle and forces level zero; a constant other than 0-3 is refused.
func TestImageGatherComponent(t *testing.T) {
	fn := parseImageModule(t,
		instWords(spvenums.OpImageGather, 15, 40, 32, 24, 26),
	)
	sample, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageSample)
		return ok
	})
	if !ok {
		t.Fatal("no ImageSample expression")
	}
	kind := sample.(ir.ExprImageSample)
	if kind.Gather == nil || *kind.Gather != ir.SwizzleZ {
		t.Errorf("gather = %v, want the z component", kind.Gather)
	}
	if _, ok := kind.Level.(ir.SampleLevelZero); !ok {
		t.Errorf("level = %T, want zero", kind.Level)
	}
}

// TestImageComparisonPatch: a sampler used only with a depth reference becomes
// a comparison sampler, and one used both ways is refused.
func TestImageComparisonPatch(t *testing.T) {
	t.Run("comparison only", func(t *testing.T) {
		module := imageModule(t,
			instWords(spvenums.OpImageSampleDrefImplicitLod, 13, 40, 32, 24, 23),
		)
		samplerPatched, imagePatched := false, false
		for _, ty := range module.Types {
			if sampler, ok := ty.Inner.(ir.SamplerType); ok && sampler.Comparison {
				samplerPatched = true
			}
			if image, ok := ty.Inner.(ir.ImageType); ok && image.Class == ir.ImageClassDepth {
				imagePatched = true
			}
		}
		if !samplerPatched {
			t.Error("no comparison sampler type")
		}
		// The image half is rewritten to a depth image too: a comparison
		// sample's image must be an ImageClassDepth in the IR.
		if !imagePatched {
			t.Error("no depth image type: the image half was not patched")
		}
	})

	t.Run("both ways", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			imageModuleGroups(
				instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24),
				instWords(spvenums.OpImageSampleDrefImplicitLod, 13, 41, 32, 24, 23),
			)...)
		if err == nil {
			t.Fatal("Parse accepted a sampler used both ways")
		}
		var parseErr *Error
		if !errors.As(err, &parseErr) {
			t.Fatalf("error %v (%T), want a *Error", err, err)
		}
		// Both handles are used both ways; which one the error names depends
		// on the arena order (Rust naga's hash order is arbitrary there too).
		if want := "inconsistent comparison sampling"; len(parseErr.Message) < len(want) || parseErr.Message[:len(want)] != want {
			t.Errorf("error = %q, want %q", parseErr.Message, want)
		}
	})
}

// TestImageQuerySizeCast: OpImageQuerySize on a 2D image with a signed result
// is cast to the signed kind.
func TestImageQuerySizeCast(t *testing.T) {
	fn := parseImageModule(t,
		instWords(spvenums.OpImageQuerySize, 27, 40, 30),
	)
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageQuery)
		return ok
	}); !ok {
		t.Fatal("no ImageQuery expression")
	}
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		as, ok := k.(ir.ExprAs)
		return ok && as.Kind == ir.ScalarSint
	}); !ok {
		t.Error("no signed cast of the query result")
	}
}

// imageVariant parses a module with another image type and extra type and
// constant groups, and returns the parsed function.
func imageVariant(t *testing.T, imageType []uint32, extra [][]uint32, body ...[]uint32) *ir.Function {
	t.Helper()
	module, _, err := parseWords(t, Options{}, imageVariantGroups(imageType, extra, body...)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(module.Functions))
	}
	return &module.Functions[0]
}

// vec3Float is the extra declaration of vec3<f32> (id 28) and its constant one
// (id 29), which the arrayed and projected sample tests use as a coordinate.
func vec3Float() [][]uint32 {
	return [][]uint32{
		instWords(spvenums.OpTypeVector, 28, 13, 3),
		instWords(spvenums.OpConstantComposite, 28, 29, 23, 23, 23),
	}
}

// TestImageSampleArrayLayer: a 2D arrayed image takes its layer from the last
// coordinate component, cast to i32; the coordinate is the two spatial
// components composed.
func TestImageSampleArrayLayer(t *testing.T) {
	arrayed2D := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 1, 0, 1, 0)
	fn := imageVariant(t, arrayed2D, vec3Float(),
		instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 29),
	)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageSample)
		return ok
	})
	if !ok {
		t.Fatal("no ImageSample expression")
	}
	sample := kind.(ir.ExprImageSample)
	if sample.ArrayIndex == nil {
		t.Fatal("the arrayed sample has no array index")
	}
	arrayIndex, ok := fn.Expressions[*sample.ArrayIndex].Kind.(ir.ExprAs)
	if !ok || arrayIndex.Kind != ir.ScalarSint {
		t.Fatalf("array index = %T, want the i32 cast", fn.Expressions[*sample.ArrayIndex].Kind)
	}
	access, ok := fn.Expressions[arrayIndex.Expr].Kind.(ir.ExprAccessIndex)
	if !ok || access.Index != 2 {
		t.Fatalf("array index operand = %T, want the third coordinate component", fn.Expressions[arrayIndex.Expr].Kind)
	}
	if compose, ok := fn.Expressions[sample.Coordinate].Kind.(ir.ExprCompose); !ok || len(compose.Components) != 2 {
		t.Fatalf("coordinate = %T, want the two spatial components composed", fn.Expressions[sample.Coordinate].Kind)
	}
}

// TestImageSampleProjection: a projected sample divides the coordinates by the
// last component and has no array index.
func TestImageSampleProjection(t *testing.T) {
	fn := imageVariant(t, instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0), vec3Float(),
		instWords(spvenums.OpImageSampleProjImplicitLod, 15, 40, 32, 29),
	)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageSample)
		return ok
	})
	if !ok {
		t.Fatal("no ImageSample expression")
	}
	sample := kind.(ir.ExprImageSample)
	if sample.ArrayIndex != nil {
		t.Error("the projected sample has an array index")
	}
	compose, ok := fn.Expressions[sample.Coordinate].Kind.(ir.ExprCompose)
	if !ok || len(compose.Components) != 2 {
		t.Fatalf("coordinate = %T, want the divided components composed", fn.Expressions[sample.Coordinate].Kind)
	}
	divide, ok := fn.Expressions[compose.Components[0]].Kind.(ir.ExprBinary)
	if !ok || divide.Op != ir.BinaryDivide {
		t.Fatalf("coordinate component = %T, want a division", fn.Expressions[compose.Components[0]].Kind)
	}
}

// TestImageQuerySizeArrayed: SPIR-V's OpImageQuerySize(Lod) returns the layer
// count with the spatial size; the IR query is the spatial size, so the
// layers come from ImageQueryNumLayers and are composed back in (Rust naga
// keeps the spatial size alone and its instruction result then has the wrong
// type).
func TestImageQuerySizeArrayed(t *testing.T) {
	arrayed2D := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 1, 0, 1, 0)
	extra := [][]uint32{instWords(spvenums.OpTypeVector, 33, 25, 3)} // vec3<i32>
	fn := imageVariant(t, arrayed2D, extra,
		instWords(spvenums.OpImageQuerySizeLod, 33, 40, 30, 23),
	)
	extended := false
	for _, expression := range fn.Expressions {
		compose, ok := expression.Kind.(ir.ExprCompose)
		if !ok || len(compose.Components) != 3 {
			continue
		}
		last, ok := fn.Expressions[compose.Components[2]].Kind.(ir.ExprImageQuery)
		if ok {
			if _, ok := last.Query.(ir.ImageQueryNumLayers); ok {
				extended = true
			}
		}
	}
	if !extended {
		t.Error("no recomposed vec3 whose last component is ImageQueryNumLayers")
	}
}

// TestImageFetchDepthSplat: a depth image load is a scalar in the IR; the
// vector result SPIR-V asks for is a splat of it.
func TestImageFetchDepthSplat(t *testing.T) {
	depth := instWords(spvenums.OpTypeImage, 16, 13, 1, 1, 0, 0, 1, 0)
	fn := imageVariant(t, depth, nil,
		instWords(spvenums.OpImageFetch, 15, 40, 30, 24),
	)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		splat, ok := k.(ir.ExprSplat)
		return ok && splat.Size == ir.Vec4
	})
	if !ok {
		t.Fatal("no vec4 splat of the depth load")
	}
	splat := kind.(ir.ExprSplat)
	if _, ok := fn.Expressions[splat.Value].Kind.(ir.ExprImageLoad); !ok {
		t.Errorf("splat operand = %T, want the image load", fn.Expressions[splat.Value].Kind)
	}
}

// TestImageGatherDepthNoSplat: a gather on a depth image is a vec4 already and
// must not be splatted (Rust naga splats it, which its own typifier rejects).
func TestImageGatherDepthNoSplat(t *testing.T) {
	depth := instWords(spvenums.OpTypeImage, 16, 13, 1, 1, 0, 0, 1, 0)
	fn := imageVariant(t, depth, nil,
		instWords(spvenums.OpImageGather, 15, 41, 32, 24, 26),
	)
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		sample, ok := k.(ir.ExprImageSample)
		return ok && sample.Gather != nil
	}); !ok {
		t.Fatal("no gather ImageSample expression")
	}
	if _, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprSplat)
		return ok
	}); ok {
		t.Error("the depth gather is splatted")
	}
}

// TestImageSamplingArgumentCallerFirst: a comparison sample inside a helper
// function patches the caller's sampler even when the entry point is declared
// before the helper. The propagation walks the call graph after the call
// targets are patched; before that the call still holds a placeholder, and a
// caller declared first then reads the wrong function's flags (silently, both
// modules validate).
func TestImageSamplingArgumentCallerFirst(t *testing.T) {
	module, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelFragment, 24, 26),
		instWords(spvenums.OpDecorate, 24, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 24, uint32(spvenums.DecorationBinding), 0),
		instWords(spvenums.OpDecorate, 26, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 26, uint32(spvenums.DecorationBinding), 1),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeFloat, 13, 32),
		instWords(spvenums.OpTypeVector, 14, 13, 2),
		instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0),
		instWords(spvenums.OpTypeSampler, 17),
		instWords(spvenums.OpTypeSampledImage, 18, 16),
		instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassUniformConstant), 16),
		instWords(spvenums.OpTypePointer, 20, uint32(spvenums.StorageClassUniformConstant), 17),
		instWords(spvenums.OpTypeFunction, 27, 10, 16, 17, 14),
		instWords(spvenums.OpConstant, 13, 21, 0x3f800000),
		instWords(spvenums.OpConstantComposite, 14, 22, 21, 21),
		instWords(spvenums.OpConstant, 13, 23, 0x3f000000),
		instWords(spvenums.OpVariable, 19, 24, uint32(spvenums.StorageClassUniformConstant)),
		instWords(spvenums.OpVariable, 20, 26, uint32(spvenums.StorageClassUniformConstant)),
		// The entry point is declared first, the helper it calls after it.
		instWords(spvenums.OpFunction, 10, 1, 0, 11),
		instWords(spvenums.OpLabel, 3),
		instWords(spvenums.OpLoad, 16, 30, 24),
		instWords(spvenums.OpLoad, 17, 31, 26),
		instWords(spvenums.OpFunctionCall, 10, 32, 2, 30, 31, 22),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
		instWords(spvenums.OpFunction, 10, 2, 0, 27),
		instWords(spvenums.OpFunctionParameter, 16, 40),
		instWords(spvenums.OpFunctionParameter, 17, 41),
		instWords(spvenums.OpFunctionParameter, 14, 42),
		instWords(spvenums.OpLabel, 4),
		instWords(spvenums.OpSampledImage, 18, 50, 40, 41),
		instWords(spvenums.OpImageSampleDrefImplicitLod, 13, 51, 50, 42, 23),
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	patched := false
	for _, global := range module.GlobalVariables {
		if sampler, ok := module.Types[global.Type].Inner.(ir.SamplerType); ok && sampler.Comparison {
			patched = true
		}
	}
	if !patched {
		t.Error("the sampler sampled through the helper was not patched to its comparison type")
	}
}

// TestImageUncouple: OpImage takes the image half of a sampled-image pair, and
// an id that is not such a pair is refused.
func TestImageUncouple(t *testing.T) {
	fn := parseImageModule(t,
		instWords(spvenums.OpImage, 16, 40, 32),
		instWords(spvenums.OpImageQuerySize, 27, 41, 40),
	)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageQuery)
		return ok
	})
	if !ok {
		t.Fatal("no ImageQuery expression")
	}
	global, ok := fn.Expressions[kind.(ir.ExprImageQuery).Image].Kind.(ir.ExprGlobalVariable)
	if !ok || global.Variable != 0 {
		t.Fatalf("query image = %T, want the image global", fn.Expressions[kind.(ir.ExprImageQuery).Image].Kind)
	}

	_, _, err := parseWords(t, Options{}, imageModuleGroups(
		instWords(spvenums.OpImage, 16, 40, 99),
	)...)
	if err == nil {
		t.Fatal("Parse accepted OpImage on an id that is not a sampled image")
	}
	if want := errInvalidID(99).Message; requireError(t, err).Message != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// TestImageOperandErrors: an operand id the module never defined is the id
// error of the lookup that reads it, for the couple, sample and load
// instructions.
func TestImageOperandErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []uint32
	}{
		{"couple image", instWords(spvenums.OpSampledImage, 18, 50, 99, 31)},
		{"couple sampler", instWords(spvenums.OpSampledImage, 18, 50, 30, 99)},
		{"sample sampled image", instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 99, 24)},
		{"sample coordinate", instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 99)},
		{"sample level", instWords(spvenums.OpImageSampleExplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsLod), 99)},
		{"fetch image", instWords(spvenums.OpImageFetch, 15, 40, 99, 24)},
		{"fetch coordinate", instWords(spvenums.OpImageFetch, 15, 40, 30, 99)},
		{"fetch level", instWords(spvenums.OpImageFetch, 15, 40, 30, 24, uint32(spvenums.ImageOperandsLod), 99)},
		{"fetch sample", instWords(spvenums.OpImageFetch, 15, 40, 30, 24, uint32(spvenums.ImageOperandsSample), 99)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseWords(t, Options{}, imageModuleGroups(tc.body)...)
			if err == nil {
				t.Fatal("Parse accepted an undefined operand id")
			}
			if got := requireError(t, err).Message; got != errInvalidID(99).Message {
				t.Errorf("message = %q, want %q", got, errInvalidID(99).Message)
			}
		})
	}
}

// imageStore parses a module whose first body instruction is an OpImageWrite
// the caller gives and returns the ImageStore statement it produced.
func imageStore(t *testing.T, imageType []uint32, extra [][]uint32, body ...[]uint32) (*ir.Function, ir.StmtImageStore) {
	t.Helper()
	fn := imageVariant(t, imageType, extra, body...)
	statement, ok := findStatement(fn, func(k ir.StatementKind) bool {
		_, ok := k.(ir.StmtImageStore)
		return ok
	})
	if !ok {
		t.Fatal("no ImageStore statement")
	}
	return fn, statement.(ir.StmtImageStore)
}

// TestImageWrite: OpImageWrite widens the stored value to four components the
// way Rust naga's expanded_value does: a scalar is splatted, vec2 and vec3 get
// their last component repeated, vec4 is stored as is, and any other type is
// refused.
func TestImageWrite(t *testing.T) {
	storage := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 2, uint32(spvenums.ImageFormatRgba32f))
	vec4Constant := instWords(spvenums.OpConstantComposite, 15, 35, 23, 23, 23, 23)

	// valueKind returns the kind of the store's value expression.
	valueKind := func(t *testing.T, fn *ir.Function, store ir.StmtImageStore) ir.ExpressionKind {
		t.Helper()
		return fn.Expressions[store.Value].Kind
	}

	t.Run("scalar splat", func(t *testing.T) {
		fn, store := imageStore(t, storage, nil, instWords(spvenums.OpImageWrite, 30, 24, 23))
		splat, ok := valueKind(t, fn, store).(ir.ExprSplat)
		if !ok || splat.Size != ir.Vec4 {
			t.Fatalf("value = %T, want a vec4 splat", valueKind(t, fn, store))
		}
	})

	t.Run("vec2 repeats y", func(t *testing.T) {
		fn, store := imageStore(t, storage, nil, instWords(spvenums.OpImageWrite, 30, 24, 24))
		swizzle, ok := valueKind(t, fn, store).(ir.ExprSwizzle)
		if !ok {
			t.Fatalf("value = %T, want a swizzle", valueKind(t, fn, store))
		}
		want := [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleY, ir.SwizzleY}
		if swizzle.Size != ir.Vec4 || swizzle.Pattern != want {
			t.Errorf("swizzle = %v %v, want %v", swizzle.Size, swizzle.Pattern, want)
		}
	})

	t.Run("vec3 repeats z", func(t *testing.T) {
		fn, store := imageStore(t, storage, vec3Float(), instWords(spvenums.OpImageWrite, 30, 24, 29))
		swizzle, ok := valueKind(t, fn, store).(ir.ExprSwizzle)
		if !ok {
			t.Fatalf("value = %T, want a swizzle", valueKind(t, fn, store))
		}
		want := [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleZ}
		if swizzle.Pattern != want {
			t.Errorf("pattern = %v, want %v", swizzle.Pattern, want)
		}
	})

	t.Run("vec4 as is", func(t *testing.T) {
		fn, store := imageStore(t, storage, [][]uint32{vec4Constant},
			instWords(spvenums.OpImageWrite, 30, 24, 35))
		if _, ok := valueKind(t, fn, store).(ir.ExprConstant); !ok {
			t.Errorf("value = %T, want the constant itself", valueKind(t, fn, store))
		}
	})

	t.Run("arrayed", func(t *testing.T) {
		arrayed := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 1, 0, 2, uint32(spvenums.ImageFormatRgba32f))
		_, store := imageStore(t, arrayed, vec3Float(),
			instWords(spvenums.OpImageWrite, 30, 29, 23))
		if store.ArrayIndex == nil {
			t.Error("the write to an arrayed image has no array index")
		}
	})

	t.Run("operands skipped", func(t *testing.T) {
		fn, store := imageStore(t, storage, nil,
			instWords(spvenums.OpImageWrite, 30, 24, 23, uint32(spvenums.ImageOperandsLod), 26))
		// The image operands are not carried into the IR, as in Rust naga.
		if _, ok := valueKind(t, fn, store).(ir.ExprSplat); !ok {
			t.Errorf("value = %T, want the scalar splat", valueKind(t, fn, store))
		}
	})

	t.Run("invalid value type", func(t *testing.T) {
		extra := [][]uint32{
			instWords(spvenums.OpTypeStruct, 35, 13),
			instWords(spvenums.OpConstantNull, 35, 36),
		}
		_, _, err := parseWords(t, Options{}, imageVariantGroups(storage, extra,
			instWords(spvenums.OpImageWrite, 30, 24, 36))...)
		if err == nil {
			t.Fatal("Parse accepted a struct as a stored value")
		}
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "invalid vector type") {
			t.Errorf("message = %q, want invalid vector type", got)
		}
	})

	t.Run("image expression is no image", func(t *testing.T) {
		// The sampler half used as the image: a valid expression, no image
		// type.
		_, _, err := parseWords(t, Options{}, imageVariantGroups(storage, nil,
			instWords(spvenums.OpImageWrite, 31, 24, 23))...)
		if err == nil {
			t.Fatal("Parse accepted a sampler as the image of a write")
		}
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "invalid image") {
			t.Errorf("message = %q, want invalid image", got)
		}
	})

	t.Run("coordinate of no vector type", func(t *testing.T) {
		extra := [][]uint32{
			instWords(spvenums.OpTypeStruct, 35, 13),
			instWords(spvenums.OpConstantNull, 35, 36),
		}
		_, _, err := parseWords(t, Options{}, imageVariantGroups(storage, extra,
			instWords(spvenums.OpImageWrite, 30, 36, 23))...)
		if err == nil {
			t.Fatal("Parse accepted a struct coordinate of a write")
		}
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "invalid image coordinate type") {
			t.Errorf("message = %q, want invalid image coordinate type", got)
		}
	})

	t.Run("unknown operands", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			body []uint32
		}{
			{"image", instWords(spvenums.OpImageWrite, 99, 24, 23)},
			{"coordinate", instWords(spvenums.OpImageWrite, 30, 99, 23)},
			{"value", instWords(spvenums.OpImageWrite, 30, 24, 99)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := parseWords(t, Options{}, imageVariantGroups(storage, nil, tc.body)...)
				if err == nil {
					t.Fatalf("Parse accepted an unknown %s id", tc.name)
				}
				if got := requireError(t, err).Message; got != errInvalidID(99).Message {
					t.Errorf("message = %q, want %q", got, errInvalidID(99).Message)
				}
			})
		}
	})
}

// imageSample parses a module and returns the function and its first
// ImageSample expression.
func imageSample(t *testing.T, imageType []uint32, extra [][]uint32, body ...[]uint32) (*ir.Function, ir.ExprImageSample) {
	t.Helper()
	fn := imageVariant(t, imageType, extra, body...)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageSample)
		return ok
	})
	if !ok {
		t.Fatal("no ImageSample expression")
	}
	return fn, kind.(ir.ExprImageSample)
}

// TestImageSampleOperands: the image operands of the sample family select the
// IR's level: bias, explicit level (an integer level when the image is a
// depth image), gradient, and a constant offset; a comparison sample's level
// is zero whatever the instruction names, an operand the IR has no field for
// is skipped, and a projected comparison divides the reference by the
// projection component.
func TestImageSampleOperands(t *testing.T) {
	plain := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0)
	depth := instWords(spvenums.OpTypeImage, 16, 13, 1, 1, 0, 0, 1, 0)

	t.Run("bias", func(t *testing.T) {
		_, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsBias), 23))
		if _, ok := sample.Level.(ir.SampleLevelBias); !ok {
			t.Errorf("level = %T, want a bias", sample.Level)
		}
	})

	t.Run("lod", func(t *testing.T) {
		fn, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleExplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsLod), 23))
		level, ok := sample.Level.(ir.SampleLevelExact)
		if !ok {
			t.Fatalf("level = %T, want exact", sample.Level)
		}
		if _, ok := fn.Expressions[level.Level].Kind.(ir.ExprConstant); !ok {
			t.Errorf("level expression = %T, want the constant", fn.Expressions[level.Level].Kind)
		}
	})

	t.Run("lod on a depth image casts", func(t *testing.T) {
		fn, sample := imageSample(t, depth, nil,
			instWords(spvenums.OpImageSampleExplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsLod), 23))
		level, ok := sample.Level.(ir.SampleLevelExact)
		if !ok {
			t.Fatalf("level = %T, want exact", sample.Level)
		}
		cast, ok := fn.Expressions[level.Level].Kind.(ir.ExprAs)
		if !ok || cast.Kind != ir.ScalarSint {
			t.Errorf("level expression = %T, want the i32 cast", fn.Expressions[level.Level].Kind)
		}
	})

	t.Run("gradient", func(t *testing.T) {
		_, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleExplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsGrad), 23, 23))
		if _, ok := sample.Level.(ir.SampleLevelGradient); !ok {
			t.Errorf("level = %T, want a gradient", sample.Level)
		}
	})

	t.Run("constant offset", func(t *testing.T) {
		_, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsConstOffset), 26))
		if sample.Offset == nil {
			t.Error("the constant offset was dropped")
		}
	})

	t.Run("unknown operand skipped", func(t *testing.T) {
		_, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24, uint32(spvenums.ImageOperandsMakeTexelAvailable), 26))
		if _, ok := sample.Level.(ir.SampleLevelAuto); !ok {
			t.Errorf("level = %T, want automatic", sample.Level)
		}
	})

	t.Run("comparison lod is zero", func(t *testing.T) {
		_, sample := imageSample(t, plain, nil,
			instWords(spvenums.OpImageSampleDrefExplicitLod, 13, 40, 32, 24, 23, uint32(spvenums.ImageOperandsLod), 23))
		if _, ok := sample.Level.(ir.SampleLevelZero); !ok {
			t.Errorf("level = %T, want zero", sample.Level)
		}
	})

	t.Run("projected comparison divides", func(t *testing.T) {
		fn, sample := imageSample(t, plain, vec3Float(),
			instWords(spvenums.OpImageSampleProjDrefImplicitLod, 13, 40, 32, 29, 23))
		if sample.DepthRef == nil {
			t.Fatal("the projected comparison has no depth reference")
		}
		divide, ok := fn.Expressions[*sample.DepthRef].Kind.(ir.ExprBinary)
		if !ok || divide.Op != ir.BinaryDivide {
			t.Errorf("depth reference = %T, want the division", fn.Expressions[*sample.DepthRef].Kind)
		}
	})

	t.Run("gather components", func(t *testing.T) {
		extra := [][]uint32{
			instWords(spvenums.OpConstant, 25, 35, 1),
			instWords(spvenums.OpConstant, 25, 36, 3),
		}
		for _, tc := range []struct {
			id   uint32
			want ir.SwizzleComponent
		}{
			{35, ir.SwizzleY},
			{36, ir.SwizzleW},
		} {
			_, sample := imageSample(t, plain, extra,
				instWords(spvenums.OpImageGather, 15, 40, 32, 24, tc.id))
			if sample.Gather == nil || *sample.Gather != tc.want {
				t.Errorf("component %d: gather = %v, want %v", tc.id, sample.Gather, tc.want)
			}
		}
	})

	t.Run("gather component errors", func(t *testing.T) {
		// A component that is not a constant expression, and a constant
		// outside 0-3.
		_, _, err := parseWords(t, Options{}, imageModuleGroups(
			instWords(spvenums.OpImageGather, 15, 40, 32, 24, 24),
		)...)
		if err == nil {
			t.Fatal("Parse accepted a non-constant gather component")
		}
		if got := requireError(t, err).Message; got != errInvalidOperand.Message {
			t.Errorf("message = %q, want %q", got, errInvalidOperand.Message)
		}

		extra := [][]uint32{instWords(spvenums.OpConstant, 25, 35, 4)}
		_, _, err = parseWords(t, Options{}, imageVariantGroups(plain, extra,
			instWords(spvenums.OpImageGather, 15, 40, 32, 24, 35))...)
		if err == nil {
			t.Fatal("Parse accepted a gather component of 4")
		}
		if got := requireError(t, err).Message; got != errInvalidOperand.Message {
			t.Errorf("message = %q, want %q", got, errInvalidOperand.Message)
		}
	})

	t.Run("coordinate cut to the image size", func(t *testing.T) {
		// A 2D image sampled with a vec3 coordinate: the third component is
		// dropped with a swizzle to the two components the image takes.
		fn, sample := imageSample(t, plain, vec3Float(),
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 29))
		if _, ok := fn.Expressions[sample.Coordinate].Kind.(ir.ExprSwizzle); !ok {
			t.Errorf("coordinate = %T, want the cut swizzle", fn.Expressions[sample.Coordinate].Kind)
		}
	})

	t.Run("1D arrayed layer from a vec2", func(t *testing.T) {
		// A 1D image has no spatial vector to cut: the first component is the
		// coordinate, the second the layer.
		arrayed1D := instWords(spvenums.OpTypeImage, 16, 13, 0, 0, 1, 0, 1, 0)
		fn, sample := imageSample(t, arrayed1D, nil,
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 24))
		if sample.ArrayIndex == nil {
			t.Fatal("the 1D arrayed sample has no array index")
		}
		access, ok := fn.Expressions[sample.Coordinate].Kind.(ir.ExprAccessIndex)
		if !ok || access.Index != 0 {
			t.Errorf("coordinate = %T, want the first component", fn.Expressions[sample.Coordinate].Kind)
		}
	})

	t.Run("1D projection divides by the second component", func(t *testing.T) {
		fn, sample := imageSample(t, instWords(spvenums.OpTypeImage, 16, 13, 0, 0, 0, 0, 1, 0), nil,
			instWords(spvenums.OpImageSampleProjImplicitLod, 15, 40, 32, 24))
		divide, ok := fn.Expressions[sample.Coordinate].Kind.(ir.ExprBinary)
		if !ok || divide.Op != ir.BinaryDivide {
			t.Errorf("coordinate = %T, want a division", fn.Expressions[sample.Coordinate].Kind)
		}
	})
}

// imageLoad parses a module and returns the function and its first ImageLoad
// expression.
func imageLoad(t *testing.T, imageType []uint32, extra [][]uint32, body ...[]uint32) (*ir.Function, ir.ExprImageLoad) {
	t.Helper()
	fn := imageVariant(t, imageType, extra, body...)
	kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
		_, ok := k.(ir.ExprImageLoad)
		return ok
	})
	if !ok {
		t.Fatal("no ImageLoad expression")
	}
	return fn, kind.(ir.ExprImageLoad)
}

// TestImageLoadOperands: OpImageFetch and OpImageRead carry the sample index
// or the level they name into the ImageLoad, and skip an operand the IR has
// no field for.
func TestImageLoadOperands(t *testing.T) {
	plain := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0)

	t.Run("sample", func(t *testing.T) {
		_, load := imageLoad(t, plain, nil,
			instWords(spvenums.OpImageFetch, 15, 40, 30, 24, uint32(spvenums.ImageOperandsSample), 26))
		if load.Sample == nil {
			t.Error("the sample index was dropped")
		}
		if load.Level != nil {
			t.Error("the fetch has a level")
		}
	})

	t.Run("level", func(t *testing.T) {
		_, load := imageLoad(t, plain, nil,
			instWords(spvenums.OpImageRead, 15, 40, 30, 24, uint32(spvenums.ImageOperandsLod), 23))
		if load.Level == nil {
			t.Error("the level was dropped")
		}
		if load.Sample != nil {
			t.Error("the read has a sample index")
		}
	})

	t.Run("unknown operand skipped", func(t *testing.T) {
		_, load := imageLoad(t, plain, nil,
			instWords(spvenums.OpImageFetch, 15, 40, 30, 24, uint32(spvenums.ImageOperandsMakeTexelAvailable), 26))
		if load.Sample != nil || load.Level != nil {
			t.Error("an operand the IR has no field for was carried into the load")
		}
	})
}

// TestImageQueryOther: OpImageQueryLevels and OpImageQuerySamples are the
// ImageQuery expressions the IR has for them.
func TestImageQueryOther(t *testing.T) {
	queryExpr := func(t *testing.T, body ...[]uint32) ir.ExprImageQuery {
		t.Helper()
		fn := parseImageModule(t, body...)
		kind, ok := findExpression(fn, func(k ir.ExpressionKind) bool {
			_, ok := k.(ir.ExprImageQuery)
			return ok
		})
		if !ok {
			t.Fatal("no ImageQuery expression")
		}
		return kind.(ir.ExprImageQuery)
	}

	if query := queryExpr(t, instWords(spvenums.OpImageQueryLevels, 25, 40, 30)); query.Query != (ir.ImageQueryNumLevels{}) {
		t.Errorf("query = %T, want the level count", query.Query)
	}
	if query := queryExpr(t, instWords(spvenums.OpImageQuerySamples, 25, 40, 30)); query.Query != (ir.ImageQueryNumSamples{}) {
		t.Errorf("query = %T, want the sample count", query.Query)
	}
}

// TestImageQuerySizeArrayShapes: a 1D arrayed image's query result is the
// spatial size plus the layer count, and a result type that does not have the
// extended size stays the spatial size alone.
func TestImageQuerySizeArrayShapes(t *testing.T) {
	t.Run("1D arrayed", func(t *testing.T) {
		arrayed1D := instWords(spvenums.OpTypeImage, 16, 13, 0, 0, 1, 0, 1, 0)
		fn := imageVariant(t, arrayed1D, nil,
			instWords(spvenums.OpImageQuerySizeLod, 27, 40, 30, 23),
		)
		found := false
		for _, expression := range fn.Expressions {
			compose, ok := expression.Kind.(ir.ExprCompose)
			if !ok || len(compose.Components) != 2 {
				continue
			}
			if _, ok := fn.Expressions[compose.Components[1]].Kind.(ir.ExprImageQuery); ok {
				found = true
			}
		}
		if !found {
			t.Error("no vec2 whose last component is the layer count")
		}
	})

	t.Run("result without the layers", func(t *testing.T) {
		arrayed2D := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 1, 0, 1, 0)
		fn := imageVariant(t, arrayed2D, nil,
			instWords(spvenums.OpImageQuerySizeLod, 27, 40, 30, 23),
		)
		for _, expression := range fn.Expressions {
			if query, ok := expression.Kind.(ir.ExprImageQuery); ok {
				if _, ok := query.Query.(ir.ImageQueryNumLayers); ok {
					t.Error("the layers were composed into a result that did not ask for them")
				}
			}
		}
	})
}

// TestImageExpressionErrors: an operand that is not an image is an error with
// a message, never a panic: the image operand of a load, the image half of a
// sample, and a coordinate of neither scalar nor vector type.
func TestImageExpressionErrors(t *testing.T) {
	plain := instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0)

	t.Run("fetch of a non-image", func(t *testing.T) {
		_, _, err := parseWords(t, Options{}, imageModuleGroups(
			instWords(spvenums.OpImageFetch, 15, 40, 24, 24),
		)...)
		if err == nil {
			t.Fatal("Parse accepted a fetch of a vector")
		}
		if got := requireError(t, err).Message; got != errInvalidImageExpression.Message {
			t.Errorf("message = %q, want %q", got, errInvalidImageExpression.Message)
		}
	})

	t.Run("sample of a non-image pair", func(t *testing.T) {
		_, _, err := parseWords(t, Options{}, imageModuleGroups(
			instWords(spvenums.OpSampledImage, 18, 50, 24, 31),
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 50, 24),
		)...)
		if err == nil {
			t.Fatal("Parse accepted a sample whose image half is a vector")
		}
		if got := requireError(t, err).Message; got != errInvalidGlobalVar.Message {
			t.Errorf("message = %q, want %q", got, errInvalidGlobalVar.Message)
		}
	})

	t.Run("coordinate of no vector type", func(t *testing.T) {
		extra := [][]uint32{
			instWords(spvenums.OpTypeStruct, 35, 13),
			instWords(spvenums.OpConstantNull, 35, 36),
		}
		_, _, err := parseWords(t, Options{}, imageVariantGroups(plain, extra,
			instWords(spvenums.OpImageSampleImplicitLod, 15, 40, 32, 36))...)
		if err == nil {
			t.Fatal("Parse accepted a struct coordinate")
		}
		if got := requireError(t, err).Message; !strings.HasPrefix(got, "invalid image coordinate type") {
			t.Errorf("message = %q, want invalid image coordinate type", got)
		}
	})
}

// TestParameterSamplingWalk: the flags a callee samples its arguments with
// travel to the caller's global variables and to its own arguments through
// every statement kind, and a call the walk cannot resolve is an error rather
// than a silent drop.
func TestParameterSamplingWalk(t *testing.T) {
	globalCall := ir.Statement{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0}}}
	argumentCall := ir.Statement{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{1}}}
	// A call with more arguments than the callee's flags cover, and a call to
	// a function with no flags at all, pass through.
	extraArguments := ir.Statement{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0, 0, 0}}}
	unknownFunction := ir.Statement{Kind: ir.StmtCall{Function: 7, Arguments: []ir.ExpressionHandle{0}}}
	block := ir.Block{
		globalCall,
		{Kind: ir.StmtBlock{Block: ir.Block{argumentCall}}},
		{Kind: ir.StmtIf{Accept: ir.Block{argumentCall}, Reject: ir.Block{globalCall}}},
		{Kind: ir.StmtLoop{Body: ir.Block{globalCall}, Continuing: ir.Block{argumentCall}}},
		{Kind: ir.StmtSwitch{Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{argumentCall}},
		}}},
		extraArguments,
		unknownFunction,
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
		Body: block,
	}
	p := &parser{handleSampling: map[ir.GlobalVariableHandle]samplingFlags{0: 0}}
	own := make([]samplingFlags, 1)
	callee := [][]samplingFlags{{samplingComparison}}
	if err := p.propagateBlockSampling(fn, &own, callee); err != nil {
		t.Fatalf("propagate: %v", err)
	}
	if p.handleSampling[0] != samplingComparison {
		t.Errorf("global flags = %v, want comparison sampling", p.handleSampling[0])
	}
	if own[0] != samplingComparison {
		t.Errorf("argument flags = %v, want comparison sampling", own[0])
	}

	for _, tc := range []struct {
		name       string
		expression ir.ExpressionKind
		arguments  []ir.ExpressionHandle
	}{
		{"expression out of range", ir.ExprGlobalVariable{Variable: 0}, []ir.ExpressionHandle{99}},
		{"argument out of range", ir.ExprFunctionArgument{Index: 1}, []ir.ExpressionHandle{0}},
		{"unsupported expression", ir.Literal{Value: ir.LiteralI32(1)}, []ir.ExpressionHandle{0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := &ir.Function{
				Expressions: []ir.Expression{{Kind: tc.expression}},
				Body:        ir.Block{{Kind: ir.StmtCall{Function: 0, Arguments: tc.arguments}}},
			}
			p := &parser{handleSampling: map[ir.GlobalVariableHandle]samplingFlags{0: 0}}
			own := make([]samplingFlags, 1)
			err := p.propagateBlockSampling(fn, &own, [][]samplingFlags{{samplingComparison}})
			if err == nil {
				t.Fatal("the walk accepted the call")
			}
			if got := requireError(t, err).Message; got != errInvalidGlobalVar.Message {
				t.Errorf("message = %q, want %q", got, errInvalidGlobalVar.Message)
			}
		})
	}
}
