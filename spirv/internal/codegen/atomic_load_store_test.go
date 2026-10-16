package codegen

import (
	"fmt"
	"strings"
	"testing"
)

// BUG-SPV-ATOMIC-001: naga IR has no atomic load/store statement. WGSL
// atomicLoad lowers to Expression::Load and atomicStore to Statement::Store on a
// pointer to TypeInner::Atomic. Rust naga's SPIR-V writer
// (back/spv/block.rs write_checked_load and Statement::Store) checks the
// pointee type and emits OpAtomicLoad / OpAtomicStore with the scope from
// AddressSpace::to_spirv_semantics_and_scope (Storage → Device,
// WorkGroup → Workgroup) as an i32 constant (get_scope_constant) and empty
// (relaxed) memory semantics as a u32 constant (get_index_constant). A plain
// OpLoad/OpStore there is a data race: SPIR-V has no atomic type, so
// spirv-val cannot catch it.

// spvAccessKind classifies a memory access found in a SPIR-V module.
type spvAccessKind string

const (
	accessAtomicLoad  spvAccessKind = "OpAtomicLoad"
	accessAtomicStore spvAccessKind = "OpAtomicStore"
	accessAtomicRMW   spvAccessKind = "OpAtomic(read-modify-write)"
	// accessLoad is an OpLoad whose result is used.
	accessLoad spvAccessKind = "OpLoad"
	// accessDeadLoad is an OpLoad whose result is never used.
	accessDeadLoad spvAccessKind = "OpLoad(dead)"
	accessStore    spvAccessKind = "OpStore"
	// accessZeroInit is a whole-variable OpStore of OpConstantNull: the
	// workgroup zero-initialization polyfill, a plain store in Rust too
	// (back/spv/writer.rs generate_workgroup_vars_init_block).
	accessZeroInit spvAccessKind = "OpStore(zero-init)"
)

// spvMemAccess is one memory access in a SPIR-V module, attributed to the
// variable at the root of its pointer operand.
type spvMemAccess struct {
	kind spvAccessKind
	ptr  uint32 // pointer operand of an OpLoad
	op   OpCode
	root string
	// Scope and semantics operands; set for atomic instructions only.
	scope     spvIntConstant
	semantics []spvIntConstant // one entry; two for OpAtomicCompareExchange
}

// spvIntConstant is a 32-bit integer OpConstant: its value and the
// signedness of its OpTypeInt.
type spvIntConstant struct {
	value  uint32
	signed bool
	ok     bool // false when the operand is not a 32-bit integer OpConstant
}

func (c spvIntConstant) String() string {
	if !c.ok {
		return "<not an int32 constant>"
	}
	if c.signed {
		return fmt.Sprintf("i32 %d", int32(c.value))
	}
	return fmt.Sprintf("u32 %d", c.value)
}

// spvAtomicOperandLayout gives the word positions of pointer, scope and
// semantics operands for the atomic opcodes this test inspects.
var spvAtomicOperandLayout = map[OpCode]struct{ ptr, scope, firstSem, nSem int }{
	OpAtomicLoad:        {3, 4, 5, 1},
	OpAtomicStore:       {1, 2, 3, 1},
	OpAtomicExchange:    {3, 4, 5, 1},
	OpAtomicCompareExch: {3, 4, 5, 2},
	OpAtomicIIncrement:  {3, 4, 5, 1},
	OpAtomicIDecrement:  {3, 4, 5, 1},
	OpAtomicIAdd:        {3, 4, 5, 1},
	OpAtomicFAddEXT:     {3, 4, 5, 1},
	OpAtomicISub:        {3, 4, 5, 1},
	OpAtomicSMin:        {3, 4, 5, 1},
	OpAtomicUMin:        {3, 4, 5, 1},
	OpAtomicSMax:        {3, 4, 5, 1},
	OpAtomicUMax:        {3, 4, 5, 1},
	OpAtomicAnd:         {3, 4, 5, 1},
	OpAtomicOr:          {3, 4, 5, 1},
	OpAtomicXor:         {3, 4, 5, 1},
}

// spvPureValueOps are side-effect-free instructions with layout
// "ResultType Result Operands..." through which a dead value may flow.
var spvPureValueOps = map[OpCode]bool{
	OpLoad:             true,
	OpAccessChain:      true,
	OpCopyLogical:      true,
	OpCompositeExtract: true,
	OpBitcast:          true,
}

