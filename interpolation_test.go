package naga

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// interpolationShader carries one varying per case: Rust naga's
// interpolate.wgsl (tests/in/wgsl) with the two samplings the IR could not
// hold (first, either) and the frontend default on a float (location 4).
const interpolationShader = `
struct Varyings {
  @builtin(position) pos: vec4<f32>,
  @location(0) @interpolate(flat, first) flat_first: u32,
  @location(1) @interpolate(flat, either) flat_either: u32,
  @location(2) @interpolate(perspective, centroid) perspective_centroid: f32,
  @location(3) @interpolate(linear, sample) linear_sample: vec3<f32>,
  @location(4) default_float: vec2<f32>,
}
@vertex fn vs() -> Varyings {
  var o: Varyings;
  o.pos = vec4<f32>(1.0);
  o.flat_first = 1u;
  o.flat_either = 2u;
  o.perspective_centroid = 3.0;
  o.linear_sample = vec3<f32>(4.0);
  o.default_float = vec2<f32>(5.0);
  return o;
}
@fragment fn fs(v: Varyings) -> @location(0) vec4<f32> {
  return vec4<f32>(f32(v.flat_first + v.flat_either), v.perspective_centroid, v.linear_sample.x, v.default_float.x);
}`

// interpolationShaderNoFirst is interpolationShader without location 0:
// Rust's GLSL writer refuses first sampling (FirstSamplingNotSupported), so
// Rust tests GLSL with interpolate_compat.wgsl, which drops it the same way.
var interpolationShaderNoFirst = strings.Replace(strings.Replace(strings.Replace(interpolationShader,
	"  @location(0) @interpolate(flat, first) flat_first: u32,\n", "", 1),
	"  o.flat_first = 1u;\n", "", 1),
	"v.flat_first + v.flat_either", "v.flat_either", 1)

func parseInterpolation(t *testing.T, src string) *ir.Module {
	t.Helper()
	module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return module
}

