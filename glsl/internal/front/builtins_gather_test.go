package front

// This file covers GLSL 4.50 §8.9 Texture Gather Functions: textureGather and
// textureGatherOffset, and the deliberate refusal of textureGatherOffsets.
// Rust naga's GLSL frontend has no gather, so the IR shape follows the WGSL
// lowerer's textureGather (wgsl/internal/lower/lower.go, lowerTextureGather and
// lowerTextureGatherCompare): an ExprImageSample with Gather set, level zero,
// an optional constant Offset and, for a depth image, a DepthRef.
//
// Which forms exist is glslang's, not a guess: every signature in the table was
// compiled with glslangValidator from a #version 450 shader first. It accepts
// 2D, 2D array, cube and cube array images (sampled and depth, and sampled as
// float, int or uint), has no 1D, 3D or multisampled gather, takes an offset
// only for a 2D image, and takes a depth reference as its own argument rather
// than inside the coordinate vector.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// gatherOffsetConst builds a constant ivec2 offset, the shape
// ir.ExprImageSample.Offset must have.
func gatherOffsetConst(t *testing.T, f *Frontend, c *Context) ir.ExpressionHandle {
	t.Helper()
	ty := f.registerType(vecOf(ir.Vec2, scalarI32))
	x := mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(1)})
	y := mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(-1)})
	return mustExpr(t, c, ir.ExprCompose{Type: ty, Components: []ir.ExpressionHandle{x, y}})
}

// gatherCase is one implemented gather form.
type gatherCase struct {
	name       string
	builtin    string
	image      ir.ImageType
	variations builtinVariations
	coordSize  int
	// depth is the separate depth reference a shadow gather takes.
	depth bool
	// offset asks for the textureGatherOffset form.
	offset bool
	// comp is the component argument, or -1 for the overloads without one.
	comp int

	wantComponent ir.SwizzleComponent
	// wantScalar is the component type of the vec4 result: the image's sampled
	// kind for a sampled image, f32 for a depth image.
	wantScalar ir.ScalarKind
}

