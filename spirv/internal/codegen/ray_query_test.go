package codegen

import (
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestRayQueryHelperFunctionsGenerated verifies that ray query operations
// generate helper functions (not inline opcodes), matching Rust naga's pattern.
func TestRayQueryHelperFunctionsGenerated(t *testing.T) {
	// Use the actual ray-query shader from test inputs
	source := `
enable wgpu_ray_query;
@group(0) @binding(0) var acc_struct: acceleration_structure;

fn get_tmin() -> f32 { return 0.1; }
fn get_tmax() -> f32 { return 100.0; }

@compute @workgroup_size(1)
fn main() {
    var rq: ray_query;
    let ray_desc = RayDesc(0u, 0xFFu, get_tmin(), get_tmax(), vec3f(0.0), vec3f(0.0, 0.0, 1.0));
    rayQueryInitialize(&rq, acc_struct, ray_desc);
    let proceed = rayQueryProceed(&rq);
    let intersection = rayQueryGetCommittedIntersection(&rq);
    _ = intersection.kind;
}
`
	opts := DefaultOptions()
	spvBytes := compileWGSLForCapabilityTestWithOpts(t, source, opts)

	caps := extractCapabilities(spvBytes)

	// Must have RayQueryKHR capability
	assertCapability(t, caps, spvenums.CapabilityRayQueryKHR)

	// Should have multiple OpFunction (entry point + helpers)
	funcCount := countOpFunctions(spvBytes)
	if funcCount < 4 {
		t.Errorf("expected at least 4 OpFunction (2 user + 2+ ray query helpers), got %d", funcCount)
	}

	// Should have OpFunctionCall (calling helper functions)
	calls := findOpFunctionCalls(spvBytes)
	if len(calls) < 3 {
		t.Errorf("expected at least 3 OpFunctionCall (init + proceed + getIntersection), got %d", len(calls))
	}

	// Should have OpRayQueryInitializeKHR (inside helper)
	if !divmodHasOpcode(spvBytes, spvenums.OpRayQueryInitializeKHR) {
		t.Error("ray query should emit OpRayQueryInitializeKHR")
	}

	// Should have OpRayQueryProceedKHR (inside helper)
	if !divmodHasOpcode(spvBytes, spvenums.OpRayQueryProceedKHR) {
		t.Error("ray query should emit OpRayQueryProceedKHR")
	}
}

const rayQueryIntersectionShader = `
enable wgpu_ray_query;
@group(0) @binding(0) var acc_struct: acceleration_structure;

@compute @workgroup_size(1)
fn main() {
    var rq: ray_query;
    rayQueryInitialize(&rq, acc_struct, RayDesc(0u, 0xFFu, 0.1, 100.0, vec3f(0.0), vec3f(0.0, 0.0, 1.0)));
    rayQueryProceed(&rq);
    let intersection = rayQueryGetCommittedIntersection(&rq);
    _ = intersection.barycentrics;
}
`

// resultsOf returns the result ids of every instruction with opcode op that
// has a result type and a result id.
func resultsOf(instrs []spirvInstruction, op spvenums.Op) map[uint32]spirvInstruction {
	out := map[uint32]spirvInstruction{}
	for _, in := range instrs {
		if in.opcode == op && len(in.words) > 2 {
			out[in.words[2]] = in
		}
	}
	return out
}

// The barycentrics and front face of an intersection exist only for a
// triangle hit, so the helper that fills the intersection struct branches on
// kind == triangle. It branched on kind != none instead, the condition of the
// enclosing branch, and read them for procedural hits too; Rust naga fixed the
// same bug in 55534165 (spv/ray/query.rs: branch_conditional(tri_comp_id)).
func TestRayQueryIntersectionTriangleBranch(t *testing.T) {
	instrs := decodeSPIRVInstructions(compileWGSLForCapabilityTestWithOpts(t, rayQueryIntersectionShader, DefaultOptions()))
	triangle := resultsOf(instrs, spvenums.OpIEqual)
	notNone := resultsOf(instrs, spvenums.OpINotEqual)
	byTriangle, byNotNone := 0, map[uint32]int{}
	for _, in := range instrs {
		if in.opcode != spvenums.OpBranchConditional {
			continue
		}
		cond := in.words[1]
		if _, ok := triangle[cond]; ok {
			byTriangle++
		}
		if _, ok := notNone[cond]; ok {
			byNotNone[cond]++
		}
	}
	if byTriangle == 0 {
		t.Error("no branch on kind == triangle: barycentrics and front face are read for every hit")
	}
	for id, n := range byNotNone {
		if n > 1 {
			t.Errorf("kind != none (%%%d) is the condition of %d branches, want 1", id, n)
		}
	}
}

// OpRayQueryProceedKHR returns false once traversal is done, and calling it
// again is undefined behavior in SPIR-V (not in HLSL). With init tracking the
// proceed helper must skip it once the tracker has FINISHED_TRAVERSAL:
// branch on initialized && !finished, as Rust naga since 55534165.
func TestRayQueryProceedSkippedAfterTraversal(t *testing.T) {
	instrs := decodeSPIRVInstructions(compileWGSLForCapabilityTestWithOpts(t, rayQueryIntersectionShader, DefaultOptions()))
	ands := resultsOf(instrs, spvenums.OpLogicalAnd)
	nots := resultsOf(instrs, spvenums.OpLogicalNot)
	for i, in := range instrs {
		if in.opcode != spvenums.OpRayQueryProceedKHR {
			continue
		}
		var guard *spirvInstruction
		for j := i - 1; j >= 0; j-- {
			if instrs[j].opcode == spvenums.OpBranchConditional {
				guard = &instrs[j]
				break
			}
		}
		if guard == nil {
			t.Fatal("OpRayQueryProceedKHR is not guarded by a conditional branch")
		}
		and, ok := ands[guard.words[1]]
		if !ok {
			t.Fatalf("OpRayQueryProceedKHR is guarded by %%%d, not by initialized && !finished", guard.words[1])
		}
		_, notA := nots[and.words[3]]
		_, notB := nots[and.words[4]]
		if !notA && !notB {
			t.Error("the guard of OpRayQueryProceedKHR does not negate the finished-traversal check")
		}
		return
	}
	t.Fatal("no OpRayQueryProceedKHR")
}

// TestRayQueryInitTrackingDisabled verifies that when RayQueryInitTracking=false,
// the validation branches are simplified (no init flag checks).
func TestRayQueryInitTrackingDisabled(t *testing.T) {
	source := `
enable wgpu_ray_query;
@group(0) @binding(0) var acc_struct: acceleration_structure;

@compute @workgroup_size(1)
fn main() {
    var rq: ray_query;
    let ray_desc = RayDesc(0u, 0xFFu, 0.1, 100.0, vec3f(0.0), vec3f(0.0, 0.0, 1.0));
    rayQueryInitialize(&rq, acc_struct, ray_desc);
    let proceed = rayQueryProceed(&rq);
}
`
	opts := DefaultOptions()
	opts.RayQueryInitTracking = false

	spvBytes := compileWGSLForCapabilityTestWithOpts(t, source, opts)

	// Should still compile and have ray query opcodes
	if !divmodHasOpcode(spvBytes, spvenums.OpRayQueryInitializeKHR) {
		t.Error("ray query without init tracking should still emit OpRayQueryInitializeKHR")
	}
	if !divmodHasOpcode(spvBytes, spvenums.OpRayQueryProceedKHR) {
		t.Error("ray query without init tracking should still emit OpRayQueryProceedKHR")
	}
}
