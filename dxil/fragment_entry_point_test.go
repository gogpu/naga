package dxil

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// A vertex stage compiled for a fragment stage writes only the outputs the
// fragment stage reads, as the HLSL writer does: D3D12 matches the two
// signatures register for register. That holds also when the fragment
// stage reads no location, and the caller's module keeps its bindings.
func TestFragmentEntryPointFiltersVertexOutputs(t *testing.T) {
	vs := lowerForTest(t, `
struct VertexOut {
    @builtin(position) position: vec4<f32>,
    @location(0) unused: vec4<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) unused2: f32,
}
@vertex fn vs_main(@builtin(vertex_index) i: u32) -> VertexOut {
    return VertexOut(vec4(f32(i)), vec4(1.0), vec2(0.5), 2.0);
}
`)
	for _, tc := range []struct {
		name string
		fs   string
		want uint32
	}{
		{"reads uv", `@fragment fn fs_main(@location(1) uv: vec2<f32>) -> @location(0) vec4<f32> { return vec4(uv, 0.0, 1.0); }`, 2},
		{"reads no location", `@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`, 1},
		{"no fragment stage", "", 4},
	} {
		opts := DefaultOptions()
		if tc.fs != "" {
			fs := lowerForTest(t, tc.fs)
			opts.FragmentEntryPoint = &FragmentEntryPoint{Module: fs, Function: &fs.EntryPoints[0].Function}
		}
		blob, err := Compile(vs, opts)
		if err != nil {
			t.Fatalf("%s: Compile: %v", tc.name, err)
		}
		osg := dxbcParts(t, blob)["OSG1"]
		if got := binary.LittleEndian.Uint32(osg[0:4]); got != tc.want {
			t.Errorf("%s: %d output signature elements, want %d", tc.name, got, tc.want)
		}
	}
	for _, m := range vs.Types[vs.EntryPoints[0].Function.Result.Type].Inner.(ir.StructType).Members {
		if m.Binding == nil {
			t.Errorf("Compile took the binding off the caller's %s", m.Name)
		}
	}
}

func lowerForTest(t *testing.T, src string) *ir.Module {
	t.Helper()
	tokens, err := wgsl.NewLexer(src).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	m, err := wgsl.LowerWithSource(ast, src)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
