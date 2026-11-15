package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// tn is a plain (non-combined) type name.
func tn(inner ir.TypeInner) typeName { return typeName{Inner: inner} }

func TestParseTypeNames(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	f64 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	f16 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	boolean := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}

	tests := []struct {
		name string
		want typeName
	}{
		{"bool", tn(boolean)},
		{"int", tn(i32)},
		{"uint", tn(u32)},
		{"float", tn(f32)},
		{"double", tn(f64)},
		{"float16_t", tn(f16)},
		{"sampler", tn(ir.SamplerType{Comparison: false})},
		{"samplerShadow", tn(ir.SamplerType{Comparison: true})},

		{"vec3", tn(ir.VectorType{Size: ir.Vec3, Scalar: f32})},
		{"bvec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: boolean})},
		{"ivec4", tn(ir.VectorType{Size: ir.Vec4, Scalar: i32})},
		{"uvec3", tn(ir.VectorType{Size: ir.Vec3, Scalar: u32})},
		{"dvec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: f64})},
		{"f16vec2", tn(ir.VectorType{Size: ir.Vec2, Scalar: f16})},

		{"mat3", tn(ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32})},
		{"mat2x4", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec4, Scalar: f32})},
		{"dmat2", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f64})},
		{"f16mat2x3", tn(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f16})},

		{"texture2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat})},
		{"texture2DMSArray", tn(ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, Multisampled: true, SampledKind: ir.ScalarFloat})},
		{"itexture2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint})},
		{"utextureCubeArray", tn(ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarUint})},
		{"image2D", tn(ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage, SampledKind: ir.ScalarFloat, StorageFormat: ir.StorageFormatR8Uint, StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore})},
		{"image1DArray", tn(ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassStorage, SampledKind: ir.ScalarFloat, StorageFormat: ir.StorageFormatR8Uint, StorageAccess: ir.StorageAccessLoad | ir.StorageAccessStore})},

		{"sampler2D", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D}},
		{"usampler2DArray", typeName{Combined: true, Kind: ir.ScalarUint, Dim: ir.Dim2D, Arrayed: true}},
		{"isampler3D", typeName{Combined: true, Kind: ir.ScalarSint, Dim: ir.Dim3D}},
		{"sampler1DArray", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim1D, Arrayed: true}},
		{"sampler2DMSArray", typeName{Combined: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D, Arrayed: true, Multi: true}},
		{"sampler2DShadow", typeName{Combined: true, Comparison: true, Kind: ir.ScalarFloat, Dim: ir.Dim2D}},
		{"samplerCubeArrayShadow", typeName{Combined: true, Comparison: true, Kind: ir.ScalarFloat, Dim: ir.DimCube, Arrayed: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTypeName(tt.name)
			if !ok {
				t.Fatalf("parseTypeName(%q) = not a type", tt.name)
			}
			if got != tt.want {
				t.Fatalf("parseTypeName(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
			if tok := classify(ppToken{kind: ppIdent, text: tt.name}); tok.kind != tokTypeName || tok.typeName == nil {
				t.Errorf("classify(%q) = %v", tt.name, tok.kind)
			}
		})
	}
}

func TestParseTypeNamesReject(t *testing.T) {
	for _, name := range []string{
		"void", "struct", "float32", "vec5", "vec", "mat5", "mat2x5", "dvec", "f16vec",
		"imageCube", "image2DMS", "textureBuffer", "samplerBuffer", "sampler2DRect",
		"isampler2DShadow", "sampler3DShadow", "sampler2DMSShadow", "Sampler2D", "",
	} {
		if tn, ok := parseTypeName(name); ok {
			t.Errorf("parseTypeName(%q) = %+v, want not a type", name, tn)
		}
		if tok := classify(ppToken{kind: ppIdent, text: name}); tok.kind != tokIdent && name != "void" && name != "struct" {
			t.Errorf("classify(%q) = %v, want an identifier", name, tok.kind)
		}
	}
}
