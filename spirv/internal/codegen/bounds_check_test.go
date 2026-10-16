package codegen

import (
	"testing"
)

// spvView indexes a decoded SPIR-V module for bounds-check assertions.
type spvView struct {
	instrs []spirvInstruction
	defs   map[uint32]int    // result id -> instruction index
	block  map[int]uint32    // instruction index -> label of the block holding it
	consts map[uint32]uint32 // OpConstant result id -> 32-bit value
	nulls  map[uint32]bool   // OpConstantNull result ids
}

// resultIDWord is the word index of the result id for the opcodes these
// tests look at (instructions with a result type have it at 2).
func resultIDWord(op OpCode) int {
	switch op {
	case OpLabel:
		return 1
	case OpExtInst, OpAccessChain, OpLoad, OpULessThan, OpLogicalAnd, OpPhi, OpArrayLength, OpISub,
		OpVectorExtractDynamic, OpConstant, OpConstantNull, OpFunctionParameter, OpVariable,
		OpAtomicLoad, OpAtomicIAdd, OpCompositeExtract, OpBitcast:
		return 2
	}
	return 0
}

func newSPVView(t *testing.T, spv []byte) *spvView {
	t.Helper()
	v := &spvView{
		instrs: decodeSPIRVInstructions(spv),
		defs:   map[uint32]int{},
		block:  map[int]uint32{},
		consts: map[uint32]uint32{},
		nulls:  map[uint32]bool{},
	}
	var label uint32
	for i, in := range v.instrs {
		if in.opcode == OpLabel {
			label = in.words[1]
		}
		v.block[i] = label
		if w := resultIDWord(in.opcode); w != 0 && len(in.words) > w {
			v.defs[in.words[w]] = i
		}
		switch in.opcode {
		case OpConstant:
			if len(in.words) == 4 {
				v.consts[in.words[2]] = in.words[3]
			}
		case OpConstantNull:
			v.nulls[in.words[2]] = true
		}
	}
	return v
}

// def returns the instruction defining id, or ok=false.
func (v *spvView) def(id uint32) (spirvInstruction, bool) {
	i, ok := v.defs[id]
	if !ok {
		return spirvInstruction{}, false
	}
	return v.instrs[i], true
}

// blockOf returns the label of the block that defines id.
func (v *spvView) blockOf(id uint32) uint32 { return v.block[v.defs[id]] }

// all returns the instructions with the given opcode (for OpExtInst, with the
// given GLSL.std.450 instruction number).
func (v *spvView) all(op OpCode, ext uint32) []spirvInstruction {
	var out []spirvInstruction
	for _, in := range v.instrs {
		if in.opcode != op {
			continue
		}
		if op == OpExtInst && in.words[4] != ext {
			continue
		}
		out = append(out, in)
	}
	return out
}

func (v *spvView) isConst(id, value uint32) bool {
	c, ok := v.consts[id]
	return ok && c == value
}

func compileBoundsTest(t *testing.T, source string, policies BoundsCheckPolicies) *spvView {
	t.Helper()
	spv := compileWGSLForCapabilityTestWithOpts(t, source, Options{Version: Version1_3, BoundsCheckPolicies: policies})
	validateWithVulkanSDK(t, spv)
	return newSPVView(t, spv)
}

var (
	restrictAll = BoundsCheckPolicies{Index: BoundsCheckRestrict, Buffer: BoundsCheckRestrict}
	rzswAll     = BoundsCheckPolicies{Index: BoundsCheckReadZeroSkipWrite, Buffer: BoundsCheckReadZeroSkipWrite}
)

