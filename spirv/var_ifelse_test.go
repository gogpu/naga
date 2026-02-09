package spirv

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// Shader A: BROKEN — uses var + if/else to assign result, then reads it after merge.
const shaderA_VarIfElse = `
struct Params { value: f32, flag: u32, width: u32, height: u32 }
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    if idx >= params.width * params.height { return; }

    var result: f32;
    if params.flag != 0u {
        result = params.value * 2.0;
    } else {
        result = params.value;
    }
    out[idx] = u32(result * 255.0);
}
`

// Shader B: WORKING — no var, no if/else, just let binding.
const shaderB_NoVar = `
struct Params { value: f32, flag: u32, width: u32, height: u32 }
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    if idx >= params.width * params.height { return; }

    let result = params.value;
    out[idx] = u32(result * 255.0);
}
`

// compileWGSLToSPIRV compiles WGSL source to SPIR-V bytes. Returns nil on error after logging.
func compileWGSLToSPIRV(t *testing.T, label, source string) []byte {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("[%s] Tokenize failed: %v", label, err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("[%s] Parse failed: %v", label, err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("[%s] Lower failed: %v", label, err)
	}

	opts := Options{
		Version: Version1_3,
		Debug:   true,
	}
	backend := NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("[%s] SPIR-V compile failed: %v", label, err)
	}

	validateSPIRVBinary(t, spirvBytes)
	return spirvBytes
}

// TestVarIfElseSPIRV compiles two shaders — one with var+if/else (broken) and one without (working) —
// and dumps both SPIR-V disassemblies to find the bug in var+if/else code generation.
func TestVarIfElseSPIRV(t *testing.T) {
	// ---------------------------------------------------------------
	// Compile Shader A (var + if/else)
	// ---------------------------------------------------------------
	t.Log("=== Compiling Shader A (var + if/else) ===")
	spirvA := compileWGSLToSPIRV(t, "ShaderA", shaderA_VarIfElse)
	t.Logf("Shader A: %d bytes (%d words)", len(spirvA), len(spirvA)/4)

	// ---------------------------------------------------------------
	// Compile Shader B (no var, no if/else)
	// ---------------------------------------------------------------
	t.Log("\n=== Compiling Shader B (no var) ===")
	spirvB := compileWGSLToSPIRV(t, "ShaderB", shaderB_NoVar)
	t.Logf("Shader B: %d bytes (%d words)", len(spirvB), len(spirvB)/4)

	// ---------------------------------------------------------------
	// Disassemble both
	// ---------------------------------------------------------------
	disasmA := disassembleSPIRV(spirvA)
	disasmB := disassembleSPIRV(spirvB)

	t.Log("\n\n" + strings.Repeat("=", 80))
	t.Log("SHADER A DISASSEMBLY (var + if/else — BROKEN)")
	t.Log(strings.Repeat("=", 80))
	t.Log("\n" + disasmA)

	t.Log("\n\n" + strings.Repeat("=", 80))
	t.Log("SHADER B DISASSEMBLY (no var — WORKING)")
	t.Log(strings.Repeat("=", 80))
	t.Log("\n" + disasmB)

	// ---------------------------------------------------------------
	// Focused analysis of Shader A control flow
	// ---------------------------------------------------------------
	t.Log("\n\n" + strings.Repeat("=", 80))
	t.Log("FOCUSED ANALYSIS: Shader A — var + if/else control flow")
	t.Log(strings.Repeat("=", 80))
	analyzeVarIfElse(t, spirvA)

	// ---------------------------------------------------------------
	// Compare key instructions
	// ---------------------------------------------------------------
	t.Log("\n\n" + strings.Repeat("=", 80))
	t.Log("COMPARISON: Key instruction counts")
	t.Log(strings.Repeat("=", 80))
	compareInstructionCounts(t, spirvA, spirvB)
}

// spirvInstruction represents a decoded SPIR-V instruction with offset info.
type spirvInstruction struct {
	offset    int
	opcode    OpCode
	wordCount int
	words     []uint32
}

// decodeSPIRVInstructions parses all instructions from SPIR-V binary (skipping header).
func decodeSPIRVInstructions(data []byte) []spirvInstruction {
	if len(data) < 20 || len(data)%4 != 0 {
		return nil
	}

	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}

	var instrs []spirvInstruction
	offset := 5 // skip header
	for offset < len(words) {
		wc := int(words[offset] >> 16)
		op := OpCode(words[offset] & 0xFFFF)
		if wc == 0 || offset+wc > len(words) {
			break
		}
		instrs = append(instrs, spirvInstruction{
			offset:    offset,
			opcode:    op,
			wordCount: wc,
			words:     words[offset : offset+wc],
		})
		offset += wc
	}
	return instrs
}

