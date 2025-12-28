// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestScalarToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		scalar   ir.ScalarType
		expected string
	}{
		// Bool
		{"bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},

		// Signed integers
		{"int_1byte", ir.ScalarType{Kind: ir.ScalarSint, Width: 1}, "int"},
		{"int_2bytes", ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "int"},
		{"int_4bytes", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"int64_t", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "int64_t"},

		// Unsigned integers
		{"uint_1byte", ir.ScalarType{Kind: ir.ScalarUint, Width: 1}, "uint"},
		{"uint_2bytes", ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, "uint"},
		{"uint_4bytes", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"uint64_t", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "uint64_t"},

		// Floats
		{"half", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{"float", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"double", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScalarToHLSL(tt.scalar)
			if got != tt.expected {
				t.Errorf("ScalarToHLSL(%+v) = %q, want %q", tt.scalar, got, tt.expected)
			}
		})
	}
}

func TestVectorToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		vector   ir.VectorType
		expected string
	}{
		{"float2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float2"},
		{"float3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3"},
		{"float4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4"},
		{"int2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int2"},
		{"int3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int3"},
		{"int4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int4"},
		{"uint2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint2"},
		{"uint3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint3"},
		{"uint4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint4"},
		{"bool2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bool2"},
		{"half4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half4"},
		{"double3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "double3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VectorToHLSL(tt.vector)
			if got != tt.expected {
				t.Errorf("VectorToHLSL(%+v) = %q, want %q", tt.vector, got, tt.expected)
			}
		})
	}
}

func TestMatrixToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		matrix   ir.MatrixType
		expected string
	}{
		{"float2x2", ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float2x2"},
		{"float3x3", ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3x3"},
		{"float4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4x4"},
		{"float3x4", ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float3x4"},
		{"float4x3", ir.MatrixType{Columns: 4, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "float4x3"},
		{"half4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "half4x4"},
		{"double4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "double4x4"},
		{"int4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int4x4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatrixToHLSL(tt.matrix)
			if got != tt.expected {
				t.Errorf("MatrixToHLSL(%+v) = %q, want %q", tt.matrix, got, tt.expected)
			}
		})
	}
}

func TestScalarCast(t *testing.T) {
	tests := []struct {
		name     string
		kind     ir.ScalarKind
		expected string
	}{
		{"float", ir.ScalarFloat, "asfloat"},
		{"sint", ir.ScalarSint, "asint"},
		{"uint", ir.ScalarUint, "asuint"},
		{"bool", ir.ScalarBool, "asfloat"}, // Falls back to asfloat
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScalarCast(tt.kind)
			if got != tt.expected {
				t.Errorf("ScalarCast(%v) = %q, want %q", tt.kind, got, tt.expected)
			}
		})
	}
}

func TestBuiltInToSemantic(t *testing.T) {
	tests := []struct {
		name     string
		builtin  ir.BuiltinValue
		expected string
	}{
		// Vertex shader
		{"position", ir.BuiltinPosition, "SV_Position"},
		{"vertex_index", ir.BuiltinVertexIndex, "SV_VertexID"},
		{"instance_index", ir.BuiltinInstanceIndex, "SV_InstanceID"},
		// Fragment shader
		{"front_facing", ir.BuiltinFrontFacing, "SV_IsFrontFace"},
		{"frag_depth", ir.BuiltinFragDepth, "SV_Depth"},
		{"sample_index", ir.BuiltinSampleIndex, "SV_SampleIndex"},
		{"sample_mask", ir.BuiltinSampleMask, "SV_Coverage"},
		// Compute shader
		{"global_invocation_id", ir.BuiltinGlobalInvocationID, "SV_DispatchThreadID"},
		{"local_invocation_id", ir.BuiltinLocalInvocationID, "SV_GroupThreadID"},
		{"local_invocation_index", ir.BuiltinLocalInvocationIndex, "SV_GroupIndex"},
		{"workgroup_id", ir.BuiltinWorkGroupID, "SV_GroupID"},
		{"num_workgroups", ir.BuiltinNumWorkGroups, "SV_GroupID"}, // Placeholder
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuiltInToSemantic(tt.builtin)
			if got != tt.expected {
				t.Errorf("BuiltInToSemantic(%v) = %q, want %q", tt.builtin, got, tt.expected)
			}
		})
	}
}

func TestInterpolationToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		kind     ir.InterpolationKind
		expected string
	}{
		{"flat", ir.InterpolationFlat, "nointerpolation"},
		{"linear", ir.InterpolationLinear, "noperspective"},
		{"perspective", ir.InterpolationPerspective, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InterpolationToHLSL(tt.kind)
			if got != tt.expected {
				t.Errorf("InterpolationToHLSL(%v) = %q, want %q", tt.kind, got, tt.expected)
			}
		})
	}
}

func TestSamplingToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		sampling ir.InterpolationSampling
		expected string
	}{
		{"center", ir.SamplingCenter, ""},
		{"centroid", ir.SamplingCentroid, "centroid"},
		{"sample", ir.SamplingSample, "sample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SamplingToHLSL(tt.sampling)
			if got != tt.expected {
				t.Errorf("SamplingToHLSL(%v) = %q, want %q", tt.sampling, got, tt.expected)
			}
		})
	}
}

func TestAtomicOpToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		expected string
	}{
		{"add", "add", "Add"},
		{"sub", "sub", "Add"},
		{"and", "and", "And"},
		{"or", "or", "Or"},
		{"xor", "xor", "Xor"},
		{"min", "min", "Min"},
		{"max", "max", "Max"},
		{"exchange", "exchange", "Exchange"},
		{"compare_exchange", "compare_exchange", "CompareExchange"},
		{"unknown", "unknown", "Exchange"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AtomicOpToHLSL(tt.op)
			if got != tt.expected {
				t.Errorf("AtomicOpToHLSL(%q) = %q, want %q", tt.op, got, tt.expected)
			}
		})
	}
}

func TestAddressSpaceToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		space    ir.AddressSpace
		expected string
	}{
		{"workgroup", ir.SpaceWorkGroup, "groupshared"},
		{"uniform", ir.SpaceUniform, "uniform"},
		{"storage", ir.SpaceStorage, "globallycoherent"},
		{"function", ir.SpaceFunction, ""},
		{"private", ir.SpacePrivate, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddressSpaceToHLSL(tt.space)
			if got != tt.expected {
				t.Errorf("AddressSpaceToHLSL(%v) = %q, want %q", tt.space, got, tt.expected)
			}
		})
	}
}

func TestImageDimToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		dim      ir.ImageDimension
		arrayed  bool
		expected string
	}{
		{"1d", ir.Dim1D, false, "1D"},
		{"1d_array", ir.Dim1D, true, "1DArray"},
		{"2d", ir.Dim2D, false, "2D"},
		{"2d_array", ir.Dim2D, true, "2DArray"},
		{"3d", ir.Dim3D, false, "3D"},
		{"3d_array", ir.Dim3D, true, "3D"}, // 3D can't be array
		{"cube", ir.DimCube, false, "Cube"},
		{"cube_array", ir.DimCube, true, "CubeArray"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageDimToHLSL(tt.dim, tt.arrayed)
			if got != tt.expected {
				t.Errorf("ImageDimToHLSL(%v, %v) = %q, want %q", tt.dim, tt.arrayed, got, tt.expected)
			}
		})
	}
}

func TestImageClassToHLSL(t *testing.T) {
	tests := []struct {
		name      string
		class     ir.ImageClass
		readWrite bool
		expected  string
	}{
		{"sampled", ir.ImageClassSampled, false, "Texture"},
		{"depth", ir.ImageClassDepth, false, "Texture"},
		{"storage_read", ir.ImageClassStorage, false, "Texture"},
		{"storage_rw", ir.ImageClassStorage, true, "RWTexture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageClassToHLSL(tt.class, tt.readWrite)
			if got != tt.expected {
				t.Errorf("ImageClassToHLSL(%v, %v) = %q, want %q", tt.class, tt.readWrite, got, tt.expected)
			}
		})
	}
}

