package codegen

import (
	"strings"
	"testing"
)

// TestSubgroupOperandLoadsAreBaked: the operands of the subgroup statements
// count as uses of their expressions, as Rust naga's analyzer counts every
// statement operand, so a Load used only there is baked like any other Load
// (bake_ref_count 1): Rust writes `uint _e5 = v;` and passes `_e5`. naga
// left these statements out of its reference count and inlined the load.
func TestSubgroupOperandLoadsAreBaked(t *testing.T) {
	code := wgslToHLSL(t, `enable subgroups;
@group(0) @binding(0) var<storage, read_write> o: array<u32, 4>;
@compute @workgroup_size(64) fn main(@builtin(subgroup_invocation_id) id: u32) {
    var v: u32 = id;
    var i: u32 = 4u;
    var p: bool = id == 0u;
    v = subgroupAdd(v);
    v = subgroupBroadcast(v, 4u);
    v = subgroupShuffle(v, i);
    let b = subgroupBallot(p);
    o[0] = v + b.x;
}`)
	for _, call := range []string{"WaveActiveSum(", "WaveReadLaneAt(", "WaveActiveBallot("} {
		i := strings.Index(code, call)
		if i < 0 {
			t.Fatalf("no %s in:\n%s", call, code)
		}
		arg := code[i+len(call):]
		if strings.HasPrefix(arg, "v") || strings.HasPrefix(arg, "p") || strings.Contains(arg[:strings.IndexByte(arg, ')')], ", i") {
			t.Errorf("%s takes an inlined load: %s", call, arg[:strings.IndexByte(arg, ';')])
		}
	}
	for _, want := range []string{"uint _e", "bool _e"} {
		if !strings.Contains(code, want) {
			t.Errorf("no baked load %q in:\n%s", want, code)
		}
	}
}