// TestBoundsCheckRestrict pins BoundsCheckPolicy Restrict as Rust naga's
// write_restricted_index emits it (back/spv/index.rs): a dynamic index is
// replaced by GLSL.std.450 UMin(index, length - 1), the length being the
// type's for vectors, matrices and sized arrays and OpArrayLength - 1 for a
// runtime-sized array; an index known to be in bounds is left alone.
func TestBoundsCheckRestrict(t *testing.T) {
	t.Run("local_array", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 8>;
    a[i] = 1.0;
    out = a[i + 1u];
}`, restrictAll)
		umins := v.all(OpExtInst, GLSLstd450UMin)
		if len(umins) != 2 {
			t.Fatalf("UMin count = %d, want 2 (one per dynamic index)", len(umins))
		}
		for _, u := range umins {
			if !v.isConst(u.words[6], 7) {
				t.Errorf("UMin bound operand is not the constant 7 (length 8 - 1)")
			}
		}
		for _, ac := range v.all(OpAccessChain, 0) {
			for _, ix := range ac.words[4:] {
				if _, isConst := v.consts[ix]; isConst {
					continue
				}
				if d, ok := v.def(ix); !ok || d.opcode != OpExtInst || d.words[4] != GLSLstd450UMin {
					t.Errorf("access chain index %%%d is not restricted by UMin", ix)
				}
			}
		}
	})

	t.Run("runtime_array_uses_array_length", func(t *testing.T) {
		v := compileBoundsTest(t, `
struct Data { count: u32, items: array<f32> }
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    data.items[i] = 2.0;
}`, restrictAll)
		umins := v.all(OpExtInst, GLSLstd450UMin)
		if len(umins) != 1 {
			t.Fatalf("UMin count = %d, want 1", len(umins))
		}
		sub, ok := v.def(umins[0].words[6])
		if !ok || sub.opcode != OpISub || !v.isConst(sub.words[4], 1) {
			t.Fatalf("UMin bound is not ISub(length, 1)")
		}
		if l, ok := v.def(sub.words[3]); !ok || l.opcode != OpArrayLength {
			t.Errorf("UMin bound is not computed from OpArrayLength")
		}
	})

	t.Run("vector_by_value", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
fn pick(v: vec4<f32>, i: u32) -> f32 { return v[i]; }
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    out = pick(vec4<f32>(1.0, 2.0, 3.0, 4.0), i);
}`, restrictAll)
		extracts := v.all(OpVectorExtractDynamic, 0)
		if len(extracts) != 1 {
			t.Fatalf("OpVectorExtractDynamic count = %d, want 1", len(extracts))
		}
		u, ok := v.def(extracts[0].words[4])
		if !ok || u.opcode != OpExtInst || u.words[4] != GLSLstd450UMin || !v.isConst(u.words[6], 3) {
			t.Errorf("vector index is not UMin(i, 3)")
		}
	})

	t.Run("matrix_column_through_pointer", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> m: mat3x4<f32>;
@group(0) @binding(1) var<storage, read_write> out: vec4<f32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    out = m[i];
}`, restrictAll)
		umins := v.all(OpExtInst, GLSLstd450UMin)
		if len(umins) != 1 || !v.isConst(umins[0].words[6], 2) {
			t.Errorf("want one UMin(i, 2) for a 3-column matrix, got %d", len(umins))
		}
	})

	t.Run("known_in_bounds_unchanged", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
@compute @workgroup_size(1)
fn main() {
    var a: array<f32, 8>;
    var w = vec4<f32>(1.0);
    out = a[7] + w[3];
}`, restrictAll)
		if n := len(v.all(OpExtInst, GLSLstd450UMin)); n != 0 {
			t.Errorf("UMin count = %d, want 0 for indices known to be in bounds", n)
		}
	})

	t.Run("index_and_buffer_policies_apply_separately", func(t *testing.T) {
		const source = `
@group(0) @binding(0) var<storage, read_write> buf: array<f32, 16>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var local: array<f32, 4>;
    local[i] = buf[i];
}`
		// Rust choose_policy: storage/uniform pointers use `buffer`, the rest `index`.
		onlyIndex := compileBoundsTest(t, source, BoundsCheckPolicies{Index: BoundsCheckRestrict})
		if umins := onlyIndex.all(OpExtInst, GLSLstd450UMin); len(umins) != 1 || !onlyIndex.isConst(umins[0].words[6], 3) {
			t.Errorf("Index=Restrict: want one UMin(i, 3) for the local array, got %d", len(umins))
		}
		onlyBuffer := compileBoundsTest(t, source, BoundsCheckPolicies{Buffer: BoundsCheckRestrict})
		if umins := onlyBuffer.all(OpExtInst, GLSLstd450UMin); len(umins) != 1 || !onlyBuffer.isConst(umins[0].words[6], 15) {
			t.Errorf("Buffer=Restrict: want one UMin(i, 15) for the storage array, got %d", len(umins))
		}
	})

	t.Run("signed_index", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 8>;
    out = a[i32(i) - 3];
}`, restrictAll)
		// Rust: UMin typed u32 over the i32 index (the unsigned view maps a
		// negative index to the last element).
		if n := len(v.all(OpExtInst, GLSLstd450UMin)); n != 1 {
			t.Errorf("UMin count = %d, want 1", n)
		}
	})
}

// rzswGuarded checks the ReadZeroSkipWrite shape around the instruction at
// index target: its block is entered through OpBranchConditional on cond, and
// returns that branch's merge label.
func rzswGuardedBlock(t *testing.T, v *spvView, target uint32) (cond uint32, merge uint32) {
	t.Helper()
	label := v.blockOf(target)
	for i, in := range v.instrs {
		if in.opcode == OpBranchConditional && in.words[2] == label {
			if i > 0 && v.instrs[i-1].opcode == OpSelectionMerge {
				return in.words[1], v.instrs[i-1].words[1]
			}
			return in.words[1], in.words[3]
		}
	}
	t.Fatalf("%%%d is not in a block entered by OpBranchConditional", target)
	return 0, 0
}

// TestBoundsCheckReadZeroSkipWrite pins BoundsCheckPolicy ReadZeroSkipWrite
// as Rust naga emits it (back/spv/index.rs write_index_comparison,
// block.rs write_access_chain / write_checked_load / Statement::Store): each
// dynamic index is compared with OpULessThan against the length, checks are
// combined with OpLogicalAnd, and the access chain itself with its load or
// store runs only in the in-bounds branch; a load merges its value with zero
// through OpPhi. Better than Rust, atomics stay atomic on that path and
// read-modify-write atomics are supported (Rust: "Atomics out-of-bounds
// handling" not implemented).
func TestBoundsCheckReadZeroSkipWrite(t *testing.T) {
	t.Run("local_array_load", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 8>;
    out = a[i];
}`, rzswAll)
		lts := v.all(OpULessThan, 0)
		if len(lts) != 1 || !v.isConst(lts[0].words[4], 8) {
			t.Fatalf("want one ULessThan(i, 8), got %d", len(lts))
		}
		var load spirvInstruction
		for _, ac := range v.all(OpAccessChain, 0) {
			for _, ld := range v.all(OpLoad, 0) {
				if ld.words[3] == ac.words[2] {
					load = ld
				}
			}
		}
		if load.opcode == 0 {
			t.Fatal("no load through an access chain")
		}
		cond, merge := rzswGuardedBlock(t, v, load.words[2])
		if cond != lts[0].words[2] {
			t.Errorf("branch condition is not the ULessThan")
		}
		if v.blockOf(load.words[3]) != v.blockOf(load.words[2]) {
			t.Errorf("the access chain is not in the in-bounds block")
		}
		var phi spirvInstruction
		for _, p := range v.all(OpPhi, 0) {
			if v.blockOf(p.words[2]) == merge {
				phi = p
			}
		}
		if phi.opcode == 0 || len(phi.words) != 7 {
			t.Fatal("no two-way OpPhi in the merge block")
		}
		// pairs: (value, parent)
		gotNull := v.nulls[phi.words[3]] || v.nulls[phi.words[5]]
		gotLoad := phi.words[3] == load.words[2] || phi.words[5] == load.words[2]
		if !gotNull || !gotLoad {
			t.Errorf("OpPhi does not merge the loaded value with zero")
		}
	})

	t.Run("store_is_skipped", func(t *testing.T) {
		v := compileBoundsTest(t, `
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 8>;
    a[i] = 3.0;
}`, rzswAll)
		if n := len(v.all(OpULessThan, 0)); n != 1 {
			t.Fatalf("ULessThan count = %d, want 1", n)
		}
		found := false
		for _, in := range v.all(OpStore, 0) {
			if d, ok := v.def(in.words[1]); ok && d.opcode == OpAccessChain {
				found = true
				label := v.block[v.defs[in.words[1]]]
				// the store is in the same in-bounds block as its access chain
				for i, x := range v.instrs {
					if x.opcode == OpStore && x.words[1] == in.words[1] && v.block[i] != label {
						t.Errorf("store and its access chain are in different blocks")
					}
				}
				rzswGuardedBlock(t, v, in.words[1])
			}
		}
		if !found {
			t.Fatal("no store through an access chain")
		}
		if n := len(v.all(OpPhi, 0)); n != 0 {
			t.Errorf("OpPhi count = %d, want 0 for a store", n)
		}
	})

	t.Run("two_indices_combined", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<array<f32, 4>, 8>;
    out = a[i][i + 1u];
}`, rzswAll)
		ands := v.all(OpLogicalAnd, 0)
		if len(v.all(OpULessThan, 0)) != 2 || len(ands) != 1 {
			t.Fatalf("want two ULessThan combined by one LogicalAnd")
		}
		used := false
		for _, b := range v.all(OpBranchConditional, 0) {
			used = used || b.words[1] == ands[0].words[2]
		}
		if !used {
			t.Errorf("the branch does not use the combined condition")
		}
	})

	t.Run("runtime_array_compares_with_array_length", func(t *testing.T) {
		v := compileBoundsTest(t, `
