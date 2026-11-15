package front

// Contracts of GLSL module-scope declarations, Rust naga's
// front/glsl/variables.rs: the builtin variable table (111-221), add_builtin
// (41-99), make_variable_invariant (226-242), the layout qualifier consumption
// (ast.rs:283-337) and add_global_var (415-663), plus the block and precision
// declarations of parser/declarations.rs:487-685.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// builtinsOf collects the builtin values of an entry point's arguments and
// result members.
func builtinsOf(t *testing.T, mod *ir.Module) map[ir.BuiltinValue]bool {
	t.Helper()
	entry := onlyEntry(t, mod)
	found := map[ir.BuiltinValue]bool{}
	for _, arg := range entry.Function.Arguments {
		if arg.Binding == nil {
			continue
		}
		if binding, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
			found[binding.Builtin] = true
		}
	}
	if entry.Function.Result != nil {
		result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
		for _, member := range result.Members {
			if member.Binding == nil {
				continue
			}
			if binding, ok := (*member.Binding).(ir.BuiltinBinding); ok {
				found[binding.Builtin] = true
			}
		}
	}
	return found
}

// TestBuiltinVariablesPerStage is the builtin table of variables.rs:111-221:
// every name the default lookup knows resolves to its BuiltinValue, with the
// storage class the table gives it.
func TestBuiltinVariablesPerStage(t *testing.T) {
	tests := []struct {
		name   string
		stage  ir.ShaderStage
		source string
		want   []ir.BuiltinValue
	}{
		{
			name:  "fragment",
			stage: ir.StageFragment,
			source: `#version 450
layout(location = 0) out vec4 o;
void main() {
    gl_FragDepth = 0.5;
    gl_PointSize = 1.0;
    vec4 a = vec4(gl_FragCoord.xy, gl_PointCoord);
    vec4 b = vec4(float(gl_SampleID), float(gl_PrimitiveID), float(gl_FrontFacing), 0.0);
    vec4 c = vec4(gl_BaryCoordEXT.x, gl_BaryCoordNoPerspEXT.y, 0.0, 0.0);
    o = a + b + c;
}
`,
			want: []ir.BuiltinValue{
				ir.BuiltinFragDepth, ir.BuiltinPointSize, ir.BuiltinPosition,
				ir.BuiltinPointCoord, ir.BuiltinSampleIndex, ir.BuiltinPrimitiveIndex,
				ir.BuiltinFrontFacing, ir.BuiltinBarycentric, ir.BuiltinBarycentricNoPerspective,
			},
		},
		{
			name:  "vertex",
			stage: ir.StageVertex,
			source: `#version 450
layout(location = 0) out vec4 o;
void main() {
    gl_Position = vec4(float(gl_VertexIndex), float(gl_InstanceIndex), float(gl_BaseVertex), float(gl_BaseInstance));
    gl_PointSize = 1.0;
    gl_ClipDistance[0] = 1.0;
    gl_CullDistance[0] = 0.5;
    o = vec4(float(gl_DrawID), 0.0, 0.0, 1.0);
}
`,
			want: []ir.BuiltinValue{
				ir.BuiltinPosition, ir.BuiltinVertexIndex, ir.BuiltinInstanceIndex,
				ir.BuiltinBaseVertex, ir.BuiltinBaseInstance, ir.BuiltinPointSize,
				ir.BuiltinClipDistances, ir.BuiltinCullDistance, ir.BuiltinDrawIndex,
			},
		},
		{
			name:  "compute",
			stage: ir.StageCompute,
			source: `#version 450
layout(local_size_x = 4) in;
layout(set = 0, binding = 0) buffer B { float data[]; } b;
void main() {
    uint i = gl_GlobalInvocationID.x + gl_NumWorkGroups.y + gl_WorkGroupSize.z
        + gl_WorkGroupID.x + gl_LocalInvocationID.y + gl_LocalInvocationIndex;
    b.data[i] = 1.0;
}
`,
			want: []ir.BuiltinValue{
				ir.BuiltinGlobalInvocationID, ir.BuiltinNumWorkGroups, ir.BuiltinWorkGroupSize,
				ir.BuiltinWorkGroupID, ir.BuiltinLocalInvocationID, ir.BuiltinLocalInvocationIndex,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, _, err := Parse(tt.source, Options{Stage: tt.stage})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			found := builtinsOf(t, mod)
			for _, want := range tt.want {
				if !found[want] {
					t.Errorf("the entry point carries no %v binding; found %v", want, found)
				}
			}
		})
	}
}

