package front

import (
	"errors"
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