struct Data { count: u32, items: array<f32> }
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    data.count = u32(data.items[i]);
}`, rzswAll)
		lts := v.all(OpULessThan, 0)
		if len(lts) != 1 {
			t.Fatalf("ULessThan count = %d, want 1", len(lts))
		}
		if l, ok := v.def(lts[0].words[4]); !ok || l.opcode != OpArrayLength {
			t.Errorf("the length operand is not OpArrayLength")
		}
	})

	t.Run("vector_by_value", func(t *testing.T) {
		v := compileBoundsTest(t, `
@group(0) @binding(0) var<storage, read_write> out: f32;
fn pick(v: vec4<f32>, i: u32) -> f32 { return v[i]; }
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    out = pick(vec4<f32>(1.0), i);
}`, rzswAll)
		extracts := v.all(OpVectorExtractDynamic, 0)
		if len(extracts) != 1 {
			t.Fatalf("OpVectorExtractDynamic count = %d, want 1", len(extracts))
		}
		_, merge := rzswGuardedBlock(t, v, extracts[0].words[2])
		phiFound := false
		for _, p := range v.all(OpPhi, 0) {
			if v.blockOf(p.words[2]) == merge && (p.words[3] == extracts[0].words[2] || p.words[5] == extracts[0].words[2]) {
				phiFound = true
			}
		}
		if !phiFound {
			t.Errorf("no OpPhi merging the extracted component")
		}
	})

	t.Run("atomics_stay_atomic", func(t *testing.T) {
		v := compileBoundsTest(t, `