// TestInterpolationIR: @interpolate(kind[, sampling]) sets Rust's two
// separate options (front/wgsl/parse/conv.rs map_interpolation, map_sampling);
// a missing sampling stays None, and the default applies to floats only
// (front/interpolator.rs apply_default_interpolation).
func TestInterpolationIR(t *testing.T) {
	samp := func(s ir.Sampling) *ir.Sampling { return &s }
	interp := func(i ir.Interpolation) *ir.Interpolation { return &i }
	for _, tc := range []struct {
		attr     string
		ty       string
		interp   *ir.Interpolation
		sampling *ir.Sampling
	}{
		{"@interpolate(flat)", "u32", interp(ir.InterpolationFlat), nil},
		{"@interpolate(flat, first)", "u32", interp(ir.InterpolationFlat), samp(ir.SamplingFirst)},
		{"@interpolate(flat, either)", "u32", interp(ir.InterpolationFlat), samp(ir.SamplingEither)},
		{"@interpolate(linear)", "f32", interp(ir.InterpolationLinear), nil},
		{"@interpolate(linear, center)", "f32", interp(ir.InterpolationLinear), samp(ir.SamplingCenter)},
		{"@interpolate(perspective, centroid)", "f32", interp(ir.InterpolationPerspective), samp(ir.SamplingCentroid)},
		{"@interpolate(perspective, sample)", "f32", interp(ir.InterpolationPerspective), samp(ir.SamplingSample)},
		{"", "vec2<f32>", interp(ir.InterpolationPerspective), samp(ir.SamplingCenter)},
		{"", "i32", nil, nil},
	} {
		t.Run(tc.attr+" "+tc.ty, func(t *testing.T) {
			src := fmt.Sprintf("@fragment fn fs(@location(0) %s x: %s) -> @location(0) vec4<f32> { return vec4<f32>(); }", tc.attr, tc.ty)
			module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			loc := (*module.EntryPoints[0].Function.Arguments[0].Binding).(ir.LocationBinding)
			if !equalPtr(loc.Interpolation, tc.interp) || !equalPtr(loc.Sampling, tc.sampling) {
				t.Errorf("interpolation %v sampling %v, want %v %v",
					show(loc.Interpolation), show(loc.Sampling), show(tc.interp), show(tc.sampling))
			}
		})
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func show[T any](p *T) string {
	if p == nil {
		return "None"
	}
	return fmt.Sprint(*p)
}

// TestInterpolationParseErrors: Rust's UnknownAttribute for an unknown word,
// and per_vertex needs its enable extension (conv.rs map_interpolation).
func TestInterpolationParseErrors(t *testing.T) {
	for _, tc := range []struct{ attr, want string }{
		{"@interpolate(curved)", "unknown attribute: `curved`"},
		{"@interpolate(flat, last)", "unknown attribute: `last`"},
		{"@interpolate(per_vertex)", "the `wgpu_per_vertex` enable extension is not enabled"},
	} {
		src := fmt.Sprintf("@fragment fn fs(@location(0) %s x: f32) -> @location(0) vec4<f32> { return vec4<f32>(); }", tc.attr)
		_, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", tc.attr, err, tc.want)
		}
	}
}

// TestInterpolationValidation: Rust's VaryingContext::validate
// (valid/interface.rs): integers need an explicit flat on a vertex output or
// a fragment input (upstream b051a87e), and only Rust's interpolation ×
// sampling pairs are valid.
func TestInterpolationValidation(t *testing.T) {
	const intFlat = "`@interpolate(flat)` must be explicitly specified for integer I/O"
	for _, tc := range []struct{ name, src, want string }{
		{"int fragment input", "@fragment fn fs(@location(0) x: i32) -> @location(0) vec4<f32> { return vec4<f32>(); }", intFlat},
		{"int vertex output", `struct O { @builtin(position) p: vec4<f32>, @location(0) x: vec2<u32>, }
@vertex fn vs() -> O { return O(vec4<f32>(), vec2<u32>()); }`, intFlat},
		{"flat center", "@fragment fn fs(@location(0) @interpolate(flat, center) x: u32) -> @location(0) vec4<f32> { return vec4<f32>(); }",
			"Cannot combine Flat interpolation with the Center sample type"},
		{"perspective first", "@fragment fn fs(@location(0) @interpolate(perspective, first) x: f32) -> @location(0) vec4<f32> { return vec4<f32>(); }",
			"Cannot combine Perspective interpolation with the First sample type"},
		{"linear either", "@fragment fn fs(@location(0) @interpolate(linear, either) x: f32) -> @location(0) vec4<f32> { return vec4<f32>(); }",
			"Cannot combine Linear interpolation with the Either sample type"},
		{"per_vertex sample", "enable wgpu_per_vertex;\n@fragment fn fs(@location(0) @interpolate(per_vertex, sample) x: array<f32, 3>) -> @location(0) vec4<f32> { return vec4<f32>(); }",
			"Per vertex can only have Center sampling or no sampling modifier"},
		{"per_vertex not array", "enable wgpu_per_vertex;\n@fragment fn fs(@location(0) @interpolate(per_vertex) x: f32) -> @location(0) vec4<f32> { return vec4<f32>(); }",
			"Per vertex fragment inputs must be an array of length 3."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := wgsl.Parse(tc.src, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want %q", err, tc.want)
			}
		})
	}
	// Integers that are not interpolated need no flat: a vertex input and a
	// fragment output.
	ok := `@vertex fn vs(@location(0) i: vec4<i32>) -> @builtin(position) vec4<f32> { return vec4<f32>(i); }
@fragment fn fs() -> @location(0) vec4<u32> { return vec4<u32>(); }`
	parseInterpolation(t, ok)
}

// SPIR-V decorations (spirv.core.grammar.json).
const (
	decNoPerspective = 13
	decFlat          = 14
	decCentroid      = 16
	decSample        = 17
	decLocation      = 30
	decPerVertexKHR  = 5285

	capSampleRateShading      = 35
	capFragmentBarycentricKHR = 5284
)

