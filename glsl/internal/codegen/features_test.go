// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// featuresManager Tests
// =============================================================================

func TestFeaturesManager_RequestContains(t *testing.T) {
	fm := featuresManager{}

	if fm.contains(FeatureComputeShader) {
		t.Error("new manager should not contain any features")
	}

	fm.request(FeatureComputeShader)
	if !fm.contains(FeatureComputeShader) {
		t.Error("after request, should contain FeatureComputeShader")
	}

	// Request another
	fm.request(FeatureBufferStorage)
	if !fm.contains(FeatureComputeShader) {
		t.Error("should still contain FeatureComputeShader")
	}
	if !fm.contains(FeatureBufferStorage) {
		t.Error("should contain FeatureBufferStorage")
	}

	// Should not contain unrequested
	if fm.contains(FeatureDoubleType) {
		t.Error("should not contain FeatureDoubleType")
	}
}

func TestFeaturesManager_MultipleFlags(t *testing.T) {
	fm := featuresManager{}
	fm.request(FeatureComputeShader | FeatureBufferStorage)

	if !fm.contains(FeatureComputeShader) {
		t.Error("should contain FeatureComputeShader from combined request")
	}
	if !fm.contains(FeatureBufferStorage) {
		t.Error("should contain FeatureBufferStorage from combined request")
	}
}

// =============================================================================
// writeExtensions Tests — individual features
// =============================================================================

func TestWriteExtensions_ComputeShader(t *testing.T) {
	module := &ir.Module{}

	// Desktop: should emit GL_ARB_compute_shader
	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureComputeShader)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_compute_shader") {
		t.Errorf("desktop should emit GL_ARB_compute_shader, got:\n%s", output)
	}

	// ES: should NOT emit GL_ARB_compute_shader
	w2 := newWriter(module, &Options{LangVersion: VersionES310})
	w2.features.request(FeatureComputeShader)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_compute_shader") {
		t.Errorf("ES should NOT emit GL_ARB_compute_shader, got:\n%s", output2)
	}
}

func TestWriteExtensions_BufferStorage(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureBufferStorage)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_shader_storage_buffer_object") {
		t.Errorf("expected GL_ARB_shader_storage_buffer_object, got:\n%s", output)
	}

	// ES: should NOT emit this extension
	w2 := newWriter(module, &Options{LangVersion: VersionES310})
	w2.features.request(FeatureBufferStorage)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_shader_storage_buffer_object") {
		t.Errorf("ES should NOT emit GL_ARB_shader_storage_buffer_object, got:\n%s", output2)
	}
}

func TestWriteExtensions_DoubleType(t *testing.T) {
	module := &ir.Module{}

	// Desktop 330 (< 400): should emit extension
	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureDoubleType)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_gpu_shader_fp64") {
		t.Errorf("desktop 330 should emit GL_ARB_gpu_shader_fp64, got:\n%s", output)
	}

	// Desktop 400: core, no extension needed
	w2 := newWriter(module, &Options{LangVersion: Version400})
	w2.features.request(FeatureDoubleType)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_gpu_shader_fp64") {
		t.Errorf("desktop 400 should NOT emit GL_ARB_gpu_shader_fp64, got:\n%s", output2)
	}
}

func TestWriteExtensions_CubeTexturesArray(t *testing.T) {
	module := &ir.Module{}

	// Desktop < 400: ARB extension
	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureCubeTexturesArray)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_texture_cube_map_array") {
		t.Errorf("desktop 330 should emit GL_ARB_texture_cube_map_array, got:\n%s", output)
	}

	// Desktop >= 400: core
	w2 := newWriter(module, &Options{LangVersion: Version400})
	w2.features.request(FeatureCubeTexturesArray)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_texture_cube_map_array") {
		t.Errorf("desktop 400 should NOT emit cube map array extension, got:\n%s", output2)
	}

	// ES: EXT extension
	w3 := newWriter(module, &Options{LangVersion: VersionES310})
	w3.features.request(FeatureCubeTexturesArray)
	w3.features.writeExtensions(w3)
	output3 := w3.String()
	if !strings.Contains(output3, "GL_EXT_texture_cube_map_array") {
		t.Errorf("ES should emit GL_EXT_texture_cube_map_array, got:\n%s", output3)
	}
}