// gatherCases is every form the frontend implements, one per glslang signature.
func gatherCases() []gatherCase {
	twoD := ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	twoDArray := ir.ImageType{Dim: ir.Dim2D, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	cube := ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	cubeArray := ir.ImageType{Dim: ir.DimCube, Arrayed: true, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}

	return []gatherCase{
		{
			name: "2D without comp", builtin: "textureGather", image: twoD, coordSize: 2,
			comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D with comp", builtin: "textureGather", image: twoD, coordSize: 2,
			comp: 2, wantComponent: ir.SwizzleZ, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D array", builtin: "textureGather", image: twoDArray, coordSize: 3,
			comp: 1, wantComponent: ir.SwizzleY, wantScalar: ir.ScalarFloat,
		},
		{
			name: "cube", builtin: "textureGather", image: cube, coordSize: 3,
			comp: 0, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "cube array", builtin: "textureGather", image: cubeArray, coordSize: 4,
			variations: variationCubeTexturesArray,
			comp:       3, wantComponent: ir.SwizzleW, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D integer", builtin: "textureGather",
			image:     ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarSint},
			coordSize: 2, comp: 1, wantComponent: ir.SwizzleY, wantScalar: ir.ScalarSint,
		},
		{
			name: "2D offset", builtin: "textureGatherOffset", image: twoD, coordSize: 2,
			offset: true, comp: 1, wantComponent: ir.SwizzleY, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D array offset without comp", builtin: "textureGatherOffset", image: twoDArray, coordSize: 3,
			offset: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D depth", builtin: "textureGather", image: gatherDepthImage(ir.Dim2D, false),
			coordSize: 2, depth: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D array depth", builtin: "textureGather", image: gatherDepthImage(ir.Dim2D, true),
			coordSize: 3, depth: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "cube depth", builtin: "textureGather", image: gatherDepthImage(ir.DimCube, false),
			coordSize: 3, depth: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "cube array depth", builtin: "textureGather", image: gatherDepthImage(ir.DimCube, true),
			coordSize: 4, variations: variationCubeTexturesArray,
			depth: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D depth offset", builtin: "textureGatherOffset", image: gatherDepthImage(ir.Dim2D, false),
			coordSize: 2, depth: true, offset: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
		{
			name: "2D array depth offset", builtin: "textureGatherOffset", image: gatherDepthImage(ir.Dim2D, true),
			coordSize: 3, depth: true, offset: true, comp: -1, wantComponent: ir.SwizzleX, wantScalar: ir.ScalarFloat,
		},
	}
}

// gatherDepthImage is depthImage as a value, for a table literal.
func gatherDepthImage(dim ir.ImageDimension, arrayed bool) ir.ImageType {
	return depthImage(dim, arrayed).(ir.ImageType)
}

// TestBuiltinTextureGatherForms pins the IR of every implemented gather form:
// the gathered component, the depth reference, the constant offset, the array
// index split and the result type.
func TestBuiltinTextureGatherForms(t *testing.T) {
	for _, tt := range gatherCases() {
		t.Run(tt.name, func(t *testing.T) {
			f, c, imageExpr := textureContext(t, tt.image)

			decl := &functionDeclaration{}
			f.injectBuiltin(decl, tt.builtin, variationStandard|tt.variations)

			params := []ir.TypeInner{tt.image, makeCoordsArg(tt.coordSize, ir.ScalarFloat)}
			if tt.depth {
				params = append(params, scalarF32)
			}
			if tt.offset {
				params = append(params, gatherOffsetCoord())
			}
			if tt.comp >= 0 {
				params = append(params, scalarI32)
			}
			ov := builtinOverload(t, f, decl, params...)

			coord := dynamicArg(t, c, f.registerType(makeCoordsArg(tt.coordSize, ir.ScalarFloat)))
			args := []ir.ExpressionHandle{imageExpr, coord}
			if tt.depth {
				args = append(args, dynamicArg(t, c, f.registerType(scalarF32)))
			}
			if tt.offset {
				args = append(args, gatherOffsetConst(t, f, c))
			}
			if tt.comp >= 0 {
				args = append(args, mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(tt.comp)}))
			}

			h, ok := callBuiltin(t, c, ov, args...)
			if !ok {
				t.Fatal("the gather reported no value")
			}
			sample := exprAs[ir.ExprImageSample](t, c, h, "gather")

			if sample.Gather == nil {
				t.Fatal("the sample carries no Gather component")
			}
			if *sample.Gather != tt.wantComponent {
				t.Fatalf("gathered component is %d, want %d", *sample.Gather, tt.wantComponent)
			}
			if (sample.DepthRef != nil) != tt.depth {
				t.Fatalf("DepthRef is %v, want present=%v", sample.DepthRef, tt.depth)
			}
			if (sample.Offset != nil) != tt.offset {
				t.Fatalf("Offset is %v, want present=%v", sample.Offset, tt.offset)
			}
			if _, isZero := sample.Level.(ir.SampleLevelZero); !isZero {
				t.Fatalf("level is %T, want SampleLevelZero", sample.Level)
			}

			// An arrayed image's layer is the last coordinate component and
			// becomes the array index, converted to the integer the IR wants.
			if !tt.image.Arrayed {
				if sample.ArrayIndex != nil {
					t.Fatalf("a gather of the plain image %+v has array index %d", tt.image, *sample.ArrayIndex)
				}
			} else {
				if sample.ArrayIndex == nil {
					t.Fatalf("array index is missing for the arrayed image %+v", tt.image)
				}
				layer := *sample.ArrayIndex
				// The float coordinate component is converted to an integer
				// before it becomes the array index.
				if conversion, isAs := c.Expression(layer).(ir.ExprAs); isAs {
					layer = conversion.Expr
				}
				index := exprAs[ir.ExprAccessIndex](t, c, layer, "array index")
				wantIndex := uint32(tt.coordSize - 1)
				if index.Index != wantIndex {
					t.Fatalf("array index reads component %d, want %d", index.Index, wantIndex)
				}
			}

			result, err := c.resolveType(h, ir.Span{})
			if err != nil {
				t.Fatalf("resolve the result type: %v", err)
			}
			vector, isVector := result.(ir.VectorType)
			if !isVector || vector.Size != ir.Vec4 {
				t.Fatalf("result type is %T, want a vec4", result)
			}
			if vector.Scalar.Kind != tt.wantScalar || vector.Scalar.Width != 4 {
				t.Fatalf("result component is %+v, want a 32-bit %v", vector.Scalar, tt.wantScalar)
			}
		})
	}
}

