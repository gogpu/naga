package front

import (
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/valid"
)

// combinedSamplerModuleGroups assembles what glslc emits for
// `uniform sampler2D tex`: one variable of OpTypeSampledImage loaded straight
// into OpImageSampleImplicitLod, with no OpSampledImage. Ids: 1 the function,
// 10 void, 11 fn void(), 13 f32, 14 vec2<f32>, 15 vec4<f32>, 16 the image
// type, 17 the sampler type, 18 the sampled image type, 19 its
// UniformConstant pointer, 20 the variable, 21 the f32 constant 1.0, 22 the
// coordinate vec2(1.0).
func combinedSamplerModuleGroups(body ...[]uint32) [][]uint32 {
	groups := [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		entryPointOp(spvenums.ExecutionModelFragment, 20),
		instWords(spvenums.OpDecorate, 20, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 20, uint32(spvenums.DecorationBinding), 0),
		instWords(spvenums.OpTypeVoid, 10),
		instWords(spvenums.OpTypeFunction, 11, 10),
		instWords(spvenums.OpTypeFloat, 13, 32),
		instWords(spvenums.OpTypeVector, 14, 13, 2),
		instWords(spvenums.OpTypeVector, 15, 13, 4),
		instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0),
		instWords(spvenums.OpTypeSampler, 17),
		instWords(spvenums.OpTypeSampledImage, 18, 16),
		instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassUniformConstant), 18),
		instWords(spvenums.OpConstant, 13, 21, 0x3f800000),
		instWords(spvenums.OpConstantComposite, 14, 22, 21, 21),
		instWords(spvenums.OpVariable, 19, 20, uint32(spvenums.StorageClassUniformConstant)),
		voidFunctionBody(),
		instWords(spvenums.OpLabel, 2),
	}
	groups = append(groups, body...)
	return append(groups,
		instWords(spvenums.OpReturn),
		instWords(spvenums.OpFunctionEnd),
	)
}

// sampleCombined is the body every test here parses: load the variable and
// sample through it.
var sampleCombined = [][]uint32{
	instWords(spvenums.OpLoad, 18, 30, 20),
	instWords(spvenums.OpImageSampleImplicitLod, 15, 31, 30, 22),
}