// collectMemAccesses decodes every OpLoad, OpStore and atomic instruction of a
// module and resolves its pointer operand through OpAccessChain to the root
// OpVariable. Roots are named without relying on OpName (the backend only
// emits names with Options.Debug): a resource by its binding ("binding3"), a
// workgroup variable by its declaration order among workgroup variables
// ("workgroup0"); globals are declared in module order.
func collectMemAccesses(t *testing.T, spv []byte) []spvMemAccess {
	t.Helper()
	instrs := decodeSPIRVInstructions(spv)
	if instrs == nil {
		t.Fatal("cannot decode SPIR-V")
	}

	names := map[uint32]string{}
	workgroupVars := 0
	type intType struct {
		width  uint32
		signed bool
	}
	intTypes := map[uint32]intType{}
	type constant struct{ typ, value uint32 }
	constants := map[uint32]constant{}
	nulls := map[uint32]bool{}
	parent := map[uint32]uint32{} // access chain result → base pointer
	// users maps an id to the instructions of function bodies that read it:
	// the result id of a pure value instruction (spvPureValueOps), or 0 for
	// any other instruction, which counts as a real use. Literal operands
	// (extract indices, merge masks) are small numbers; a collision with a
	// load's id can only make a dead load look used — a false failure, never
	// a false pass.
	users := map[uint32][]uint32{}
	inFunction := false
	for _, in := range instrs {
		switch in.opcode {
		case OpDecorate:
			if Decoration(in.words[2]) == DecorationBinding {
				names[in.words[1]] = fmt.Sprintf("binding%d", in.words[3])
			}
		case OpVariable:
			if StorageClass(in.words[3]) == StorageClassWorkgroup {
				names[in.words[2]] = fmt.Sprintf("workgroup%d", workgroupVars)
				workgroupVars++
			}
		case OpTypeInt:
			intTypes[in.words[1]] = intType{width: in.words[2], signed: in.words[3] == 1}
		case OpConstant:
			if len(in.words) == 4 {
				constants[in.words[2]] = constant{typ: in.words[1], value: in.words[3]}
			}
		case OpConstantNull:
			nulls[in.words[2]] = true
		case OpAccessChain:
			parent[in.words[2]] = in.words[3]
		case OpFunction:
			inFunction = true
		}
		if inFunction {
			if spvPureValueOps[in.opcode] {
				for _, w := range in.words[3:] { // skip result type and result id
					users[w] = append(users[w], in.words[2])
				}
			} else {
				for _, w := range in.words[1:] {
					users[w] = append(users[w], 0)
				}
			}
		}
	}
	// dead reports whether a pure value's result is never observed: every
	// reader is itself a pure value instruction with a dead result.
	var dead func(id uint32) bool
	dead = func(id uint32) bool {
		for _, u := range users[id] {
			if u == 0 || !dead(u) {
				return false
			}
		}
		return true
	}
	root := func(ptr uint32) uint32 {
		for {
			base, ok := parent[ptr]
			if !ok {
				return ptr
			}
			ptr = base
		}
	}
	rootName := func(ptr uint32) string {
		r := root(ptr)
		if n, ok := names[r]; ok {
			return n
		}
		return fmt.Sprintf("%%%d", r)
	}
	intConst := func(id uint32) spvIntConstant {
		c, ok := constants[id]
		if !ok {
			return spvIntConstant{}
		}
		it, ok := intTypes[c.typ]
		if !ok || it.width != 32 {
			return spvIntConstant{}
		}
		return spvIntConstant{value: c.value, signed: it.signed, ok: true}
	}

	var out []spvMemAccess
	for _, in := range instrs {
		switch in.opcode {
		case OpLoad:
			kind := accessLoad
			if dead(in.words[2]) {
				kind = accessDeadLoad
			}
			out = append(out, spvMemAccess{kind: kind, op: in.opcode, ptr: in.words[3], root: rootName(in.words[3])})
		case OpStore:
			ptr, value := in.words[1], in.words[2]
			kind := accessStore
			if root(ptr) == ptr && nulls[value] {
				kind = accessZeroInit
			}
			out = append(out, spvMemAccess{kind: kind, op: in.opcode, root: rootName(ptr)})
		default:
			l, ok := spvAtomicOperandLayout[in.opcode]
			if !ok {
				continue
			}
			kind := accessAtomicRMW
			switch in.opcode {
			case OpAtomicLoad:
				kind = accessAtomicLoad
			case OpAtomicStore:
				kind = accessAtomicStore
			}
			a := spvMemAccess{
				kind:  kind,
				op:    in.opcode,
				root:  rootName(in.words[l.ptr]),
				scope: intConst(in.words[l.scope]),
			}
			for i := 0; i < l.nSem; i++ {
				a.semantics = append(a.semantics, intConst(in.words[l.firstSem+i]))
			}
			out = append(out, a)
		}
	}
	return out
}

