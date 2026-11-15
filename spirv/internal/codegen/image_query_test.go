// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// buildImageQueryModule builds a fragment module with one image-typed global
// and one image query over it. The module is hand-built rather than written in
// GLSL or WGSL because no text frontend reaches these shapes: WGSL has no
// arrayed 1D or arrayed multisampled texture, and Rust naga's GLSL frontend
// refuses the 1D-array sampler declaration outright.
func buildImageQueryModule(image ir.ImageType, query ir.ImageQuery) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Inner: image}},
		GlobalVariables: []ir.GlobalVariable{{
			Name:    "image",
			Space:   ir.SpaceHandle,
			Type:    0,
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
		}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageFragment,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprImageQuery{Image: 0, Query: query}},
				},
				Body: []ir.Statement{{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}}},
			},
		}},
	}
}

// compileImageQueryModule compiles a hand-built module and returns its SPIR-V.
func compileImageQueryModule(t *testing.T, module *ir.Module) []byte {
	t.Helper()
	binary, err := NewBackend(Options{}).Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(binary) == 0 {
		t.Fatal("compile produced no binary")
	}
	return binary
}

// TestImageQuerySizeArrayed pins the arrayed size query's result extraction for
// every dimension. SPIR-V's OpImageQuerySize/OpImageQuerySizeLod return the
// spatial size plus one component per array layer, while the IR result keeps
// only the spatial size (Rust's image_query_size_result_type). The excess
// components are dropped with OpVectorShuffle, except for a 1D arrayed image,
// whose IR result is a scalar: OpVectorShuffle's Result Type must be a vector,
// so the scalar comes from OpCompositeExtract component 0 instead
// (spirv-val: "The Result Type of OpVectorShuffle must be a vector type").
func TestImageQuerySizeArrayed(t *testing.T) {
	cases := []struct {
		name      string
		image     ir.ImageType
		wantOp    spvenums.Op
		wantEx    bool // an OpCompositeExtract carries the scalar result
		noShuffle bool
	}{
		{
			name:  "1d-arrayed-scalar-result",
			image: ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			// A sampled image with one sample takes a level of detail.
			wantOp:    spvenums.OpImageQuerySizeLod,
			wantEx:    true,
			noShuffle: true,
		},
		{
			name:   "2d-arrayed",
			image:  ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			wantOp: spvenums.OpImageQuerySizeLod,
		},
		{
			name:   "3d-arrayed",
			image:  ir.ImageType{Dim: ir.Dim3D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			wantOp: spvenums.OpImageQuerySizeLod,
		},
		{
			name:   "cube-arrayed",
			image:  ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			wantOp: spvenums.OpImageQuerySizeLod,
		},
		{
			// A multisampled image has one level, so its size query carries no
			// level (SPIR-V: OpImageQuerySize is the query for MS = 1).
			name:   "2d-multisampled-arrayed",
			image:  ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			wantOp: spvenums.OpImageQuerySize,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binary := compileImageQueryModule(t, buildImageQueryModule(tc.image, ir.ImageQuerySize{}))
			validateWithVulkanSDK(t, binary)

			instrs := decodeSPIRVInstructions(binary)
			if !hasOpcodeInInstrs(instrs, tc.wantOp) {
				t.Errorf("no %v for the arrayed size query", tc.wantOp)
			}
			if hasOpcodeInInstrs(instrs, spvenums.OpCompositeExtract) != tc.wantEx {
				t.Errorf("OpCompositeExtract present = %v, want %v",
					!tc.wantEx, tc.wantEx)
			}
			if tc.noShuffle && hasOpcodeInInstrs(instrs, spvenums.OpVectorShuffle) {
				t.Error("a scalar result is shuffled as if it were a vector")
			}
		})
	}
}

// TestImageQueryNumLayers checks how the layer count is read off the extended
// size. A level-carrying query (OpImageQuerySizeLod) is only valid for a
// sampled image without multiple samples; a multisampled image and a storage
// image (Sampled = 0) take OpImageQuerySize, which returns the same vector with
// the layer count in the last component (spirv-val: "Image 'MS' must be 0").
func TestImageQueryNumLayers(t *testing.T) {
	cases := []struct {
		name      string
		image     ir.ImageType
		withLevel bool
	}{
		{
			name:      "1d-arrayed-sampled",
			image:     ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			withLevel: true,
		},
		{
			name:      "2d-arrayed-sampled",
			image:     ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			withLevel: true,
		},
		{
			name:      "cube-arrayed-sampled",
			image:     ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			withLevel: true,
		},
		{
			name:      "2d-arrayed-multisampled",
			image:     ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Multisampled: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			withLevel: false,
		},
		{
			name:      "2d-arrayed-multisampled-depth",
			image:     ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Multisampled: true, Class: ir.ImageClassDepth},
			withLevel: false,
		},
		{
			name: "2d-arrayed-storage",
			image: ir.ImageType{
				Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Uint, StorageAccess: ir.StorageAccessLoad,
			},
			withLevel: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binary := compileImageQueryModule(t, buildImageQueryModule(tc.image, ir.ImageQueryNumLayers{}))
			validateWithVulkanSDK(t, binary)

			instrs := decodeSPIRVInstructions(binary)
			if got := hasOpcodeInInstrs(instrs, spvenums.OpImageQuerySizeLod); got != tc.withLevel {
				t.Errorf("OpImageQuerySizeLod present = %v, want %v", got, tc.withLevel)
			}
			if got := hasOpcodeInInstrs(instrs, spvenums.OpImageQuerySize); got == tc.withLevel {
				t.Errorf("OpImageQuerySize present = %v, want %v", got, !tc.withLevel)
			}
			// The layer count is the last component of the extended size.
			if !hasOpcodeInInstrs(instrs, spvenums.OpCompositeExtract) {
				t.Error("the layer count is not extracted from the extended size")
			}
		})
	}
}