// spirvLocationDecorations maps each @location to the set of interpolation
// decorations of every variable at it (a vertex output and a fragment input
// share a location), one string per variable such as "Flat+Sample".
func spirvLocationDecorations(t *testing.T, bin []byte) map[uint32][]string {
	t.Helper()
	ops, operands := spirvOps(t, bin)
	location := map[uint32]uint32{}
	decs := map[uint32][]string{}
	names := map[uint32]string{decNoPerspective: "NoPerspective", decFlat: "Flat", decCentroid: "Centroid",
		decSample: "Sample", decPerVertexKHR: "PerVertexKHR"}
	var order []uint32
	for i, op := range ops {
		if op != opDecorate || len(operands[i]) < 2 {
			continue
		}
		id, dec := operands[i][0], operands[i][1]
		if dec == decLocation {
			location[id] = operands[i][2]
			order = append(order, id)
		} else if n, ok := names[dec]; ok {
			decs[id] = append(decs[id], n)
		}
	}
	out := map[uint32][]string{}
	for _, id := range order {
		out[location[id]] = append(out[location[id]], strings.Join(decs[id], "+"))
	}
	return out
}

// TestInterpolationSPIRV follows Rust's map_binding (back/spv/writer.rs):
// Flat and NoPerspective for the kinds, Centroid and Sample for the
// samplings, nothing for perspective, center, first and either; Sample needs
// SampleRateShading. The fragment output at location 0 has no decorations
// (VUID-StandaloneSpirv-Flat-06201).
func TestInterpolationSPIRV(t *testing.T) {
	bin, err := spirvBinary(parseInterpolation(t, interpolationShader), spirv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := spirvLocationDecorations(t, bin)
	want := map[uint32][]string{
		0: {"Flat", "Flat", ""}, // vertex output, fragment input, fragment output
		1: {"Flat", "Flat"},
		2: {"Centroid", "Centroid"},
		3: {"NoPerspective+Sample", "NoPerspective+Sample"},
		4: {"", ""},
	}
	for loc, w := range want {
		if fmt.Sprint(got[loc]) != fmt.Sprint(w) {
			t.Errorf("location %d: decorations %q, want %q", loc, got[loc], w)
		}
	}
	_, caps := spirvBuiltins(t, bin)
	if !containsU32(caps, capSampleRateShading) {
		t.Errorf("capabilities %v lack SampleRateShading", caps)
	}
}

func containsU32(s []uint32, v uint32) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestInterpolationGLSL follows Rust's glsl_interpolation and glsl_sampling
// (back/glsl/conv.rs): smooth, noperspective, flat; centroid and sample as
// auxiliary qualifiers; center and either write none; first is
// FirstSamplingNotSupported.
func TestInterpolationGLSL(t *testing.T) {
	module := parseInterpolation(t, interpolationShaderNoFirst)
	vs, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450, EntryPoint: &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "vs"}})
	if err != nil {
		t.Fatal(err)
	}
	fs, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450, EntryPoint: &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "fs"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"layout(location = 1) flat out uint _vs2fs_location1;",
		"layout(location = 2) smooth centroid out float _vs2fs_location2;",
		"layout(location = 3) noperspective sample out vec3 _vs2fs_location3;",
		"layout(location = 4) smooth out vec2 _vs2fs_location4;",
	} {
		if !strings.Contains(vs, line) {
			t.Errorf("vertex GLSL lacks %q:\n%s", line, vs)
		}
		in := strings.Replace(line, " out ", " in ", 1)
		if !strings.Contains(fs, in) {
			t.Errorf("fragment GLSL lacks %q:\n%s", in, fs)
		}
	}

	_, _, err = glsl.Compile(parseInterpolation(t, interpolationShader), glsl.Options{LangVersion: glsl.Version450,
		EntryPoint: &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "fs"}})
	if !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), "`First` sampling is unsupported") {
		t.Errorf("first sampling: error %v, want an ErrUnsupported naming First", err)
	}
}

