package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// With SpecialConstantsBinding, a vertex stage adds first_vertex and
// first_instance of the special constants to vertex_index and
// instance_index, which SV_VertexID and SV_InstanceID count from zero, as
// the HLSL writer adds _NagaConstants.first_vertex, also for builtins in a
// struct argument; a compute stage reads num_workgroups from them.
func TestSpecialConstantsOffsetVertexAndInstance(t *testing.T) {
	opts := DefaultOptions()
	opts.SpecialConstantsBinding = &BindTarget{Space: 1, Register: 5}
	for name, tc := range map[string]struct {
		src         string
		wantAdds    int // adds of a special constant
		wantLoads   int // cbufferLoadLegacy calls
		wantSpecial bool
	}{
		"vertex": {`
@vertex fn main(@builtin(vertex_index) v: u32, @builtin(instance_index) i: u32) -> @builtin(position) vec4<f32> {
    return vec4(f32(v), f32(i), 0.0, 1.0);
}`, 2, 2, true},
		"vertex struct": {`
struct In { @builtin(vertex_index) v: u32, @location(0) p: vec2<f32> }
@vertex fn main(in: In) -> @builtin(position) vec4<f32> {
    return vec4(in.p, f32(in.v), 1.0);
}`, 1, 1, true},
		"compute": {`
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(num_workgroups) n: vec3<u32>) {
    out[0] = n.y;
}`, 0, 1, true},
		"no special input": {`
@vertex fn main(@location(0) p: vec4<f32>) -> @builtin(position) vec4<f32> {
    return p;
}`, 0, 0, false},
	} {
		mod, fn := emitMainModuleOpts(t, tc.src, opts)
		loads, adds := 0, 0
		fromCBuf := map[int]bool{}
		for _, bb := range fn.BasicBlocks {
			for _, in := range bb.Instructions {
				switch {
				case in.Kind == module.InstrCall && in.CalledFunc != nil && in.CalledFunc.Name == "dx.op.cbufferLoadLegacy.i32":
					loads++
					fromCBuf[in.ValueID] = true
				case in.Kind == module.InstrExtractVal && fromCBuf[in.Operands[0]]:
					fromCBuf[in.ValueID] = true
				case in.Kind == module.InstrBinOp && len(in.Operands) >= 2 && fromCBuf[in.Operands[0]]:
					adds++
				}
			}
		}
		if loads != tc.wantLoads || adds != tc.wantAdds {
			t.Errorf("%s: %d special constant loads and %d adds, want %d and %d", name, loads, adds, tc.wantLoads, tc.wantAdds)
		}
		if got := hasCBVAt(mod, 1, 5); got != tc.wantSpecial {
			t.Errorf("%s: CBV at b5 space1 declared: %v, want %v", name, got, tc.wantSpecial)
		}
	}
}

// hasCBVAt reports whether the module's dx.resources metadata declares a
// constant buffer at register in space.
func hasCBVAt(mod *module.Module, space, register int64) bool {
	for _, nm := range mod.NamedMetadata {
		if nm.Name != "dx.resources" || len(nm.Operands) == 0 {
			continue
		}
		classes := nm.Operands[0].SubNodes
		if len(classes) < 3 || classes[2] == nil {
			return false
		}
		for _, cbv := range classes[2].SubNodes {
			if len(cbv.SubNodes) > 4 && metadataInt(cbv.SubNodes[3]) == space && metadataInt(cbv.SubNodes[4]) == register {
				return true
			}
		}
	}
	return false
}

func metadataInt(n *module.MetadataNode) int64 {
	if n == nil || n.ValueConst == nil {
		return -1
	}
	return n.ValueConst.IntValue
}
