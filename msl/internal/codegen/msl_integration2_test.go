package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Constant value emission (covers writeConstantValue, writeScalarValue)
// =============================================================================

func TestIntegration_ConstantScalarValues(t *testing.T) {
	tests := []struct {
		name  string
		value ir.ConstantValue
		ty    ir.TypeInner
		want  string
	}{
		{"bool_true", ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}, ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "true"},
		{"bool_false", ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}, ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "false"},
		{"i32_positive", ir.ScalarValue{Bits: 42, Kind: ir.ScalarSint}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "42"},
		{"i32_negative", ir.ScalarValue{Bits: 0xFFFFFFFFFFFFFFFE, Kind: ir.ScalarSint}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "-2"},
		{"u32", ir.ScalarValue{Bits: 100, Kind: ir.ScalarUint}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "100u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tHandle := ir.TypeHandle(0)
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: tt.ty},
				},
				Constants: []ir.Constant{
					{Name: "MY_CONST", Type: tHandle, Value: tt.value},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
			mustContainMSL(t, result, "MY_CONST")
		})
	}
}

func TestIntegration_ConstantZeroValue(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "ZERO", Type: tF32, Value: ir.ZeroConstantValue{}},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "ZERO")
	mustContainMSL(t, result, "{}")
}

func TestIntegration_ConstantComposite(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec3 := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants: []ir.Constant{
			{Name: "", Type: tF32, Value: ir.ScalarValue{Bits: 0x3F800000, Kind: ir.ScalarFloat}},
			{Name: "", Type: tF32, Value: ir.ScalarValue{Bits: 0x40000000, Kind: ir.ScalarFloat}},
			{Name: "", Type: tF32, Value: ir.ScalarValue{Bits: 0x40400000, Kind: ir.ScalarFloat}},
			{Name: "VEC", Type: tVec3, Value: ir.CompositeValue{Components: []ir.ConstantHandle{0, 1, 2}}},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "VEC")
	mustContainMSL(t, result, "metal::float3(")
}

// =============================================================================
// Test: Type emission coverage (covers writeTypeInnerName, storageFormatToMSLType)
// =============================================================================

func TestIntegration_TypeInnerName(t *testing.T) {
	tests := []struct {
		name  string
		inner ir.TypeInner
		want  string
	}{
		{"scalar_bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
		{"scalar_f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{"scalar_f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"scalar_i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"scalar_u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"vec2_f32", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float2"},
		{"mat4x4_f32", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float4x4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typeInnerToMSL(tt.inner)
			if result != tt.want {
				t.Errorf("typeInnerToMSL(%T) = %q, want %q", tt.inner, result, tt.want)
			}
		})
	}
}

func TestIntegration_TypeInnerToMSL_Unsupported(t *testing.T) {
	// Struct and array return empty string (need handle).
	result := typeInnerToMSL(ir.StructType{})
	if result != "" {
		t.Errorf("expected empty string for StructType, got %q", result)
	}
}

// =============================================================================
// Test: typeSize coverage
// =============================================================================

func TestIntegration_TypeSize(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                             // [0] 4 bytes
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},       // [1] 16 bytes
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [2] 64 bytes
			{Name: "", Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},                       // [3] 4 bytes
			{Name: "", Inner: ir.StructType{Span: 24, Members: []ir.StructMember{
				{Name: "a", Type: 0, Offset: 0},
				{Name: "b", Type: 1, Offset: 4},
			}}}, // [4] 24 bytes
		},
	}

	w := &Writer{module: module}

	tests := []struct {
		name   string
		handle ir.TypeHandle
		want   uint32
	}{
		{"scalar_f32", 0, 4},
		{"vec4_f32", 1, 16},
		{"mat4x4_f32", 2, 64},
		{"atomic_u32", 3, 4},
		{"struct", 4, 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.typeSize(tt.handle)
			if got != tt.want {
				t.Errorf("typeSize(%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

func TestIntegration_TypeSize_Array(t *testing.T) {
	size := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size}, Stride: 4}}, // 40 bytes
		},
	}
	w := &Writer{module: module}
	got := w.typeSize(1)
	if got != 40 {
		t.Errorf("typeSize(array<f32, 10>) = %d, want 40", got)
	}
}

