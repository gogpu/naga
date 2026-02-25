package spirv

import (
	"testing"
)

// TestVelloWorkaround_NestedForLoops tests whether nested for-loops compile
// correctly. Bug: outer loop executes only 1 iteration.
// Workaround in shaders: flat loop with div/mod.
func TestVelloWorkaround_NestedForLoops(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var sum = 0u;
    for (var y = 0u; y < 4u; y = y + 1u) {
        for (var x = 0u; x < 4u; x = x + 1u) {
            sum = sum + 1u;
        }
    }
    out[gid.x] = sum;  // Should be 16
}
`
	spirvBytes := compileWGSLToSPIRV(t, "NestedForLoops", shader)
	validateSPIRVControlFlow(t, spirvBytes)
	t.Logf("Compiled %d bytes, control flow valid", len(spirvBytes))
}

// TestVelloWorkaround_VarAtomicInit tests whether `var x = atomicAdd(...)`
// compiles correctly. Bug: "atomic result expression not found".
// Workaround: split var declaration and assignment.
func TestVelloWorkaround_VarAtomicInit(t *testing.T) {
	const shader = `
struct BumpAlloc {
    counter: atomic<u32>,
    _pad1: atomic<u32>,
    _pad2: atomic<u32>,
    _pad3: atomic<u32>,
}

@group(0) @binding(0) var<storage, read_write> bump: BumpAlloc;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var seg_ix = atomicAdd(&bump.counter, 1u);
    out[gid.x] = seg_ix;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "VarAtomicInit", shader)
	validateSPIRVControlFlow(t, spirvBytes)
	t.Logf("Compiled %d bytes, control flow valid", len(spirvBytes))
}

// TestVelloWorkaround_StoreInIfElse tests whether stores inside if/else
// blocks persist after the merge point.
// Bug: stores are lost at merge point.
// Workaround: use select() instead of if/else.
func TestVelloWorkaround_StoreInIfElse(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var result = 0.0;
    if config.x > 0u {
        result = 42.0;
    } else {
        result = 7.0;
    }
    out[gid.x] = u32(result);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "StoreInIfElse", shader)
	validateSPIRVControlFlow(t, spirvBytes)
	t.Logf("Compiled %d bytes, control flow valid", len(spirvBytes))
}

// TestVelloWorkaround_Vec2VarReassignment tests whether vec2 var reassignment
// works correctly.
// Bug: `xy0 = select(xy0, new_val, cond)` silently dropped.
// Workaround: use scalar let-chain.
func TestVelloWorkaround_Vec2VarReassignment(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<uniform> config: vec4<u32>;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let p0 = vec2<f32>(10.0, 20.0);
    let p1 = vec2<f32>(30.0, 40.0);
    let is_down = p1.y >= p0.y;

    var xy0 = select(p1, p0, is_down);
    var xy1 = select(p0, p1, is_down);

    // Reassignment — this is where the bug occurs
    let do_clip = config.x > 0u;
    let clipped = vec2<f32>(15.0, 25.0);
    xy0 = select(xy0, clipped, do_clip);

    out[gid.x] = u32(xy0.x) + u32(xy0.y) * 256u;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "Vec2VarReassign", shader)
	validateSPIRVControlFlow(t, spirvBytes)

	// Disassemble to check OpStore
	instrs := decodeSPIRVInstructions(spirvBytes)
	names := make(map[uint32]string)
	for _, inst := range instrs {
		if inst.opcode == OpName && inst.wordCount >= 3 {
			names[inst.words[1]] = decodeString(inst.words[2:])
		}
	}

	storeCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpStore {
			storeCount++
			t.Logf("OpStore *%s = %s (word %d)",
				idStr(inst.words[1], names), idStr(inst.words[2], names), inst.offset)
		}
	}
	t.Logf("Total OpStore instructions: %d", storeCount)
	// Need at least 3 stores: xy0 init, xy1 init, xy0 reassignment
	if storeCount < 3 {
		t.Errorf("Expected at least 3 OpStore (xy0 init, xy1 init, xy0 reassign), got %d", storeCount)
	}
}

// TestVelloWorkaround_FuncCallResult tests whether `let x = func()` properly
// captures the function result.
// Bug: inlining bug where call result is lost.
// Workaround: use `var x: T; x = func();` instead.
func TestVelloWorkaround_FuncCallResult(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

fn compute_value(x: f32) -> f32 {
    return x * 2.0 + 1.0;
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let result = compute_value(5.0);
    out[gid.x] = u32(result);
}
`
	spirvBytes := compileWGSLToSPIRV(t, "FuncCallResult", shader)
	validateSPIRVControlFlow(t, spirvBytes)
	t.Logf("Compiled %d bytes, control flow valid", len(spirvBytes))
}

// TestVelloWorkaround_TwoCallsSameFunc tests whether calling the same function
// twice works correctly.
// Bug: second call result is lost.
// Workaround: inline the function body instead.
func TestVelloWorkaround_TwoCallsSameFunc(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

fn span(a: f32, b: f32) -> u32 {
    return u32(max(ceil(max(a, b)) - floor(min(a, b)), 1.0));
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.5, 2.5);
    let s1 = vec2<f32>(3.5, 4.5);
    let count_x = span(s0.x, s1.x);
    let count_y = span(s0.y, s1.y);
    out[gid.x] = count_x + count_y;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "TwoCallsSameFunc", shader)
	validateSPIRVControlFlow(t, spirvBytes)

	// Check for two OpFunctionCall instructions
	instrs := decodeSPIRVInstructions(spirvBytes)
	callCount := 0
	for _, inst := range instrs {
		if inst.opcode == OpFunctionCall {
			callCount++
		}
	}
	t.Logf("Compiled %d bytes, %d OpFunctionCall instructions", len(spirvBytes), callCount)
	if callCount < 2 {
		t.Errorf("Expected 2 OpFunctionCall (span called twice), got %d — possible inlining issue", callCount)
	}
}

// TestVelloWorkaround_VarInitWithTwoFuncCalls tests the specific pattern
// that was fixed in d56b50b: `var count = span(a,b) + span(c,d)`.
// Bug: first span() result triggers deferred store before second span() is cached.
func TestVelloWorkaround_VarInitWithTwoFuncCalls(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

fn span(a: f32, b: f32) -> u32 {
    return u32(max(ceil(max(a, b)) - floor(min(a, b)), 1.0));
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.5, 2.5);
    let s1 = vec2<f32>(3.5, 4.5);
    var count = span(s0.x, s1.x) + span(s0.y, s1.y);
    out[gid.x] = count;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "VarInitTwoCalls", shader)
	validateSPIRVControlFlow(t, spirvBytes)
	t.Logf("Compiled %d bytes, control flow valid", len(spirvBytes))
}
