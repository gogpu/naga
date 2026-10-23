// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"
	"testing"
)

// The clip_distances output of a vertex entry point is written as vectors
// of up to four distances, SV_ClipDistance0 and SV_ClipDistance1, as Dawn
// writes it: each element of an array with a semantic would take a register
// of its own, and clip and cull distances may take two.
func TestClipDistancesArePackedInVectors(t *testing.T) {
	for _, tc := range []struct {
		n       int
		members []string
		value   string
	}{
		{1, []string{"float clip_distances0_ : SV_ClipDistance0;"}, "vertexoutput.clip_distances[0] };"},
		{3, []string{"float3 clip_distances0_ : SV_ClipDistance0;"},
			"float3(vertexoutput.clip_distances[0], vertexoutput.clip_distances[1], vertexoutput.clip_distances[2]) };"},
		{6, []string{"float4 clip_distances0_ : SV_ClipDistance0;", "float2 clip_distances1_ : SV_ClipDistance1;"},
			"float4(vertexoutput.clip_distances[0], vertexoutput.clip_distances[1], vertexoutput.clip_distances[2], vertexoutput.clip_distances[3]), " +
				"float2(vertexoutput.clip_distances[4], vertexoutput.clip_distances[5]) };"},
	} {
		got := compileWGSLToHLSL(t, fmt.Sprintf(`enable clip_distances;
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @builtin(clip_distances) clip_distances: array<f32, %d>,
}
@vertex
fn main() -> VertexOutput {
    var out: VertexOutput;
    out.clip_distances[0] = 0.5;
    return out;
}`, tc.n), nil)
		mustContain(t, got, append(tc.members, "float clip_distances["+fmt.Sprint(tc.n)+"] : SV_ClipDistance;", tc.value))
		if strings.Contains(got, "float[") {
			t.Errorf("array type written before the name:\n%s", got)
		}
	}
}