func TestWriteExtensions_MultisampledTexArrays(t *testing.T) {
	module := &ir.Module{}

	// Only ES emits this extension
	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.features.request(FeatureMultisampledTexArrays)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_OES_texture_storage_multisample_2d_array") {
		t.Errorf("ES should emit GL_OES_texture_storage_multisample_2d_array, got:\n%s", output)
	}

	// Desktop: no extension
	w2 := newWriter(module, &Options{LangVersion: Version330})
	w2.features.request(FeatureMultisampledTexArrays)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_OES_texture_storage_multisample_2d_array") {
		t.Errorf("desktop should NOT emit multisample tex array extension, got:\n%s", output2)
	}
}

func TestWriteExtensions_ImageLoadStore(t *testing.T) {
	module := &ir.Module{}

	// Desktop < 420: needs extension
	w := newWriter(module, &Options{LangVersion: Version410})
	w.features.request(FeatureImageLoadStore)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_shader_image_load_store") {
		t.Errorf("desktop 410 should emit GL_ARB_shader_image_load_store, got:\n%s", output)
	}

	// Desktop >= 420: core
	w2 := newWriter(module, &Options{LangVersion: Version420})
	w2.features.request(FeatureImageLoadStore)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_shader_image_load_store") {
		t.Errorf("desktop 420 should NOT emit image load store extension, got:\n%s", output2)
	}
}

func TestWriteExtensions_ConservativeDepth(t *testing.T) {
	module := &ir.Module{}

	// Desktop < 420
	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureConservativeDepth)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_conservative_depth") {
		t.Errorf("desktop 330 should emit GL_ARB_conservative_depth, got:\n%s", output)
	}

	// ES
	w2 := newWriter(module, &Options{LangVersion: VersionES300})
	w2.features.request(FeatureConservativeDepth)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if !strings.Contains(output2, "GL_EXT_conservative_depth") {
		t.Errorf("ES should emit GL_EXT_conservative_depth, got:\n%s", output2)
	}

	// Desktop >= 420: core
	w3 := newWriter(module, &Options{LangVersion: Version420})
	w3.features.request(FeatureConservativeDepth)
	w3.features.writeExtensions(w3)
	output3 := w3.String()
	if strings.Contains(output3, "conservative_depth") {
		t.Errorf("desktop 420 should NOT emit conservative depth extension, got:\n%s", output3)
	}
}

func TestWriteExtensions_ClipCullDistance(t *testing.T) {
	module := &ir.Module{}

	// Only ES emits GL_EXT_clip_cull_distance
	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.features.request(FeatureClipDistance)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_EXT_clip_cull_distance") {
		t.Errorf("ES should emit GL_EXT_clip_cull_distance, got:\n%s", output)
	}

	// CullDistance also triggers it
	w2 := newWriter(module, &Options{LangVersion: VersionES310})
	w2.features.request(FeatureCullDistance)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if !strings.Contains(output2, "GL_EXT_clip_cull_distance") {
		t.Errorf("ES with CullDistance should emit GL_EXT_clip_cull_distance, got:\n%s", output2)
	}
}

func TestWriteExtensions_SampleVariables(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: VersionES300})
	w.features.request(FeatureSampleVariables)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_OES_sample_variables") {
		t.Errorf("ES should emit GL_OES_sample_variables, got:\n%s", output)
	}
}

func TestWriteExtensions_MultiView(t *testing.T) {
	module := &ir.Module{}

	// ES WebGL (3.0)
	w := newWriter(module, &Options{LangVersion: Version{Major: 3, Minor: 0, ES: true}})
	w.features.request(FeatureMultiView)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_OVR_multiview2") {
		t.Errorf("WebGL should emit GL_OVR_multiview2, got:\n%s", output)
	}

	// ES non-WebGL
	w2 := newWriter(module, &Options{LangVersion: Version{Major: 3, Minor: 10, ES: true}})
	w2.features.request(FeatureMultiView)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if !strings.Contains(output2, "GL_EXT_multiview") {
		t.Errorf("ES non-WebGL should emit GL_EXT_multiview, got:\n%s", output2)
	}

	// Desktop
	w3 := newWriter(module, &Options{LangVersion: Version330})
	w3.features.request(FeatureMultiView)
	w3.features.writeExtensions(w3)
	output3 := w3.String()
	if !strings.Contains(output3, "GL_EXT_multiview") {
		t.Errorf("desktop should emit GL_EXT_multiview, got:\n%s", output3)
	}
}

