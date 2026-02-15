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
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},                // 1: vec2<f32>
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},                // 2: vec4<f32>
		{Name: "", Inner: ir.SamplerType{Comparison: false}},                        // 3: sampler
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}}, // 4: texture_2d<f32>
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Must have combined sampler declaration
	mustContain(t, source, "uniform sampler2D tex_texSampler;")

	// Must NOT have separate sampler or texture declarations
	mustNotContain(t, source, "sampler texSampler;")
	mustNotContain(t, source, "sampler _texSampler;") // escaped keyword form

	// Must use texture() with the combined name
	mustContain(t, source, "texture(tex_texSampler,")

	// TranslationInfo should report the pair
	if len(info.TextureSamplerPairs) == 0 {
		t.Error("Expected TextureSamplerPairs to contain at least one pair")
	}
	found := false
	for _, pair := range info.TextureSamplerPairs {
		if pair == "tex_texSampler" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected TextureSamplerPairs to contain 'tex_texSampler', got %v", info.TextureSamplerPairs)
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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Binding comes from the texture's binding (binding=1)
	mustContain(t, source, "layout(binding = 1) uniform sampler2D myTexture_mySampler;")

	// Must NOT have individual declarations
	mustNotContain(t, source, "sampler mySampler;")
	mustNotContain(t, source, "sampler2D myTexture;")
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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should have TWO combined sampler declarations
	mustContain(t, source, "uniform sampler2D albedoTex_commonSampler;")
	mustContain(t, source, "uniform sampler2D normalTex_commonSampler;")

	// Must NOT have individual declarations
	mustNotContain(t, source, "sampler commonSampler;")
	mustNotContain(t, source, "sampler _commonSampler;")

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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should use textureLod with combined name
	mustContain(t, source, "textureLod(tex_samp,")
	mustContain(t, source, "uniform sampler2D tex_samp;")
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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	mustContain(t, source, "textureLod(t_s,")
	mustContain(t, source, "0.0)")
	mustContain(t, source, "uniform sampler2D t_s;")
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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Should declare as sampler3D (not sampler2D)
	mustContain(t, source, "uniform sampler3D vol_samp;")
	mustContain(t, source, "texture(vol_samp,")
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Depth texture should produce sampler2DShadow
	mustContain(t, source, "uniform sampler2DShadow shadowMap_shadowSampler;")
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
		{Name: "", Inner: ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	mustContain(t, source, "uniform samplerCube envMap_samp;")
	mustContain(t, source, "texture(envMap_samp,")
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
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
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
		Functions: []ir.Function{
			{
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
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	opts := DefaultOptions()
	opts.TextureBindingBase = 10 // Offset binding by 10

	source, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Binding 1 + offset 10 = 11
	mustContain(t, source, "layout(binding = 11) uniform sampler2D tex_samp;")
}

// =============================================================================
// Test: Non-sampled globals (uniform buffer) still work alongside combined
// =============================================================================

func TestCompile_MixedUniformsAndTextures(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	mat4Type := ir.MatrixType{Columns: 4, Rows: 4, Scalar: f32}

	types := []ir.Type{
		{Name: "", Inner: f32}, // 0
		{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},                // 1
		{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},                // 2
		{Name: "", Inner: ir.SamplerType{Comparison: false}},                        // 3
		{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}}, // 4
		{Name: "", Inner: mat4Type},                                                 // 5
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

	module := &ir.Module{
		Types:           types,
		GlobalVariables: globals,
		Functions: []ir.Function{
			{
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
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	t.Logf("Generated GLSL:\n%s", source)

	// Uniform buffer should be declared as a UBO block (not plain struct uniform).
	// WebGPU var<uniform> maps to GLSL uniform blocks for glBindBufferRange.
	mustContain(t, source, "uniform _Uniforms_ubo {")
	mustContain(t, source, "} uniforms;")

	// Combined texture-sampler should be declared
	mustContain(t, source, "uniform sampler2D tex_samp;")

	// The sampler and texture should NOT be declared individually
	mustNotContain(t, source, "sampler samp;")
	mustNotContain(t, source, "sampler _samp;")

	// Count "uniform" keyword occurrences to verify structure
	// 3 expected: UBO block "uniform _Uniforms_ubo {", sampler "uniform sampler2D", std140 layout prefix
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