// TestCombinedSamplerSplit: a combined image sampler variable becomes an image
// global on its declared binding and a sampler global on the next free binding
// of the same group; the load yields the pair, so the sample resolves.
func TestCombinedSamplerSplit(t *testing.T) {
	module, info, err := parseWords(t, Options{}, combinedSamplerModuleGroups(sampleCombined...)...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.GlobalVariables) != 2 {
		t.Fatalf("globals = %d, want the image and the sampler", len(module.GlobalVariables))
	}
	image, sampler := module.GlobalVariables[0], module.GlobalVariables[1]
	if image.Binding == nil || *image.Binding != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Errorf("image binding = %v, want (0,0)", image.Binding)
	}
	if _, ok := module.Types[image.Type].Inner.(ir.ImageType); !ok {
		t.Errorf("first global type = %T, want an image", module.Types[image.Type].Inner)
	}
	if sampler.Binding == nil || *sampler.Binding != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Errorf("sampler binding = %v, want (0,1)", sampler.Binding)
	}
	if _, ok := module.Types[sampler.Type].Inner.(ir.SamplerType); !ok {
		t.Errorf("second global type = %T, want a sampler", module.Types[sampler.Type].Inner)
	}
	if len(info.CombinedSamplers) != 1 {
		t.Fatalf("combined samplers = %d, want 1", len(info.CombinedSamplers))
	}
	resolved := info.CombinedSamplers[0]
	if resolved.Sampler != (ir.ResourceBinding{Group: 0, Binding: 0}) ||
		resolved.Image != (ir.ResourceBinding{Group: 0, Binding: 0}) ||
		resolved.Split != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Errorf("report = %#v, want declared (0,0) split to (0,1)", resolved)
	}
	if _, ok := findExpression(&module.Functions[0], func(k ir.ExpressionKind) bool {
		sample, ok := k.(ir.ExprImageSample)
		return ok && sample.Image != sample.Sampler
	}); !ok {
		t.Error("the sample does not use the split image and sampler pair")
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestCombinedSamplerBindingMap: ParseOptions.SamplerBindings places the
// sampler where the caller says, and the next free binding skips the ones the
// module already uses.
func TestCombinedSamplerBindingMap(t *testing.T) {
	t.Run("explicit", func(t *testing.T) {
		_, info, err := parseWords(t, Options{SamplerBindings: frontend.SamplerBindings{
			{Group: 0, Binding: 0}: {Group: 3, Binding: 7},
		}}, combinedSamplerModuleGroups(sampleCombined...)...)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(info.CombinedSamplers) != 1 || info.CombinedSamplers[0].Split != (ir.ResourceBinding{Group: 3, Binding: 7}) {
			t.Fatalf("report = %#v, want the sampler at (3,7)", info.CombinedSamplers)
		}
	})

	t.Run("taken", func(t *testing.T) {
		// The image itself already uses (0,0): the caller asked for two
		// globals on one binding.
		_, _, err := parseWords(t, Options{SamplerBindings: frontend.SamplerBindings{
			{Group: 0, Binding: 0}: {Group: 0, Binding: 0},
		}}, combinedSamplerModuleGroups(sampleCombined...)...)
		if err == nil {
			t.Fatal("Parse accepted a sampler binding another resource already uses")
		}
		parseErr := requireError(t, err)
		want := errCombinedSamplerBindingTaken(ir.ResourceBinding{Group: 0, Binding: 0}, ir.ResourceBinding{Group: 0, Binding: 0}).Message
		if parseErr.Message != want {
			t.Errorf("error = %q, want %q", parseErr.Message, want)
		}
	})

	t.Run("next free", func(t *testing.T) {
		// A second handle global on (0,1) pushes the split sampler to (0,2).
		groups := [][]uint32{
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			entryPointOp(spvenums.ExecutionModelFragment, 20, 25),
			instWords(spvenums.OpDecorate, 20, uint32(spvenums.DecorationDescriptorSet), 0),
			instWords(spvenums.OpDecorate, 20, uint32(spvenums.DecorationBinding), 0),
			instWords(spvenums.OpDecorate, 25, uint32(spvenums.DecorationDescriptorSet), 0),
			instWords(spvenums.OpDecorate, 25, uint32(spvenums.DecorationBinding), 1),
			instWords(spvenums.OpTypeVoid, 10),
			instWords(spvenums.OpTypeFunction, 11, 10),
			instWords(spvenums.OpTypeFloat, 13, 32),
			instWords(spvenums.OpTypeVector, 14, 13, 2),
			instWords(spvenums.OpTypeVector, 15, 13, 4),
			instWords(spvenums.OpTypeImage, 16, 13, 1, 0, 0, 0, 1, 0),
			instWords(spvenums.OpTypeSampler, 17),
			instWords(spvenums.OpTypeSampledImage, 18, 16),
			instWords(spvenums.OpTypePointer, 19, uint32(spvenums.StorageClassUniformConstant), 18),
			instWords(spvenums.OpTypePointer, 24, uint32(spvenums.StorageClassUniformConstant), 16),
			instWords(spvenums.OpConstant, 13, 21, 0x3f800000),
			instWords(spvenums.OpConstantComposite, 14, 22, 21, 21),
			instWords(spvenums.OpVariable, 19, 20, uint32(spvenums.StorageClassUniformConstant)),
			instWords(spvenums.OpVariable, 24, 25, uint32(spvenums.StorageClassUniformConstant)),
			voidFunctionBody(),
			instWords(spvenums.OpLabel, 2),
		}
		groups = append(groups, sampleCombined...)
		groups = append(groups,
			instWords(spvenums.OpReturn),
			instWords(spvenums.OpFunctionEnd),
		)
		_, info, err := parseWords(t, Options{}, groups...)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(info.CombinedSamplers) != 1 || info.CombinedSamplers[0].Split != (ir.ResourceBinding{Group: 0, Binding: 2}) {
			t.Fatalf("report = %#v, want the sampler at (0,2)", info.CombinedSamplers)
		}
	})
}

// TestCombinedSamplerReject: the reject policy refuses the variable, as Rust
// naga does.
func TestCombinedSamplerReject(t *testing.T) {
	_, _, err := parseWords(t, Options{CombinedImageSampler: frontend.CombinedImageSamplerReject},
		combinedSamplerModuleGroups(sampleCombined...)...)
	if err == nil {
		t.Fatal("Parse accepted a combined sampler under the reject policy")
	}
	parseErr := requireError(t, err)
	if parseErr.Message != errCombinedSamplerRejected(20).Message {
		t.Errorf("error = %q, want %q", parseErr.Message, errCombinedSamplerRejected(20).Message)
	}
}