func TestWriteExtensions_TextureSamples(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.features.request(FeatureTextureSamples)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_shader_texture_image_samples") {
		t.Errorf("expected GL_ARB_shader_texture_image_samples, got:\n%s", output)
	}
}

func TestWriteExtensions_TextureLevels(t *testing.T) {
	module := &ir.Module{}

	// Desktop < 430
	w := newWriter(module, &Options{LangVersion: Version420})
	w.features.request(FeatureTextureLevels)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_ARB_texture_query_levels") {
		t.Errorf("desktop 420 should emit GL_ARB_texture_query_levels, got:\n%s", output)
	}

	// Desktop >= 430: core
	w2 := newWriter(module, &Options{LangVersion: Version430})
	w2.features.request(FeatureTextureLevels)
	w2.features.writeExtensions(w2)
	output2 := w2.String()
	if strings.Contains(output2, "GL_ARB_texture_query_levels") {
		t.Errorf("desktop 430 should NOT emit texture query levels extension, got:\n%s", output2)
	}
}

func TestWriteExtensions_DualSourceBlending(t *testing.T) {
	module := &ir.Module{}

	// Only ES
	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.features.request(FeatureDualSourceBlending)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_EXT_blend_func_extended") {
		t.Errorf("ES should emit GL_EXT_blend_func_extended, got:\n%s", output)
	}
}

func TestWriteExtensions_SubgroupOperations(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: Version450})
	w.features.request(FeatureSubgroupOperations)
	w.features.writeExtensions(w)
	output := w.String()

	expected := []string{
		"GL_KHR_shader_subgroup_basic",
		"GL_KHR_shader_subgroup_vote",
		"GL_KHR_shader_subgroup_arithmetic",
		"GL_KHR_shader_subgroup_ballot",
		"GL_KHR_shader_subgroup_shuffle",
		"GL_KHR_shader_subgroup_shuffle_relative",
		"GL_KHR_shader_subgroup_quad",
	}
	for _, ext := range expected {
		if !strings.Contains(output, ext) {
			t.Errorf("expected %s in output, got:\n%s", ext, output)
		}
	}
}

func TestWriteExtensions_TextureAtomics(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.features.request(FeatureTextureAtomics)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_OES_shader_image_atomic") {
		t.Errorf("expected GL_OES_shader_image_atomic, got:\n%s", output)
	}
}

func TestWriteExtensions_ShaderBarycentrics(t *testing.T) {
	module := &ir.Module{}

	w := newWriter(module, &Options{LangVersion: Version450})
	w.features.request(FeatureShaderBarycentrics)
	w.features.writeExtensions(w)
	output := w.String()
	if !strings.Contains(output, "GL_EXT_fragment_shader_barycentric") {
		t.Errorf("expected GL_EXT_fragment_shader_barycentric, got:\n%s", output)
	}
}

// =============================================================================
// collectFeatures Tests — global variables
// =============================================================================

func TestCollectFeatures_WorkgroupGlobal(t *testing.T) {
	module := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "shared_var", Space: ir.SpaceWorkGroup, Type: 0},
		},
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageCompute, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version430})
	w.collectFeatures()

	if !w.features.contains(FeatureComputeShader) {
		t.Error("WorkGroup global should request FeatureComputeShader")
	}
}

func TestCollectFeatures_StorageGlobal(t *testing.T) {
	module := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 0},
		},
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageCompute, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version430})
	w.collectFeatures()

	if !w.features.contains(FeatureBufferStorage) {
		t.Error("Storage global should request FeatureBufferStorage")
	}
}

func TestCollectFeatures_DoubleType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureDoubleType) {
		t.Error("float64 type should request FeatureDoubleType")
	}
}

func TestCollectFeatures_CubeArrayImage(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageFragment, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.collectFeatures()

	if !w.features.contains(FeatureCubeTexturesArray) {
		t.Error("cube array image should request FeatureCubeTexturesArray")
	}
}

func TestCollectFeatures_MultisampledImage(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageFragment, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureMultisampledTextures) {
		t.Error("multisampled image should request FeatureMultisampledTextures")
	}
}

func TestCollectFeatures_MultisampledArrayImage(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageFragment, Function: ir.Function{}},
		},
	}

	w := newWriter(module, &Options{LangVersion: VersionES310})
	w.collectFeatures()

	if !w.features.contains(FeatureMultisampledTexArrays) {
		t.Error("multisampled array image should request FeatureMultisampledTexArrays")
	}
}

