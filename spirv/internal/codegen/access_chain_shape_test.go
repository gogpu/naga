package codegen

import (
	"testing"
)

// spvChainShape describes how the pointers that loads, stores and atomics
// dereference were built.
type spvChainShape struct {
	// derefs is the number of loads, stores and atomic operations whose
	// pointer is the result of an OpAccessChain.
	derefs int
	// nested counts those whose OpAccessChain is based on another
	// OpAccessChain that is not a wrapper of a global (an access chain on a
	// variable whose only index is the constant 0).
	nested int
}

// spvAccessChainShape analyzes the pointer operands of every load, store and
// atomic instruction in spv.
func spvAccessChainShape(t *testing.T, spv []byte) spvChainShape {
	t.Helper()
	instrs := decodeSPIRVInstructions(spv)

	chains := map[uint32][]uint32{} // result id -> [base, indices...]
	variables := map[uint32]bool{}  // OpVariable and OpFunctionParameter results
	zero := map[uint32]bool{}       // integer constants with value 0
	for _, in := range instrs {
		switch in.opcode {
		case OpAccessChain:
			chains[in.words[2]] = in.words[3:]
		case OpVariable, OpFunctionParameter:
			variables[in.words[2]] = true
		case OpConstant:
			if len(in.words) == 4 && in.words[3] == 0 {
				zero[in.words[2]] = true
			}
		}
	}
	isWrapper := func(id uint32) bool {
		c, ok := chains[id]
		return ok && len(c) == 2 && variables[c[0]] && zero[c[1]]
	}

	var shape spvChainShape
	check := func(ptr uint32) {
		c, ok := chains[ptr]
		if !ok {
			return
		}
		shape.derefs++
		if base := c[0]; !variables[base] && !isWrapper(base) {
			shape.nested++
		}
	}
	for _, in := range instrs {
		switch in.opcode {
		case OpLoad, OpAtomicLoad, OpAtomicExchange, OpAtomicCompareExch, OpAtomicFAddEXT,
			OpAtomicIIncrement, OpAtomicIDecrement, OpAtomicIAdd, OpAtomicISub,
			OpAtomicSMin, OpAtomicUMin, OpAtomicSMax, OpAtomicUMax,
			OpAtomicAnd, OpAtomicOr, OpAtomicXor:
			check(in.words[3])
		case OpStore, OpAtomicStore:
			check(in.words[1])
		}
	}
	return shape
}

// TestAccessChainIsOneInstructionPerDereference: Rust naga's SPIR-V writer
// builds the pointer a load, store or atomic dereferences as ONE OpAccessChain
// from the root variable, with every index of the Access/AccessIndex chain
// (back/spv/block.rs write_access_chain). The access chain is then a single
// instruction that a bounds check can place inside its in-bounds branch.
// Building one OpAccessChain per link, each on top of the previous one,
// produces equivalent pointers but more instructions and pointer types.
func TestAccessChainIsOneInstructionPerDereference(t *testing.T) {
	tests := []struct {
		name   string
		source string
		derefs int
	}{
		{
			name: "local_struct_array_vector",
			source: `
struct S { a: array<vec4<f32>, 4>, b: u32 }
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var s: S;
    s.a[i].y = 1.0;
    s.a[i % 4u][2u] = s.a[1].x;
}`,
			derefs: 3,
		},
		{
			name: "storage_struct_runtime_array",
			source: `
struct Item { pos: vec3<f32>, ids: array<u32, 3> }
struct Data { count: u32, items: array<Item> }
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    data.items[id.x].ids[id.y] = data.items[id.y].ids[2];
    data.items[id.x].pos.z = 3.0;
}`,
			derefs: 3,
		},
		{
			name: "workgroup_nested_arrays",
			source: `
var<workgroup> grid: array<array<u32, 8>, 8>;
@compute @workgroup_size(8)
fn main(@builtin(local_invocation_id) id: vec3<u32>) {
    grid[id.x][id.y] = grid[id.y][id.x] + 1u;
}`,
			// two accesses plus the zero-init store of the whole variable
			derefs: 2,
		},
		{
			name: "wrapped_storage_array",
			source: `
@group(0) @binding(0) var<storage, read_write> v: array<vec4<f32>>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    v[id.x].w = v[id.y][1];
}`,
			derefs: 2,
		},
		{
			name: "pointer_parameter",
			source: `
struct P { m: mat4x4<f32>, k: array<f32, 2> }
fn bump(p: ptr<function, P>, i: u32) {
    (*p).m[i][i] += (*p).k[i % 2u];
}
@compute @workgroup_size(1)
fn main() {
    var p: P;
    bump(&p, 1u);
}`,
			derefs: 3,
		},
		{
			name: "atomics_in_struct_array",
			source: `
struct Counters { hits: array<atomic<u32>, 4> }
@group(0) @binding(0) var<storage, read_write> c: Counters;
var<workgroup> w: array<array<atomic<u32>, 2>, 2>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    atomicAdd(&c.hits[i % 4u], 1u);
    atomicStore(&w[i % 2u][1], atomicLoad(&c.hits[1]));
}`,
			derefs: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spv := compileAtomicTestShader(t, tt.source)
			shape := spvAccessChainShape(t, spv)
			if shape.derefs < tt.derefs {
				t.Fatalf("only %d dereferences through an access chain, want at least %d; the analysis missed them", shape.derefs, tt.derefs)
			}
			if shape.nested != 0 {
				t.Errorf("%d of %d dereferenced pointers come from an access chain on another access chain; want one OpAccessChain from the root",
					shape.nested, shape.derefs)
			}
		})
	}
}
