package dxil

import (
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
