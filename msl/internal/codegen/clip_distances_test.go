package codegen

import (
	"strings"
	"testing"
)

// @builtin(clip_distances) is an array output Metal reads only from a member
// with the [[clip_distance]] attribute, declared as a raw array; the entry
// point copies the WGSL array into it element by element, as Rust naga writes
// it (back/msl/mod.rs ResolvedBinding for ClipDistances, back/msl/writer.rs
// the return of an entry point). The member was written as a plain wrapped
// array without the attribute, so Metal clipped nothing
// (BUG-MSL-CLIP-DISTANCES-001).
func TestClipDistancesOutput(t *testing.T) {
	const src = `
enable clip_distances;
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @builtin(clip_distances) clip_distances: array<f32, 2>,
}

@vertex
fn main() -> VertexOutput {
    var out: VertexOutput;
    out.clip_distances[0] = 0.5;
    out.clip_distances[1] = -0.5;
    return out;
}
`
	out := compileWGSL(t, src)
	for _, want := range []string{
		"float clip_distances [[clip_distance]] [2];",
		"return main_Output { _tmp.position, {_tmp.clip_distances.inner[0], _tmp.clip_distances.inner[1]} };",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