// TestBuiltinTextureGatherArrayIndexAbsentForPlainImage checks that a 2D image
// keeps the coordinate untouched: no split, no array index.
func TestBuiltinTextureGatherArrayIndexAbsentForPlainImage(t *testing.T) {
	image := ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	f, c, imageExpr := textureContext(t, image)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "textureGather", variationStandard)
	ov := builtinOverload(t, f, decl, image, makeCoordsArg(2, ir.ScalarFloat))
	coord := dynamicArg(t, c, f.registerType(makeCoordsArg(2, ir.ScalarFloat)))

	h, _ := callBuiltin(t, c, ov, imageExpr, coord)
	sample := exprAs[ir.ExprImageSample](t, c, h, "gather")
	if sample.ArrayIndex != nil {
		t.Fatalf("a 2D gather has an array index %d", *sample.ArrayIndex)
	}
	if sample.Coordinate != coord {
		t.Fatalf("coordinate is %d, want the argument %d unsplit", sample.Coordinate, coord)
	}
}

// TestBuiltinTextureGatherRejectsComponent checks the component argument: it
// must be a constant in 0 to 3, which is also glslang's rule ("must be 0, 1, 2,
// or 3: component argument").
func TestBuiltinTextureGatherRejectsComponent(t *testing.T) {
	tests := []struct {
		name string
		// comp builds the argument; nil asks for a run-time one.
		comp func(t *testing.T, f *Frontend, c *Context) ir.ExpressionHandle
		want string
	}{
		{
			name: "four",
			comp: func(t *testing.T, f *Frontend, c *Context) ir.ExpressionHandle {
				return mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(4)})
			},
			want: "outside the component range 0 to 3",
		},
		{
			name: "negative",
			comp: func(t *testing.T, f *Frontend, c *Context) ir.ExpressionHandle {
				return mustExpr(t, c, ir.Literal{Value: ir.LiteralI32(-1)})
			},
			want: "outside the component range 0 to 3",
		},
		{
			name: "run-time",
			comp: func(t *testing.T, f *Frontend, c *Context) ir.ExpressionHandle {
				return dynamicArg(t, c, f.registerType(scalarI32))
			},
			want: "must be a constant expression",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
			f, c, imageExpr := textureContext(t, image)
			decl := &functionDeclaration{}
			f.injectBuiltin(decl, "textureGather", variationStandard)
			ov := builtinOverload(t, f, decl, image, makeCoordsArg(2, ir.ScalarFloat), scalarI32)

			coord := dynamicArg(t, c, f.registerType(makeCoordsArg(2, ir.ScalarFloat)))
			_, _, err := ov.macro.call(c, []loweredArg{
				{expr: imageExpr},
				{expr: coord},
				{expr: tt.comp(t, f, c)},
			}, ir.Span{})
			if err == nil {
				t.Fatal("the gather accepted the component argument")
			}
			if !strings.Contains(err.Message, tt.want) {
				t.Fatalf("error %q does not say %q", err.Message, tt.want)
			}
		})
	}
}

// TestBuiltinTextureGatherRejectsRunTimeOffset checks that a non-constant
// offset is refused: ir.ExprImageSample.Offset is a const-expression, and GLSL
// 4.50 §8.9 requires a constant expression there too. glslang is laxer and
// emits a non-constant operand, so the check is ours.
func TestBuiltinTextureGatherRejectsRunTimeOffset(t *testing.T) {
	image := ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	f, c, imageExpr := textureContext(t, image)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "textureGatherOffset", variationStandard)
	ov := builtinOverload(t, f, decl, image, makeCoordsArg(2, ir.ScalarFloat), gatherOffsetCoord())

	coord := dynamicArg(t, c, f.registerType(makeCoordsArg(2, ir.ScalarFloat)))
	offset := dynamicArg(t, c, f.registerType(vecOf(ir.Vec2, scalarI32)))
	_, _, err := ov.macro.call(c, []loweredArg{{expr: imageExpr}, {expr: coord}, {expr: offset}}, ir.Span{})
	if err == nil {
		t.Fatal("the gather accepted a run-time offset")
	}
	if !strings.Contains(err.Message, "constant expression") {
		t.Fatalf("error %q does not name the constant-expression requirement", err.Message)
	}
}

