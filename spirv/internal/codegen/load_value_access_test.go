package codegen

import (
	"testing"
)

// spvValueOrigin describes where the value stored to the single variable at
// binding `outBinding` comes from: the opcode that produced it after looking
// through conversions, and for a load, the storage class of its root variable
// and whether it ran before the module's first store to `dataRoot`.
type spvValueOrigin struct {
	op OpCode
	// For OpCompositeExtract: the composite came from an OpLoad of dataRoot
	// that precedes the first store to dataRoot.
	extractFromEarlyLoad bool
	// For OpLoad: storage class of the root variable of its pointer.
	loadRootClass StorageClass
}

// dataInWorkgroup, as valueStoredTo's dataBinding, selects the module's
// workgroup variable as the data root instead of a resource binding.
const dataInWorkgroup = ^uint32(0)

// valueStoredTo traces the value of the one OpStore whose pointer is rooted at
// the variable decorated with Binding outBinding.
func valueStoredTo(t *testing.T, spv []byte, outBinding, dataBinding uint32) spvValueOrigin {
	t.Helper()
	instrs := decodeSPIRVInstructions(spv)
	def := map[uint32]int{}        // result id → instruction index
	parent := map[uint32]uint32{}  // access chain result → base
	binding := map[uint32]uint32{} // variable id → binding
	varClass := map[uint32]StorageClass{}
	for i, in := range instrs {
		switch in.opcode {
		case OpDecorate:
			if Decoration(in.words[2]) == DecorationBinding {
				binding[in.words[1]] = in.words[3]
			}
		case OpVariable:
			varClass[in.words[2]] = StorageClass(in.words[3])
		case OpAccessChain:
			parent[in.words[2]] = in.words[3]
		}
		if len(in.words) > 2 {
			switch in.opcode {
			case OpStore, OpDecorate, OpName, OpEntryPoint, OpExecutionMode, OpBranch, OpBranchConditional,
				OpSelectionMerge, OpLoopMerge, OpReturn, OpReturnValue, OpMemberDecorate:
			default:
				def[in.words[2]] = i
			}
		}
	}
	root := func(p uint32) uint32 {
		for {
			b, ok := parent[p]
			if !ok {
				return p
			}
			p = b
		}
	}
	isData := func(p uint32) bool {
		r := root(p)
		if dataBinding == dataInWorkgroup {
			return varClass[r] == StorageClassWorkgroup
		}
		b, ok := binding[r]
		return ok && b == dataBinding
	}

	nulls := map[uint32]bool{}
	for _, in := range instrs {
		if in.opcode == OpConstantNull {
			nulls[in.words[2]] = true
		}
	}
	firstDataStore := -1
	storeIdx := -1
	var value uint32
	for i, in := range instrs {
		if in.opcode != OpStore {
			continue
		}
		// The workgroup zero-initialization stores a null to the whole
		// variable before the shader body; it is not the store under test.
		zeroInit := root(in.words[1]) == in.words[1] && nulls[in.words[2]]
		if isData(in.words[1]) && firstDataStore < 0 && !zeroInit {
			firstDataStore = i
		}
		if b, ok := binding[root(in.words[1])]; ok && b == outBinding {
			if storeIdx >= 0 {
				t.Fatalf("more than one store to binding %d", outBinding)
			}
			storeIdx, value = i, in.words[2]
		}
	}
	if storeIdx < 0 {
		t.Fatalf("no store to binding %d", outBinding)
	}

	// Look through value-preserving conversions to the producing instruction.
	for {
		in := instrs[def[value]]
		switch in.opcode {
		case OpBitcast, OpConvertUToF, OpConvertSToF, OpCopyLogical:
			value = in.words[3]
			continue
		}
		origin := spvValueOrigin{op: in.opcode}
		switch in.opcode {
		case OpCompositeExtract:
			composite := in.words[3]
			for {
				c := instrs[def[composite]]
				if c.opcode == OpCopyLogical {
					composite = c.words[3]
					continue
				}
				if c.opcode == OpLoad && isData(c.words[3]) && def[composite] < firstDataStore {
					origin.extractFromEarlyLoad = true
				}
				break
			}
		case OpLoad:
			origin.loadRootClass = varClass[root(in.words[3])]
		}
		return origin
	}
}