// compileAtomicTestShader compiles WGSL and validates the result with
// spirv-val when available.
func compileAtomicTestShader(t *testing.T, source string) []byte {
	t.Helper()
	spv := compileWGSLForCapabilityTestWithOpts(t, source, Options{Version: Version1_3})
	validateWithVulkanSDK(t, spv)
	return spv
}

// accessCounts counts accesses per root and kind.
func accessCounts(accesses []spvMemAccess) map[string]map[spvAccessKind]int {
	counts := map[string]map[spvAccessKind]int{}
	for _, a := range accesses {
		if counts[a.root] == nil {
			counts[a.root] = map[spvAccessKind]int{}
		}
		counts[a.root][a.kind]++
	}
	return counts
}

// checkAtomicOperands verifies Rust's scope/semantics encoding for every
// atomic instruction on root: scope is an i32 constant equal to wantScope,
// every semantics operand is the u32 constant 0 (relaxed).
func checkAtomicOperands(t *testing.T, accesses []spvMemAccess, root string, wantScope uint32) {
	t.Helper()
	wantScopeConst := spvIntConstant{value: wantScope, signed: true, ok: true}
	wantSem := spvIntConstant{value: MemorySemanticsNone, signed: false, ok: true}
	for _, a := range accesses {
		if a.root != root || a.semantics == nil {
			continue
		}
		if a.scope != wantScopeConst {
			t.Errorf("%s (opcode %d) on %s: scope = %v, want %v", a.kind, a.op, root, a.scope, wantScopeConst)
		}
		for i, s := range a.semantics {
			if s != wantSem {
				t.Errorf("%s (opcode %d) on %s: semantics[%d] = %v, want %v", a.kind, a.op, root, i, s, wantSem)
			}
		}
	}
}

