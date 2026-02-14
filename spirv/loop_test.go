package spirv

import (
	"fmt"
	"strings"
	"testing"
)

// TestForLoopCompilation tests that a simple for loop compiles to valid SPIR-V.
// This is a regression test for NAGA-SPV-005: loops only executing first iteration.
//
// The WGSL for loop:
//
//	for (var i: u32 = 0u; i < 4u; i = i + 1u) { sum = sum + 1.0; }
//
// Lowers to IR StmtLoop:
//
//	var i: u32 = 0u;
//	loop {
//	    if (!(i < 4u)) { break; }
//	    sum = sum + 1.0;
//	    continuing { i = i + 1u; }
//	}
//
// Expected SPIR-V structure:
//
//	entry:          OpBranch -> header
//	header:         OpLoopMerge(merge, continue) -> OpBranch -> body
//	body:           condition check -> OpBranchConditional(true->break_body, false->loop_body)
//	                break_body: OpBranch -> merge
//	                loop_body:  ... -> OpBranch -> continue
//	continue:       i = i + 1 -> OpBranch -> header  (BACK-EDGE)
//	merge:          (after loop)
func TestForLoopCompilation(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 4u; i = i + 1u) {
        sum = sum + 1.0;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "ForLoop", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for ForLoop shader ===")
	dumpFunctionBlocks(t, instrs, names)

	// Verify the loop structure in SPIR-V
	verifyLoopStructure(t, instrs, names)
}