// TestBuiltinTextureGatherVariations checks the overload set the generator
// builds for the standard variation: no 1D, 3D or multisampled gather, and no
// offset for a cube image, which is exactly what glslang refuses.
func TestBuiltinTextureGatherVariations(t *testing.T) {
	f, _ := newTestContext(t)
	decl := &functionDeclaration{}
	f.injectBuiltin(decl, "textureGather", variationStandard|variationCubeTexturesArray)

	// imageShape keys an overload's image parameter.
	type imageShape struct {
		dim     ir.ImageDimension
		arrayed bool
		class   ir.ImageClass
	}
	seen := make(map[imageShape]bool)
	for _, ov := range decl.overloads {
		image := f.typeInner(ov.parameters[0])
		img, ok := image.(ir.ImageType)
		if !ok {
			t.Fatalf("overload parameter 0 is %T, want an image", image)
		}
		seen[imageShape{dim: img.Dim, arrayed: img.Arrayed, class: img.Class}] = true
		if img.Dim == ir.Dim1D || img.Dim == ir.Dim3D || img.Multisampled {
			t.Fatalf("a gather overload exists for the unsupported image %+v", img)
		}
	}

	want := make([]imageShape, 0, 8)
	for _, dim := range []ir.ImageDimension{ir.Dim2D, ir.DimCube} {
		for _, arrayed := range []bool{false, true} {
			for _, class := range []ir.ImageClass{ir.ImageClassSampled, ir.ImageClassDepth} {
				want = append(want, imageShape{dim: dim, arrayed: arrayed, class: class})
			}
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("the catalog has %d image shapes, want %d: %v", len(seen), len(want), seen)
	}
	for _, shape := range want {
		if !seen[shape] {
			t.Fatalf("no gather overload for the image %+v; have %v", shape, seen)
		}
	}
}

// TestParseRefusesTextureGatherOffsets checks the deliberate refusal: the
// limitation is named, it is not a bare "unknown function".
func TestParseRefusesTextureGatherOffsets(t *testing.T) {
	source := `#version 450
layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
ivec2 offsets[4] = ivec2[4](ivec2(1), ivec2(1), ivec2(1), ivec2(1));
void main() { color = textureGatherOffsets(tex, vec2(0.5), offsets, 2); }
`
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("Parse accepted textureGatherOffsets")
	}
	if !strings.Contains(err.Error(), "ConstOffsets") {
		t.Fatalf("error %q does not name the ConstOffsets limitation", err.Error())
	}
	if len(err.Errors) == 0 || !err.Errors[0].Span.IsDefined() {
		t.Fatalf("refusal carries no source location: %+v", err.Errors)
	}
}