func TestCollectFeatures_EarlyDepthTest(t *testing.T) {
	t.Run("unchanged", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			EntryPoints: []ir.EntryPoint{
				{
					Name:  "main",
					Stage: ir.StageFragment,
					EarlyDepthTest: &ir.EarlyDepthTest{
						Conservative: ir.ConservativeDepthUnchanged,
					},
					Function: ir.Function{},
				},
			},
		}
		w := newWriter(module, &Options{LangVersion: Version330})
		w.collectFeatures()

		if !w.features.contains(FeatureImageLoadStore) {
			t.Error("EarlyDepthTest Unchanged should request FeatureImageLoadStore")
		}
	})

	t.Run("conservative", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			EntryPoints: []ir.EntryPoint{
				{
					Name:  "main",
					Stage: ir.StageFragment,
					EarlyDepthTest: &ir.EarlyDepthTest{
						Conservative: ir.ConservativeDepthGreaterEqual,
					},
					Function: ir.Function{},
				},
			},
		}
		w := newWriter(module, &Options{LangVersion: Version330})
		w.collectFeatures()

		if !w.features.contains(FeatureConservativeDepth) {
			t.Error("EarlyDepthTest GreaterEqual should request FeatureConservativeDepth")
		}
	})
}

func TestCollectFeatures_NoEntryPoint(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})
	// Should not panic
	w.collectFeatures()
}

// =============================================================================
// scanVaryingFeatures Tests
// =============================================================================

func TestScanVaryingFeatures_NoPerspective(t *testing.T) {
	interp := ir.Interpolation{Kind: ir.InterpolationLinear}
	loc := ir.Binding(ir.LocationBinding{Location: 0, Interpolation: &interp})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "v", Type: 0, Binding: &loc},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureNoPerspective) {
		t.Error("linear interpolation should request FeatureNoPerspective")
	}
}

func TestScanVaryingFeatures_SampleQualifier(t *testing.T) {
	interp := ir.Interpolation{Kind: ir.InterpolationFlat, Sampling: ir.SamplingSample}
	loc := ir.Binding(ir.LocationBinding{Location: 0, Interpolation: &interp})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "v", Type: 0, Binding: &loc},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureSampleQualifier) {
		t.Error("sample sampling should request FeatureSampleQualifier")
	}
}

func TestScanVaryingFeatures_DualSourceBlending(t *testing.T) {
	blendSrc := uint32(1)
	loc := ir.Binding(ir.LocationBinding{Location: 0, BlendSrc: &blendSrc})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Result: &ir.FunctionResult{Type: 0, Binding: &loc},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureDualSourceBlending) {
		t.Error("BlendSrc should request FeatureDualSourceBlending")
	}
}

func TestScanVaryingFeatures_BuiltinSampleIndex(t *testing.T) {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinSampleIndex})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "sample", Type: 0, Binding: &binding},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureSampleVariables) {
		t.Error("BuiltinSampleIndex should request FeatureSampleVariables")
	}
}

func TestScanVaryingFeatures_BuiltinInstanceIndex(t *testing.T) {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinInstanceIndex})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "inst", Type: 0, Binding: &binding},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()

	if !w.features.contains(FeatureInstanceIndex) {
		t.Error("BuiltinInstanceIndex should request FeatureInstanceIndex")
	}
}

func TestScanVaryingFeatures_BuiltinBarycentric(t *testing.T) {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinBarycentric})
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "bary", Type: 0, Binding: &binding},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version450})
	w.collectFeatures()

	if !w.features.contains(FeatureShaderBarycentrics) {
		t.Error("BuiltinBarycentric should request FeatureShaderBarycentrics")
	}
}

// =============================================================================
// scanStatementsForFeatures Tests
// =============================================================================

func TestScanStatementsForFeatures_ImageAtomic(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Body: []ir.Statement{
						{Kind: ir.StmtImageAtomic{Image: 0, Coordinate: 1, Value: 2, Fun: ir.AtomicAdd{}}},
					},
				},
			},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version430})
	w.collectFeatures()

	if !w.features.contains(FeatureTextureAtomics) {
		t.Error("StmtImageAtomic should request FeatureTextureAtomics")
	}
}
