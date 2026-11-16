package front

// textureQueryLod lowers to the IR's level-of-detail query. These tests pin
// what the frontend must produce (the query's operands and result type) and
// the shape of the injected overloads, the half the end-to-end matrix in
// snapshot/glsl_texture_query_lod_test.go does not see.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestBuiltinTextureQueryLodShape checks that textureQueryLod becomes an
// ExprImageQuery carrying the sampler the combined sampler was split into and
// the call's coordinate, with the vec2<f32> result SPIR-V's OpImageQueryLod
// returns.
func TestBuiltinTextureQueryLodShape(t *testing.T) {
	image := sampledImage(ir.Dim2D, false, false, ir.ScalarFloat)
	f, c, imageExpr := textureContext(t, image)
	coordType := f.registerType(ir.VectorType{Size: ir.Vec2, Scalar: scalarF32})
	coord := dynamicArg(t, c, coordType)

	h, ok := expandedBuiltin(t, f, c, "textureQueryLod",
		[]ir.TypeInner{image, ir.VectorType{Size: ir.Vec2, Scalar: scalarF32}}, imageExpr, coord)
	if !ok {
		t.Fatal("textureQueryLod reported no value")
	}

	query := exprAs[ir.ExprImageQuery](t, c, h, "query")
	if query.Image != imageExpr {
		t.Fatalf("query image is %d, want %d", query.Image, imageExpr)
	}
	lod, isLod := query.Query.(ir.ImageQueryLod)
	if !isLod {
		t.Fatalf("query is %T, want ImageQueryLod", query.Query)
	}
	if lod.Sampler != c.samplers[imageExpr] {
		t.Fatalf("query sampler is %d, want the combined sampler %d", lod.Sampler, c.samplers[imageExpr])
	}
	if lod.Coordinate != coord {
		t.Fatalf("query coordinate is %d, want %d", lod.Coordinate, coord)
	}

	inner, err := c.resolveType(h, ir.Span{})
	if err != nil {
		t.Fatalf("resolveType: %v", err)
	}
	want := ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	if inner != want {
		t.Fatalf("result type is %v, want %v", inner, want)
	}
}

// TestBuiltinTextureQueryLodCoordinates pins the coordinate of each overload:
// the spatial texture coordinate without the array layer, which is what GLSL's
// textureQueryLod takes. A cube image is the case that tells the two apart —
// its size query is a vec2, its lod coordinate a vec3.
func TestBuiltinTextureQueryLodCoordinates(t *testing.T) {
	f, _ := newTestContext(t)
	cases := []struct {
		name      string
		image     ir.ImageType
		variation builtinVariations
		want      ir.TypeInner
	}{
		{"1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, scalarF32},
		{"1D-array", ir.ImageType{Dim: ir.Dim1D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, scalarF32},
		{"2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, ir.VectorType{Size: ir.Vec2, Scalar: scalarF32}},
		{"2D-array", ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, ir.VectorType{Size: ir.Vec2, Scalar: scalarF32}},
		{"3D", ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, ir.VectorType{Size: ir.Vec3, Scalar: scalarF32}},
		{"cube", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationStandard, ir.VectorType{Size: ir.Vec3, Scalar: scalarF32}},
		{"cube-array", ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}, variationCubeTexturesArray, ir.VectorType{Size: ir.Vec3, Scalar: scalarF32}},
		{"2D-shadow", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth, SampledKind: ir.ScalarFloat}, variationStandard, ir.VectorType{Size: ir.Vec2, Scalar: scalarF32}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			decl := &functionDeclaration{}
			f.injectBuiltin(decl, "textureQueryLod", tt.variation)
			ov := builtinOverload(t, f, decl, tt.image, tt.want)
			if ov.macro == nil || ov.macro.kind != macroTextureQueryLod {
				t.Fatalf("overload expands to %+v, want macroTextureQueryLod", ov.macro)
			}
		})
	}
}

// TestBuiltinTextureQueryLodHasNoMultisampledForm: glslang declares no
// textureQueryLod overload for a multisampled sampler ("no matching overloaded
// function found", sampler2DMS), and a multisampled texture has no mip levels
// to query anyway, so the injection must not create one.
func TestBuiltinTextureQueryLodHasNoMultisampledForm(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "textureQueryLod", variationStandard|variationDouble|variationCubeTexturesArray|variationD2MultiTexturesArray)
	for _, dim := range []ir.ImageDimension{ir.Dim1D, ir.Dim2D, ir.Dim3D, ir.DimCube} {
		for _, arrayed := range []bool{false, true} {
			want := ir.ImageType{Dim: dim, Arrayed: arrayed, Multisampled: true,
				Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
			for _, ov := range decl.overloads {
				if len(ov.parameters) == 0 {
					continue
				}
				if typeInnerEqual(f.typeInner(ov.parameters[0]), want) {
					t.Fatalf("an overload takes the multisampled image %+v", want)
				}
			}
		}
	}
}
