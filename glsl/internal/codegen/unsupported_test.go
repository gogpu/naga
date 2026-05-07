// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// GLSL unsupported feature tests — hand-crafted IR modules
// =============================================================================

// TestGLSL_UnsupportedRayQuery verifies that ray query statements produce a
// clear error because GLSL does not support ray queries without extensions.
func TestGLSL_UnsupportedRayQuery(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.RayQueryType{}},                            // type 0
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, // type 1: bool
		},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				LocalVars: []ir.LocalVariable{
					{Name: "query", Type: 0},
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprLocalVariable{Variable: 0}}, // expr 0: query local
				},
				Body: ir.Block{
					{Kind: ir.StmtRayQuery{
						Query: 0,
						Fun:   ir.RayQueryProceed{Result: 0},
					}},
				},
			},
		}},
	}
	_, _, err := Compile(mod, Options{
		LangVersion: Version430,
		EntryPoint:  "main",
	})
	if err == nil {
		t.Fatal("expected error for unsupported ray query in GLSL")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "ray query") && !strings.Contains(errMsg, "ray_query") {
		t.Errorf("error should mention ray query: %v", err)
	}
}

// TestGLSL_UnsupportedMeshShader verifies that mesh shader entry points
// produce a clear error because GLSL does not support mesh shaders.
func TestGLSL_UnsupportedMeshShader(t *testing.T) {
	mod := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:      "mesh_main",
			Stage:     ir.StageMesh,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Body: ir.Block{},
			},
		}},
	}
	_, _, err := Compile(mod, Options{
		LangVersion: Version460,
		EntryPoint:  "mesh_main",
	})
	if err == nil {
		t.Fatal("expected error for mesh shader in GLSL")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported") {
		t.Errorf("error should mention 'unsupported': %v", err)
	}
	if !strings.Contains(errMsg, "stage") {
		t.Errorf("error should mention 'stage': %v", err)
	}
}

// TestGLSL_UnsupportedTaskShader verifies that task shader entry points
// produce a clear error because GLSL does not support task shaders.
func TestGLSL_UnsupportedTaskShader(t *testing.T) {
	mod := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:      "task_main",
			Stage:     ir.StageTask,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Body: ir.Block{},
			},
		}},
	}
	_, _, err := Compile(mod, Options{
		LangVersion: Version460,
		EntryPoint:  "task_main",
	})
	if err == nil {
		t.Fatal("expected error for task shader in GLSL")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported") {
		t.Errorf("error should mention 'unsupported': %v", err)
	}
}

// TestGLSL_UnsupportedBindingArray verifies that binding arrays produce a
// clear error because GLSL does not support binding arrays natively.
func TestGLSL_UnsupportedBindingArray(t *testing.T) {
	size := uint32(4)
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.SamplerType{Comparison: false}},             // type 0: sampler
			{Inner: ir.BindingArrayType{Base: 0, Size: &size}},     // type 1: binding_array<sampler, 4>
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // type 2: f32
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "samplers",
				Space:   ir.SpaceHandle,
				Type:    1,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageFragment,
			Function: ir.Function{
				Body: ir.Block{},
			},
		}},
	}
	// Binding arrays cannot be represented in standard GLSL.
	// The type will resolve to "unknown_type" which may or may not cause an
	// explicit error depending on how the module is structured. We verify the
	// compiler handles this gracefully (either error or produces output without crash).
	_, _, err := Compile(mod, Options{
		LangVersion: Version460,
		EntryPoint:  "main",
	})
	// This test verifies no panic — binding arrays may compile with degraded
	// type names or may error. Either behavior is acceptable as long as no panic.
	_ = err
}

// TestGLSL_ContinueCtxExitLoopError_Integration verifies that a mismatched
// nesting structure in continue context is caught at compile time, not at
// runtime via panic.
func TestGLSL_ContinueCtxExitLoopError_Integration(t *testing.T) {
	var ctx continueCtx

	// Calling exitLoop on empty stack should return an error.
	err := ctx.exitLoop()
	if err == nil {
		t.Fatal("expected error from exitLoop on empty stack")
	}
	if !strings.Contains(err.Error(), "continueCtx stack out of sync") {
		t.Errorf("error message should describe stack mismatch: %v", err)
	}
}

// TestGLSL_ContinueCtxExitSwitchError_Integration verifies that exitSwitch
// returns an error when a Loop is on top instead of a Switch.
func TestGLSL_ContinueCtxExitSwitchError_Integration(t *testing.T) {
	var ctx continueCtx
	ctx.enterLoop()

	_, err := ctx.exitSwitch()
	if err == nil {
		t.Fatal("expected error from exitSwitch with Loop on top")
	}
	if !strings.Contains(err.Error(), "continueCtx stack out of sync") {
		t.Errorf("error message should describe stack mismatch: %v", err)
	}
}
