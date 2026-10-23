package dxil

import (
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// A depth texture samples, compares and loads to a scalar: vec4(v, v, v, 1)
// of one writes it to the first three components, where it wrote the other
// components of the sample result.
func TestDepthTextureResultIsAScalar(t *testing.T) {
	for _, read := range []string{
		"textureSample(d, s, p.xy)",
		"textureSampleCompare(d, sc, p.xy, 0.5)",
		"textureLoad(d, vec2<i32>(p.xy), 0)",
	} {
		_, fn := emitMainModule(t, `
@group(0) @binding(0) var d: texture_depth_2d;
@group(0) @binding(1) var s: sampler;
@group(0) @binding(2) var sc: sampler_comparison;
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
	let v = `+read+`;
	return vec4<f32>(v, v, v, 1.0);
}
`)
		out := callOperands(fn, "dx.op.storeOutput", 4)
		if len(out) != 4 || out[0] != out[1] || out[0] != out[2] {
			t.Errorf("%s: components written from values %v, want the same value in the first three", read, out)
		}
	}
}

// callOperands returns operand i of every call in fn to a function whose
// name starts with prefix, in order.
func callOperands(fn *module.Function, prefix string, i int) []int {
	var ops []int
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrCall && in.CalledFunc != nil && strings.HasPrefix(in.CalledFunc.Name, prefix) && i < len(in.Operands) {
				ops = append(ops, in.Operands[i])
			}
		}
	}
	return ops
}

// A sampler reads its place in the sampler heap from the slot of its bind
// group's sampler index buffer at the register the binding map gives it,
// as the HLSL writer reads it, where it read the slot of its @binding.
func TestSamplerIndexAtItsRegister(t *testing.T) {
	opts := DefaultOptions()
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0},
		{Group: 0, Binding: 1}: {Space: 255, Register: 0},
		{Group: 0, Binding: 2}: {Space: 255, Register: 1},
	}
	mod, fn := emitMainModuleOpts(t, `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var a: sampler;
@group(0) @binding(2) var b: sampler;
@fragment fn main() -> @location(0) vec4<f32> {
	return textureSample(t, a, vec2(0.25)) + textureSample(t, b, vec2(0.75));
}
`, opts)
	if slots := intConstants(mod, callOperands(fn, "dx.op.bufferLoad", 2)); !slices.Equal(slots, []int64{0, 1}) {
		t.Errorf("samplers read index buffer slots %v, want [0 1]", slots)
	}
}

// intConstants returns the values of the integer constants among ids.
func intConstants(mod *module.Module, ids []int) []int64 {
	values := map[int]int64{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			values[c.ValueID] = c.IntValue
		}
	}
	var out []int64
	for _, id := range ids {
		if v, ok := values[id]; ok {
			out = append(out, v)
		}
	}
	return out
}

// A sampler's handle is at its heap's first register plus its index in the
// heap, as for any other array of resources: with the comparison samplers
// at register 2048, as wgpu-hal puts them, a comparison sampler's index
// read from the index buffer gets 2048 added. It got 0 added, the register
// of a sampler in the other heap.
func TestSamplerHandleAtItsHeapRegister(t *testing.T) {
	opts := DefaultOptions()
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0},
		{Group: 0, Binding: 1}: {Space: 255, Register: 0},
		{Group: 0, Binding: 2}: {Space: 255, Register: 1},
	}
	opts.SamplerHeapTargets = &SamplerHeapBindTargets{
		StandardSamplers:   BindTarget{Register: 0},
		ComparisonSamplers: BindTarget{Register: 2048},
	}
	mod, fn := emitMainModuleOpts(t, `
@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler;
@group(0) @binding(2) var sc: sampler_comparison;
@fragment fn main() -> @location(0) vec4<f32> {
	return vec4(textureSample(t, s, vec2(0.5)), textureSampleCompare(t, sc, vec2(0.5), 0.5), 0.0, 1.0);
}
`, opts)
	var bases []int64
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrBinOp && in.Operands[2] == 0 {
				bases = append(bases, intConstants(mod, in.Operands[1:2])...)
			}
		}
	}
	if !slices.Equal(bases, []int64{0, 2048}) {
		t.Errorf("sampler indices offset by %v, want [0 2048]", bases)
	}
}

// A texture_external is refused: the DXIL writer has none of its planes
// and conversion, and wrote it as a texture of the wrong type.
func TestExternalTexturesAreRefused(t *testing.T) {
	_, err := Compile(lowerForTest(t, `
@group(0) @binding(0) var tex: texture_external;
@fragment fn main() -> @location(0) vec4<f32> {
	return textureLoad(tex, vec2<u32>(0u));
}
`), DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "external textures") {
		t.Errorf("err = %v, want the external textures error", err)
	}
}