struct Counters { hits: array<atomic<u32>, 4> }
@group(0) @binding(0) var<storage, read_write> c: Counters;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let old = atomicAdd(&c.hits[i], 1u);
    atomicStore(&c.hits[i + 1u], atomicLoad(&c.hits[i + 2u]) + old);
}`, rzswAll)
		if n := len(v.all(OpLoad, 0)) + len(v.all(OpStore, 0)); n != 0 {
			for _, in := range append(v.all(OpLoad, 0), v.all(OpStore, 0)...) {
				ptr := in.words[3]
				if in.opcode == OpStore {
					ptr = in.words[1]
				}
				if d, ok := v.def(ptr); ok && d.opcode == OpAccessChain {
					t.Errorf("non-atomic %v through an access chain into atomic data", in.opcode)
				}
			}
		}
		for _, op := range []OpCode{OpAtomicIAdd, OpAtomicLoad} {
			ins := v.all(op, 0)
			if len(ins) != 1 {
				t.Fatalf("%v count = %d, want 1", op, len(ins))
			}
			rzswGuardedBlock(t, v, ins[0].words[2])
		}
		stores := v.all(OpAtomicStore, 0)
		if len(stores) != 1 {
			t.Fatalf("OpAtomicStore count = %d, want 1", len(stores))
		}
		rzswGuardedBlock(t, v, stores[0].words[1])
		if n := len(v.all(OpULessThan, 0)); n != 3 {
			t.Errorf("ULessThan count = %d, want 3", n)
		}
	})
}

// TestBoundsCheckUncheckedEmitsNothing: the default policy adds no checks.
func TestBoundsCheckUncheckedEmitsNothing(t *testing.T) {
	v := compileBoundsTest(t, `
