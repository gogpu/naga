// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: "sampler" is in the GLSL keywords list
// =============================================================================

func TestSamplerIsKeyword(t *testing.T) {
	if !isKeyword("sampler") {
		t.Error(`"sampler" should be a GLSL keyword`)
	}
}

// =============================================================================
// Test: Basic texture sampling with separate texture and sampler
// =============================================================================

func TestCompile_TextureSamplerCombined(t *testing.T) {
	// Build an IR module that models this WGSL:
	//
	//   @group(1) @binding(0) var texSampler: sampler;
	//   @group(1) @binding(1) var tex: texture_2d<f32>;
	//
	//   @fragment
	//   fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	//       return textureSample(tex, texSampler, uv);
	//   }

	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32}, // 0: f32
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},                                             // 1: vec2<f32>
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},                                             // 2: vec4<f32>
		{Name: "", Inner: ir.SamplerType{Comparison: false}},                                                     // 3: sampler
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}}, // 4: texture_2d<f32>
	}

	globals := []ir.GlobalVariable{
		{
			Name:    "texSampler",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 1, Binding: 0},
			Type:    3, // sampler
		},
		{
			Name:    "tex",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 1, Binding: 1},
			Type:    4, // texture_2d
		},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)}, // vec2<f32>
				},
				Result: &ir.FunctionResult{
					Type:    2, // vec4<f32>
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},  // [0] = uv
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // [1] = tex
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // [2] = texSampler
					{Kind: ir.ExprImageSample{ // [3] = textureSample(tex, texSampler, uv)
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil, // SampleLevelAuto
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Must have combined sampler declaration with _group naming
	mustContain(t, source, "uniform sampler2D _group_1_binding_1_fs;")

	// Must NOT have separate sampler or texture declarations
	mustNotContain(t, source, "sampler texSampler;")

	// Must use texture() with the combined name
	mustContain(t, source, "texture(_group_1_binding_1_fs,")

	// TranslationInfo should report the pair
	if len(info.TextureSamplerPairs) == 0 {
		t.Error("Expected TextureSamplerPairs to contain at least one pair")
	}
}

// =============================================================================
// Test: Texture sampling with binding layout
// =============================================================================

func TestCompile_TextureSamplerWithBinding(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{
			Name:    "mySampler",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			Type:    3,
		},
		{
			Name:    "myTexture",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
			Type:    4,
		},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},  // [0] = uv
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // [1] = myTexture
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // [2] = mySampler
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil,
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Combined sampler uses _group naming
	mustContain(t, source, "uniform sampler2D _group_0_binding_1_fs;")

	// Must NOT have individual declarations
	mustNotContain(t, source, "sampler mySampler;")
}

// =============================================================================
// Test: Same sampler used with multiple textures
// =============================================================================

func TestCompile_SameSamplerMultipleTextures(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{
			Name:    "commonSampler",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			Type:    3,
		},
		{
			Name:    "albedoTex",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
			Type:    4,
		},
		{
			Name:    "normalTex",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 2},
			Type:    4,
		},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},  // [0] = uv
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // [1] = albedoTex
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // [2] = commonSampler
					{Kind: ir.ExprImageSample{ // [3] = textureSample(albedoTex, commonSampler, uv)
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil,
					}},
					{Kind: ir.ExprGlobalVariable{Variable: 2}}, // [4] = normalTex
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // [5] = commonSampler
					{Kind: ir.ExprImageSample{ // [6] = textureSample(normalTex, commonSampler, uv)
						Image:      4,
						Sampler:    5,
						Coordinate: 0,
						Level:      nil,
					}},
					// Add albedo + normal
					{Kind: ir.ExprBinary{
						Op:    ir.BinaryAdd,
						Left:  3,
						Right: 6,
					}}, // [7] = albedo + normal
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 8}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(7)}},
				},
			}},
		},
	}

	source, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should have TWO combined sampler declarations with _group naming
	mustContain(t, source, "uniform sampler2D _group_0_binding_1_fs;")
	mustContain(t, source, "uniform sampler2D _group_0_binding_2_fs;")

	// Must NOT have individual sampler declarations
	mustNotContain(t, source, "sampler commonSampler;")

	// TranslationInfo should report both pairs
	if len(info.TextureSamplerPairs) != 2 {
		t.Errorf("Expected 2 TextureSamplerPairs, got %d: %v", len(info.TextureSamplerPairs), info.TextureSamplerPairs)
	}
}

// =============================================================================
// Test: textureLod (explicit LOD)
// =============================================================================

func TestCompile_TextureSampleLod(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{
			Name:    "samp",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			Type:    3,
		},
		{
			Name:    "tex",
			Space:   ir.SpaceHandle,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
			Type:    4,
		},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] = uv
					{Kind: ir.ExprGlobalVariable{Variable: 1}},    // [1] = tex
					{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [2] = samp
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}}, // [3] = 0.0 (lod level)
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      ir.SampleLevelExact{Level: 3},
					}}, // [4]
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(4)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should use textureLod with _group naming (binding present)
	mustContain(t, source, "textureLod(_group_0_binding_1_fs,")
	mustContain(t, source, "uniform sampler2D _group_0_binding_1_fs;")
}

// =============================================================================
// Test: textureLod with SampleLevelZero
// =============================================================================