// TestBuiltinTableMatchesNameSet keeps the lazy-lookup name set and the type
// table in step: every name builtinSymbol admits has data in builtinDataFor.
func TestBuiltinTableMatchesNameSet(t *testing.T) {
	for name := range builtinVariableNames {
		if _, ok := builtinDataFor(name, ir.StageVertex); !ok {
			t.Errorf("builtinVariableNames lists %q but builtinDataFor has no data for it", name)
		}
	}
}

// TestStorageImageFormatAndAccess is the storage-image arm of add_global_var
// (variables.rs:714-740): the layout format replaces the placeholder and the
// memory qualifiers narrow the access.
func TestStorageImageFormatAndAccess(t *testing.T) {
	tests := []struct {
		name       string
		qualifiers string
		body       string
		wantFormat ir.StorageFormat
		wantAccess ir.StorageAccess
	}{
		{
			name:       "format only",
			body:       "    imageStore(img, ivec2(0), vec4(1.0));\n    o = imageLoad(img, ivec2(0));\n",
			wantFormat: ir.StorageFormatRgba32Float,
			wantAccess: ir.StorageAccessLoad | ir.StorageAccessStore,
		},
		{
			name:       "writeonly",
			qualifiers: "writeonly",
			body:       "    imageStore(img, ivec2(0), vec4(1.0));\n",
			wantFormat: ir.StorageFormatRgba32Float,
			wantAccess: ir.StorageAccessStore,
		},
		{
			name:       "readonly",
			qualifiers: "readonly",
			body:       "    o = imageLoad(img, ivec2(0));\n",
			wantFormat: ir.StorageFormatRgba32Float,
			wantAccess: ir.StorageAccessLoad,
		},
		{
			name:       "canceling",
			qualifiers: "writeonly readonly",
			body:       "    imageStore(img, ivec2(0), vec4(1.0));\n    o = imageLoad(img, ivec2(0));\n",
			wantFormat: ir.StorageFormatRgba32Float,
			wantAccess: ir.StorageAccessLoad | ir.StorageAccessStore,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := `#version 450
layout(set = 0, binding = 0, rgba32f) ` + tt.qualifiers + ` uniform image2D img;
layout(location = 0) out vec4 o;
void main() {
` + tt.body + `}
`
			mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			image, ok := mod.Types[findGlobal(t, mod, "img").Type].Inner.(ir.ImageType)
			if !ok {
				t.Fatal("the image global does not carry an image type")
			}
			if image.StorageFormat != tt.wantFormat {
				t.Fatalf("format = %v, want %v", image.StorageFormat, tt.wantFormat)
			}
			if image.StorageAccess != tt.wantAccess {
				t.Fatalf("access = %v, want %v", image.StorageAccess, tt.wantAccess)
			}
			if image.Class != ir.ImageClassStorage {
				t.Fatalf("class = %v, want storage", image.Class)
			}
		})
	}
}

// TestStorageImageRequiresFormat is variables.rs:727-732: a storage image
// without a format layout qualifier is an error.
func TestStorageImageRequiresFormat(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(set = 0, binding = 0) uniform image2D img;
layout(location = 0) out vec4 o;
void main() { o = imageLoad(img, ivec2(0)); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "image types require a format layout qualifier") {
		t.Fatalf("error = %v, want the missing-format error", err)
	}
}

// TestStorageImageAccessMismatch is function_call's access check
// (functions.rs:639-650): a builtin that reads a write-only image is reported
// with both accesses named.
func TestStorageImageAccessMismatch(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(set = 0, binding = 0, rgba32f) writeonly uniform image2D img;
layout(location = 0) out vec4 o;
void main() { o = imageLoad(img, ivec2(0)); }
`, Options{Stage: ir.StageFragment})
	want := "'imageLoad': image needs StorageAccess(LOAD) access but only StorageAccess(STORE) was provided"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// TestPushConstantBecomesImmediateSpace is variables.rs:744-747: a uniform
// block declared push_constant becomes the immediate address space.
func TestPushConstantBecomesImmediateSpace(t *testing.T) {
	source := `#version 450
layout(push_constant) uniform Push { vec4 color; } pc;
layout(location = 0) out vec4 o;
void main() { o = pc.color; }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pc := findGlobal(t, mod, "pc")
	if pc.Space != ir.SpaceImmediate {
		t.Fatalf("push constant space = %v, want immediate", pc.Space)
	}
	if pc.Binding != nil {
		t.Fatalf("push constant binding = %+v, want none", pc.Binding)
	}
}