// TestAtomicLoadStoreEmitsAtomicInstructions pins BUG-SPV-ATOMIC-001: every
// atomicLoad / atomicStore (and workgroupUniformLoad of an atomic) becomes
// OpAtomicLoad / OpAtomicStore with Rust's scope and semantics, for every
// atomic scalar type and every shape of atomic data, while plain stores of
// non-atomic data stay OpStore. No load of atomic data may be plain, not even
// a dead one (BUG-SPV-PTR-EMIT-001): a dead non-atomic read still races with
// the atomic writes.
func TestAtomicLoadStoreEmitsAtomicInstructions(t *testing.T) {
	type rootWant struct {
		atomicLoads, atomicStores int
		zeroInit                  int // workgroup zero-initialization stores
		scope                     uint32
	}
	tests := []struct {
		name   string
		source string
		// Roots are keyed as collectMemAccesses names them (binding/workgroup order).
		atomic map[string]rootWant
		// plainStores: non-atomic root → expected OpStore count (no atomics).
		plainStores map[string]int
	}{
		{
			name: "storage_u32_i32_all_shapes",
			source: `
struct S {
    a: atomic<u32>,
    b: array<atomic<i32>, 2>,
}
@group(0) @binding(0) var<storage, read_write> s_u32: atomic<u32>;
@group(0) @binding(1) var<storage, read_write> s_i32: atomic<i32>;
@group(0) @binding(2) var<storage, read_write> s_arr: array<atomic<u32>, 2>;
@group(0) @binding(3) var<storage, read_write> s_struct: S;
@group(0) @binding(4) var<storage, read_write> out: array<u32, 5>;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&s_u32, 1u);
    atomicStore(&s_i32, 1i);
    atomicStore(&s_arr[1], 1u);
    atomicStore(&s_struct.a, 1u);
    atomicStore(&s_struct.b[1], 1i);
    out[0] = atomicLoad(&s_u32);
    out[1] = u32(atomicLoad(&s_i32));
    out[2] = atomicLoad(&s_arr[1]);
    out[3] = atomicLoad(&s_struct.a);
    out[4] = u32(atomicLoad(&s_struct.b[1]));
}
`,
			atomic: map[string]rootWant{
				"binding0": {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice}, // s_u32
				"binding1": {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice}, // s_i32
				"binding2": {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice}, // s_arr
				"binding3": {atomicLoads: 2, atomicStores: 2, scope: ScopeDevice}, // s_struct
			},
			plainStores: map[string]int{"binding4": 5},
		},
		{
			name: "workgroup_u32_i32_all_shapes",
			source: `
struct S {
    a: atomic<u32>,
    b: array<atomic<i32>, 2>,
}
var<workgroup> w_u32: atomic<u32>;
var<workgroup> w_arr: array<atomic<i32>, 2>;
var<workgroup> w_struct: S;
@group(0) @binding(0) var<storage, read_write> out: array<u32, 4>;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&w_u32, 1u);
    atomicStore(&w_arr[1], 1i);
    atomicStore(&w_struct.a, 1u);
    atomicStore(&w_struct.b[1], 1i);
    out[0] = atomicLoad(&w_u32);
    out[1] = u32(atomicLoad(&w_arr[1]));
    out[2] = atomicLoad(&w_struct.a);
    out[3] = u32(atomicLoad(&w_struct.b[1]));
}
`,
			atomic: map[string]rootWant{
				"workgroup0": {atomicLoads: 1, atomicStores: 1, zeroInit: 1, scope: ScopeWorkgroup}, // w_u32
				"workgroup1": {atomicLoads: 1, atomicStores: 1, zeroInit: 1, scope: ScopeWorkgroup}, // w_arr
				"workgroup2": {atomicLoads: 2, atomicStores: 2, zeroInit: 1, scope: ScopeWorkgroup}, // w_struct
			},
			plainStores: map[string]int{"binding0": 4},
		},
		{
			// Rust: Statement::WorkGroupUniformLoad goes through
			// write_checked_load, so an atomic pointee is OpAtomicLoad.
			name: "workgroup_uniform_load_atomic",
			source: `
var<workgroup> w_u32: atomic<u32>;
@group(0) @binding(0) var<storage, read_write> out: u32;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&w_u32, 1u);
    out = workgroupUniformLoad(&w_u32);
}
`,
			atomic: map[string]rootWant{
				"workgroup0": {atomicLoads: 1, atomicStores: 1, zeroInit: 1, scope: ScopeWorkgroup}, // w_u32
			},
			plainStores: map[string]int{"binding0": 1},
		},
		{
			name: "storage_and_workgroup_u64_i64",
			source: `
@group(0) @binding(0) var<storage, read_write> s_u64: atomic<u64>;
@group(0) @binding(1) var<storage, read_write> s_i64: array<atomic<i64>, 2>;
var<workgroup> w_u64: atomic<u64>;
@group(0) @binding(2) var<storage, read_write> out: array<u64, 3>;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&s_u64, 1lu);
    atomicStore(&s_i64[1], 1li);
    atomicStore(&w_u64, 1lu);
    out[0] = atomicLoad(&s_u64);
    out[1] = u64(atomicLoad(&s_i64[1]));
    out[2] = atomicLoad(&w_u64);
}
`,
			atomic: map[string]rootWant{
				"binding0":   {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice},                 // s_u64
				"binding1":   {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice},                 // s_i64
				"workgroup0": {atomicLoads: 1, atomicStores: 1, zeroInit: 1, scope: ScopeWorkgroup}, // w_u64
			},
			plainStores: map[string]int{"binding2": 3},
		},
		{
			name: "storage_f32",
			source: `
@group(0) @binding(0) var<storage, read_write> s_f32: atomic<f32>;
@group(0) @binding(1) var<storage, read_write> out: f32;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&s_f32, 1.5);
    out = atomicLoad(&s_f32);
}
`,
			atomic: map[string]rootWant{
				"binding0": {atomicLoads: 1, atomicStores: 1, scope: ScopeDevice}, // s_f32
			},
			plainStores: map[string]int{"binding1": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accesses := collectMemAccesses(t, compileAtomicTestShader(t, tt.source))
			counts := accessCounts(accesses)

			for root, want := range tt.atomic {
				got := counts[root]
				if got[accessAtomicLoad] != want.atomicLoads || got[accessAtomicStore] != want.atomicStores {
					t.Errorf("%s: OpAtomicLoad=%d OpAtomicStore=%d, want %d and %d",
						root, got[accessAtomicLoad], got[accessAtomicStore], want.atomicLoads, want.atomicStores)
				}
				if got[accessLoad] != 0 || got[accessDeadLoad] != 0 || got[accessStore] != 0 {
					t.Errorf("%s: non-atomic access to atomic data: OpLoad=%d (dead %d) OpStore=%d, want 0",
						root, got[accessLoad], got[accessDeadLoad], got[accessStore])
				}
				if got[accessZeroInit] != want.zeroInit {
					t.Errorf("%s: zero-init stores = %d, want %d", root, got[accessZeroInit], want.zeroInit)
				}
				checkAtomicOperands(t, accesses, root, want.scope)
			}
			for root, wantStores := range tt.plainStores {
				got := counts[root]
				if got[accessStore] != wantStores {
					t.Errorf("%s: OpStore=%d, want %d", root, got[accessStore], wantStores)
				}
				if got[accessDeadLoad] != 0 {
					t.Errorf("%s: %d dead OpLoad of a store target", root, got[accessDeadLoad])
				}
				if got[accessAtomicLoad] != 0 || got[accessAtomicStore] != 0 || got[accessAtomicRMW] != 0 {
					t.Errorf("%s: atomic access to non-atomic data: %v", root, got)
				}
			}
		})
	}
}

