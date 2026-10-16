package codegen

import (
	"testing"
)

// spvDeadValues counts, per opcode, the results of pure value instructions
// (spvPureValueOps) in function bodies that are never observed: no
// instruction other than a dead pure value reads them.
func spvDeadValues(t *testing.T, spv []byte) map[OpCode]int {
	t.Helper()
	instrs := decodeSPIRVInstructions(spv)
	if instrs == nil {
		t.Fatal("cannot decode SPIR-V")
	}
	users := map[uint32][]uint32{}
	type def struct {
		op OpCode
		id uint32
	}
	var defs []def
	inFunction := false
	for _, in := range instrs {
		if in.opcode == OpFunction {
			inFunction = true
		}
		if !inFunction {
			continue
		}
		if spvPureValueOps[in.opcode] {
			defs = append(defs, def{in.opcode, in.words[2]})
			for _, w := range in.words[3:] {
				users[w] = append(users[w], in.words[2])
			}
			continue
		}
		for _, w := range in.words[1:] {
			users[w] = append(users[w], 0)
		}
	}
	var dead func(id uint32) bool
	dead = func(id uint32) bool {
		for _, u := range users[id] {
			if u == 0 || !dead(u) {
				return false
			}
		}
		return true
	}
	counts := map[OpCode]int{}
	for _, d := range defs {
		if dead(d.id) {
			counts[d.op]++
		}
	}
	return counts
}

// TestEmitDoesNotMaterializePointers pins BUG-SPV-PTR-EMIT-001. A pointer-typed
// Access/AccessIndex (s[i].field, w[1].a[2], &arr[i]) is not a value: Rust
// naga's SPIR-V writer emits nothing for it at Emit and builds the access chain
// where the pointer is used (back/spv/block.rs cache_expression_value, "Put off
// generating any code for this until we find the Expression that actually
// dereferences the pointer"). Materializing it at Emit leaves an access chain,
// an OpLoad and, for workgroup data, an OpCopyLogical whose results nothing
// uses — on atomic data that dead load is a non-atomic read.
func TestEmitDoesNotMaterializePointers(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "storage_array_of_structs",
			source: `
struct P {
    pos: vec4<f32>,
    vel: vec4<f32>,
    ids: array<u32, 4>,
}
@group(0) @binding(0) var<storage, read_write> ps: array<P>;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let i = id.x;
    let p = ps[i].pos;
    ps[i].vel = p * 2.0;
    ps[i].ids[2] = 7u;
    out[i] = ps[i].vel.y + f32(ps[i].ids[1]);
}
`,
		},
		{
			name: "workgroup_struct_members",
			source: `
struct W {
    a: array<f32, 4>,
    b: vec2<f32>,
}
var<workgroup> w: array<W, 2>;
@group(0) @binding(0) var<storage, read_write> out: array<f32, 2>;

@compute @workgroup_size(1)
fn main() {
    w[1].a[2] = 1.0;
    out[0] = w[1].a[2] + w[0].b.x;
    out[1] = w[1].a[3];
}
`,
		},
		{
			name: "uniform_and_local",
			source: `
struct U {
    m: mat4x4<f32>,
    v: array<vec4<f32>, 2>,
}
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<storage, read_write> out: vec4<f32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var arr: array<f32, 4>;
    arr[i % 4u] = u.v[1].z;
    out = u.m[2] + vec4<f32>(arr[1]);
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spv := compileAtomicTestShader(t, tt.source)
			for op, n := range spvDeadValues(t, spv) {
				switch op {
				case OpLoad, OpCopyLogical, OpAccessChain:
					t.Errorf("%d dead results of opcode %d (OpLoad=61, OpAccessChain=65, OpCopyLogical=400)", n, op)
				}
			}
		})
	}
}

// TestNonUniformBindingArrayPointerDecorated pins the pointer path of
// VUID-RuntimeSpirv-NonUniform-06274 after BUG-SPV-PTR-EMIT-001: the pointer
// a load reads through must be decorated NonUniform when the chain indexes a
// binding array with a non-uniform index. Rust decorates the final pointer of
// the access chain (back/spv/block.rs write_access_chain); for the corpus
// shader binding-buffer-arrays that is exactly one pointer.
func TestNonUniformBindingArrayPointerDecorated(t *testing.T) {
	const source = `
struct UniformIndex {
    index: u32
}
struct Foo { x: u32, far: array<i32> }
@group(0) @binding(0)
var<storage, read> storage_array: binding_array<Foo, 1>;
@group(0) @binding(10)
var<uniform> uni: UniformIndex;

struct FragmentIn {
    @location(0) index: u32,
}

@fragment
fn main(fragment_in: FragmentIn) -> @location(0) u32 {
    let uniform_index = uni.index;
    let non_uniform_index = fragment_in.index;
    var u1 = 0u;
    u1 += storage_array[0].x;
    u1 += storage_array[uniform_index].x;
    u1 += storage_array[non_uniform_index].x;
    return u1;
}
`
	spv := compileAtomicTestShader(t, source)
	instrs := decodeSPIRVInstructions(spv)

	nonUniform := map[uint32]bool{}
	hasCapability := false
	for _, in := range instrs {
		switch {
		case in.opcode == OpDecorate && Decoration(in.words[2]) == DecorationNonUniform:
			nonUniform[in.words[1]] = true
		case in.opcode == OpCapability && Capability(in.words[1]) == CapabilityShaderNonUniform:
			hasCapability = true
		}
	}
	if !hasCapability {
		t.Error("ShaderNonUniform capability missing")
	}

	// Used loads from storage_array (binding 0), and those whose pointer
	// operand is decorated.
	decorated, total := 0, 0
	for _, a := range collectMemAccesses(t, spv) {
		if a.root != "binding0" || a.kind != accessLoad {
			continue
		}
		total++
		if nonUniform[a.ptr] {
			decorated++
		}
	}
	if total != 3 || decorated != 1 {
		t.Errorf("loads from storage_array: %d, with a NonUniform pointer: %d; want 3 and 1", total, decorated)
	}
}