// TestWhileLoopCompilation tests a while-style loop.
func TestWhileLoopCompilation(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    var i: u32 = 0u;
    while (i < 4u) {
        sum = sum + 1.0;
        i = i + 1u;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "WhileLoop", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for WhileLoop shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// TestNestedForLoopCompilation tests nested for loops.
func TestNestedForLoopCompilation(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 3u; i = i + 1u) {
        for (var j: u32 = 0u; j < 2u; j = j + 1u) {
            sum = sum + 1.0;
        }
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "NestedForLoop", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for NestedForLoop shader ===")
	dumpFunctionBlocks(t, instrs, names)

	// Should have exactly 2 OpLoopMerge instructions (outer + inner loop)
	loopMergeCount := countOpcode(instrs, OpLoopMerge)
	if loopMergeCount != 2 {
		t.Errorf("Expected 2 OpLoopMerge for nested loops, got %d", loopMergeCount)
	}
}

// TestForLoopWithBreak tests a for loop with an early break condition.
func TestForLoopWithBreak(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 10u; i = i + 1u) {
        if (i == 5u) {
            break;
        }
        sum = sum + 1.0;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "ForLoopWithBreak", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for ForLoopWithBreak shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// TestForLoopWithContinue tests a for loop with continue statement.
func TestForLoopWithContinue(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 10u; i = i + 1u) {
        if (i == 3u) {
            continue;
        }
        sum = sum + 1.0;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "ForLoopWithContinue", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for ForLoopWithContinue shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// TestLoopVariableAccumulation tests that a loop counter actually accumulates values.
// This specifically validates that the back-edge works and the loop iterates.
func TestLoopVariableAccumulation(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var count: u32 = 0u;
    for (var i: u32 = 0u; i < 8u; i = i + 1u) {
        count = count + 1u;
    }
    output[id.x] = count;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "LoopAccumulation", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for LoopAccumulation shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// verifyLoopStructure validates the SPIR-V loop control flow structure.
// It checks:
// 1. OpLoopMerge exists with valid merge and continue labels
// 2. Back-edge: the continue block branches back to the header
// 3. Break condition: OpBranchConditional exists in the body that can branch to merge
// 4. All referenced labels exist as OpLabel instructions
func verifyLoopStructure(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	// Collect all labels (OpLabel instruction result IDs)
	labelSet := make(map[uint32]bool)
	for _, inst := range instrs {
		if inst.opcode == OpLabel && inst.wordCount >= 2 {
			labelSet[inst.words[1]] = true
		}
	}

	// Find OpLoopMerge instructions
	type loopInfo struct {
		headerLabel   uint32 // The label immediately before OpLoopMerge
		mergeLabel    uint32
		continueLabel uint32
	}

	var loops []loopInfo
	var currentLabel uint32

	for _, inst := range instrs {
		if inst.opcode == OpLabel && inst.wordCount >= 2 {
			currentLabel = inst.words[1]
		}
		if inst.opcode == OpLoopMerge && inst.wordCount >= 4 {
			loops = append(loops, loopInfo{
				headerLabel:   currentLabel,
				mergeLabel:    inst.words[1],
				continueLabel: inst.words[2],
			})
		}
	}

	if len(loops) == 0 {
		t.Fatal("No OpLoopMerge found in SPIR-V output — loop was not emitted")
	}

	for i, loop := range loops {
		t.Logf("Loop %d: header=%%%d, merge=%%%d, continue=%%%d",
			i, loop.headerLabel, loop.mergeLabel, loop.continueLabel)

		// Check that merge and continue labels exist
		if !labelSet[loop.mergeLabel] {
			t.Errorf("Loop %d: merge label %%%d does not exist as OpLabel", i, loop.mergeLabel)
		}
		if !labelSet[loop.continueLabel] {
			t.Errorf("Loop %d: continue label %%%d does not exist as OpLabel", i, loop.continueLabel)
		}

		// Check that the continue block has a back-edge to the header
		backEdgeFound := verifyBackEdge(t, instrs, loop.continueLabel, loop.headerLabel)
		if !backEdgeFound {
			t.Errorf("Loop %d: NO back-edge from continue block %%%d to header %%%d — loop will NOT iterate!",
				i, loop.continueLabel, loop.headerLabel)
		}

		// Check that there's a branch to the merge label (break path)
		breakPathFound := verifyBreakPath(t, instrs, loop.mergeLabel)
		if !breakPathFound {
			t.Errorf("Loop %d: no branch to merge label %%%d found — loop may never exit",
				i, loop.mergeLabel)
		}
	}
}

// verifyBackEdge checks that the continue block (identified by continueLabel) contains
// an OpBranch back to headerLabel. This is the critical back-edge that makes the loop iterate.
func verifyBackEdge(t *testing.T, instrs []spirvInstruction, continueLabel, headerLabel uint32) bool {
	t.Helper()

	inContinueBlock := false
	for _, inst := range instrs {
		if inst.opcode == OpLabel && inst.wordCount >= 2 && inst.words[1] == continueLabel {
			inContinueBlock = true
			continue
		}
		if inContinueBlock {
			// Check for terminator instructions
			switch inst.opcode {
			case OpBranch:
				if inst.wordCount >= 2 && inst.words[1] == headerLabel {
					t.Logf("  Back-edge found: continue block %%%d -> header %%%d (OpBranch)",
						continueLabel, headerLabel)
					return true
				}
				// OpBranch to somewhere else means block ended
				return false
			case OpBranchConditional:
				// break_if: OpBranchConditional with one target being header
				if inst.wordCount >= 4 {
					if inst.words[2] == headerLabel || inst.words[3] == headerLabel {
						t.Logf("  Back-edge found: continue block %%%d -> header %%%d (OpBranchConditional)",
							continueLabel, headerLabel)
						return true
					}
				}
				return false
			case OpReturn, OpReturnValue, OpKill, OpUnreachable:
				return false
			case OpLabel:
				// Hit next block without finding terminator
				return false
			}
		}
	}
	return false
}

// verifyBreakPath checks that some instruction branches to the merge label.
// This ensures the loop has an exit path.
func verifyBreakPath(t *testing.T, instrs []spirvInstruction, mergeLabel uint32) bool {
	t.Helper()
	for _, inst := range instrs {
		if inst.opcode == OpBranch && inst.wordCount >= 2 && inst.words[1] == mergeLabel {
			return true
		}
		if inst.opcode == OpBranchConditional && inst.wordCount >= 4 {
			if inst.words[2] == mergeLabel || inst.words[3] == mergeLabel {
				return true
			}
		}
	}
	return false
}

// countOpcode counts how many times a given opcode appears in the instruction stream.
func countOpcode(instrs []spirvInstruction, opcode OpCode) int {
	count := 0
	for _, inst := range instrs {
		if inst.opcode == opcode {
			count++
		}
	}
	return count
}

// TestRawLoopWithBreak tests the WGSL `loop { ... }` construct with manual break.
func TestRawLoopWithBreak(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    var i: u32 = 0u;
    loop {
        if (i >= 4u) {
            break;
        }
        sum = sum + 1.0;
        i = i + 1u;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "RawLoopWithBreak", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for RawLoopWithBreak shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// TestForLoopWithSignedCounter tests a for loop with a signed integer counter.
func TestForLoopWithSignedCounter(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: i32 = 0; i < 6; i = i + 1) {
        sum = sum + 1.0;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "ForLoopSigned", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for ForLoopSigned shader ===")
	dumpFunctionBlocks(t, instrs, names)

	verifyLoopStructure(t, instrs, names)
}

// TestSDFBatchLoopPattern tests the loop pattern used by gg's GPU SDF accelerator.
// This was the primary use case motivating NAGA-SPV-005 fix: the multi-pass dispatch
// workaround was needed because loops only ran one iteration.
func TestSDFBatchLoopPattern(t *testing.T) {
	const shader = `
struct ShapeData {
    shape_type: u32,
    cx: f32,
    cy: f32,
    rx: f32,
}

@group(0) @binding(0) var<uniform> shape_count: u32;
@group(0) @binding(1) var<storage, read> shapes: array<ShapeData>;
@group(0) @binding(2) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let pixel_idx = gid.x;
    var color: f32 = 0.0;

    for (var i: u32 = 0u; i < shape_count; i = i + 1u) {
        let shape = shapes[i];
        let dist = shape.cx * shape.cx + shape.cy * shape.cy;
        if (dist < shape.rx * shape.rx) {
            color = color + 1.0;
        }
    }

    output[pixel_idx] = color;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "SDFBatchLoop", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for SDFBatchLoop shader ===")
	dumpFunctionBlocks(t, instrs, names)

	// This pattern requires a working loop to iterate over shapes
	verifyLoopStructure(t, instrs, names)
}

// TestForLoopSPIRVStructureDetailed provides a detailed analysis of the SPIR-V
// loop structure, printing each block's instructions for debugging.
func TestForLoopSPIRVStructureDetailed(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 4u; i = i + 1u) {
        sum = sum + 1.0;
    }
    output[id.x] = sum;
}
`

	spirvBytes := compileWGSLToSPIRV(t, "ForLoopDetailed", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	// Detailed dump of every block and instruction in the function
	t.Log("=== Detailed Block-by-Block Analysis ===")
	dumpAllBlocksDetailed(t, instrs, names)
}

// dumpAllBlocksDetailed dumps every instruction in every block, with block boundaries.
func dumpAllBlocksDetailed(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	inFunction := false
	blockLabel := uint32(0)
	blockInstructions := 0

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			if inst.wordCount >= 5 {
				funcName := names[inst.words[2]]
				if funcName == "" {
					funcName = fmt.Sprintf("%%%d", inst.words[2])
				}
				t.Logf("=== FUNCTION %s ===", funcName)
			}
			continue
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
			t.Log("=== FUNCTION END ===")
			continue
		}
		if !inFunction {
			continue
		}

		if inst.opcode == OpLabel && inst.wordCount >= 2 {
			if blockLabel != 0 {
				t.Logf("  (block %%%d had %d instructions)", blockLabel, blockInstructions)
			}
			blockLabel = inst.words[1]
			blockInstructions = 0
			labelName := names[blockLabel]
			if labelName != "" {
				t.Logf("BLOCK %%%d (%s):", blockLabel, labelName)
			} else {
				t.Logf("BLOCK %%%d:", blockLabel)
			}
			continue
		}

		blockInstructions++
		desc := describeInstruction(inst, names)
		t.Logf("  %s", desc)
	}
}

// describeInstruction returns a human-readable description of a SPIR-V instruction.
func describeInstruction(inst spirvInstruction, names map[uint32]string) string {
	idStr := func(id uint32) string {
		if name, ok := names[id]; ok && name != "" {
			return fmt.Sprintf("%%%d(%s)", id, name)
		}
		return fmt.Sprintf("%%%d", id)
	}

	switch inst.opcode {
	case OpVariable:
		if inst.wordCount >= 4 {
			return fmt.Sprintf("OpVariable %s sc=%d", idStr(inst.words[2]), inst.words[3])
		}
	case OpStore:
		if inst.wordCount >= 3 {
			return fmt.Sprintf("OpStore %s <- %s", idStr(inst.words[1]), idStr(inst.words[2]))
		}
	case OpLoad:
		if inst.wordCount >= 4 {
			return fmt.Sprintf("%s = OpLoad %s", idStr(inst.words[2]), idStr(inst.words[3]))
		}
	case OpBranch:
		if inst.wordCount >= 2 {
			return fmt.Sprintf("OpBranch -> %s", idStr(inst.words[1]))
		}
	case OpBranchConditional:
		if inst.wordCount >= 4 {
			return fmt.Sprintf("OpBranchConditional %s true:%s false:%s",
				idStr(inst.words[1]), idStr(inst.words[2]), idStr(inst.words[3]))
		}
	case OpLoopMerge:
		if inst.wordCount >= 4 {
			return fmt.Sprintf("OpLoopMerge merge:%s continue:%s", idStr(inst.words[1]), idStr(inst.words[2]))
		}
	case OpSelectionMerge:
		if inst.wordCount >= 3 {
			return fmt.Sprintf("OpSelectionMerge merge:%s", idStr(inst.words[1]))
		}
	case OpReturn:
		return "OpReturn"
	case OpReturnValue:
		if inst.wordCount >= 2 {
			return fmt.Sprintf("OpReturnValue %s", idStr(inst.words[1]))
		}
	case OpIAdd:
		if inst.wordCount >= 5 {
			return fmt.Sprintf("%s = OpIAdd %s, %s", idStr(inst.words[2]), idStr(inst.words[3]), idStr(inst.words[4]))
		}
	case OpFAdd:
		if inst.wordCount >= 5 {
			return fmt.Sprintf("%s = OpFAdd %s, %s", idStr(inst.words[2]), idStr(inst.words[3]), idStr(inst.words[4]))
		}
	case OpULessThan:
		if inst.wordCount >= 5 {
			return fmt.Sprintf("%s = OpULessThan %s, %s", idStr(inst.words[2]), idStr(inst.words[3]), idStr(inst.words[4]))
		}
	case OpLogicalNot:
		if inst.wordCount >= 4 {
			return fmt.Sprintf("%s = OpLogicalNot %s", idStr(inst.words[2]), idStr(inst.words[3]))
		}
	case OpAccessChain:
		var indices []string
		for i := 4; i < inst.wordCount; i++ {
			indices = append(indices, idStr(inst.words[i]))
		}
		return fmt.Sprintf("%s = OpAccessChain %s [%s]",
			idStr(inst.words[2]), idStr(inst.words[3]), strings.Join(indices, ", "))
	case OpCompositeExtract:
		if inst.wordCount >= 5 {
			return fmt.Sprintf("%s = OpCompositeExtract %s [%d]",
				idStr(inst.words[2]), idStr(inst.words[3]), inst.words[4])
		}
	case OpUnreachable:
		return "OpUnreachable"
	case OpKill:
		return "OpKill"
	case OpFunctionParameter:
		if inst.wordCount >= 3 {
			return fmt.Sprintf("OpFunctionParameter %s", idStr(inst.words[2]))
		}
	}

	// Fallback: generic description
	var parts []string
	for i := 1; i < inst.wordCount && i < len(inst.words); i++ {
		parts = append(parts, fmt.Sprintf("%d", inst.words[i]))
	}
	return fmt.Sprintf("Op%d(%s)", inst.opcode, strings.Join(parts, ", "))
}
