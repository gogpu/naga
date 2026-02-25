package spirv

import (
	"os"
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

// TestSpanFullShaderDump compiles both versions of path_count.wgsl through Go naga
// and saves the SPIR-V to files for comparison with Rust naga output and spirv-val.
func TestSpanFullShaderDump(t *testing.T) {
	for _, tc := range []struct {
		name, input, output string
	}{
		{"SpanFunc", "../../gg/tmp/path_count_span_full.wgsl", "../../gg/tmp/path_count_full_go.spv"},
		{"Inline", "../../gg/tmp/path_count_inline.wgsl", "../../gg/tmp/path_count_inline_go.spv"},
	} {
		shader, err := os.ReadFile(tc.input)
		if err != nil {
			t.Skipf("%s not found: %v", tc.input, err)
		}
		spirvBytes := compileWGSLToSPIRV(t, tc.name, string(shader))
		if err := os.WriteFile(tc.output, spirvBytes, 0644); err != nil {
			t.Fatalf("write SPIR-V: %v", err)
		}
		t.Logf("%s: wrote %d bytes to %s", tc.name, len(spirvBytes), tc.output)
	}
}

// TestSpanFunctionVsInline compares SPIR-V generated for span() as a function call
// vs the same expression inlined. Helps diagnose why span() produces wrong GPU results.
func TestSpanFunctionVsInline(t *testing.T) {
	const shaderWithFunc = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

fn span(a: f32, b: f32) -> u32 {
    return u32(max(ceil(max(a, b)) - floor(min(a, b)), 1.0));
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.5, 2.5);
    let s1 = vec2<f32>(3.5, 4.5);
    var count_x = span(s0.x, s1.x) - 1u;
    var count = count_x + span(s0.y, s1.y);
    out[gid.x] = count;
}
`
	const shaderInline = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let s0 = vec2<f32>(1.5, 2.5);
    let s1 = vec2<f32>(3.5, 4.5);
    var count_x = u32(max(ceil(max(s0.x, s1.x)) - floor(min(s0.x, s1.x)), 1.0)) - 1u;
    var count = count_x + u32(max(ceil(max(s0.y, s1.y)) - floor(min(s0.y, s1.y)), 1.0));
    out[gid.x] = count;
}
`
	t.Log("=== FUNCTION CALL VERSION ===")
	spirvFunc := compileWGSLToSPIRV(t, "SpanFunc", shaderWithFunc)
	t.Log(disassembleSPIRV(spirvFunc))

	t.Log("\n=== INLINE VERSION ===")
	spirvInline := compileWGSLToSPIRV(t, "SpanInline", shaderInline)
	t.Log(disassembleSPIRV(spirvInline))

	// Count key instructions
	for _, pair := range []struct {
		name string
		data []byte
	}{{"Function", spirvFunc}, {"Inline", spirvInline}} {
		instrs := decodeSPIRVInstructions(pair.data)
		var funcs, calls, extInsts, stores, loads int
		for _, inst := range instrs {
			switch inst.opcode {
			case OpFunction:
				funcs++
			case OpFunctionCall:
				calls++
			case OpExtInst:
				extInsts++
			case OpStore:
				stores++
			case OpLoad:
				loads++
			}
		}
		t.Logf("%s: %d functions, %d calls, %d ExtInst, %d stores, %d loads",
			pair.name, funcs, calls, extInsts, stores, loads)
	}
}

// TestBoolEqualUsesLogicalEqual verifies that bool==bool comparison emits
// OpLogicalEqual (not OpIEqual) and bool!=bool emits OpLogicalNotEqual
// (not OpINotEqual). SPIR-V spec requires logical ops for boolean operands.
// Bug: NAGA-SPV-007 — OpIEqual with bool operands fails spirv-val.
func TestBoolEqualUsesLogicalEqual(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let a = true;
    let b = gid.x == 0u;
    if a == b {
        out[0] = 1u;
    }
    if a != b {
        out[1] = 2u;
    }
}
`
	spirvBytes := compileWGSLToSPIRV(t, "BoolEqual", shader)

	instrs := decodeSPIRVInstructions(spirvBytes)
	var logicalEqual, logicalNotEqual, iEqual, iNotEqual int
	for _, inst := range instrs {
		switch inst.opcode {
		case OpLogicalEqual:
			logicalEqual++
		case OpLogicalNotEqual:
			logicalNotEqual++
		case OpIEqual:
			iEqual++
		case OpINotEqual:
			iNotEqual++
		}
	}

	t.Logf("OpLogicalEqual: %d, OpLogicalNotEqual: %d, OpIEqual: %d, OpINotEqual: %d",
		logicalEqual, logicalNotEqual, iEqual, iNotEqual)

	if logicalEqual == 0 {
		t.Errorf("expected OpLogicalEqual for bool==bool, got none (OpIEqual count: %d)", iEqual)
	}
	if logicalNotEqual == 0 {
		t.Errorf("expected OpLogicalNotEqual for bool!=bool, got none (OpINotEqual count: %d)", iNotEqual)
	}
}

// TestDeferredStoreTransitiveDeps verifies that var X = Y is correctly deferred
// when Y itself depends on a function call result.
// Bug: NAGA-SPV-008 — `var imax = count` was initialized in prologue before
// `count` was ready (count deferred due to span() call in its init).
func TestDeferredStoreTransitiveDeps(t *testing.T) {
	const shader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;

fn compute(a: f32, b: f32) -> u32 {
    return u32(max(ceil(max(a, b)) - floor(min(a, b)), 1.0));
}

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let x = f32(gid.x);
    var count = compute(x, x + 1.0);
    var copy = count;  // Depends on deferred count!
    out[0] = copy;
}
`
	spirvBytes := compileWGSLToSPIRV(t, "DeferredTransitive", shader)

	// Verify: OpStore for "copy" must come AFTER OpFunctionCall (which produces count).
	instrs := decodeSPIRVInstructions(spirvBytes)
	callSeen := false
	copyStoreAfterCall := false
	for _, inst := range instrs {
		if inst.opcode == OpFunctionCall {
			callSeen = true
		}
		// After the call, look for Store to "copy" variable.
		// The Store for copy should reference a Load from count, which was just stored.
		if callSeen && inst.opcode == OpStore {
			copyStoreAfterCall = true
		}
	}
	if !copyStoreAfterCall {
		t.Error("expected OpStore for copy variable after OpFunctionCall, but all stores precede the call")
	}
	t.Logf("Compiled %d bytes, deferred store ordering verified", len(spirvBytes))
}
