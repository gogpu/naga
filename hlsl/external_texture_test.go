// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// TestExternalTexturePlaneDecomposition
// =============================================================================

func TestExternalTexturePlaneDecomposition(t *testing.T) {
	paramsTypeHandle := ir.TypeHandle(1)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassExternal}}, // 0: texture_external
			{Name: "NagaExternalTextureParams", Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "num_planes", Type: ir.TypeHandle(0)},
			}}}, // 1: params struct
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "tex",
				Type:    ir.TypeHandle(0),
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		SpecialTypes: ir.SpecialTypes{
			ExternalTextureParams: &paramsTypeHandle,
		},
	}

	opts := DefaultOptions()
	opts.ExternalTextureBindingMap = ExternalTextureBindingMap{
		ResourceBinding{Group: 0, Binding: 0}: ExternalTextureBindTarget{
			Planes: [3]BindTarget{
				{Register: 0, Space: 0},
				{Register: 1, Space: 0},
				{Register: 2, Space: 0},
			},
			Params: BindTarget{Register: 3, Space: 0},
		},
	}

	w := newWriter(module, opts)
	w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 0}] = "tex"
	w.typeNames[ir.TypeHandle(1)] = "NagaExternalTextureParams"

	// Test isExternalTexture
	if !w.isExternalTexture(ir.TypeHandle(0)) {
		t.Error("expected type 0 to be external texture")
	}

	// Test resolveExternalTextureBinding
	binding := w.resolveExternalTextureBinding(module.GlobalVariables[0].Binding)
	if binding == nil {
		t.Fatal("expected external texture binding")
	}
	if binding.Planes[1].Register != 1 {
		t.Errorf("plane 1 register = %d, want 1", binding.Planes[1].Register)
	}
	if binding.Params.Register != 3 {
		t.Errorf("params register = %d, want 3", binding.Params.Register)
	}

	// Test writeGlobalExternalTexture output
	w.writeGlobalExternalTexture(0, &module.GlobalVariables[0])
	output := w.out.String()

	// Should contain 3 plane declarations
	if !strings.Contains(output, "Texture2D<float4>") {
		t.Error("output should contain Texture2D<float4> plane declarations")
	}
	if !strings.Contains(output, "register(t0)") {
		t.Error("output should contain register(t0)")
	}
	if !strings.Contains(output, "register(t1)") {
		t.Error("output should contain register(t1)")
	}
	if !strings.Contains(output, "register(t2)") {
		t.Error("output should contain register(t2)")
	}

	// Should contain params cbuffer
	if !strings.Contains(output, "cbuffer") {
		t.Error("output should contain cbuffer declaration")
	}
	if !strings.Contains(output, "NagaExternalTextureParams") {
		t.Error("output should contain NagaExternalTextureParams type name")
	}
	if !strings.Contains(output, "register(b3)") {
		t.Error("output should contain register(b3)")
	}
}

// =============================================================================
// TestExternalTextureHelperFunctions
// =============================================================================

func TestExternalTextureHelperFunctions(t *testing.T) {
	paramsTypeHandle := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "NagaExternalTextureParams"},
		},
		SpecialTypes: ir.SpecialTypes{
			ExternalTextureParams: &paramsTypeHandle,
		},
	}

	t.Run("SampleHelper", func(t *testing.T) {
		w := newWriter(module, DefaultOptions())
		w.typeNames[ir.TypeHandle(0)] = "NagaExternalTextureParams"

		w.writeExternalTextureSampleHelper()
		output := w.out.String()

		if !strings.Contains(output, "float4 nagaTextureSampleBaseClampToEdge(") {
			t.Error("missing sample helper function signature")
		}
		if !strings.Contains(output, "Texture2D<float4> plane0") {
			t.Error("missing plane0 parameter")
		}
		if !strings.Contains(output, "NagaExternalTextureParams params") {
			t.Error("missing params parameter")
		}
		if !strings.Contains(output, "SamplerState samp") {
			t.Error("missing samp parameter")
		}
		if !strings.Contains(output, "sample_transform_0") {
			t.Error("missing sample_transform_0 decomposed member access")
		}
		if !strings.Contains(output, "yuv_conversion_matrix") {
			t.Error("missing YUV conversion code")
		}

		// Second call should be no-op (already written)
		w.out.Reset()
		w.writeExternalTextureSampleHelper()
		if w.out.Len() != 0 {
			t.Error("second call should not write anything")
		}
	})

	t.Run("LoadHelper", func(t *testing.T) {
		w := newWriter(module, DefaultOptions())
		w.typeNames[ir.TypeHandle(0)] = "NagaExternalTextureParams"

		w.writeExternalTextureLoadHelper()
		output := w.out.String()

		if !strings.Contains(output, "float4 nagaTextureLoadExternal(") {
			t.Error("missing load helper function signature")
		}
		if !strings.Contains(output, "uint2 coords)") {
			t.Error("missing coords parameter")
		}
		if !strings.Contains(output, "load_transform_0") {
			t.Error("missing load_transform_0 decomposed member access")
		}
		if !strings.Contains(output, "plane0.Load") {
			t.Error("missing Load call")
		}
	})

	t.Run("DimensionsHelper", func(t *testing.T) {
		w := newWriter(module, DefaultOptions())
		w.typeNames[ir.TypeHandle(0)] = "NagaExternalTextureParams"

		w.writeExternalTextureDimensionsHelper()
		output := w.out.String()

		if !strings.Contains(output, "uint2 NagaExternalDimensions2D(") {
			t.Error("missing dimensions helper function signature")
		}
		if !strings.Contains(output, "any(params.size)") {
			t.Error("missing size check")
		}
		if !strings.Contains(output, "plane0.GetDimensions") {
			t.Error("missing GetDimensions call")
		}
	})
}

// =============================================================================
// TestExternalTextureBindTarget
// =============================================================================

func TestExternalTextureBindTarget(t *testing.T) {
	target := ExternalTextureBindTarget{
		Planes: [3]BindTarget{
			{Register: 0, Space: 0},
			{Register: 1, Space: 0},
			{Register: 2, Space: 0},
		},
		Params: BindTarget{Register: 3, Space: 0},
	}

	if target.Planes[0].Register != 0 {
		t.Errorf("plane 0 register = %d, want 0", target.Planes[0].Register)
	}
	if target.Planes[2].Register != 2 {
		t.Errorf("plane 2 register = %d, want 2", target.Planes[2].Register)
	}
	if target.Params.Register != 3 {
		t.Errorf("params register = %d, want 3", target.Params.Register)
	}

	// Test ExternalTextureBindingMap
	bindMap := ExternalTextureBindingMap{
		ResourceBinding{Group: 0, Binding: 0}: target,
	}
	if len(bindMap) != 1 {
		t.Errorf("binding map size = %d, want 1", len(bindMap))
	}
	retrieved, ok := bindMap[ResourceBinding{Group: 0, Binding: 0}]
	if !ok {
		t.Error("binding not found in map")
	}
	if retrieved.Params.Register != 3 {
		t.Errorf("retrieved params register = %d, want 3", retrieved.Params.Register)
	}
}
