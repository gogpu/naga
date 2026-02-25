package spirv

import (
	"os"
	"os/exec"
	"testing"
)

// TestVec2IfElseCacheBug reproduces the exact pattern from path_tiling.wgsl:
//   - var xy0 = vec2<f32>(...);
//   - read xy0.x (first read → cached)
//   - if cond { xy0 = vec2<f32>(...); }
//   - read xy0.x AGAIN (second read → must be FRESH, not cached!)
//
// If the second read returns the cached (stale) value from the first read,
// the GPU output will be wrong — this is the bug causing 12.5% pixel diff.
func TestVec2IfElseCacheBug(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    // Initialize var xy0 (mutable)
    var xy0 = vec2<f32>(1.0, 2.0);
    let xy1 = vec2<f32>(5.0, 6.0);

    // First read of xy0.x — goes through emitAccessIndex → OpAccessChain + OpLoad
    let first_x = xy0.x;

    // Conditional store — modifies xy0 inside if/else
    if config.x > 0u {
        // Compute new x based on xy0 and xy1 (mimics path_tiling clipping)
        let xt = xy0.x + (xy1.x - xy0.x) * 0.5;
        xy0 = vec2<f32>(xt, 10.0);
    } else {
        xy0 = vec2<f32>(99.0, 10.0);
    }

    // Second read of xy0.x — MUST see the modified value!
    // BUG: if ExprAccessIndex result is cached, this returns first_x (stale)
    let second_x = xy0.x;

    // Output both values — GPU should show different first_x and second_x
    out[0] = first_x;    // Should be 1.0
    out[1] = second_x;   // Should be 3.0 (if config.x > 0) or 99.0 (else)
    out[2] = xy0.y;      // Should be 10.0
}
`
	spirvBytes := compileWGSLToSPIRV(t, "Vec2IfElseCache", shader)

	// Write SPIR-V to temp file for spirv-dis
	spvFile := "../../gg/tmp/vec2_ifelse_cache.spv"
	if err := os.WriteFile(spvFile, spirvBytes, 0o644); err != nil {
		t.Fatalf("write SPIR-V: %v", err)
	}

	// Try spirv-dis for nice disassembly
	disCmd := exec.Command("spirv-dis", spvFile)
	disOut, err := disCmd.CombinedOutput()
	if err != nil {
		t.Logf("spirv-dis failed: %v", err)
		// Fallback to our own disassembler
		t.Log(disassembleSPIRV(spirvBytes))
	} else {
		t.Logf("spirv-dis output:\n%s", string(disOut))
	}

	// Try spirv-val
	valCmd := exec.Command("spirv-val", spvFile)
	valOut, err := valCmd.CombinedOutput()
	if err != nil {
		t.Errorf("spirv-val FAILED: %v\n%s", err, string(valOut))
	} else {
		t.Log("spirv-val: PASS")
	}

	// Analyze SPIR-V instructions to check for the bug
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Count OpLoad instructions in the main function body (after merge)
	// We need at least 2 separate OpLoad of xy0's .x component
	var loadCount, storeCount, accessChainCount int
	for _, inst := range instrs {
		switch inst.opcode {
		case OpLoad:
			loadCount++
		case OpStore:
			storeCount++
		case OpAccessChain:
			accessChainCount++
		}
	}
	t.Logf("Total: %d OpLoad, %d OpStore, %d OpAccessChain", loadCount, storeCount, accessChainCount)

	// Check if there are enough loads — we need at minimum:
	// - Load config.x for the condition
	// - Load xy0.x for first_x
	// - Load xy0.x/xy0.y inside the if branch for computing xt
	// - Store xy0 in both branches
	// - Load xy0.x for second_x (MUST BE FRESH — not reusing first load)
	// - Load xy0.y for the third output
	t.Logf("Stores: %d (need >=4: xy0 init, xy0 in if, xy0 in else, outputs)", storeCount)
}

// TestVec2IfElseCacheBugMinimal is an even more minimal reproduction
// that isolates the exact expression handle reuse pattern.
func TestVec2IfElseCacheBugMinimal(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> flag: u32;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var v = vec2<f32>(1.0, 2.0);

    // First read
    let a = v.x;

    if flag > 0u {
        v = vec2<f32>(100.0, 200.0);
    }

    // Second read — same .x member, but v was modified!
    let b = v.x;

    out[0] = a;  // Must be 1.0
    out[1] = b;  // Must be 100.0 (if flag > 0) or 1.0 (if flag == 0)
}
`
	spirvBytes := compileWGSLToSPIRV(t, "Vec2CacheMinimal", shader)

	// Write SPIR-V to temp file for spirv-dis
	spvFile := "../../gg/tmp/vec2_cache_minimal.spv"
	if err := os.WriteFile(spvFile, spirvBytes, 0o644); err != nil {
		t.Fatalf("write SPIR-V: %v", err)
	}

	// spirv-dis
	disCmd := exec.Command("spirv-dis", spvFile)
	disOut, err := disCmd.CombinedOutput()
	if err != nil {
		t.Logf("spirv-dis failed: %v", err)
		t.Log(disassembleSPIRV(spirvBytes))
	} else {
		t.Logf("spirv-dis output:\n%s", string(disOut))
	}

	// spirv-val
	valCmd := exec.Command("spirv-val", spvFile)
	valOut, err := valCmd.CombinedOutput()
	if err != nil {
		t.Errorf("spirv-val FAILED: %v\n%s", err, string(valOut))
	} else {
		t.Log("spirv-val: PASS")
	}
}