// analyzeVarIfElse performs a focused analysis of Shader A's var + if/else pattern.
func analyzeVarIfElse(t *testing.T, data []byte) {
	t.Helper()

	instrs := decodeSPIRVInstructions(data)
	if instrs == nil {
		t.Fatal("Failed to decode SPIR-V instructions")
	}

	// Collect names for readability
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// ---------------------------------------------------------------
	// 1. Find all OpVariable with Function storage class
	// ---------------------------------------------------------------
	t.Log("\n--- 1. Function-scope OpVariable instructions ---")
	var funcVars []spirvInstruction
	inFunction := false
	entryBlockStarted := false
	firstLabelSeen := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			firstLabelSeen = false
			entryBlockStarted = false
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if inFunction && inst.opcode == OpLabel && !firstLabelSeen {
			firstLabelSeen = true
			entryBlockStarted = true
		}
		// Any non-OpVariable, non-OpLabel instruction after entry label ends the entry block variable section
		if entryBlockStarted && inst.opcode != OpVariable && inst.opcode != OpLabel {
			entryBlockStarted = false
		}

		if inst.opcode == OpVariable && inst.wordCount >= 4 {
			sc := StorageClass(inst.words[3])
			if sc == StorageClassFunction {
				location := "NON-ENTRY"
				if entryBlockStarted {
					location = "ENTRY BLOCK"
				}
				t.Logf("  word %d: %s = OpVariable Function (type=%s) [%s]",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[1], names), location)
				funcVars = append(funcVars, inst)
			}
		}
	}
	if len(funcVars) == 0 {
		t.Log("  WARNING: No Function-scope OpVariable found! The 'var result: f32' is missing!")
	}

	// ---------------------------------------------------------------
	// 2. Find OpSelectionMerge and control flow structure
	// ---------------------------------------------------------------
	t.Log("\n--- 2. Selection merge and control flow ---")
	var selMerges []spirvInstruction
	for _, inst := range instrs {
		if inst.opcode == OpSelectionMerge {
			mergeLabel := inst.words[1]
			t.Logf("  word %d: OpSelectionMerge merge:%s control:%d",
				inst.offset, idStr(mergeLabel, names), inst.words[2])
			selMerges = append(selMerges, inst)
		}
	}

	// ---------------------------------------------------------------
	// 3. Find all OpBranchConditional
	// ---------------------------------------------------------------
	t.Log("\n--- 3. OpBranchConditional instructions ---")
	for _, inst := range instrs {
		if inst.opcode == OpBranchConditional && inst.wordCount >= 4 {
			t.Logf("  word %d: OpBranchConditional cond:%s true:%s false:%s",
				inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names), idStr(inst.words[3], names))
		}
	}

	// ---------------------------------------------------------------
	// 4. Find all OpStore instructions
	// ---------------------------------------------------------------
	t.Log("\n--- 4. OpStore instructions ---")
	for _, inst := range instrs {
		if inst.opcode == OpStore && inst.wordCount >= 3 {
			t.Logf("  word %d: OpStore *%s = %s",
				inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names))
		}
	}

	// ---------------------------------------------------------------
	// 5. Find all OpLoad instructions
	// ---------------------------------------------------------------
	t.Log("\n--- 5. OpLoad instructions ---")
	for _, inst := range instrs {
		if inst.opcode == OpLoad && inst.wordCount >= 4 {
			t.Logf("  word %d: OpLoad %s %s = load %s",
				inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names), idStr(inst.words[3], names))
		}
	}

	// ---------------------------------------------------------------
	// 6. Trace the full control flow graph (labels and branches)
	// ---------------------------------------------------------------
	t.Log("\n--- 6. Full control flow graph ---")
	for _, inst := range instrs {
		switch inst.opcode {
		case OpFunction:
			if inst.wordCount >= 5 {
				t.Logf("  word %d: OpFunction %s %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[4], names))
			}
		case OpLabel:
			t.Logf("  word %d: OpLabel %s", inst.offset, idStr(inst.words[1], names))
		case OpBranch:
			t.Logf("  word %d:   OpBranch -> %s", inst.offset, idStr(inst.words[1], names))
		case OpBranchConditional:
			if inst.wordCount >= 4 {
				t.Logf("  word %d:   OpBranchConditional cond:%s true:%s false:%s",
					inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpSelectionMerge:
			t.Logf("  word %d:   OpSelectionMerge merge:%s",
				inst.offset, idStr(inst.words[1], names))
		case OpReturn:
			t.Logf("  word %d:   OpReturn", inst.offset)
		case OpUnreachable:
			t.Logf("  word %d:   OpUnreachable", inst.offset)
		case OpFunctionEnd:
			t.Logf("  word %d: OpFunctionEnd", inst.offset)
		case OpVariable:
			if inst.wordCount >= 4 && StorageClass(inst.words[3]) == StorageClassFunction {
				t.Logf("  word %d:   OpVariable(Function) %s", inst.offset, idStr(inst.words[2], names))
			}
		case OpStore:
			if inst.wordCount >= 3 {
				t.Logf("  word %d:   OpStore *%s = %s",
					inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names))
			}
		case OpLoad:
			if inst.wordCount >= 4 {
				t.Logf("  word %d:   OpLoad %s = load %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpConvertFToU:
			if inst.wordCount >= 4 {
				t.Logf("  word %d:   OpConvertFToU %s = %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpFMul:
			if inst.wordCount >= 5 {
				t.Logf("  word %d:   OpFMul %s = %s * %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), idStr(inst.words[4], names))
			}
		case OpAccessChain:
			var indices []string
			for i := 4; i < inst.wordCount; i++ {
				indices = append(indices, idStr(inst.words[i], names))
			}
			t.Logf("  word %d:   OpAccessChain %s = base:%s [%s]",
				inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), strings.Join(indices, ", "))
		}
	}

	// ---------------------------------------------------------------
	// 7. Verify: OpVariable for 'result' must be in entry block
	// ---------------------------------------------------------------
	t.Log("\n--- 7. CRITICAL CHECK: Is OpVariable for 'result' in entry block? ---")
	checkEntryBlockVariables(t, instrs, names)

	// ---------------------------------------------------------------
	// 8. Verify: OpStore in both if/else branches target same variable
	// ---------------------------------------------------------------
	t.Log("\n--- 8. CRITICAL CHECK: Do both if/else branches store to the same variable? ---")
	checkBranchStores(t, instrs, names, selMerges)

	// ---------------------------------------------------------------
	// 9. Verify: OpLoad after merge loads from the var
	// ---------------------------------------------------------------
	t.Log("\n--- 9. CRITICAL CHECK: Does OpLoad after merge load from the correct variable? ---")
	checkPostMergeLoad(t, instrs, names, selMerges, funcVars)
}

// checkEntryBlockVariables verifies all Function-scope OpVariable are in the entry block.
func checkEntryBlockVariables(t *testing.T, instrs []spirvInstruction, names map[uint32]string) {
	t.Helper()

	inFunction := false
	entryBlock := false
	entryBlockEnded := false

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			entryBlock = false
			entryBlockEnded = false
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
		}
		if !inFunction {
			continue
		}

		if inst.opcode == OpLabel && !entryBlock && !entryBlockEnded {
			entryBlock = true
			continue
		}

		// In entry block, OpVariable is OK. Any other non-debug instruction ends the variable section.
		if entryBlock && inst.opcode != OpVariable &&
			inst.opcode != OpName && inst.opcode != OpMemberName {
			entryBlock = false
			entryBlockEnded = true
		}

		if inst.opcode == OpVariable && inst.wordCount >= 4 {
			sc := StorageClass(inst.words[3])
			if sc == StorageClassFunction {
				if entryBlock {
					t.Logf("  OK: OpVariable %s is in entry block (word %d)",
						idStr(inst.words[2], names), inst.offset)
				} else {
					t.Errorf("  BUG: OpVariable %s is NOT in entry block (word %d) — SPIR-V VIOLATION!",
						idStr(inst.words[2], names), inst.offset)
				}
			}
		}
	}
}

// checkBranchStores checks if OpStore instructions exist in both branches of the if/else
// and whether they target the same pointer (the 'result' variable).
func checkBranchStores(t *testing.T, instrs []spirvInstruction, names map[uint32]string, selMerges []spirvInstruction) {
	t.Helper()

	if len(selMerges) == 0 {
		t.Log("  No OpSelectionMerge found — cannot check branch stores")
		return
	}

	// For each selection merge, find the branch conditional and trace both branches
	for i, merge := range selMerges {
		mergeLabel := merge.words[1]
		t.Logf("  Selection %d: merge label = %s", i, idStr(mergeLabel, names))

		// Find the OpBranchConditional that follows this merge
		mergeIdx := -1
		for j, inst := range instrs {
			if inst.offset == merge.offset {
				mergeIdx = j
				break
			}
		}
		if mergeIdx < 0 || mergeIdx+1 >= len(instrs) {
			continue
		}

		branchCond := instrs[mergeIdx+1]
		if branchCond.opcode != OpBranchConditional {
			t.Logf("  WARNING: Instruction after OpSelectionMerge is not OpBranchConditional but %s",
				opcodeName(branchCond.opcode))
			continue
		}

		trueLabel := branchCond.words[2]
		falseLabel := branchCond.words[3]
		t.Logf("  True branch: %s, False branch: %s", idStr(trueLabel, names), idStr(falseLabel, names))

		// Collect stores in each branch
		trueStores := collectStoresInBlock(instrs, trueLabel)
		falseStores := collectStoresInBlock(instrs, falseLabel)

		t.Logf("  True branch stores (%d):", len(trueStores))
		for _, s := range trueStores {
			t.Logf("    word %d: OpStore *%s = %s", s.offset, idStr(s.words[1], names), idStr(s.words[2], names))
		}
		t.Logf("  False branch stores (%d):", len(falseStores))
		for _, s := range falseStores {
			t.Logf("    word %d: OpStore *%s = %s", s.offset, idStr(s.words[1], names), idStr(s.words[2], names))
		}

		// Check if both branches store to the same target
		if len(trueStores) > 0 && len(falseStores) > 0 {
			trueTarget := trueStores[len(trueStores)-1].words[1]
			falseTarget := falseStores[len(falseStores)-1].words[1]
			if trueTarget == falseTarget {
				t.Logf("  OK: Both branches store to same variable %s", idStr(trueTarget, names))
			} else {
				t.Errorf("  BUG: True branch stores to %s, false branch stores to %s — DIFFERENT TARGETS!",
					idStr(trueTarget, names), idStr(falseTarget, names))
			}
		} else if len(trueStores) == 0 && len(falseStores) == 0 {
			t.Log("  NOTE: Neither branch has OpStore (may use OpReturn in both branches)")
		} else {
			t.Logf("  WARNING: Asymmetric stores — true has %d, false has %d",
				len(trueStores), len(falseStores))
		}
	}
}

// collectStoresInBlock collects OpStore instructions in a basic block starting at the given label ID.
func collectStoresInBlock(instrs []spirvInstruction, labelID uint32) []spirvInstruction {
	var stores []spirvInstruction
	inBlock := false
	for _, inst := range instrs {
		if inst.opcode == OpLabel && inst.words[1] == labelID {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		// Block ends at next terminator
		if inst.opcode == OpBranch || inst.opcode == OpBranchConditional ||
			inst.opcode == OpReturn || inst.opcode == OpReturnValue ||
			inst.opcode == OpUnreachable || inst.opcode == OpKill {
			break
		}
		if inst.opcode == OpStore {
			stores = append(stores, inst)
		}
	}
	return stores
}

// checkPostMergeLoad verifies that after the merge block, OpLoad reads from the correct variable.
func checkPostMergeLoad(t *testing.T, instrs []spirvInstruction, names map[uint32]string,
	selMerges []spirvInstruction, funcVars []spirvInstruction) {
	t.Helper()

	if len(selMerges) == 0 {
		t.Log("  No selections to check")
		return
	}

	// Build a set of Function-scope variable IDs
	varIDs := make(map[uint32]bool)
	for _, v := range funcVars {
		varIDs[v.words[2]] = true
	}

	for i, merge := range selMerges {
		mergeLabel := merge.words[1]
		t.Logf("  Selection %d: Looking for OpLoad after merge label %s", i, idStr(mergeLabel, names))

		// Find the merge label in the instruction stream
		inMergeBlock := false
		for _, inst := range instrs {
			if inst.opcode == OpLabel && inst.words[1] == mergeLabel {
				inMergeBlock = true
				continue
			}
			if !inMergeBlock {
				continue
			}
			// Look for first OpLoad
			if inst.opcode == OpLoad && inst.wordCount >= 4 {
				loadSource := inst.words[3]
				t.Logf("    OpLoad at word %d: %s = load %s",
					inst.offset, idStr(inst.words[2], names), idStr(loadSource, names))
				if varIDs[loadSource] {
					t.Logf("    OK: Load source %s is a Function-scope variable", idStr(loadSource, names))
				} else {
					t.Logf("    NOTE: Load source %s is NOT a Function-scope variable (may be global/param)",
						idStr(loadSource, names))
				}
				break
			}
			// Stop at terminator
			if inst.opcode == OpBranch || inst.opcode == OpBranchConditional ||
				inst.opcode == OpReturn || inst.opcode == OpReturnValue ||
				inst.opcode == OpUnreachable {
				t.Logf("    No OpLoad found in merge block before terminator at word %d", inst.offset)
				break
			}
		}
	}
}

// compareInstructionCounts shows side-by-side comparison of key instruction counts.
func compareInstructionCounts(t *testing.T, spirvA, spirvB []byte) {
	t.Helper()

	countOps := func(data []byte) map[OpCode]int {
		counts := make(map[OpCode]int)
		instrs := decodeSPIRVInstructions(data)
		for _, inst := range instrs {
			counts[inst.opcode]++
		}
		return counts
	}

	countsA := countOps(spirvA)
	countsB := countOps(spirvB)

	keyOps := []struct {
		op   OpCode
		name string
	}{
		{OpVariable, "OpVariable"},
		{OpStore, "OpStore"},
		{OpLoad, "OpLoad"},
		{OpSelectionMerge, "OpSelectionMerge"},
		{OpBranchConditional, "OpBranchConditional"},
		{OpBranch, "OpBranch"},
		{OpLabel, "OpLabel"},
		{OpReturn, "OpReturn"},
		{OpUnreachable, "OpUnreachable"},
		{OpConvertFToU, "OpConvertFToU"},
		{OpFMul, "OpFMul"},
		{OpIMul, "OpIMul"},
		{OpAccessChain, "OpAccessChain"},
		{OpFunctionCall, "OpFunctionCall"},
	}

	t.Logf("  %-25s  ShaderA(var+if/else)  ShaderB(no var)", "Instruction")
	t.Logf("  %-25s  --------------------  ---------------", "---------")
	for _, kop := range keyOps {
		a := countsA[kop.op]
		b := countsB[kop.op]
		marker := ""
		if a != b {
			marker = " <-- DIFFERENT"
		}
		t.Logf("  %-25s  %4d                  %4d%s", kop.name, a, b, marker)
	}

	// Also show total word count
	t.Logf("\n  Total words: ShaderA=%d, ShaderB=%d", len(spirvA)/4, len(spirvB)/4)
}

// TestVarIfElseSPIRV_ControlFlowValidation runs the existing control flow validator on Shader A.
func TestVarIfElseSPIRV_ControlFlowValidation(t *testing.T) {
	spirvA := compileWGSLToSPIRV(t, "ShaderA", shaderA_VarIfElse)

	t.Log("Running control flow validation on Shader A (var + if/else)...")
	validateSPIRVControlFlow(t, spirvA)
	t.Log("Control flow validation passed!")
}

// TestVarIfElse_DetailedBlockDump dumps each basic block of Shader A with all instructions,
// making it easy to follow the exact execution path.
func TestVarIfElse_DetailedBlockDump(t *testing.T) {
	spirvA := compileWGSLToSPIRV(t, "ShaderA", shaderA_VarIfElse)
	instrs := decodeSPIRVInstructions(spirvA)

	// Collect names
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Collect type info for better display
	typeInfo := make(map[uint32]string)
	for _, inst := range instrs {
		switch inst.opcode {
		case OpTypeVoid:
			typeInfo[inst.words[1]] = "void"
		case OpTypeBool:
			typeInfo[inst.words[1]] = "bool"
		case OpTypeInt:
			if inst.wordCount >= 4 {
				sign := "u"
				if inst.words[3] == 1 {
					sign = "i"
				}
				typeInfo[inst.words[1]] = fmt.Sprintf("%s%d", sign, inst.words[2])
			}
		case OpTypeFloat:
			if inst.wordCount >= 3 {
				typeInfo[inst.words[1]] = fmt.Sprintf("f%d", inst.words[2])
			}
		case OpTypeVector:
			if inst.wordCount >= 4 {
				typeInfo[inst.words[1]] = fmt.Sprintf("vec%d<%s>", inst.words[3], typeInfo[inst.words[2]])
			}
		case OpTypePointer:
			if inst.wordCount >= 4 {
				typeInfo[inst.words[1]] = fmt.Sprintf("ptr<%s,%s>",
					storageClassName(inst.words[2]), typeInfo[inst.words[3]])
			}
		case OpTypeRuntimeArray:
			if inst.wordCount >= 3 {
				typeInfo[inst.words[1]] = fmt.Sprintf("rtarray<%s>", typeInfo[inst.words[2]])
			}
		case OpTypeStruct:
			name := names[inst.words[1]]
			if name == "" {
				name = "struct"
			}
			typeInfo[inst.words[1]] = name
		}
	}

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("DETAILED BLOCK-BY-BLOCK DUMP OF SHADER A")
	t.Log(strings.Repeat("=", 80))

	inFunction := false
	blockNum := 0

	for _, inst := range instrs {
		if inst.opcode == OpFunction {
			inFunction = true
			blockNum = 0
			if inst.wordCount >= 5 {
				t.Logf("\n--- FUNCTION %s (type=%s) ---",
					idStr(inst.words[2], names), idStr(inst.words[4], names))
			}
			continue
		}
		if inst.opcode == OpFunctionEnd {
			inFunction = false
			t.Log("--- FUNCTION END ---")
			continue
		}
		if inst.opcode == OpFunctionParameter {
			if inst.wordCount >= 3 {
				t.Logf("  PARAM: %s %s (type=%s)",
					idStr(inst.words[2], names), typeInfo[inst.words[1]], idStr(inst.words[1], names))
			}
			continue
		}

		if !inFunction {
			continue
		}

		if inst.opcode == OpLabel {
			blockNum++
			t.Logf("\n  BLOCK %d (label %s):", blockNum, idStr(inst.words[1], names))
			continue
		}

		// Print each instruction with type info
		switch inst.opcode {
		case OpVariable:
			if inst.wordCount >= 4 {
				sc := storageClassName(inst.words[3])
				t.Logf("    %4d: %s = OpVariable %s (type=%s/%s)",
					inst.offset, idStr(inst.words[2], names), sc,
					idStr(inst.words[1], names), typeInfo[inst.words[1]])
			}
		case OpLoad:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %s = OpLoad %s (type=%s/%s)",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names),
					idStr(inst.words[1], names), typeInfo[inst.words[1]])
			}
		case OpStore:
			if inst.wordCount >= 3 {
				t.Logf("    %4d: OpStore *%s = %s",
					inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names))
			}
		case OpAccessChain:
			var indices []string
			for i := 4; i < inst.wordCount; i++ {
				indices = append(indices, idStr(inst.words[i], names))
			}
			t.Logf("    %4d: %s = OpAccessChain base:%s [%s] (type=%s)",
				inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names),
				strings.Join(indices, ", "), typeInfo[inst.words[1]])
		case OpSelectionMerge:
			t.Logf("    %4d: OpSelectionMerge merge:%s control:%d",
				inst.offset, idStr(inst.words[1], names), inst.words[2])
		case OpBranchConditional:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: OpBranchConditional cond:%s true:%s false:%s",
					inst.offset, idStr(inst.words[1], names), idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpBranch:
			t.Logf("    %4d: OpBranch -> %s", inst.offset, idStr(inst.words[1], names))
		case OpReturn:
			t.Logf("    %4d: OpReturn", inst.offset)
		case OpUnreachable:
			t.Logf("    %4d: OpUnreachable", inst.offset)
		case OpFMul:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %s = OpFMul %s, %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), idStr(inst.words[4], names))
			}
		case OpIMul:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %s = OpIMul %s, %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), idStr(inst.words[4], names))
			}
		case OpConvertFToU:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %s = OpConvertFToU %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpConvertUToF:
			if inst.wordCount >= 4 {
				t.Logf("    %4d: %s = OpConvertUToF %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names))
			}
		case OpCompositeExtract:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %s = OpCompositeExtract %s [%d]",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), inst.words[4])
			}
		case OpINotEqual:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %s = OpINotEqual %s, %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), idStr(inst.words[4], names))
			}
		case OpUGreaterThanEqual:
			if inst.wordCount >= 5 {
				t.Logf("    %4d: %s = OpUGreaterThanEqual %s, %s",
					inst.offset, idStr(inst.words[2], names), idStr(inst.words[3], names), idStr(inst.words[4], names))
			}
		case OpConstant:
			// skip in function body
		default:
			// Generic display for other opcodes
			var operands []string
			for i := 1; i < inst.wordCount; i++ {
				operands = append(operands, idStr(inst.words[i], names))
			}
			t.Logf("    %4d: %s %s",
				inst.offset, opcodeName(inst.opcode), strings.Join(operands, " "))
		}
	}
}