func TestIntegration_TypeSize_DynamicArray(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{}, Stride: 4}}, // dynamic
		},
	}
	w := &Writer{module: module}
	got := w.typeSize(1)
	if got != 4 {
		t.Errorf("typeSize(array<f32>) = %d, want 4", got)
	}
}

func TestIntegration_TypeSize_OutOfRange(t *testing.T) {
	module := &ir.Module{Types: []ir.Type{}}
	w := &Writer{module: module}
	got := w.typeSize(999)
	if got != 4 {
		t.Errorf("typeSize(out of range) = %d, want 4 (default)", got)
	}
}

func TestIntegration_TypeSize_MatrixVariants(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2*2*4=16
			{Name: "", Inner: ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 3*4*4=48
			{Name: "", Inner: ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2*4*4=32
			{Name: "", Inner: ir.MatrixType{Columns: 4, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 4*2*4=32
		},
	}
	w := &Writer{module: module}

	tests := []struct {
		name   string
		handle ir.TypeHandle
		want   uint32
	}{
		{"mat2x2", 0, 16},
		{"mat3x3", 1, 48},
		{"mat2x3", 2, 32},
		{"mat4x2", 3, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.typeSize(tt.handle)
			if got != tt.want {
				t.Errorf("typeSize(%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Image type name emission
// =============================================================================

func TestIntegration_ImageTypes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"storage_rgba8unorm_write", `
@group(0) @binding(0) var img: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(1.0f));
}
`, "texture2d<float"},
		{"storage_rgba32float_write", `
@group(0) @binding(0) var img: texture_storage_2d<rgba32float, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(1.0f));
}
`, "texture2d<float"},
		{"storage_r32uint_write", `
@group(0) @binding(0) var img: texture_storage_2d<r32uint, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(1u));
}
`, "texture2d<uint"},
		{"storage_r32sint_write", `
@group(0) @binding(0) var img: texture_storage_2d<r32sint, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(1i));
}
`, "texture2d<int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, tt.src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Texture store (covers writeImageStore)
// =============================================================================

func TestIntegration_TextureStore(t *testing.T) {
	src := `
@group(0) @binding(0) var img: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(1.0f, 0.0f, 0.0f, 1.0f));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".write(")
}

// =============================================================================
// Test: Texture load with bounds checking (covers writeImageLoad)
// =============================================================================

func TestIntegration_TextureLoad(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".read(")
}

// =============================================================================
// Test: Address space name emission
// =============================================================================

func TestIntegration_AddressSpaceNames(t *testing.T) {
	tests := []struct {
		space ir.AddressSpace
		want  string
	}{
		{ir.SpaceFunction, "thread"},
		{ir.SpacePrivate, "thread"},
		{ir.SpaceWorkGroup, "threadgroup"},
		{ir.SpaceUniform, "constant"},
		{ir.SpaceStorage, "device"},
		{ir.SpaceHandle, ""},
		{ir.SpacePushConstant, "constant"},
	}

	for _, tt := range tests {
		got := addressSpaceName(tt.space)
		if got != tt.want {
			t.Errorf("addressSpaceName(%v) = %q, want %q", tt.space, got, tt.want)
		}
	}
}

// =============================================================================
// Test: Atomic type name emission
// =============================================================================

func TestIntegration_AtomicTypeNames(t *testing.T) {
	module := &ir.Module{Types: []ir.Type{}}
	w := &Writer{module: module}

	tests := []struct {
		name   string
		atomic ir.AtomicType
		want   string
	}{
		{"u32", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "metal::atomic_uint"},
		{"i32", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "metal::atomic_int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.atomicTypeName(tt.atomic)
			if got != tt.want {
				t.Errorf("atomicTypeName(%+v) = %q, want %q", tt.atomic, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Packed vector type name
// =============================================================================

func TestIntegration_PackedVectorTypeName(t *testing.T) {
	module := &ir.Module{Types: []ir.Type{}}
	w := &Writer{module: module}

	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "metal::packed_float3"},
		{"f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "metal::packed_half3"},
		{"i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "metal::packed_int3"},
		{"u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "metal::packed_uint3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.packedVectorTypeName(tt.scalar)
			if got != tt.want {
				t.Errorf("packedVectorTypeName(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: shouldPackMember coverage
// =============================================================================

func TestIntegration_ShouldPackMember(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // [0]
			{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [1] vec3
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [2] vec4
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [3] vec2
		},
	}
	w := &Writer{module: module}

	st := ir.StructType{
		Members: []ir.StructMember{
			{Name: "a", Type: 0, Offset: 0},  // scalar
			{Name: "b", Type: 1, Offset: 4},  // vec3
			{Name: "c", Type: 2, Offset: 16}, // vec4
			{Name: "d", Type: 3, Offset: 32}, // vec2
		},
		Span: 40,
	}

	tests := []struct {
		name    string
		idx     int
		wantNil bool
	}{
		{"scalar_no_pack", 0, true},
		{"vec3_pack", 1, false},
		{"vec4_no_pack", 2, true},
		{"vec2_no_pack", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.shouldPackMember(st, tt.idx)
			if tt.wantNil && got != nil {
				t.Errorf("shouldPackMember(st, %d) = %v, want nil", tt.idx, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("shouldPackMember(st, %d) = nil, want non-nil", tt.idx)
			}
		})
	}
}

// =============================================================================
// Test: More complex WGSL integration — textureLoad with explicit LOD
// =============================================================================

func TestIntegration_TextureLoadWithLod(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(tex, vec2(10i, 20i), 2i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".read(")
}

// =============================================================================
// Test: Multiple storage formats for storage textures
// =============================================================================

func TestIntegration_StorageFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"rgba8unorm", "rgba8unorm", "access::write"},
		{"rgba32float", "rgba32float", "access::write"},
		{"r32uint", "r32uint", "access::write"},
		{"r32sint", "r32sint", "access::write"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `
@group(0) @binding(0) var img: texture_storage_2d<` + tt.format + `, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(0i));
}
`
			// Adjust for int/uint types.
			if strings.Contains(tt.format, "uint") {
				src = `
@group(0) @binding(0) var img: texture_storage_2d<` + tt.format + `, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(0u));
}
`
			}
			if strings.Contains(tt.format, "sint") {
				src = `
@group(0) @binding(0) var img: texture_storage_2d<` + tt.format + `, write>;
@compute @workgroup_size(1) fn main() {
    textureStore(img, vec2(0i, 0i), vec4(0i));
}
`
			}
			code := compileWGSL(t, src)
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Texture sample with offset
// =============================================================================

func TestIntegration_TextureSampleWithOffset(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(tex, samp, uv, 0.0, vec2(1i, 1i));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
}

// =============================================================================
// Test: Texture sample bias
// =============================================================================

func TestIntegration_TextureSampleBias(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, 2.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "bias(")
}

// =============================================================================
// Test: Texture sample gradient
// =============================================================================

func TestIntegration_TextureSampleGrad(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleGrad(tex, samp, vec2(0.5f), vec2(0.1f, 0.0f), vec2(0.0f, 0.1f));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "gradient2d(")
}

// =============================================================================
// Test: Various vector length functions
// =============================================================================

func TestIntegration_VectorLengthFunctions(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { len: f32, dist: f32, norm: vec3<f32>, refl: vec3<f32>, refr: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.len = length(inp.a);
    out.dist = distance(inp.a, inp.b);
    out.norm = normalize(inp.a);
    out.refl = reflect(inp.a, inp.b);
    out.refr = refract(inp.a, inp.b, 1.5f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::length(")
	mustContainMSL(t, code, "metal::distance(")
	mustContainMSL(t, code, "metal::normalize(")
	mustContainMSL(t, code, "metal::reflect(")
	mustContainMSL(t, code, "metal::refract(")
}

// =============================================================================
// Test: Cross product
// =============================================================================

func TestIntegration_CrossProduct(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = cross(inp.a, inp.b); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::cross(")
}

// =============================================================================
// Test: Half precision (f16) literals and operations
// =============================================================================

func TestIntegration_HalfPrecisionOps(t *testing.T) {
	src := `
enable f16;
struct In { a: f16, b: f16 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f16 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = inp.a + inp.b; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "half")
}

// =============================================================================
// Test: First leading bit / first trailing bit (covers firstLeadingBitResultType)
// =============================================================================

func TestIntegration_FirstLeadingBit(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = firstLeadingBit(inp.v); }
`
	code := compileWGSL(t, src)
	// MSL uses clz for leading bit calculations.
	if !strings.Contains(code, "clz") && !strings.Contains(code, "firstLeadingBit") {
		t.Error("Expected clz or firstLeadingBit in output")
	}
}

func TestIntegration_FirstTrailingBit(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = firstTrailingBit(inp.v); }
`
	code := compileWGSL(t, src)
	if !strings.Contains(code, "ctz") && !strings.Contains(code, "firstTrailingBit") {
		t.Error("Expected ctz or firstTrailingBit in output")
	}
}

// =============================================================================
// Test: extractBits / insertBits (covers more math functions)
// =============================================================================

func TestIntegration_ExtractBits(t *testing.T) {
	src := `
struct In { v: u32, o: u32, c: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = extractBits(inp.v, inp.o, inp.c); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "extract_bits(")
}

func TestIntegration_InsertBits(t *testing.T) {
	src := `
struct In { v: u32, n: u32, o: u32, c: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = insertBits(inp.v, inp.n, inp.o, inp.c); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "insert_bits(")
}

// =============================================================================
// Test: determinant and transpose
// =============================================================================

func TestIntegration_MatrixOps(t *testing.T) {
	src := `
struct In { m: mat3x3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { det: f32, t_elem: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.det = determinant(inp.m);
    let tr = transpose(inp.m);
    out.t_elem = tr[0][0];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::determinant(")
	mustContainMSL(t, code, "metal::transpose(")
}

// =============================================================================
// Test: Nested struct access through storage buffer
// =============================================================================

func TestIntegration_DeepStructAccess(t *testing.T) {
	src := `
struct Inner { x: f32, y: f32 };
struct Middle { inner: Inner, z: f32 };
struct Outer { middle: Middle, w: f32 };
@group(0) @binding(0) var<storage, read> data: Outer;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = data.middle.inner.x + data.middle.z + data.w;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".middle")
	mustContainMSL(t, code, ".inner")
}

// =============================================================================
// Test: Array indexing through storage buffer
// =============================================================================

func TestIntegration_ArrayAccess(t *testing.T) {
	src := `
struct Data { arr: array<vec4<f32>, 16> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out.v = data.arr[gid.x];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".inner")
}

// =============================================================================
// Test: Bounds check policy — Restrict mode
// =============================================================================

func TestIntegration_BoundsCheckRestrict(t *testing.T) {
	src := `
struct Data { arr: array<f32, 16> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out.v = data.arr[gid.x];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies = BoundsCheckPolicies{
		Index:  BoundsCheckRestrict,
		Buffer: BoundsCheckRestrict,
		Image:  BoundsCheckRestrict,
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "metal::min(")
}

// =============================================================================
// Test: Unchecked bounds check policy
// =============================================================================

func TestIntegration_BoundsCheckUnchecked(t *testing.T) {
	src := `
struct Data { arr: array<f32, 16> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out.v = data.arr[gid.x];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies = BoundsCheckPolicies{
		Index:  BoundsCheckUnchecked,
		Buffer: BoundsCheckUnchecked,
		Image:  BoundsCheckUnchecked,
	}
	code := compileWGSLWithOpts(t, src, opts)
	// Unchecked means no bounds checking code emitted.
	mustNotContainMSL(t, code, "metal::min(")
}

// =============================================================================
// Test: Texture sample compare level zero
// =============================================================================

func TestIntegration_TextureSampleCompareLevel(t *testing.T) {
	src := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var shadow_sampler: sampler_comparison;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleCompareLevel(depth_tex, shadow_sampler, vec2(0.5f), 0.5f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample_compare(")
}

// =============================================================================
// Test: Unary bitwise not
// =============================================================================

func TestIntegration_BitwiseNot(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = ~inp.v; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "~")
}

// =============================================================================
// Test: Texture sample with comparison sampler depth cube
// =============================================================================

func TestIntegration_DepthTextureCube(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_depth_cube;
@group(0) @binding(1) var samp: sampler_comparison;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureSampleCompareLevel(tex, samp, vec3(0.0f, 0.0f, 1.0f), 0.5f);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "depthcube<float")
}

// =============================================================================
// Test: Texture 1D
// =============================================================================

func TestIntegration_Texture1D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_1d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = textureLoad(tex, 0i, 0i);
}
`
	code := compileWGSL(t, src)
	// MSL does not have texture1d for all platforms but naga emits it.
	if !strings.Contains(code, "texture1d") && !strings.Contains(code, "texture2d") {
		t.Error("Expected texture1d or texture2d in output")
	}
}