// hlslLocationLines returns the struct member lines with semantic LOC<n>.
func hlslLocationLines(code string, loc int) []string {
	var lines []string
	suffix := fmt.Sprintf(" : LOC%d;", loc)
	for _, l := range strings.Split(code, "\n") {
		if strings.HasSuffix(l, suffix) {
			lines = append(lines, strings.TrimSpace(l))
		}
	}
	return lines
}

// TestInterpolationHLSL follows Rust's to_hlsl_str (back/hlsl/conv.rs):
// nointerpolation for flat whatever the sampling, noperspective for linear,
// centroid and sample; perspective and center write nothing.
func TestInterpolationHLSL(t *testing.T) {
	code, _, err := hlsl.Compile(parseInterpolation(t, interpolationShader), hlsl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for loc, prefix := range []string{"nointerpolation uint ", "nointerpolation uint ", "centroid float ",
		"noperspective sample float3 ", "float2 "} {
		lines := hlslLocationLines(code, loc)
		if len(lines) == 0 {
			t.Errorf("no member at LOC%d:\n%s", loc, code)
		}
		for _, l := range lines {
			if !strings.HasPrefix(l, prefix) {
				t.Errorf("LOC%d: %q, want prefix %q", loc, l, prefix)
			}
		}
	}
}

// TestInterpolationMSL follows Rust's ResolvedInterpolation
// (back/msl/mod.rs): flat for every flat sampling, <sampling>_perspective
// and <sampling>_no_perspective otherwise, center when there is none.
func TestInterpolationMSL(t *testing.T) {
	code, _, err := msl.Compile(parseInterpolation(t, interpolationShader), msl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for loc, want := range []string{"flat", "flat", "centroid_perspective", "sample_no_perspective", "center_perspective"} {
		attr := fmt.Sprintf("[[user(loc%d), %s]]", loc, want)
		if n := strings.Count(code, attr); n != 2 {
			t.Errorf("%d × %s, want 2 (vertex output, fragment input):\n%s", n, attr, code)
		}
	}
}

// psvInterpolationModes returns the PSV0 interpolation mode of each
// arbitrary-semantic (user varying) input and output element of a DXIL
// container, by its first semantic index (the @location).
func psvInterpolationModes(t *testing.T, blob []byte) (in, out map[uint32]uint8) {
	t.Helper()
	le := binary.LittleEndian
	parts := le.Uint32(blob[28:])
	var psv []byte
	for i := range parts {
		off := le.Uint32(blob[32+4*i:])
		if string(blob[off:off+4]) == "PSV0" {
			size := le.Uint32(blob[off+4:])
			psv = blob[off+8 : off+8+size]
		}
	}
	if psv == nil {
		t.Fatal("no PSV0 part")
	}
	infoSize := le.Uint32(psv)
	info := psv[4 : 4+infoSize]
	nIn, nOut := int(info[28]), int(info[29])
	pos := 4 + infoSize
	if resources := le.Uint32(psv[pos:]); resources > 0 {
		pos += 4
		pos += 4 + resources*le.Uint32(psv[pos:])
	} else {
		pos += 4
	}
	pos += 4 + le.Uint32(psv[pos:]) // string table
	semIndexCount := le.Uint32(psv[pos:])
	semIndexes := psv[pos+4:]
	pos += 4 + 4*semIndexCount
	elemSize := le.Uint32(psv[pos:])
	pos += 4
	in, out = map[uint32]uint8{}, map[uint32]uint8{}
	for i := range nIn + nOut {
		e := psv[pos+uint32(i)*elemSize:]
		if e[11] != 0 { // SemanticKind: Arbitrary
			continue
		}
		loc := le.Uint32(semIndexes[4*le.Uint32(e[4:]):])
		if i < nIn {
			in[loc] = e[13]
		} else {
			out[loc] = e[13]
		}
	}
	return in, out
}

// TestInterpolationDXIL: DXC's DXIL::InterpolationMode (DxilConstants.h) for
// the qualifiers Rust's HLSL writer gives each case: nointerpolation is
// Constant (1), centroid LinearCentroid (3), noperspective sample
// LinearNoperspectiveSample (7), none Linear (2).
func TestInterpolationDXIL(t *testing.T) {
	module := parseInterpolation(t, interpolationShader)
	want := map[uint32]uint8{0: 1, 1: 1, 2: 3, 3: 7, 4: 2}
	for _, sel := range []*backend.EntryPointSelector{
		{Stage: ir.StageVertex, Name: "vs"}, {Stage: ir.StageFragment, Name: "fs"},
	} {
		blob, err := dxilBlob(module, dxil.Options{EntryPoint: sel})
		if err != nil {
			t.Fatalf("%s: %v", sel.Name, err)
		}
		in, out := psvInterpolationModes(t, blob)
		got := out
		if sel.Stage == ir.StageFragment {
			got = in
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: interpolation modes %v, want %v", sel.Name, got, want)
		}
	}
}

// perVertexShader is Rust naga's per-vertex.wgsl.
const perVertexShader = `enable wgpu_per_vertex;

@fragment
fn fs_main(@location(0) @interpolate(per_vertex) v: array<f32, 3>) -> @location(0) vec4<f32> {
    return vec4(v[0], v[1], v[2], 1.0);
}`

// TestPerVertex: Rust writes PerVertexKHR with FragmentBarycentricKHR in
// SPIR-V, vertex_value<T> in MSL 4.0 and GetAttributeAtVertex in HLSL 6.1
// (tests/out/*/wgsl-per-vertex.*); GLSL has no per-vertex inputs
// (glsl_interpolation: unreachable) and DXIL does not write them yet: both
// refuse.
func TestPerVertex(t *testing.T) {
	module := parseInterpolation(t, perVertexShader)

	bin, err := spirvBinary(module, spirv.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := spirvLocationDecorations(t, bin)[0]; fmt.Sprint(got) != "[PerVertexKHR ]" {
		t.Errorf("SPIR-V location 0 decorations %q, want [PerVertexKHR, none]", got)
	}
	_, caps := spirvBuiltins(t, bin)
	if !containsU32(caps, capFragmentBarycentricKHR) {
		t.Errorf("SPIR-V capabilities %v lack FragmentBarycentricKHR", caps)
	}
	if !strings.Contains(string(bin), "SPV_KHR_fragment_shader_barycentric") {
		t.Error("SPIR-V lacks the SPV_KHR_fragment_shader_barycentric extension")
	}

	code, _, err := msl.Compile(module, msl.Options{LangVersion: msl.Version{Major: 4}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"metal::vertex_value<float> v [[user(loc0)]];",
		"const type_1 v = { varyings.v.get(metal::vertex_index::first), varyings.v.get(metal::vertex_index::second), varyings.v.get(metal::vertex_index::third) };",
	} {
		if !strings.Contains(code, s) {
			t.Errorf("MSL lacks %q:\n%s", s, code)
		}
	}
	// Below 4.0 the msl backend refuses it, as Rust (PerVertexNotSupported);
	// it never raises the version.
	_, _, err = msl.Compile(module, msl.Options{LangVersion: msl.Version3_1})
	if vErr := (*backend.VersionError)(nil); !errors.As(err, &vErr) || vErr.Space != backend.VersionMSL ||
		vErr.Required != (backend.Version{Major: 4}) {
		t.Errorf("MSL 3.1: error %v, want an MSL VersionError for 4.0", err)
	}

	code, _, err = hlsl.Compile(module, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"nointerpolation float v_1 : LOC0;",
		"float v[3] = { GetAttributeAtVertex(fragmentinput_fs_main.v_1, 0), GetAttributeAtVertex(fragmentinput_fs_main.v_1, 1), GetAttributeAtVertex(fragmentinput_fs_main.v_1, 2) };",
	} {
		if !strings.Contains(code, s) {
			t.Errorf("HLSL lacks %q:\n%s", s, code)
		}
	}
	var vErr *backend.VersionError
	if _, _, err := hlsl.Compile(module, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6}}); !errors.As(err, &vErr) ||
		vErr.Space != backend.VersionShaderModel {
		t.Errorf("HLSL 6.0: error %v, want a shader model VersionError", err)
	}

	if _, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450}); !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("GLSL: error %v, want ErrUnsupported", err)
	}
	if _, err := dxilBlob(module, dxil.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}}); !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("DXIL: error %v, want ErrUnsupported", err)
	}
}

