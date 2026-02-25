package spirv

import (
	"testing"
)

// TestAtomicI32ResultType verifies that atomicAdd on atomic<i32> emits
// OpAtomicIAdd with int result type (not uint). SPIR-V spec requires the
// result type to match the pointed-to scalar type.
// Bug: NAGA-SPV-009 — resolveAtomicScalarKind returned ScalarUint for struct
// field access (e.g., tiles[i].backdrop) because ResolveExpressionType returns
// AtomicType directly, not wrapped in PointerType.
func TestAtomicI32ResultType(t *testing.T) {
	const shader = `
struct Tile {
    backdrop: atomic<i32>,
    seg_count: atomic<u32>,
}

@group(0) @binding(0) var<storage, read_write> tiles: array<Tile>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    atomicAdd(&tiles[idx].backdrop, 1i);
    atomicAdd(&tiles[idx].seg_count, 1u);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "AtomicI32", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)

	// Build type ID → name map from OpTypeInt declarations
	typeMap := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpTypeInt && len(inst.words) >= 4 {
			id, width, signed := inst.words[1], inst.words[2], inst.words[3]
			if width == 32 && signed == 1 {
				typeMap[id] = "int"
			} else if width == 32 && signed == 0 {
				typeMap[id] = "uint"
			}
		}
	}

	// Verify each OpAtomicIAdd has correct result type:
	// - atomic<i32> → int result type
	// - atomic<u32> → uint result type
	want := []string{"int", "uint"}
	atomicIdx := 0
	for _, inst := range instrs {
		if inst.opcode != OpAtomicIAdd || len(inst.words) < 3 {
			continue
		}
		resultTypeID := inst.words[1]
		got := typeMap[resultTypeID]
		if atomicIdx < len(want) {
			if got != want[atomicIdx] {
				t.Errorf("OpAtomicIAdd[%d]: result type = %q (ID %d), want %q",
					atomicIdx, got, resultTypeID, want[atomicIdx])
			}
		}
		atomicIdx++
	}
	if atomicIdx != len(want) {
		t.Errorf("found %d OpAtomicIAdd instructions, want %d", atomicIdx, len(want))
	}
}

// TestAtomicI32PointerType verifies that the pointer operand of OpAtomicIAdd
// points to the same type as the result type (int for i32, uint for u32).
func TestAtomicI32PointerType(t *testing.T) {
	const shader = `
struct Counter {
    signed_val: atomic<i32>,
    unsigned_val: atomic<u32>,
}

@group(0) @binding(0) var<storage, read_write> counters: array<Counter>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    atomicAdd(&counters[gid.x].signed_val, 1i);
    atomicAdd(&counters[gid.x].unsigned_val, 1u);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "AtomicI32Pointer", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)

	// Build pointer type chain: OpTypePointer → base type, OpTypeInt → signed/unsigned
	typeNames := make(map[uint32]string) // type ID → "int" or "uint"
	for _, inst := range instrs {
		if inst.opcode == OpTypeInt && len(inst.words) >= 4 {
			id, width, signed := inst.words[1], inst.words[2], inst.words[3]
			if width == 32 && signed == 1 {
				typeNames[id] = "int"
			} else if width == 32 && signed == 0 {
				typeNames[id] = "uint"
			}
		}
	}

	// For each OpAtomicIAdd, verify pointer base type matches result type
	atomicIdx := 0
	for _, inst := range instrs {
		if inst.opcode != OpAtomicIAdd || len(inst.words) < 4 {
			continue
		}
		resultTypeID := inst.words[1]
		resultTypeName := typeNames[resultTypeID]
		t.Logf("OpAtomicIAdd[%d]: result type ID=%d (%s)", atomicIdx, resultTypeID, resultTypeName)
		atomicIdx++
	}
	if atomicIdx != 2 {
		t.Errorf("found %d OpAtomicIAdd, want 2", atomicIdx)
	}
}
