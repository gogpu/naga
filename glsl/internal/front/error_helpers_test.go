package front

// Contracts of the diagnostic renderers (front/glsl/error.go): the Rust Debug
// spellings the error texts use for types, image classes, storage formats and
// accesses, and the expected-token fallback.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestTypeInnerDebugSpellings is the Debug rendering of a TypeName token's
// type: Rust's TypeInner spelling for the shapes it has, and the combined
// sampler's two-part description for the shape Rust lacks.
func TestTypeInnerDebugSpellings(t *testing.T) {
	tests := []struct {
		name string
		tn   *typeName
		want string
	}{
		{
			name: "scalar",
			tn:   &typeName{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			want: "Scalar(Scalar { kind: Float, width: 4 })",
		},
		{
			name: "vector",
			tn:   &typeName{Inner: ir.VectorType{Size: ir.Vec3, Scalar: i32Scalar}},
			want: "Vector { size: Tri, scalar: Scalar { kind: Sint, width: 4 } }",
		},
		{
			name: "matrix",
			tn:   &typeName{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar}},
			want: "Matrix { columns: Bi, rows: Bi, scalar: Scalar { kind: Float, width: 4 } }",
		},
		{
			name: "sampled image",
			tn: &typeName{Inner: ir.ImageType{
				Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled,
				SampledKind: ir.ScalarUint, Multisampled: false,
			}},
			want: "Image { dim: D2, arrayed: true, class: Sampled { kind: Uint, multi: false } }",
		},
		{
			name: "sampler",
			tn:   &typeName{Inner: ir.SamplerType{Comparison: true}},
			want: "Sampler { comparison: true }",
		},
		{
			name: "combined sampler",
			tn: &typeName{
				Combined: true, Dim: ir.Dim2D, Arrayed: true,
				Kind: ir.ScalarFloat, Multi: false,
			},
			want: "Image { dim: D2, arrayed: true, class: Sampled { kind: Float, multi: false } } and Sampler { comparison: false }",
		},
		{
			name: "combined shadow sampler",
			tn: &typeName{
				Combined: true, Comparison: true, Dim: ir.DimCube,
				Kind: ir.ScalarFloat,
			},
			want: "Image { dim: Cube, arrayed: false, class: Depth } and Sampler { comparison: true }",
		},
		{
			name: "other",
			tn:   &typeName{Inner: ir.StructType{}},
			want: "TypeInner({[] 0})",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typeInnerDebug(tt.tn); got != tt.want {
				t.Fatalf("typeInnerDebug = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestImageClassDebugSpellings is Rust's Debug of ImageClass.
func TestImageClassDebugSpellings(t *testing.T) {
	tests := []struct {
		image ir.ImageType
		want  string
	}{
		{
			ir.ImageType{Class: ir.ImageClassStorage, StorageFormat: ir.StorageFormatR8Uint, StorageAccess: ir.StorageAccessLoad},
			"Storage { format: R8Uint, access: StorageAccess(LOAD) }",
		},
		{ir.ImageType{Class: ir.ImageClassDepth}, "Depth"},
		{
			ir.ImageType{Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint, Multisampled: true},
			"Sampled { kind: Sint, multi: true }",
		},
		{ir.ImageType{Class: ir.ImageClass(99)}, "External"},
	}
	for _, tt := range tests {
		if got := imageClassDebug(tt.image); got != tt.want {
			t.Errorf("imageClassDebug(%v) = %q, want %q", tt.image.Class, got, tt.want)
		}
	}
}

// TestStorageFormatAndAccessDebug covers the format table and the bitflags
// spelling of a storage access.
func TestStorageFormatAndAccessDebug(t *testing.T) {
	formats := map[ir.StorageFormat]string{
		ir.StorageFormatR8Uint:      "R8Uint",
		ir.StorageFormatRgba32Float: "StorageFormat(34)",
	}
	for format, want := range formats {
		if got := storageFormatDebug(format); got != want {
			t.Errorf("storageFormatDebug(%v) = %q, want %q", format, got, want)
		}
	}
	accesses := map[ir.StorageAccess]string{
		ir.StorageAccessLoad:                         "StorageAccess(LOAD)",
		ir.StorageAccessStore:                        "StorageAccess(STORE)",
		ir.StorageAccessLoad | ir.StorageAccessStore: "StorageAccess(LOAD | STORE)",
		ir.StorageAccess(0):                          "StorageAccess(0x0)",
	}
	for access, want := range accesses {
		if got := storageAccessDebug(access); got != want {
			t.Errorf("storageAccessDebug(%v) = %q, want %q", access, got, want)
		}
	}
}

// TestWantPunctFallback is wantPunct for a spelling the Rust name table does
// not carry: the spelling itself identifies the token in the message.
func TestWantPunctFallback(t *testing.T) {
	known := wantPunct("(")
	if known.Text != rustPunctNames["("] {
		t.Fatalf("wantPunct(\"(\") = %q, want the table's name %q", known.Text, rustPunctNames["("])
	}
	unknown := wantPunct("@@")
	if unknown.Text != "@@" {
		t.Fatalf("wantPunct(\"@@\") = %q, want the spelling itself", unknown.Text)
	}
}

// TestRustExponentRewrite is rustExponent's normalisation of Go's exponent
// form to Rust's.
func TestRustExponentRewrite(t *testing.T) {
	tests := map[string]string{
		"1e+20":  "1e20",
		"1e-07":  "1e-7",
		"1e+00":  "1e0",
		"1.5e+0": "1.5e0",
		"5":      "5",
	}
	for input, want := range tests {
		if got := rustExponent(input); got != want {
			t.Errorf("rustExponent(%q) = %q, want %q", input, got, want)
		}
	}
}