// gatherShaders is the end-to-end corpus: one #version 450 fragment shader per
// implemented form, each written the way glslang accepted it.
func gatherShaders() []struct {
	name   string
	source string
} {
	const prelude = "#version 450\n"
	return []struct {
		name   string
		source string
	}{
		{"2D", prelude + `layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec2(0.5), 1); }
`},
		{"2D without comp", prelude + `layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec2(0.5)); }
`},
		{"2D array", prelude + `layout(binding = 0) uniform sampler2DArray tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec3(0.5), 2); }
`},
		{"cube", prelude + `layout(binding = 0) uniform samplerCube tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec3(0.5), 0); }
`},
		{"cube array", prelude + `#extension GL_EXT_texture_cube_map_array : require
layout(binding = 0) uniform samplerCubeArray tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec4(0.5), 3); }
`},
		{"integer", prelude + `layout(binding = 0) uniform isampler2D tex;
layout(location = 0) out ivec4 color;
void main() { color = textureGather(tex, vec2(0.5), 1); }
`},
		{"2D offset", prelude + `layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = textureGatherOffset(tex, vec2(0.5), ivec2(1), 2); }
`},
		{"2D array offset", prelude + `layout(binding = 0) uniform sampler2DArray tex;
layout(location = 0) out vec4 color;
void main() { color = textureGatherOffset(tex, vec3(0.5), ivec2(1)); }
`},
		{"2D depth", prelude + `layout(binding = 0) uniform sampler2DShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec2(0.5), 0.5); }
`},
		{"2D array depth", prelude + `layout(binding = 0) uniform sampler2DArrayShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec3(0.5), 0.5); }
`},
		{"cube depth", prelude + `layout(binding = 0) uniform samplerCubeShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec3(0.5), 0.5); }
`},
		{"cube array depth", prelude + `#extension GL_EXT_texture_cube_map_array : require
layout(binding = 0) uniform samplerCubeArrayShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec4(0.5), 0.5); }
`},
		{"2D depth offset", prelude + `layout(binding = 0) uniform sampler2DShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGatherOffset(tex, vec2(0.5), 0.5, ivec2(1)); }
`},
		{"2D array depth offset", prelude + `layout(binding = 0) uniform sampler2DArrayShadow tex;
layout(location = 0) out vec4 color;
void main() { color = textureGatherOffset(tex, vec3(0.5), 0.5, ivec2(1)); }
`},
	}
}

// TestParseTextureGatherShaders is the end-to-end gate: every form parses,
// validates, compiles to SPIR-V that passes spirv-val when the validator is
// installed, and compiles through the MSL and HLSL backends.
func TestParseTextureGatherShaders(t *testing.T) {
	for _, tt := range gatherShaders() {
		t.Run(tt.name, func(t *testing.T) {
			mod, _, err := Parse(tt.source, Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, verr := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}

			blob, _, cerr := spirv.Compile(mod, spirv.Options{})
			if cerr != nil {
				t.Fatalf("spirv.Compile: %v", cerr)
			}
			if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
				path := filepath.Join(t.TempDir(), "gather.spv")
				if werr := os.WriteFile(path, blob, 0o600); werr != nil {
					t.Fatalf("write the SPIR-V binary: %v", werr)
				}
				if out, verr := exec.Command(validator, path).CombinedOutput(); verr != nil {
					t.Fatalf("spirv-val: %v: %s", verr, out)
				}
			}

			if _, _, merr := msl.Compile(mod, msl.Options{}); merr != nil {
				t.Fatalf("msl.Compile: %v", merr)
			}
			hlslText, _, herr := hlsl.Compile(mod, hlsl.Options{})
			if herr != nil {
				t.Fatalf("hlsl.Compile: %v", herr)
			}
			if !strings.Contains(hlslText, ".Gather") {
				t.Fatalf("the HLSL output has no gather call:\n%s", hlslText)
			}
		})
	}
}

// TestTextureGatherHLSLCompilesWithDXC checks one emitted gather shader against
// the real HLSL compiler, which is the cheapest independent check that the
// backend output is legal.
func TestTextureGatherHLSLCompilesWithDXC(t *testing.T) {
	dxc, found := testtools.Optional(t, "dxc", testtools.InstallDXC)
	if !found {
		return
	}

	source := `#version 450
layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = textureGather(tex, vec2(0.5), 1); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	hlslText, _, herr := hlsl.Compile(mod, hlsl.Options{})
	if herr != nil {
		t.Fatalf("hlsl.Compile: %v", herr)
	}
	if len(mod.EntryPoints) != 1 {
		t.Fatalf("the module has %d entry points, want one", len(mod.EntryPoints))
	}

	path := filepath.Join(t.TempDir(), "gather.hlsl")
	if werr := os.WriteFile(path, []byte(hlslText), 0o600); werr != nil {
		t.Fatalf("write the HLSL source: %v", werr)
	}
	out, cerr := exec.Command(dxc, "-T", "ps_6_0", "-E", mod.EntryPoints[0].Name, path).CombinedOutput()
	if cerr != nil {
		t.Fatalf("dxc: %v: %s\n%s", cerr, out, hlslText)
	}
}