// TestAtomicReadModifyWriteScopeAndSemantics pins the operands of every other
// atomic instruction to Rust's Statement::Atomic (back/spv/block.rs): the
// scope of the pointer's address space as an i32 constant (workgroup data is
// Workgroup, not Device) and relaxed semantics — WGSL atomics are relaxed —
// for both semantics operands of OpAtomicCompareExchange.
func TestAtomicReadModifyWriteScopeAndSemantics(t *testing.T) {
	const ops = `
    atomicAdd(&P, V);
    atomicSub(&P, V);
    atomicMin(&P, V);
    atomicMax(&P, V);
    atomicAnd(&P, V);
    atomicOr(&P, V);
    atomicXor(&P, V);
    atomicExchange(&P, V);
    atomicCompareExchangeWeak(&P, V, V);
`
	expand := func(ptr, value string) string {
		return strings.NewReplacer("P", ptr, "V", value).Replace(ops)
	}
	tests := []struct {
		name   string
		source string
		roots  map[string]uint32 // root → wanted scope; each gets all 9 operations
	}{
		{
			name: "storage_and_workgroup_32bit",
			source: `
struct S {
    a: atomic<i32>,
}
@group(0) @binding(0) var<storage, read_write> s_u32: atomic<u32>;
@group(0) @binding(1) var<storage, read_write> s_struct: S;
var<workgroup> w_i32: atomic<i32>;
var<workgroup> w_arr: array<atomic<u32>, 2>;

@compute @workgroup_size(1)
fn main() {` + expand("s_u32", "1u") + expand("s_struct.a", "1i") +
				expand("w_i32", "1i") + expand("w_arr[1]", "1u") + `}
`,
			roots: map[string]uint32{
				"binding0":   ScopeDevice,    // s_u32
				"binding1":   ScopeDevice,    // s_struct
				"workgroup0": ScopeWorkgroup, // w_i32
				"workgroup1": ScopeWorkgroup, // w_arr
			},
		},
		{
			name: "storage_and_workgroup_64bit",
			source: `
@group(0) @binding(0) var<storage, read_write> s_u64: atomic<u64>;
var<workgroup> w_i64: atomic<i64>;

@compute @workgroup_size(1)
fn main() {` + expand("s_u64", "1lu") + expand("w_i64", "1li") + `}
`,
			roots: map[string]uint32{
				"binding0":   ScopeDevice,    // s_u64
				"workgroup0": ScopeWorkgroup, // w_i64
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accesses := collectMemAccesses(t, compileAtomicTestShader(t, tt.source))
			counts := accessCounts(accesses)
			for root, scope := range tt.roots {
				if got := counts[root][accessAtomicRMW]; got != 9 {
					t.Errorf("%s: %d read-modify-write atomics, want 9", root, got)
				}
				checkAtomicOperands(t, accesses, root, scope)
			}
		})
	}
}