// TestPerVertexStructMember: a per-vertex member of an input struct is
// declared and read as a per-vertex argument is. Rust's struct path writes
// the argument's name where the member's belongs (back/msl/writer.rs); the
// MSL here reads the member.
func TestPerVertexStructMember(t *testing.T) {
	module := parseInterpolation(t, `enable wgpu_per_vertex;
struct In {
  @location(0) @interpolate(per_vertex) v: array<f32, 3>,
  @location(1) c: vec4<f32>,
}
@fragment fn fs(i: In) -> @location(0) vec4<f32> { return i.c * i.v[1]; }`)

	code, _, err := msl.Compile(module, msl.Options{LangVersion: msl.Version{Major: 4}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"metal::vertex_value<float> v [[user(loc0)]];",
		// {}: In's padding member before c.
		"const In i = { { varyings.v.get(metal::vertex_index::first), varyings.v.get(metal::vertex_index::second), varyings.v.get(metal::vertex_index::third) }, {}, varyings.c };",
	} {
		if !strings.Contains(code, s) {
			t.Errorf("MSL lacks %q:\n%s", s, code)
		}
	}

	code, _, err = hlsl.Compile(module, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"nointerpolation float v : LOC0;",
		"In i = { { GetAttributeAtVertex(fragmentinput_fs.v, 0), GetAttributeAtVertex(fragmentinput_fs.v, 1), GetAttributeAtVertex(fragmentinput_fs.v, 2) }, fragmentinput_fs.c };",
	} {
		if !strings.Contains(code, s) {
			t.Errorf("HLSL lacks %q:\n%s", s, code)
		}
	}
}

// TestPerPrimitive: @per_primitive (Rust's per_primitive, mesh shaders)
// sets the binding's PerPrimitive. HLSL writes Rust's ": primitive"
// (back/hlsl/writer.rs write_semantic); the backends that write no mesh
// shaders refuse it.
func TestPerPrimitive(t *testing.T) {
	src := `enable wgpu_mesh_shader;
@fragment fn fs(@per_primitive @location(0) @interpolate(flat) x: u32) -> @location(0) vec4<f32> { return vec4<f32>(f32(x)); }`
	module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if loc := (*module.EntryPoints[0].Function.Arguments[0].Binding).(ir.LocationBinding); !loc.PerPrimitive {
		t.Fatal("PerPrimitive not set")
	}
	code, _, err := hlsl.Compile(module, hlsl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, " : LOC0 : primitive;") {
		t.Errorf("HLSL lacks the primitive qualifier:\n%s", code)
	}
	for name, compile := range map[string]func() error{
		"spirv": func() error { _, err := spirvBinary(module, spirv.Options{}); return err },
		// At MSL 3.0: below it Rust's uses_mesh_shaders, which counts a
		// per_primitive location, refuses the version first.
		"msl": func() error { _, _, err := msl.Compile(module, msl.Options{LangVersion: msl.Version3_0}); return err },
		"glsl": func() error {
			_, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})
			return err
		},
		"dxil": func() error { _, err := dxilBlob(module, dxil.Options{}); return err },
	} {
		// GLSL has no mesh shaders (Rust lists no MESH_SHADER for it), so its
		// capability list refuses a per_primitive location first.
		want := "per_primitive"
		if name == "glsl" {
			want = "Capabilities(MESH_SHADER)"
		}
		if err := compile(); !errors.Is(err, backend.ErrUnsupported) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want ErrUnsupported naming %s", name, err, want)
		}
	}
}