// TestIndexingALoadedValueUsesTheValue pins BUG-SPV-LOAD-PASSTHROUGH-001.
// `let snapshot = data;` loads a value; indexing it must read that value, not
// memory again. Rust naga emits OpCompositeExtract on the loaded value for a
// constant index and spills it to a temporary for a dynamic one. Re-reading
// the buffer instead returns a value written after the `let`.
func TestIndexingALoadedValueUsesTheValue(t *testing.T) {
	tests := []struct {
		name   string
		source string
		// want: the stored value is extracted from the early load, or loaded
		// from a Function-class temporary (dynamic index).
		wantExtract bool
		workgroup   bool // data is the workgroup variable, not binding 0
	}{
		{
			name: "storage_array_constant_index",
			source: `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main() {
    let snapshot = data;
    data[1] = 9u;
    out = snapshot[1];
}
`,
			wantExtract: true,
		},
		{
			name: "storage_struct_field",
			source: `
struct S { x: u32, y: u32 }
@group(0) @binding(0) var<storage, read_write> data: S;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main() {
    let s = data;
    data.x = 1u;
    out = s.x;
}
`,
			wantExtract: true,
		},
		{
			name: "storage_array_dynamic_index",
			source: `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let snapshot = data;
    data[i] = 9u;
    out = snapshot[i];
}
`,
			wantExtract: false,
		},
		{
			// Workgroup data: the snapshot is a layout-free load plus
			// OpCopyLogical, and other invocations may write in between.
			name: "workgroup_array_constant_index",
			source: `
var<workgroup> w: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> out: u32;
@compute @workgroup_size(1)
fn main() {
    let snapshot = w;
    w[1] = 9u;
    out = snapshot[1];
}
`,
			workgroup:   true,
			wantExtract: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := uint32(0)
			if tt.workgroup {
				data = dataInWorkgroup
			}
			origin := valueStoredTo(t, compileAtomicTestShader(t, tt.source), 1, data)
			if tt.wantExtract {
				if origin.op != OpCompositeExtract || !origin.extractFromEarlyLoad {
					t.Errorf("stored value comes from opcode %d (extract from the snapshot load: %v); "+
						"want OpCompositeExtract of the load taken before the store to data", origin.op, origin.extractFromEarlyLoad)
				}
				return
			}
			if origin.op != OpLoad || origin.loadRootClass != StorageClassFunction {
				t.Errorf("stored value comes from opcode %d (load root class %d); want an OpLoad from a "+
					"Function-class temporary holding the snapshot", origin.op, origin.loadRootClass)
			}
		})
	}
}

// TestPointerParameterMemberAccessUnchanged guards the IR shape Rust and our
// lowerer both produce for (*p).x: AccessIndex on the pointer, then Load. It
// must stay an access chain on the parameter plus a load.
func TestPointerParameterMemberAccessUnchanged(t *testing.T) {
	const source = `
struct S { x: u32, y: u32 }
@group(0) @binding(1) var<storage, read_write> out: u32;
fn get(p: ptr<function, S>) -> u32 {
    return (*p).y;
}
@compute @workgroup_size(1)
fn main() {
    var local = S(3u, 4u);
    out = get(&local);
}
`
	instrs := decodeSPIRVInstructions(compileAtomicTestShader(t, source))
	params := map[uint32]bool{}
	chains := map[uint32]uint32{}
	found := false
	for _, in := range instrs {
		switch in.opcode {
		case OpFunctionParameter:
			params[in.words[2]] = true
		case OpAccessChain:
			chains[in.words[2]] = in.words[3]
		case OpLoad:
			if base, ok := chains[in.words[3]]; ok && params[base] {
				found = true
			}
		}
	}
	if !found {
		t.Error("(*p).y did not compile to OpAccessChain on the parameter followed by OpLoad")
	}
}