// TestPathTilingFullShader compiles the actual path_tiling.wgsl and disassembles it.
// This is the real shader that produces 12.5% pixel diff.
func TestPathTilingFullShader(t *testing.T) {
	shaderPath := "../../gg/internal/gpu/tilecompute/shaders/path_tiling.wgsl"
	shaderSrc, err := os.ReadFile(shaderPath)
	if err != nil {
		t.Skipf("path_tiling.wgsl not found: %v", err)
	}

	spirvBytes := compileWGSLToSPIRV(t, "PathTiling", string(shaderSrc))

	spvFile := "../../gg/tmp/path_tiling_go.spv"
	if err := os.WriteFile(spvFile, spirvBytes, 0o644); err != nil {
		t.Fatalf("write SPIR-V: %v", err)
	}

	// spirv-dis
	disCmd := exec.Command("spirv-dis", spvFile)
	disOut, err := disCmd.CombinedOutput()
	if err != nil {
		t.Logf("spirv-dis failed: %v", err)
	} else {
		// Save full disassembly to file
		disFile := "../../gg/tmp/path_tiling_go.spvasm"
		os.WriteFile(disFile, disOut, 0o644)
		t.Logf("Full disassembly saved to %s (%d bytes)", disFile, len(disOut))
	}

	// spirv-val
	valCmd := exec.Command("spirv-val", spvFile)
	valOut, err := valCmd.CombinedOutput()
	if err != nil {
		t.Errorf("spirv-val FAILED: %v\n%s", err, string(valOut))
	} else {
		t.Log("spirv-val: PASS")
	}

	t.Logf("Compiled %d bytes", len(spirvBytes))
}

