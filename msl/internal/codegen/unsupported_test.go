// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// MSL unsupported feature tests — hand-crafted IR modules
// =============================================================================

// TestMSL_UnsupportedMeshShader verifies that mesh shader entry points
// produce a clear error because MSL does not support mesh shaders.
func TestMSL_UnsupportedMeshShader(t *testing.T) {
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
	_, _, err := Compile(mod, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for mesh shader in MSL")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported") {
		t.Errorf("error should mention 'unsupported': %v", err)
	}
	if !strings.Contains(errMsg, "stage") {
		t.Errorf("error should mention 'stage': %v", err)
	}
	if !strings.Contains(errMsg, "mesh_main") {
		t.Errorf("error should mention entry point name 'mesh_main': %v", err)
	}
}

// TestMSL_UnsupportedTaskShader verifies that task shader entry points
// produce a clear error because MSL does not support task shaders.
func TestMSL_UnsupportedTaskShader(t *testing.T) {
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
	_, _, err := Compile(mod, DefaultOptions())
	if err == nil {
		t.Fatal("expected error for task shader in MSL")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported") {
		t.Errorf("error should mention 'unsupported': %v", err)
	}
}
