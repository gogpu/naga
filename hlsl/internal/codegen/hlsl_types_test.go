// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// hlslTypeSize Tests — covers type size calculation
// =============================================================================

func TestHLSLTypeSize(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                         // 0: f32 -> 4
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                                          // 1: i32 -> 4
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},                                                          // 2: bool -> 4 (HLSL bool = 4 bytes)
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                   // 3: float2 -> 8
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                   // 4: float3 -> 12
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                   // 5: float4 -> 16
			{Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 6: float4x4 -> 64
		},
	}

	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name   string
		handle ir.TypeHandle
		want   uint32
	}{
		{"f32", 0, 4},
		{"i32", 1, 4},
		{"bool", 2, 1}, // IR bool Width=1 byte
		{"float2", 3, 8},
		{"float3", 4, 12},
		{"float4", 5, 16},
		{"float4x4", 6, 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.hlslTypeSize(tt.handle)
			if got != tt.want {
				t.Errorf("hlslTypeSize(%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// =============================================================================
// scalarTypeHLSL Tests — covers scalar type to HLSL string mapping
// =============================================================================

func TestScalarTypeHLSL(t *testing.T) {
	tests := []struct {
		name  string
		kind  ir.ScalarKind
		width uint8
		want  string
	}{
		{"f32", ir.ScalarFloat, 4, "float"},
		{"f16", ir.ScalarFloat, 2, "half"},
		{"f64", ir.ScalarFloat, 8, "double"},
		{"i32", ir.ScalarSint, 4, "int"},
		{"u32", ir.ScalarUint, 4, "uint"},
		{"bool", ir.ScalarBool, 1, "bool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarTypeHLSL(ir.ScalarType{Kind: tt.kind, Width: tt.width})
			if got != tt.want {
				t.Errorf("scalarTypeHLSL(%v, %d) = %q, want %q", tt.kind, tt.width, got, tt.want)
			}
		})
	}
}

// =============================================================================
// isDynamicallySized Tests — covers array dynamic size detection
// =============================================================================

func TestIsDynamicallySized(t *testing.T) {
	size4 := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                          // 0: f32
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size4}, Stride: 4}}, // 1: array<f32, 4>
			{Inner: ir.ArrayType{Base: 0, Stride: 4}},                                       // 2: array<f32> (dynamic)
		},
	}

	w := newTestWriter(module, nil, nil)

	if w.isDynamicallySized(0) {
		t.Error("f32 should not be dynamically sized")
	}
	if w.isDynamicallySized(1) {
		t.Error("array<f32, 4> should not be dynamically sized")
	}
	if !w.isDynamicallySized(2) {
		t.Error("array<f32> should be dynamically sized")
	}
}

// =============================================================================
// isScalarType / isVectorType / isMatrixType / isStructType / isArrayType Tests
// =============================================================================

func TestTypePredicates(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                         // 0: scalar
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                   // 1: vector
			{Inner: ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: matrix
			{Name: "S", Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}}},                            // 3: struct
			{Inner: ir.ArrayType{Base: 0, Stride: 4}},                                                                      // 4: array
		},
	}

	if !isScalarType(module, 0) {
		t.Error("type 0 should be scalar")
	}
	if isScalarType(module, 1) {
		t.Error("type 1 should not be scalar")
	}

	if !isVectorType(module, 1) {
		t.Error("type 1 should be vector")
	}
	if isVectorType(module, 0) {
		t.Error("type 0 should not be vector")
	}

	if !isMatrixType(module, 2) {
		t.Error("type 2 should be matrix")
	}

	if !isStructType(module, 3) {
		t.Error("type 3 should be struct")
	}

	if !isArrayType(module, 4) {
		t.Error("type 4 should be array")
	}
}

// =============================================================================
// isMatCx2 Tests — covers special matrix type detection
// =============================================================================

func TestIsMatCx2(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 0: mat3x2
			{Inner: ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: mat2x2
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: mat4x4
			{Inner: ir.MatrixType{Columns: 4, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 3: mat4x2
		},
	}

	w := newTestWriter(module, nil, nil)

	if !w.isMatCx2Type(0) {
		t.Error("mat3x2 should be MatCx2")
	}
	if !w.isMatCx2Type(1) {
		t.Error("mat2x2 should be MatCx2")
	}
	if w.isMatCx2Type(2) {
		t.Error("mat4x4 should not be MatCx2")
	}
	if !w.isMatCx2Type(3) {
		t.Error("mat4x2 should be MatCx2")
	}
}

// structsEqual is already tested in types_test.go

// =============================================================================
// float32FromBits Tests
// =============================================================================

func TestFloat32FromBits(t *testing.T) {
	// PI bits
	got := float32FromBits(0x40490fdb)
	if got < 3.14 || got > 3.15 {
		t.Errorf("expected ~3.14159, got %f", got)
	}

	// Zero
	got = float32FromBits(0)
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}

	// Negative one
	got = float32FromBits(0xBF800000)
	if got != -1.0 {
		t.Errorf("expected -1.0, got %f", got)
	}
}

