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
		instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0),
		instWords(spvenums.OpTypeSampler, 17),
		instWords(spvenums.OpTypeSampledImage, 18, 16),
		instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassUniformConstant), 16),
		instWords(spvenums.OpTypePointer, 20, uint32(spvenums.StorageClassUniformConstant), 17),
		instWords(spvenums.OpTypeInt, 25, 32, 1),
		instWords(spvenums.OpTypeVector, 27, 25, 2),
		instWords(spvenums.OpConstant, 13, 23, 0x3f800000),
		instWords(spvenums.OpConstantComposite, 14, 24, 23, 23),
		instWords(spvenums.OpConstant, 25, 26, 2),
		instWords(spvenums.OpVariable, 19, 21, uint32(spvenums.StorageClassUniformConstant)),
		instWords(spvenums.OpVariable, 20, 22, uint32(spvenums.StorageClassUniformConstant)),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
		// Both handles are loaded into the expressions the sampling
		// instructions take.
		instWords(spvenums.OpLoad, 16, 30, 21),
		instWords(spvenums.OpLoad, 17, 31, 22),
		instWords(spvenums.OpSampledImage, 18, 32, 30, 31),
	}
	for _, instruction := range body {
		groups = append(groups, instruction)
	}
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
		found := false
		for _, ty := range module.Types {
			if sampler, ok := ty.Inner.(ir.SamplerType); ok && sampler.Comparison {
				found = true
			}
		}
		if !found {
			t.Error("no comparison sampler type")
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