func TestImageToHLSL(t *testing.T) {
	tests := []struct {
		name      string
		img       ir.ImageType
		readWrite bool
		expected  string
	}{
		{
			"Texture2D",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled},
			false,
			"Texture2D",
		},
		{
			"Texture2DArray",
			ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled},
			false,
			"Texture2DArray",
		},
		{
			"Texture3D",
			ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled},
			false,
			"Texture3D",
		},
		{
			"TextureCube",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled},
			false,
			"TextureCube",
		},
		{
			"RWTexture2D",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage},
			true,
			"RWTexture2D",
		},
		{
			"Texture2DMS",
			ir.ImageType{Dim: ir.Dim2D, Multisampled: true, Class: ir.ImageClassSampled},
			false,
			"Texture2DMS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageToHLSL(tt.img, tt.readWrite)
			if got != tt.expected {
				t.Errorf("ImageToHLSL(%+v, %v) = %q, want %q", tt.img, tt.readWrite, got, tt.expected)
			}
		})
	}
}

func TestSamplerToHLSL(t *testing.T) {
	tests := []struct {
		name       string
		comparison bool
		expected   string
	}{
		{"regular", false, "SamplerState"},
		{"comparison", true, "SamplerComparisonState"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SamplerToHLSL(tt.comparison)
			if got != tt.expected {
				t.Errorf("SamplerToHLSL(%v) = %q, want %q", tt.comparison, got, tt.expected)
			}
		})
	}
}

func TestShaderStageToHLSL(t *testing.T) {
	tests := []struct {
		name     string
		stage    ir.ShaderStage
		expected string
	}{
		{"vertex", ir.StageVertex, "vs"},
		{"fragment", ir.StageFragment, "ps"},
		{"compute", ir.StageCompute, "cs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShaderStageToHLSL(tt.stage)
			if got != tt.expected {
				t.Errorf("ShaderStageToHLSL(%v) = %q, want %q", tt.stage, got, tt.expected)
			}
		})
	}
}

func TestShaderProfile(t *testing.T) {
	tests := []struct {
		name     string
		stage    ir.ShaderStage
		major    uint8
		minor    uint8
		expected string
	}{
		{"vs_5_0", ir.StageVertex, 5, 0, "vs_5_0"},
		{"vs_5_1", ir.StageVertex, 5, 1, "vs_5_1"},
		{"ps_6_0", ir.StageFragment, 6, 0, "ps_6_0"},
		{"cs_6_6", ir.StageCompute, 6, 6, "cs_6_6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShaderProfile(tt.stage, tt.major, tt.minor)
			if got != tt.expected {
				t.Errorf("ShaderProfile(%v, %d, %d) = %q, want %q", tt.stage, tt.major, tt.minor, got, tt.expected)
			}
		})
	}
}

// Benchmarks

func BenchmarkScalarToHLSL(b *testing.B) {
	scalars := []ir.ScalarType{
		{Kind: ir.ScalarFloat, Width: 4},
		{Kind: ir.ScalarSint, Width: 4},
		{Kind: ir.ScalarUint, Width: 4},
		{Kind: ir.ScalarBool, Width: 1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range scalars {
			_ = ScalarToHLSL(s)
		}
	}
}

func BenchmarkBuiltInToSemantic(b *testing.B) {
	builtins := []ir.BuiltinValue{
		ir.BuiltinPosition,
		ir.BuiltinVertexIndex,
		ir.BuiltinFragDepth,
		ir.BuiltinGlobalInvocationID,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, bi := range builtins {
			_ = BuiltInToSemantic(bi)
		}
	}
}

func BenchmarkVectorToHLSL(b *testing.B) {
	vectors := []ir.VectorType{
		{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range vectors {
			_ = VectorToHLSL(v)
		}
	}
}

func BenchmarkMatrixToHLSL(b *testing.B) {
	matrices := []ir.MatrixType{
		{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range matrices {
			_ = MatrixToHLSL(m)
		}
	}
}
