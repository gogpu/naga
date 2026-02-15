package spirv

import (
	"testing"
)

// TestCallInConditional_FuncWithIfReturn tests calling a function that contains
// if/return from within a conditional block. This is the NAGA-SPV-002 scenario:
//
//	fn helper(x: f32) -> f32 {
//	    if (x < 0.0) { return 0.0; }
//	    return x * x;
//	}
//
//	if (val > 0.0) {
//	    result = helper(val);  // call in conditional
//	}
//
// The risk: emitCall caches the call result ID in callResultIDs, but if the
// call is inside an if-block, the SPIR-V result ID is defined in that block
// and may not dominate the merge block where the result is used.
//
// In practice, because WGSL lowers `result = helper(val)` as:
//  1. StmtCall (produces result, cached in callResultIDs)
//  2. StmtStore to result variable (stores immediately in same block)
//
// ...the result is stored to a Function-scope OpVariable (in entry block)
// before the block ends, so the merge block loads via OpLoad from the variable.
// The call result ID itself is never used across block boundaries.
func TestCallInConditional_FuncWithIfReturn(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
    width: u32,
    height: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn helper(x: f32) -> f32 {
    if x < 0.0 { return 0.0; }
    return x * x;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result: f32 = 0.0;
    if params.flag != 0u {
        result = helper(params.value);
    }
    out[idx] = u32(result * 255.0);
}
`

	// Step 1: Compile the shader (this is where "call result not found" would occur)
	spirvBytes := compileWGSLToSPIRV(t, "CallInConditional", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly ===")
	dumpFunctionBlocks(t, instrs, names)

	// Step 2: Verify the helper function exists and has proper control flow
	// The helper function should have: if/return pattern with SelectionMerge
	verifyHelperFunction(t, instrs, names)

	// Step 3: Verify the main function has OpFunctionCall inside a conditional block
	verifyCallInConditionalBlock(t, instrs, names)

	// Step 4: Verify OpVariable for 'result' is in entry block
	resultVarID := findFunctionVariable(instrs, names, "result")
	if resultVarID == 0 {
		t.Fatal("BUG: Local variable 'result' not found in SPIR-V output")
	}
	t.Logf("Found result variable: %%%d", resultVarID)

	// Step 5: Verify there's an OpStore to 'result' inside the conditional block
	storeCount := countStoresToVar(instrs, resultVarID)
	if storeCount < 2 {
		// Expect at least 2 stores: init (0.0) + assignment from helper()
		t.Errorf("Expected at least 2 OpStore to 'result' (init + conditional assignment), got %d", storeCount)
	} else {
		t.Logf("Found %d OpStore(s) to 'result' (%%%d) -- OK", storeCount, resultVarID)
	}

	// Step 6: Verify the call result is used correctly (stored to var, not used cross-block)
	verifyCallResultNotUsedCrossBlock(t, instrs, names, resultVarID)
}

// TestCallInConditional_DirectUseOfResult tests the case where the call result
// is used directly in an expression within the same conditional block, rather
// than being stored to a var.
func TestCallInConditional_DirectUseOfResult(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn square(x: f32) -> f32 {
    if x < 0.0 { return 0.0; }
    return x * x;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result: f32 = 0.0;
    if params.flag != 0u {
        let tmp = square(params.value);
        result = tmp * 2.0;
    }
    out[idx] = u32(result);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "DirectUseOfResult", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly ===")
	dumpFunctionBlocks(t, instrs, names)

	// Verify compilation succeeded and has both an OpFunctionCall and OpFMul
	// in the conditional block
	foundCall := false
	foundMul := false
	inMainFunc := false
	funcCount := 0
	inConditionalBlock := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if !inMainFunc {
			continue
		}
		// Track when we enter a block after a BranchConditional
		if inst.opcode == OpBranchConditional {
			inConditionalBlock = true
		}
		if inst.opcode == OpBranch || inst.opcode == OpReturn {
			inConditionalBlock = false
		}
		if inConditionalBlock {
			if inst.opcode == OpFunctionCall {
				foundCall = true
				t.Logf("Found OpFunctionCall at word %d inside conditional block", inst.offset)
			}
			if inst.opcode == OpFMul {
				foundMul = true
				t.Logf("Found OpFMul at word %d inside conditional block", inst.offset)
			}
		}
	}

	if !foundCall {
		t.Error("BUG: No OpFunctionCall found in conditional block")
	}
	if !foundMul {
		t.Error("BUG: No OpFMul found in conditional block (result * 2.0)")
	}
}

// TestCallInConditional_NestedIf tests calling a function from a doubly-nested
// conditional block.
func TestCallInConditional_NestedIf(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
    mode: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn clamp01(x: f32) -> f32 {
    if x < 0.0 { return 0.0; }
    if x > 1.0 { return 1.0; }
    return x;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result: f32 = 0.0;
    if params.flag != 0u {
        if params.mode == 1u {
            result = clamp01(params.value);
        }
    }
    out[idx] = u32(result * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "NestedConditionalCall", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly ===")
	dumpFunctionBlocks(t, instrs, names)

	// Verify result variable and stores
	resultVarID := findFunctionVariable(instrs, names, "result")
	if resultVarID == 0 {
		t.Fatal("BUG: Local variable 'result' not found")
	}

	storeCount := countStoresToVar(instrs, resultVarID)
	if storeCount < 2 {
		t.Errorf("Expected at least 2 OpStore to 'result', got %d", storeCount)
	} else {
		t.Logf("Found %d OpStore(s) to 'result' -- OK", storeCount)
	}

	// Verify the callee function has multiple if/return blocks
	verifyMultipleSelectionsInHelper(t, instrs)
}

// TestCallInConditional_VarInitFromCallInIf tests the case where a variable
// is initialized from a function call inside an if-block.
// This exercises the deferredCallStores mechanism in a conditional context.
func TestCallInConditional_VarInitFromCallInIf(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn double(x: f32) -> f32 {
    return x * 2.0;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var output_val: f32 = 0.0;
    if params.flag != 0u {
        var tmp = double(params.value);
        output_val = tmp;
    }
    out[idx] = u32(output_val);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromCallInIf", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly ===")
	dumpFunctionBlocks(t, instrs, names)

	// Verify we have an OpFunctionCall somewhere in the main function
	foundCall := false
	inMainFunc := false
	funcCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if inMainFunc && inst.opcode == OpFunctionCall {
			foundCall = true
			t.Logf("Found OpFunctionCall at word %d", inst.offset)
		}
	}
	if !foundCall {
		t.Error("BUG: No OpFunctionCall found in main function")
	}

	// Verify output_val and tmp variables exist
	outputVarID := findFunctionVariable(instrs, names, "output_val")
	if outputVarID == 0 {
		t.Error("BUG: Local variable 'output_val' not found")
	}
}

// TestCallInConditional_BothBranchesCallDifferentFunctions tests calling
// different functions in both branches of an if/else.
func TestCallInConditional_BothBranchesCallDifferentFunctions(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn square(x: f32) -> f32 {
    return x * x;
}

fn negate(x: f32) -> f32 {
    if x == 0.0 { return 0.0; }
    return 0.0 - x;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result: f32;
    if params.flag != 0u {
        result = square(params.value);
    } else {
        result = negate(params.value);
    }
    out[idx] = u32(result * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "BothBranchesCallDifferent", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly ===")
	dumpFunctionBlocks(t, instrs, names)

	// Verify both branches have OpFunctionCall
	callCount := 0
	inMainFunc := false
	funcCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 3 { // Third function is main (after square and negate)
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if inMainFunc && inst.opcode == OpFunctionCall {
			callCount++
			t.Logf("Found OpFunctionCall #%d at word %d", callCount, inst.offset)
		}
	}

	if callCount < 2 {
		t.Errorf("Expected 2 OpFunctionCall in main (one per branch), got %d", callCount)
	} else {
		t.Logf("Found %d OpFunctionCall(s) in main -- OK", callCount)
	}

	// Verify result variable gets stores from both branches
	resultVarID := findFunctionVariable(instrs, names, "result")
	if resultVarID == 0 {
		t.Fatal("BUG: Local variable 'result' not found")
	}
	storeCount := countStoresToVar(instrs, resultVarID)
	if storeCount < 2 {
		t.Errorf("Expected at least 2 OpStore to 'result' (one per branch), got %d", storeCount)
	} else {
		t.Logf("Found %d OpStore(s) to 'result' -- OK", storeCount)
	}
}

// --- Helper verification functions ---

// verifyHelperFunction checks that the first function (helper) has proper
// if/return control flow with OpSelectionMerge.
func verifyHelperFunction(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	inFirstFunc := false
	funcCount := 0
	hasSelectionMerge := false
	hasReturnValue := false
	hasBranchConditional := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 1 {
				inFirstFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd && inFirstFunc {
			break
		}
		if !inFirstFunc {
			continue
		}

		switch inst.opcode {
		case OpSelectionMerge:
			hasSelectionMerge = true
		case OpReturnValue:
			hasReturnValue = true
		case OpBranchConditional:
			hasBranchConditional = true
		}
	}

	if !hasSelectionMerge {
		t.Error("Helper function missing OpSelectionMerge (no if/else control flow)")
	}
	if !hasReturnValue {
		t.Error("Helper function missing OpReturnValue")
	}
	if !hasBranchConditional {
		t.Error("Helper function missing OpBranchConditional")
	}
	if hasSelectionMerge && hasReturnValue && hasBranchConditional {
		t.Log("Helper function has correct if/return control flow")
	}
}

// verifyCallInConditionalBlock checks that the main function has an
// OpFunctionCall inside a conditional block (after OpBranchConditional).
func verifyCallInConditionalBlock(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	// Find the main function (second function)
	funcCount := 0
	inMainFunc := false
	foundCallInBlock := false

	// Track selection merge labels to identify conditional blocks
	var selectionMergeLabels []uint32
	afterBranch := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if !inMainFunc {
			continue
		}

		if inst.opcode == OpSelectionMerge && inst.wordCount >= 2 {
			selectionMergeLabels = append(selectionMergeLabels, inst.words[1])
		}
		if inst.opcode == OpBranchConditional {
			afterBranch = true
		}
		// Once we see the merge label, we are no longer in the conditional block
		if inst.opcode == OpLabel && len(selectionMergeLabels) > 0 {
			for _, ml := range selectionMergeLabels {
				if inst.words[1] == ml {
					afterBranch = false
				}
			}
		}

		if afterBranch && inst.opcode == OpFunctionCall {
			foundCallInBlock = true
			t.Logf("Found OpFunctionCall at word %d inside conditional block -- OK", inst.offset)
		}
	}

	if !foundCallInBlock {
		t.Error("BUG: No OpFunctionCall found inside conditional block in main function")
	}
}

// verifyCallResultNotUsedCrossBlock verifies that the call result SPIR-V ID
// is not used directly after the merge block (it should be accessed via
// OpLoad from the result variable instead).
func verifyCallResultNotUsedCrossBlock(t *testing.T, instrs []spirvInstruction, names map[uint32]string, resultVarID uint32) {
	t.Helper()

	// Find OpFunctionCall result IDs in main
	funcCount := 0
	inMainFunc := false
	var callResultIDs []uint32

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if !inMainFunc {
			continue
		}

		// OpFunctionCall: ResultType ResultID FunctionID Arg...
		if inst.opcode == OpFunctionCall && inst.wordCount >= 3 {
			callResultIDs = append(callResultIDs, inst.words[2])
		}
	}

	if len(callResultIDs) == 0 {
		t.Log("No OpFunctionCall found in main, cannot verify cross-block usage")
		return
	}

	// Check that after the merge block, the call result ID is NOT directly used
	// (it should only appear in OpStore within the conditional block)
	callResultSet := make(map[uint32]bool)
	for _, id := range callResultIDs {
		callResultSet[id] = true
	}

	// Track merge blocks
	funcCount = 0
	inMainFunc = false
	var mergeLabels []uint32
	inMergeBlock := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if !inMainFunc {
			continue
		}

		if inst.opcode == OpSelectionMerge && inst.wordCount >= 2 {
			mergeLabels = append(mergeLabels, inst.words[1])
		}

		if inst.opcode == OpLabel {
			for _, ml := range mergeLabels {
				if inst.words[1] == ml {
					inMergeBlock = true
				}
			}
		}

		// In the merge block or after, check if any instruction uses a call result ID directly
		if inMergeBlock {
			for i := 1; i < inst.wordCount; i++ {
				if callResultSet[inst.words[i]] {
					// OpStore using call result in same block as call is OK
					// But using it in merge block would be a SPIR-V domination violation
					if inst.opcode != OpStore {
						t.Errorf("BUG: Call result %%%d used directly in merge/post-merge block "+
							"at word %d (opcode=%s) - SPIR-V domination violation!",
							inst.words[i], inst.offset, opcodeName(inst.opcode))
					}
				}
			}
		}
	}

	t.Log("Call result not used cross-block -- OK (result accessed via OpLoad from variable)")
}

// verifyMultipleSelectionsInHelper checks that the first function (callee)
// has multiple OpSelectionMerge instructions (for multiple if/return patterns).
func verifyMultipleSelectionsInHelper(t *testing.T, instrs []spirvInstruction) {
	t.Helper()

	inFirstFunc := false
	funcCount := 0
	selectionCount := 0

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			if funcCount == 1 {
				inFirstFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd && inFirstFunc {
			break
		}
		if !inFirstFunc {
			continue
		}
		if inst.opcode == OpSelectionMerge {
			selectionCount++
		}
	}

	if selectionCount < 2 {
		t.Errorf("Expected at least 2 OpSelectionMerge in callee (multiple if/return), got %d", selectionCount)
	} else {
		t.Logf("Callee has %d OpSelectionMerge instructions -- OK (multiple if/return)", selectionCount)
	}
}