// TestVarInitAfterLocalVarModification is a regression test for NAGA-SPV-007:
// the SPIR-V prologue pre-computed var init expressions before the function body,
// causing stale values when the init referenced a local variable modified by
// preceding control flow.
//
// Pattern from path_tiling.wgsl:
//
//	var xy0 = select(p1, p0, is_down);
//	if cond { xy0 = clipped_value; }
//	var p0out = xy0 - tile_xy;  // MUST use post-clipping xy0!
//
// Before the fix, p0out was computed in the PROLOGUE using the initial xy0,
// producing wrong results (12.5% pixel diff in path_tiling).
func TestVarInitAfterLocalVarModification(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    // Initialize mutable local variable
    var xy0 = vec2<f32>(1.0, 2.0);

    // Modify xy0 inside control flow (mimics top-clipping in path_tiling)
    if config.x > 0u {
        xy0 = vec2<f32>(10.0, 20.0);
    } else {
        xy0 = vec2<f32>(30.0, 40.0);
    }

    // var init that references the MODIFIED local variable.
    // BUG (pre-fix): p0out computed in prologue using initial xy0 = (1, 2).
    // CORRECT: p0out computed here using post-if xy0 = (10, 20) or (30, 40).
    var p0out = xy0 - vec2<f32>(5.0, 5.0);

    out[0] = p0out.x;  // Should be 5.0 or 25.0 (NOT -4.0!)
    out[1] = p0out.y;  // Should be 15.0 or 35.0 (NOT -3.0!)
}
`
	spirvBytes := compileWGSLToSPIRV(t, "VarInitAfterModification", shader)

	// Decode instructions and collect names for analysis
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Find the OpStore for p0out and verify it comes AFTER the if/else merge.
	// Collect key instruction positions.
	var (
		p0outStoreOffset = -1
		lastMergeOffset  = -1
		lastXY0StoreInIf = -1
	)

	// Find the SPIR-V ID for p0out variable
	var p0outID uint32
	for id, name := range names {
		if name == "p0out" {
			p0outID = id
			break
		}
	}
	if p0outID == 0 {
		t.Fatal("Could not find 'p0out' variable in SPIR-V names")
	}

	// Track merge labels for if/else blocks
	mergeLabels := make(map[uint32]bool)
	for _, inst := range instrs {
		if inst.opcode == OpSelectionMerge && inst.wordCount >= 2 {
			mergeLabels[inst.words[1]] = true
		}
	}

	for _, inst := range instrs {
		// Track merge block entries (label that is a merge target)
		if inst.opcode == OpLabel && mergeLabels[inst.words[1]] {
			lastMergeOffset = inst.offset
		}

		// Track stores to xy0 inside if/else branches
		if inst.opcode == OpStore && inst.wordCount >= 3 {
			targetName := names[inst.words[1]]
			if targetName == "xy0" && lastMergeOffset == -1 {
				// xy0 store before any merge = inside if/else
				lastXY0StoreInIf = inst.offset
			}

			// Find the first OpStore to p0out (not the prologue init)
			if inst.words[1] == p0outID && p0outStoreOffset == -1 {
				p0outStoreOffset = inst.offset
			}
		}
	}

	// Key assertion: p0out store must be AFTER the merge block
	if p0outStoreOffset == -1 {
		t.Fatal("No OpStore to p0out found")
	}
	if lastMergeOffset == -1 {
		t.Fatal("No merge block found after if/else")
	}

	t.Logf("xy0 store in if/else: word %d", lastXY0StoreInIf)
	t.Logf("Merge block label:    word %d", lastMergeOffset)
	t.Logf("p0out store:          word %d", p0outStoreOffset)

	if p0outStoreOffset < lastMergeOffset {
		t.Errorf("REGRESSION: p0out store (word %d) is BEFORE merge block (word %d)! "+
			"This means p0out is computed in the prologue using stale xy0 values. "+
			"See NAGA-SPV-007.", p0outStoreOffset, lastMergeOffset)
	} else {
		t.Log("OK: p0out store is correctly positioned AFTER the if/else merge block")
	}
}

// TestVarInitChainedLocalVarModification tests a more complex scenario where
// multiple local variables are modified, and a later var init depends on
// several of them through an expression tree.
func TestVarInitChainedLocalVarModification(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var a = vec2<f32>(1.0, 2.0);
    var b = vec2<f32>(3.0, 4.0);

    // Modify 'a' conditionally
    if config.x > 0u {
        a = vec2<f32>(100.0, 200.0);
    }

    // Modify 'b' conditionally
    if config.y > 0u {
        b = vec2<f32>(300.0, 400.0);
    }

    // var init referencing BOTH modified local vars
    var result = a + b;

    out[0] = result.x;
    out[1] = result.y;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "ChainedVarInit", shader)

	// Decode and find names
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Find result variable ID
	var resultID uint32
	for id, name := range names {
		if name == "result" {
			resultID = id
			break
		}
	}
	if resultID == 0 {
		t.Fatal("Could not find 'result' variable in SPIR-V names")
	}

	// Find last merge block and result store
	mergeLabels := make(map[uint32]bool)
	for _, inst := range instrs {
		if inst.opcode == OpSelectionMerge && inst.wordCount >= 2 {
			mergeLabels[inst.words[1]] = true
		}
	}

	var lastMergeOffset, resultStoreOffset int
	mergeCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpLabel && mergeLabels[inst.words[1]] {
			lastMergeOffset = inst.offset
			mergeCount++
		}
		if inst.opcode == OpStore && inst.wordCount >= 3 && inst.words[1] == resultID {
			resultStoreOffset = inst.offset
		}
	}

	t.Logf("Found %d merge blocks, last at word %d", mergeCount, lastMergeOffset)
	t.Logf("Result store at word %d", resultStoreOffset)

	if mergeCount < 2 {
		t.Fatalf("Expected at least 2 merge blocks (two if statements), got %d", mergeCount)
	}
	if resultStoreOffset < lastMergeOffset {
		t.Errorf("REGRESSION: result store (word %d) is BEFORE last merge (word %d)! "+
			"See NAGA-SPV-007.", resultStoreOffset, lastMergeOffset)
	} else {
		t.Log("OK: result store is correctly positioned AFTER both if/else merge blocks")
	}
}

// TestVarInitConstNotAffected verifies that var inits with constant expressions
// are still pre-computed in the prologue (not needlessly moved to body position).
// This ensures the fix for NAGA-SPV-007 doesn't regress performance by moving
// ALL var inits to body position.
func TestVarInitConstNotAffected(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<f32>;
@group(0) @binding(1) var<storage, read_write> out: array<f32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    // These const-expression inits should stay in prologue:
    var x = 42.0;
    var y = vec2<f32>(1.0, 2.0);

    // This uniform-based init should also stay in prologue
    // (uniforms are immutable during execution):
    var z = config.x * 2.0;

    if config.y > 0.0 {
        x = 100.0;
    }

    out[0] = x;
    out[1] = y.x;
    out[2] = z;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "ConstInitPrologue", shader)

	// Decode and find names
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	// Find the first OpSelectionMerge to mark where body control flow begins
	var firstSelectionMergeOffset int
	for _, inst := range instrs {
		if inst.opcode == OpSelectionMerge {
			firstSelectionMergeOffset = inst.offset
			break
		}
	}

	// Find stores to x, y, z and check they're in the prologue (before selection merge)
	var xStoreOffset, yStoreOffset, zStoreOffset int
	for _, inst := range instrs {
		if inst.opcode != OpStore || inst.wordCount < 3 {
			continue
		}
		name := names[inst.words[1]]
		switch name {
		case "x":
			if xStoreOffset == 0 {
				xStoreOffset = inst.offset // first store = init
			}
		case "y":
			if yStoreOffset == 0 {
				yStoreOffset = inst.offset
			}
		case "z":
			if zStoreOffset == 0 {
				zStoreOffset = inst.offset
			}
		}
	}

	t.Logf("x init store: word %d", xStoreOffset)
	t.Logf("y init store: word %d", yStoreOffset)
	t.Logf("z init store: word %d", zStoreOffset)
	t.Logf("First selection merge: word %d", firstSelectionMergeOffset)

	// All const/uniform inits should be BEFORE the selection merge (in prologue)
	if xStoreOffset > firstSelectionMergeOffset {
		t.Errorf("x init (constant) should be in prologue but is at word %d (after merge at %d)",
			xStoreOffset, firstSelectionMergeOffset)
	}
	if yStoreOffset > firstSelectionMergeOffset {
		t.Errorf("y init (constant vec2) should be in prologue but is at word %d (after merge at %d)",
			yStoreOffset, firstSelectionMergeOffset)
	}
	if zStoreOffset > firstSelectionMergeOffset {
		t.Errorf("z init (uniform-based) should be in prologue but is at word %d (after merge at %d)",
			zStoreOffset, firstSelectionMergeOffset)
	}

	if xStoreOffset > 0 && xStoreOffset < firstSelectionMergeOffset &&
		yStoreOffset > 0 && yStoreOffset < firstSelectionMergeOffset &&
		zStoreOffset > 0 && zStoreOffset < firstSelectionMergeOffset {
		t.Log("OK: All constant/uniform-based var inits remain in prologue")
	}
}
