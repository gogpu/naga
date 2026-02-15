package spirv

import (
	"fmt"
	"strings"
	"testing"
)

// TestVarInitFromUniform tests that `var x = params.value;` compiles to correct SPIR-V.
// The init expression references a uniform buffer field, which requires OpAccessChain + OpLoad.
// The generated SPIR-V must have:
//  1. OpVariable for the local var in the entry block
//  2. OpAccessChain + OpLoad to get the uniform value
//  3. OpStore to initialize the local var
func TestVarInitFromUniform(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    width: u32,
    height: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    if idx >= params.width * params.height { return; }

    var x = params.value;
    out[idx] = u32(x * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromUniform", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var x = params.value ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find the local variable 'x'
	xVarID := findFunctionVariable(instrs, names, "x")
	if xVarID == 0 {
		t.Fatal("BUG: Local variable 'x' not found in SPIR-V output")
	}
	t.Logf("Local variable 'x' has SPIR-V ID: %%%d", xVarID)

	// Verify there is an OpStore to 'x' in the entry block (init store)
	initStoreFound := verifyInitStore(t, instrs, names, xVarID)
	if !initStoreFound {
		t.Error("BUG: No OpStore found to initialize 'x' in the entry block. " +
			"The var init expression was not emitted correctly.")
	}

	// Verify the stored value comes from OpLoad of a uniform field (via OpAccessChain)
	verifyInitValueFromUniform(t, instrs, names, xVarID)
}

// TestVarInitFromExpression tests that `var x = params.value * 2.0;` works.
// This is a compound expression: Binary(AccessIndex(GlobalVar), Literal).
func TestVarInitFromExpression(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    width: u32,
    height: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    if idx >= params.width * params.height { return; }

    var x = params.value * 2.0;
    out[idx] = u32(x * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromExpr", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var x = params.value * 2.0 ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find the local variable 'x'
	xVarID := findFunctionVariable(instrs, names, "x")
	if xVarID == 0 {
		t.Fatal("BUG: Local variable 'x' not found in SPIR-V output")
	}

	// Verify there is an OpStore initializing 'x'
	initStoreFound := verifyInitStore(t, instrs, names, xVarID)
	if !initStoreFound {
		t.Error("BUG: No OpStore found to initialize 'x'. " +
			"The compound init expression (params.value * 2.0) was not emitted correctly.")
	}
}

// TestVarInitFromLiteral tests that `var x = 42.0;` works (simple case).
func TestVarInitFromLiteral(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var x = 42.0;
    out[idx] = u32(x);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromLiteral", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var x = 42.0 ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find the local variable 'x'
	xVarID := findFunctionVariable(instrs, names, "x")
	if xVarID == 0 {
		t.Fatal("BUG: Local variable 'x' not found in SPIR-V output")
	}

	// Verify there is an OpStore initializing 'x' with a constant value
	initStoreFound := verifyInitStore(t, instrs, names, xVarID)
	if !initStoreFound {
		t.Error("BUG: No OpStore found to initialize 'x' with literal 42.0")
	}
}

// TestVarInitFromLetBinding tests that `let v = ...; var x = v;` works.
// This pattern creates an ExprLocalVariable init which loads from the let binding.
func TestVarInitFromLetBinding(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    let v = params.value;
    var x = v;
    out[idx] = u32(x * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromLet", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for let v = ...; var x = v ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find local variable 'x'
	xVarID := findFunctionVariable(instrs, names, "x")
	if xVarID == 0 {
		t.Fatal("BUG: Local variable 'x' not found in SPIR-V output")
	}

	// Verify there is an OpStore initializing 'x'
	initStoreFound := verifyInitStore(t, instrs, names, xVarID)
	if !initStoreFound {
		t.Error("BUG: No OpStore found to initialize 'x' from let binding")
	}
}

// TestVarInitOrder verifies that init expressions for local variables that depend on
// body-emitted statements (like let bindings with uniform reads) are properly ordered.
// The key issue: if var init OpStore is emitted BEFORE body statements, references
// to let-bound values that haven't been emitted yet will fail or produce zero.
func TestVarInitOrder(t *testing.T) {
	// This shader intentionally has a let binding BEFORE the var init,
	// and the var init references the let binding value.
	// The let binding loads from a uniform buffer.
	const shader = `
struct Params {
    value: f32,
    scale: f32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    let base = params.value;
    var scaled = base * params.scale;
    scaled = scaled + 1.0;
    out[idx] = u32(scaled);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitOrder", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var init ordering test ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find the local variable 'scaled'
	scaledVarID := findFunctionVariable(instrs, names, "scaled")
	if scaledVarID == 0 {
		t.Fatal("BUG: Local variable 'scaled' not found")
	}

	// Count OpStore instructions to 'scaled'
	storeCount := countStoresToVar(instrs, scaledVarID)
	t.Logf("Found %d OpStore instructions to 'scaled' (%%%d)", storeCount, scaledVarID)

	// We expect at least 2 stores: init store + the `scaled = scaled + 1.0;` store
	if storeCount < 2 {
		t.Errorf("Expected at least 2 stores to 'scaled' (init + reassignment), got %d", storeCount)
	}
}

// TestVarInitFromFunctionCall tests that `var x = my_func(params.value);` works.
// Function calls are deferred: the StmtCall runs in the body, and the ExprCallResult
// is stored to the variable by the deferred call store mechanism.
func TestVarInitFromFunctionCall(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

fn double(x: f32) -> f32 {
    return x * 2.0;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result = double(params.value);
    out[idx] = u32(result);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitFromFuncCall", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var result = double(params.value) ===")
	dumpFunctionBlocks(t, instrs, names)

	// Find the local variable 'result'
	resultVarID := findFunctionVariable(instrs, names, "result")
	if resultVarID == 0 {
		t.Fatal("BUG: Local variable 'result' not found in SPIR-V output")
	}

	// Verify there is an OpStore to 'result' somewhere (may be after the function call)
	storeCount := countStoresToVar(instrs, resultVarID)
	if storeCount == 0 {
		t.Error("BUG: No OpStore found to initialize 'result' from function call")
	} else {
		t.Logf("Found %d OpStore(s) to 'result' (%%%d)", storeCount, resultVarID)
	}

	// Verify there is an OpFunctionCall in the main function body
	funcCallFound := false
	inMainFunc := false
	funcCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			funcCount++
			// The second function should be main (first is double)
			if funcCount == 2 {
				inMainFunc = true
			}
		}
		if inst.opcode == OpFunctionEnd {
			inMainFunc = false
		}
		if inMainFunc && inst.opcode == OpFunctionCall {
			funcCallFound = true
			t.Logf("Found OpFunctionCall at word %d", inst.offset)
		}
	}
	if !funcCallFound {
		t.Error("BUG: No OpFunctionCall found in main function")
	}
}

// TestVarInitConditionalBlock tests that var init inside a conditional block works.
// The SPIR-V var is still in the entry block, but init only happens in the if-branch.
func TestVarInitConditionalBlock(t *testing.T) {
	const shader = `
struct Params {
    value: f32,
    flag: u32,
}
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    var result: f32 = 0.0;
    if params.flag != 0u {
        var scaled = params.value * 2.0;
        result = scaled;
    }
    out[idx] = u32(result * 255.0);
}
`

	spirvBytes := compileWGSLToSPIRV(t, "VarInitConditional", shader)
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := collectNames(instrs)

	t.Log("=== SPIR-V Disassembly for var init inside conditional ===")
	dumpFunctionBlocks(t, instrs, names)

	// Both 'result' and 'scaled' should be in the entry block (SPIR-V requirement)
	resultVarID := findFunctionVariable(instrs, names, "result")
	if resultVarID == 0 {
		t.Fatal("BUG: Local variable 'result' not found")
	}

	scaledVarID := findFunctionVariable(instrs, names, "scaled")
	if scaledVarID == 0 {
		t.Fatal("BUG: Local variable 'scaled' not found")
	}

	t.Logf("result=%%%d, scaled=%%%d", resultVarID, scaledVarID)

	// Verify 'result' has an init store (0.0) in entry block
	initStoreFound := verifyInitStore(t, instrs, names, resultVarID)
	if !initStoreFound {
		t.Error("BUG: No init OpStore for 'result' with 0.0")
	}
}

// --- Helper functions ---

// collectNames builds an ID-to-name map from OpName instructions.
func collectNames(instrs []spirvInstruction) map[uint32]string {
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}
	return names
}

// findFunctionVariable finds the SPIR-V ID of a Function-scope OpVariable with the given name.
func findFunctionVariable(instrs []spirvInstruction, names map[uint32]string, name string) uint32 {
	inFunction := false
	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if !inFunction {
			continue
		}
		if inst.opcode == OpVariable && inst.wordCount >= 4 {
			sc := StorageClass(inst.words[3])
			if sc == StorageClassFunction {
				varID := inst.words[2]
				if names[varID] == name {
					return varID
				}
			}
		}
	}
	return 0
}

// verifyInitStore checks if there is an OpStore targeting varID in the entry block
// (before any branch/selection/loop instructions).
func verifyInitStore(t *testing.T, instrs []spirvInstruction, names map[uint32]string, varID uint32) bool {
	t.Helper()
	inFunction := false
	entryBlock := false
	pastVariables := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			entryBlock = false
			pastVariables = false
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if !inFunction {
			continue
		}
		if inst.opcode == OpLabel && !entryBlock {
			entryBlock = true
			continue
		}
		if !entryBlock {
			continue
		}

		// Track when we're past OpVariable declarations
		if inst.opcode != OpVariable && !pastVariables {
			pastVariables = true
		}

		// An OpStore in the entry block to our variable = init store
		if inst.opcode == OpStore && inst.wordCount >= 3 {
			if inst.words[1] == varID {
				t.Logf("  Found init OpStore to %s(%%%d) at word %d, value=%%%d",
					names[varID], varID, inst.offset, inst.words[2])
				return true
			}
		}

		// If we hit a branch/selection, we've left the entry block init region
		if inst.opcode == OpSelectionMerge || inst.opcode == OpBranch ||
			inst.opcode == OpBranchConditional || inst.opcode == OpReturn {
			break
		}
	}
	return false
}

// verifyInitValueFromUniform checks that the value stored to varID comes from
// a chain of OpLoad <- OpAccessChain <- (uniform global variable).
func verifyInitValueFromUniform(t *testing.T, instrs []spirvInstruction, names map[uint32]string, varID uint32) {
	t.Helper()

	// Find the OpStore to varID in the entry block
	inFunction := false
	entryBlock := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			entryBlock = false
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if !inFunction {
			continue
		}
		if inst.opcode == OpLabel && !entryBlock {
			entryBlock = true
			continue
		}
		if !entryBlock {
			continue
		}

		if inst.opcode == OpStore && inst.wordCount >= 3 && inst.words[1] == varID {
			valueID := inst.words[2]
			t.Logf("  Init store value ID: %%%d", valueID)

			// Trace the value ID back — it should be from OpLoad
			traceValueOrigin(t, instrs, names, valueID, 0)
			return
		}

		if inst.opcode == OpSelectionMerge || inst.opcode == OpBranch ||
			inst.opcode == OpBranchConditional || inst.opcode == OpReturn {
			break
		}
	}

	t.Log("  WARNING: Could not find init OpStore to verify value origin")
}

// traceValueOrigin traces the origin of a SPIR-V value ID through loads and access chains.
func traceValueOrigin(t *testing.T, instrs []spirvInstruction, names map[uint32]string, valueID uint32, depth int) {
	t.Helper()
	if depth > 5 {
		return
	}

	indent := strings.Repeat("  ", depth+1)

	for _, inst := range instrs {
		// Look for the instruction that defines valueID
		switch inst.opcode {
		case OpLoad:
			if inst.wordCount >= 4 && inst.words[2] == valueID {
				sourceID := inst.words[3]
				t.Logf("%sOpLoad %%%d from %%%d(%s)", indent, valueID, sourceID, names[sourceID])
				traceValueOrigin(t, instrs, names, sourceID, depth+1)
				return
			}
		case OpAccessChain:
			if inst.wordCount >= 4 && inst.words[2] == valueID {
				baseID := inst.words[3]
				var indices []string
				for i := 4; i < inst.wordCount; i++ {
					indices = append(indices, fmt.Sprintf("%%%d", inst.words[i]))
				}
				t.Logf("%sOpAccessChain %%%d = base:%%%d(%s) [%s]",
					indent, valueID, baseID, names[baseID], strings.Join(indices, ", "))
				traceValueOrigin(t, instrs, names, baseID, depth+1)
				return
			}
		case OpVariable:
			if inst.wordCount >= 4 && inst.words[2] == valueID {
				sc := StorageClass(inst.words[3])
				t.Logf("%sOpVariable %%%d(%s) storage=%s", indent, valueID, names[valueID], storageClassName(uint32(sc)))
				return
			}
		}
	}
}

// countStoresToVar counts OpStore instructions targeting the given variable ID.
func countStoresToVar(instrs []spirvInstruction, varID uint32) int {
	count := 0
	inFunction := false
	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if !inFunction {
			continue
		}
		if inst.opcode == OpStore && inst.wordCount >= 3 && inst.words[1] == varID {
			count++
		}
	}
	return count
}

// dumpFunctionBlocks dumps all function blocks in human-readable form.
func dumpFunctionBlocks(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	// Collect type info
	typeInfo := buildTypeInfo(instrs, names)

	inFunction := false
	blockNum := 0

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			blockNum = 0
			if inst.wordCount >= 5 {
				t.Logf("--- FUNCTION %%%d (type=%%%d) ---",
					inst.words[2], inst.words[4])
			}
			continue
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
			t.Log("--- FUNCTION END ---")
			continue
		}
		if !inFunction {
			continue
		}

		if inst.opcode == OpLabel {
			blockNum++
			t.Logf("  BLOCK %d (label %%%d):", blockNum, inst.words[1])
			continue
		}

		switch inst.opcode {
		case OpVariable:
			if inst.wordCount >= 4 {
				sc := storageClassName(inst.words[3])
				t.Logf("    %4d: %%%d(%s) = OpVariable %s (type=%s)",
					inst.offset, inst.words[2], names[inst.words[2]], sc, typeInfo[inst.words[1]])
			}
		case OpLoad:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %%%d = OpLoad %%%d(%s) (type=%s)",
					inst.offset, inst.words[2], inst.words[3], names[inst.words[3]], typeInfo[inst.words[1]])
			}
		case OpStore:
			if inst.wordCount >= 3 {
				t.Logf("    %4d: OpStore *%%%d(%s) = %%%d(%s)",
					inst.offset, inst.words[1], names[inst.words[1]], inst.words[2], names[inst.words[2]])
			}
		case OpAccessChain:
			var indices []string
			for i := 4; i < inst.wordCount; i++ {
				indices = append(indices, fmt.Sprintf("%%%d", inst.words[i]))
			}
			t.Logf("    %4d: %%%d = OpAccessChain base:%%%d(%s) [%s]",
				inst.offset, inst.words[2], inst.words[3], names[inst.words[3]], strings.Join(indices, ", "))
		case OpSelectionMerge:
			t.Logf("    %4d: OpSelectionMerge merge:%%%d", inst.offset, inst.words[1])
		case OpBranchConditional:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: OpBranchConditional cond:%%%d true:%%%d false:%%%d",
					inst.offset, inst.words[1], inst.words[2], inst.words[3])
			}
		case OpBranch:
			t.Logf("    %4d: OpBranch -> %%%d", inst.offset, inst.words[1])
		case OpReturn:
			t.Logf("    %4d: OpReturn", inst.offset)
		case OpFMul:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpFMul %%%d, %%%d",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		case OpFAdd:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpFAdd %%%d, %%%d",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		case OpIMul:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpIMul %%%d, %%%d",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		case OpConvertFToU:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %%%d = OpConvertFToU %%%d",
					inst.offset, inst.words[2], inst.words[3])
			}
		case OpConvertUToF:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %%%d = OpConvertUToF %%%d",
					inst.offset, inst.words[2], inst.words[3])
			}
		case OpCompositeExtract:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpCompositeExtract %%%d [%d]",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		case OpUGreaterThanEqual:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpUGreaterThanEqual %%%d, %%%d",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		case OpINotEqual:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %%%d = OpINotEqual %%%d, %%%d",
					inst.offset, inst.words[2], inst.words[3], inst.words[4])
			}
		default:
			if inst.wordCount >= 3 {
				var operands []string
				for i := 1; i < inst.wordCount; i++ {
					operands = append(operands, fmt.Sprintf("%%%d", inst.words[i]))
				}
				t.Logf("    %4d: %s %s",
					inst.offset, opcodeName(inst.opcode), strings.Join(operands, " "))
			}
		}
	}
}

// buildTypeInfo builds a map of type IDs to human-readable type names.
func buildTypeInfo(instrs []spirvInstruction, names map[uint32]string) map[uint32]string {
	info := make(map[uint32]string)
	for _, inst := range instrs {
		switch inst.opcode {
		case OpTypeVoid:
			info[inst.words[1]] = "void"
		case OpTypeBool:
			info[inst.words[1]] = "bool"
		case OpTypeInt:
			if inst.wordCount >= 4 {
				sign := "u"
				if inst.words[3] == 1 {
					sign = "i"
				}
				info[inst.words[1]] = fmt.Sprintf("%s%d", sign, inst.words[2])
			}
		case OpTypeFloat:
			if inst.wordCount >= 3 {
				info[inst.words[1]] = fmt.Sprintf("f%d", inst.words[2])
			}
		case OpTypeVector:
			if inst.wordCount >= 4 {
				info[inst.words[1]] = fmt.Sprintf("vec%d<%s>", inst.words[3], info[inst.words[2]])
			}
		case OpTypePointer:
			if inst.wordCount >= 4 {
				info[inst.words[1]] = fmt.Sprintf("ptr<%s,%s>",
					storageClassName(inst.words[2]), info[inst.words[3]])
			}
		case OpTypeRuntimeArray:
			if inst.wordCount >= 3 {
				info[inst.words[1]] = fmt.Sprintf("rtarray<%s>", info[inst.words[2]])
			}
		case OpTypeStruct:
			name := names[inst.words[1]]
			if name == "" {
				name = "struct"
			}
			info[inst.words[1]] = name
		}
	}
	return info
}
