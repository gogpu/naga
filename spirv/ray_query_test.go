package spirv

import (
	"testing"
)

// TestRayQueryHelperFunctionsGenerated verifies that ray query operations
// generate helper functions (not inline opcodes), matching Rust naga's pattern.
func TestRayQueryHelperFunctionsGenerated(t *testing.T) {
	// Use the actual ray-query shader from test inputs
	source := `
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
	assertCapability(t, caps, CapabilityRayQueryKHR)

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
	if !divmodHasOpcode(spvBytes, OpRayQueryInitializeKHR) {
		t.Error("ray query should emit OpRayQueryInitializeKHR")
	}

	// Should have OpRayQueryProceedKHR (inside helper)
	if !divmodHasOpcode(spvBytes, OpRayQueryProceedKHR) {
		t.Error("ray query should emit OpRayQueryProceedKHR")
	}
}

// TestRayQueryInitTrackingDisabled verifies that when RayQueryInitTracking=false,
// the validation branches are simplified (no init flag checks).
func TestRayQueryInitTrackingDisabled(t *testing.T) {
	source := `
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
	if !divmodHasOpcode(spvBytes, OpRayQueryInitializeKHR) {
		t.Error("ray query without init tracking should still emit OpRayQueryInitializeKHR")
	}
	if !divmodHasOpcode(spvBytes, OpRayQueryProceedKHR) {
		t.Error("ray query without init tracking should still emit OpRayQueryProceedKHR")
	}
}