// =============================================================================
// vectorTypeName Tests (method on Writer)
// =============================================================================

func TestVectorTypeNameMethod(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name   string
		scalar ir.ScalarType
		size   uint8
		want   string
	}{
		{"float2", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 2, "float2"},
		{"float3", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 3, "float3"},
		{"float4", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 4, "float4"},
		{"int4", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, 4, "int4"},
		{"uint3", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, 3, "uint3"},
		{"bool2", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, 2, "bool2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.vectorTypeName(tt.scalar, tt.size)
			if got != tt.want {
				t.Errorf("vectorTypeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// writeConstantValue Tests — covers scalar constant value emission
// =============================================================================

func TestWriteConstantValue_Scalar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},  // 1: i32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // 2: u32
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},  // 3: bool
		},
	}

	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name string
		c    ir.Constant
		want string
	}{
		{"f32_pi", ir.Constant{Type: 0, Value: ir.ScalarValue{Bits: 0x40490fdb, Kind: ir.ScalarFloat}}, "3.14159"},
		{"i32_42", ir.Constant{Type: 1, Value: ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}}, "int(42)"},
		{"u32_100", ir.Constant{Type: 2, Value: ir.ScalarValue{Bits: 100, Kind: ir.ScalarUint}}, "100u"},
		{"bool_true", ir.Constant{Type: 3, Value: ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}}, "true"},
		{"bool_false", ir.Constant{Type: 3, Value: ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeConstantValue(&tt.c)
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in output, got: %q", tt.want, got)
			}
		})
	}
}

// =============================================================================
// Integration: Compile struct with padding
// =============================================================================

func TestCompile_StructWithPadding(t *testing.T) {
	src := `
struct Padded {
    a: f32,
    @align(16) b: f32,
}

fn test_padded() -> f32 {
    var p: Padded;
    p.a = 1.0;
    p.b = 2.0;
    return p.a + p.b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct Padded",
	})
}

// =============================================================================
// Integration: Compile with arrays in structs
// =============================================================================

func TestCompile_ArrayInStruct(t *testing.T) {
	src := `
struct Data {
    values: array<f32, 4>,
}

fn test() -> f32 {
    var d: Data;
    d.values[0] = 1.0;
    return d.values[0];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct Data",
		"[4]",
	})
}

// =============================================================================
// Integration: Complex shader with many code paths
// =============================================================================

func TestCompile_ComplexShader(t *testing.T) {
	src := `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) clip_pos: vec4<f32>,
    @location(0) world_pos: vec3<f32>,
    @location(1) world_normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
}

struct Uniforms {
    model: mat4x4<f32>,
    view: mat4x4<f32>,
    proj: mat4x4<f32>,
    light_dir: vec3<f32>,
    ambient: f32,
}

@group(0) @binding(0) var<uniform> u: Uniforms;

fn transform_normal(model: mat4x4<f32>, normal: vec3<f32>) -> vec3<f32> {
    return normalize((model * vec4<f32>(normal, 0.0)).xyz);
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let world_pos = u.model * vec4<f32>(in.position, 1.0);
    out.clip_pos = u.proj * u.view * world_pos;
    out.world_pos = world_pos.xyz;
    out.world_normal = transform_normal(u.model, in.normal);
    out.uv = in.uv;
    return out;
}

@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let n = normalize(in.world_normal);
    let ndotl = max(dot(n, u.light_dir), 0.0);
    let diffuse = ndotl + u.ambient;
    let color = textureSample(tex, samp, in.uv);
    return vec4<f32>(color.rgb * diffuse, color.a);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct VertexInput",
		"struct VertexOutput",
		"struct Uniforms",
		"cbuffer",
		"Texture2D",
		"SamplerState",
		"vs_main",
		"fs_main",
		"normalize(",
		"dot(",
		"max(",
		"mul(",
		".Sample(",
		"SV_Position",
	})
}