struct Data { count: u32, items: array<f32> }
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    var a: array<f32, 8>;
    a[i] = data.items[i];
    data.count = u32(a[i + 1u]);
}`, BoundsCheckPolicies{})
	if n := len(v.all(OpExtInst, GLSLstd450UMin)) + len(v.all(OpULessThan, 0)); n != 0 {
		t.Errorf("Unchecked emitted %d bounds-check instructions", n)
	}
}

// TestBoundsCheckPolicyPerAddressSpace pins Rust naga's choose_policy:
// uniform and storage buffers use Buffer; private, workgroup and function
// memory use Index.
func TestBoundsCheckPolicyPerAddressSpace(t *testing.T) {
	const source = `
var<private> p: array<u32, 5>;
var<workgroup> w: array<u32, 6>;
@group(0) @binding(0) var<uniform> u: array<vec4<u32>, 7>;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    out = p[i] + w[i] + u[i].x;
}`
	bounds := func(v *spvView) map[uint32]int {
		got := map[uint32]int{}
		for _, u := range v.all(OpExtInst, GLSLstd450UMin) {
			got[v.consts[u.words[6]]]++
		}
		return got
	}
	index := compileBoundsTest(t, source, BoundsCheckPolicies{Index: BoundsCheckRestrict})
	if got := bounds(index); len(got) != 2 || got[4] != 1 || got[5] != 1 {
		t.Errorf("Index=Restrict: UMin bounds %v, want private (4) and workgroup (5)", got)
	}
	buffer := compileBoundsTest(t, source, BoundsCheckPolicies{Buffer: BoundsCheckRestrict})
	if got := bounds(buffer); len(got) != 1 || got[6] != 1 {
		t.Errorf("Buffer=Restrict: UMin bounds %v, want uniform (6)", got)
	}
}

// TestBoundsCheckSpilledValues: an array or matrix value indexed at run time
// is spilled to a Function variable; the index is then checked with the
// Index policy (the composite is a value), as Rust naga's write_checked_load
// on the spilled access.
func TestBoundsCheckSpilledValues(t *testing.T) {
	const source = `
@group(0) @binding(0) var<storage, read_write> out: vec3<f32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let a = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let m = mat3x3<f32>(vec3(1.0), vec3(2.0), vec3(3.0));
    out = m[i] * a[i + 1u];
}`
	t.Run("restrict", func(t *testing.T) {
		v := compileBoundsTest(t, source, BoundsCheckPolicies{Index: BoundsCheckRestrict})
		got := map[uint32]int{}
		for _, u := range v.all(OpExtInst, GLSLstd450UMin) {
			got[v.consts[u.words[6]]]++
		}
		if len(got) != 2 || got[2] != 1 || got[3] != 1 {
			t.Errorf("UMin bounds %v, want matrix (2) and array (3)", got)
		}
	})
	t.Run("read_zero_skip_write", func(t *testing.T) {
		v := compileBoundsTest(t, source, BoundsCheckPolicies{Index: BoundsCheckReadZeroSkipWrite})
		lts := v.all(OpULessThan, 0)
		if len(lts) != 2 {
			t.Fatalf("ULessThan count = %d, want 2", len(lts))
		}
		phis := 0
		for _, ld := range v.all(OpLoad, 0) {
			ac, ok := v.def(ld.words[3])
			if !ok || ac.opcode != OpAccessChain {
				continue
			}
			if root, ok := v.def(ac.words[3]); !ok || root.opcode != OpVariable || StorageClass(root.words[3]) != StorageClassFunction {
				continue
			}
			_, merge := rzswGuardedBlock(t, v, ld.words[2])
			for _, p := range v.all(OpPhi, 0) {
				if v.blockOf(p.words[2]) == merge {
					phis++
				}
			}
		}
		if phis != 2 {
			t.Errorf("phis merging guarded loads from spill variables = %d, want 2", phis)
		}
	})
}

// TestBoundsCheckStructMembersAreNotChecked: in a chain through a struct only
// the array index is checked; member selection and a constant in-bounds
// vector component need no check (Rust bounds_check_iter skips struct
// members).
func TestBoundsCheckStructMembersAreNotChecked(t *testing.T) {
	v := compileBoundsTest(t, `
struct Item { pos: vec3<f32>, id: u32 }
struct Data { count: u32, items: array<Item, 16> }
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    data.items[i].pos.z = 1.0;
}`, rzswAll)
	lts := v.all(OpULessThan, 0)
	if len(lts) != 1 || !v.isConst(lts[0].words[4], 16) {
		t.Errorf("want one ULessThan(i, 16) for the array index only, got %d", len(lts))
	}
}