// TestSharedVariableBecomesWorkgroupSpace is variables.rs:751-752.
func TestSharedVariableBecomesWorkgroupSpace(t *testing.T) {
	source := `#version 450
layout(local_size_x = 4) in;
shared float s[64];
void main() { s[gl_LocalInvocationIndex] = 1.0; }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	shared := findGlobal(t, mod, "s")
	if shared.Space != ir.SpaceWorkGroup {
		t.Fatalf("shared variable space = %v, want workgroup", shared.Space)
	}
}

// TestBufferBlockUsesStd430 is the default layout of a buffer block
// (declarations.rs:544-556): a runtime-sized float array strides by 4.
func TestBufferBlockUsesStd430(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) buffer B { float data[]; } b;
layout(location = 0) out vec4 o;
void main() { b.data[0] = 1.0; o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	block := findGlobal(t, mod, "b")
	if block.Space != ir.SpaceStorage {
		t.Fatalf("buffer block space = %v, want storage", block.Space)
	}
	structTy := mod.Types[block.Type].Inner.(ir.StructType)
	array, ok := mod.Types[structTy.Members[0].Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("member type = %T, want an array", mod.Types[structTy.Members[0].Type].Inner)
	}
	if array.Stride != 4 {
		t.Fatalf("array stride = %d, want the std430 stride 4", array.Stride)
	}
	if array.Size.Constant != nil {
		t.Fatalf("the runtime array has a constant size %d", *array.Size.Constant)
	}
}

// TestUniformBlockArrayStridesByVec4 is the std140 array rule of
// offset.rs:41-59: under std140 a float array strides by a whole vec4.
func TestUniformBlockArrayStridesByVec4(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float weights[2]; } u;
layout(location = 0) out vec4 o;
void main() { o = vec4(u.weights[0]); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	block := findGlobal(t, mod, "u")
	structTy := mod.Types[block.Type].Inner.(ir.StructType)
	array, ok := mod.Types[structTy.Members[0].Type].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("member type = %T, want an array", mod.Types[structTy.Members[0].Type].Inner)
	}
	if array.Stride != 16 {
		t.Fatalf("array stride = %d, want the std140 stride 16", array.Stride)
	}
}

// TestCombinedSamplerErrors covers the three refusals of addCombinedSampler
// (design §11; variables.rs has no equivalent, gfx-rs/wgpu#4342).
func TestCombinedSamplerErrors(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options Options
		want    string
	}{
		{
			name:   "array of samplers",
			source: "#version 450\nlayout(set = 0, binding = 0) uniform sampler2D texs[2];\nvoid main() {}\n",
			want:   "Not implemented: array of combined image samplers",
		},
		{
			name:   "reject policy",
			source: "#version 450\nlayout(set = 0, binding = 0) uniform sampler2D tex;\nvoid main() {}\n",
			options: Options{
				Stage:                ir.StageFragment,
				CombinedImageSampler: frontend.CombinedImageSamplerReject,
			},
			want: "combined image samplers are refused by CombinedImageSamplerReject",
		},
		{
			name: "local sampler",
			source: `#version 450
layout(location = 0) out vec4 o;
void main() { sampler2D s; o = vec4(1.0); }
`,
			want: "combined image sampler types are only supported for module-scope uniform variables",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := tt.options
			if options.Stage == 0 && options.CombinedImageSampler == 0 {
				options = Options{Stage: ir.StageFragment}
			}
			_, _, err := Parse(tt.source, options)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestInvariantOnPositionReachesTheResult is make_variable_invariant
// (variables.rs:226-242): only the Position builtin carries the flag, and the
// result struct member reports it. The redeclaration must come before the
// first use of the builtin: a later one makes Context.variable mint a second
// gl_Position global instead of reusing the one the frontend already added
// (reported as a defect; Rust's Frontend::lookup_variable consults its global
// table first, variables.rs:101-130).
func TestInvariantOnPositionReachesTheResult(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
invariant gl_Position;
void main() { gl_Position = vec4(1.0); o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entry := onlyEntry(t, mod)
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	found := false
	for _, member := range result.Members {
		if binding, ok := (*member.Binding).(ir.BuiltinBinding); ok && binding.Builtin == ir.BuiltinPosition {
			found = true
			if !binding.Invariant {
				t.Fatal("the Position member is not marked invariant")
			}
		}
	}
	if !found {
		t.Fatalf("the result struct has no Position member: %+v", result.Members)
	}
}

// TestInvariantOnOrdinaryVariableIsANoOp is the other half of
// make_variable_invariant: a location-bound variable has no invariant flag in
// the IR, so the redeclaration does nothing and is not an error.
func TestInvariantOnOrdinaryVariableIsANoOp(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 o;
invariant o;
void main() { o = vec4(1.0); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	entry := onlyEntry(t, mod)
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	if _, ok := (*result.Members[0].Binding).(ir.LocationBinding); !ok {
		t.Fatalf("the member binding = %T, want a location binding", *result.Members[0].Binding)
	}
}

// TestQualifierDeclarationSetsMetadata is lower_qualifier_declaration
// (declarations.rs:453-476): the local size entries and the early fragment
// test request reach the entry point.
func TestQualifierDeclarationSetsMetadata(t *testing.T) {
	source := `#version 450
layout(local_size_x = 8, local_size_y = 4, local_size_z = 2) in;
void main() {}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := onlyEntry(t, mod).Workgroup; got != [3]uint32{8, 4, 2} {
		t.Fatalf("workgroup size = %v, want [8 4 2]", got)
	}

	fragment := `#version 450
layout(early_fragment_tests) in;
layout(location = 0) out vec4 o;
void main() { o = vec4(1.0); }
`
	mod, _, err = Parse(fragment, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, ok := onlyEntry(t, mod).EarlyDepthTest.(ir.EarlyDepthForce); !ok {
		t.Fatalf("early depth test = %T, want the force", onlyEntry(t, mod).EarlyDepthTest)
	}
}

// TestPrecisionDeclarationRules is the precision statement's type rule
// (GLSL 4.60 §4.5.4, GLSL ES 3.00 §4.5.4): float, int and sampler types are
// accepted, anything else is reported, and the hints themselves do not reach
// the IR. glslang refuses bool and a struct with "cannot apply precision
// statement to this type; use 'float', 'int' or a sampler type".
func TestPrecisionDeclarationRules(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"float", "precision highp float;", ""},
		{"int", "precision mediump int;", ""},
		{"sampler", "precision lowp sampler2D;", ""},
		{"bool", "precision highp bool;", "Precision statement can only work on floats, ints and sampler types"},
		{"struct", "struct S { float x; };\nprecision highp S;", "Precision statement can only work on floats, ints and sampler types"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "#version 450\n" + tt.source + "\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(1.0); }\n"
			_, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestLayoutQualifierErrors covers the qualifier consumption errors of
// ast.rs:283-337 and their reporting order.
func TestLayoutQualifierErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "uint without value",
			source: "layout(location) out vec4 o;",
			want:   "Qualifier expects a uint value",
		},
		{
			name:   "none with value",
			source: "layout(early_fragment_tests = 1) in;",
			want:   "Qualifier doesn't expect a value",
		},
		{
			name:   "unexpected entry",
			source: "layout(unexpected_entry) out vec4 o;",
			want:   "Unexpected qualifier",
		},
		{
			name:   "interpolation on a plain variable",
			source: "flat vec3 c;",
			want:   "Interpolation qualifiers can only be used in in/out variables",
		},
		{
			name:   "sampling on a plain variable",
			source: "centroid vec3 c;",
			want:   "Sampling qualifiers can only be used in in/out variables",
		},
		{
			name:   "memory qualifier on a plain variable",
			source: "readonly vec3 c;",
			want:   "Memory qualifiers can only be used in storage variables",
		},
		{
			name:   "invariant on a plain variable",
			source: "invariant vec3 c;",
			want:   "Invariant qualifier can only be used in in/out variables",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "#version 450\n" + tt.source + "\nlayout(location = 1) out vec4 o;\nvoid main() { o = vec4(1.0); }\n"
			_, _, err := Parse(source, Options{Stage: ir.StageFragment})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestConstantIDBecomesOverride is the specialization-constant arm of
// add_global_var (variables.rs:680-695): the id and the initializer reach an
// IR override.
func TestConstantIDBecomesOverride(t *testing.T) {
	source := `#version 450
layout(constant_id = 5) const int N = 3;
layout(location = 0) out vec4 o;
void main() { o = vec4(float(N)); }
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(mod.Overrides) != 1 {
		t.Fatalf("module has %d overrides, want 1", len(mod.Overrides))
	}
	override := mod.Overrides[0]
	if override.ID == nil || *override.ID != 5 {
		t.Fatalf("override id = %v, want 5", override.ID)
	}
	if override.Name != "N" {
		t.Fatalf("override name = %q, want N", override.Name)
	}
}

// TestConstantIDTooHighIsReported is Rust's SpecIdTooHigh (variables.rs:683-688).
func TestConstantIDTooHighIsReported(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(constant_id = 70000) const int N = 3;
layout(location = 0) out vec4 o;
void main() { o = vec4(float(N)); }
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "constant_id value 70000 is too high") {
		t.Fatalf("error = %v, want the too-high constant_id error", err)
	}
}

// TestModuleConstNeedsInitializer is variables.rs:696-698.
func TestModuleConstNeedsInitializer(t *testing.T) {
	_, _, err := Parse(`#version 450
const int N;
void main() {}
`, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Error(), "const values must have an initializer") {
		t.Fatalf("error = %v, want the missing-initializer error", err)
	}
}