func TestCompile_TextureSampleLevelZero(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{
			Name:  "s",
			Space: ir.SpaceHandle,
			Type:  3,
		},
		{
			Name:  "t",
			Space: ir.SpaceHandle,
			Type:  4,
		},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      ir.SampleLevelZero{},
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Globals use their original names (IDs assigned at write time)
	mustContain(t, source, "textureLod(t,")
	mustContain(t, source, "0.0)")
	mustContain(t, source, "uniform sampler2D t;")
}

// =============================================================================
// Test: 3D texture sampling
// =============================================================================

func TestCompile_Texture3DSampler(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{Name: "samp", Space: ir.SpaceHandle, Type: 3},
		{Name: "vol", Space: ir.SpaceHandle, Type: 4},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uvw", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil,
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should declare as sampler3D — globals use their original names
	mustContain(t, source, "uniform sampler3D vol;")
	mustContain(t, source, "texture(vol,")
}

// =============================================================================
// Test: Depth texture (shadow sampler)
// =============================================================================

func TestCompile_DepthTextureSampler(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: true}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth}},
	}

	globals := []ir.GlobalVariable{
		{Name: "shadowSampler", Space: ir.SpaceHandle, Type: 3},
		{Name: "shadowMap", Space: ir.SpaceHandle, Type: 4},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil,
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Depth texture should produce sampler2DShadow — globals use their original names
	mustContain(t, source, "uniform sampler2DShadow shadowMap;")
}

// =============================================================================
// Test: Cube texture sampling
// =============================================================================

func TestCompile_CubeTextureSampler(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{Name: "samp", Space: ir.SpaceHandle, Type: 3},
		{Name: "envMap", Space: ir.SpaceHandle, Type: 4},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "dir", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageSample{
						Image:      1,
						Sampler:    2,
						Coordinate: 0,
						Level:      nil,
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Globals use their original names (IDs assigned at write time)
	mustContain(t, source, "uniform samplerCube envMap;")
	mustContain(t, source, "texture(envMap,")
}

// =============================================================================
// Test: TextureBindingBase offset applies to combined declarations
// =============================================================================

func TestCompile_TextureBindingBaseOffset(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		{Name: "", Inner: ir.SamplerType{Comparison: false}},
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
	}

	globals := []ir.GlobalVariable{
		{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 3},
		{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 4},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: 1, Binding: locBinding(0)},
				},
				Result: &ir.FunctionResult{
					Type:    2,
					Binding: &outBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageSample{Image: 1, Sampler: 2, Coordinate: 0}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			}},
		},
	}

	opts := DefaultOptions()
	opts.TextureBindingBase = 10 // Offset binding by 10

	source, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Binding uses _group naming
	mustContain(t, source, "uniform sampler2D _group_0_binding_1_fs;")
}

// =============================================================================
// Test: Non-sampled globals (uniform buffer) still work alongside combined
// =============================================================================

func TestCompile_MixedUniformsAndTextures(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	mat4Type := ir.MatrixType{Columns: 4, Rows: 4, Scalar: f32}

	types := []ir.Type{
		{Name: "", Inner: f32}, // 0
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},                                             // 1
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},                                             // 2
		{Name: "", Inner: ir.SamplerType{Comparison: false}},                                                     // 3
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}}, // 4
		{Name: "", Inner: mat4Type},                                                                              // 5
		{Name: "Uniforms", Inner: ir.StructType{ // 6
			Members: []ir.StructMember{
				{Name: "mvp", Type: 5, Offset: 0},
			},
			Span: 64,
		}},
	}

	globals := []ir.GlobalVariable{
		{Name: "uniforms", Space: ir.SpaceUniform, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 6},
		{Name: "samp", Space: ir.SpaceHandle, Type: 3},
		{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}, Type: 4},
	}

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	epFunc := ir.Function{
		Name: "fs_main",
		Arguments: []ir.FunctionArgument{
			{Name: "uv", Type: 1, Binding: locBinding(0)},
		},
		Result: &ir.FunctionResult{
			Type:    2,
			Binding: &outBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                       // [0] = uv
			{Kind: ir.ExprGlobalVariable{Variable: 2}},                      // [1] = tex
			{Kind: ir.ExprGlobalVariable{Variable: 1}},                      // [2] = samp
			{Kind: ir.ExprImageSample{Image: 1, Sampler: 2, Coordinate: 0}}, // [3]
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [4] = uniforms
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
		},
		NamedExpressions: map[ir.ExpressionHandle]string{0: "uv"},
	}

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: epFunc},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Uniform buffer should use Rust naga block naming convention
	mustContain(t, source, "uniform Uniforms_block_")
	mustContain(t, source, "_group_0_binding_0_fs")

	// Combined texture-sampler should use _group naming
	mustContain(t, source, "uniform sampler2D")
	mustContain(t, source, "_group_0_binding_2_fs")

	// The sampler should NOT be declared individually (GLSL has no standalone samplers)
	mustNotContain(t, source, "sampler _group_0_binding")

	// Should have uniform declarations
	uniformCount := strings.Count(source, "uniform ")
	if uniformCount < 2 {
		t.Errorf("Expected at least 2 uniform declarations, got %d", uniformCount)
	}
}

// =============================================================================
// Test: No texture-sampler pairs (regression)
// =============================================================================

func TestCompile_NoTextureSamplerPairs(t *testing.T) {
	// This module has no textures/samplers at all — verify it still compiles.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !strings.HasPrefix(source, "#version") {
		t.Error("Expected version directive in output")
	}
}
