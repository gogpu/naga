// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/viewid"
	"github.com/gogpu/naga/internal/typifier"
)

// viewIndexVS reads @builtin(view_index) and forwards it into the position,
// so the position output is view dependent.
const viewIndexVS = `
@vertex
fn main(@builtin(vertex_index) i: u32, @builtin(view_index) vi: u32) -> @builtin(position) vec4<f32> {
    return vec4(f32(i) + f32(vi), 0.0, 0.0, 1.0);
}
`

// viewIndexFS reads @builtin(view_index) and multiplies the interpolated
// color by it, so the color output is view dependent while the pass-through
// of the location input alone would not be.
const viewIndexFS = `
@fragment
fn main(@builtin(view_index) vi: u32, @location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color * f32(vi);
}
`

// viewIndexPartialFS writes one view-dependent output next to one that does
// not depend on the view index, so only the first may carry mask bits.
const viewIndexPartialFS = `
struct Out {
    @location(0) a: vec4<f32>,
    @location(1) b: vec4<f32>,
}

@fragment
fn main(@builtin(view_index) vi: u32, @location(0) color: vec4<f32>) -> Out {
    var o: Out;
    o.a = color * f32(vi);
    o.b = color;
    return o;
}
`

// sm61 asks for shader model 6.1, the first model with dx.op.viewID.
func sm61(t *testing.T) Options {
	t.Helper()
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 1}
	return opts
}

// TestViewIndexPSV0DependencyTable (P2-DXIL-109): a shader that reads
// @builtin(view_index) failed IDxcValidator with
//
//	HRESULT 0x80aa0013: 'PSV0 part' is not well-formed
//
// at shader model 6.1 in both stages. PSVRuntimeInfo1.UsesViewID was set,
// but the PSV0 part carried no ViewID dependency masks, while DXC's
// SimplePSV walker (DxilContainerValidation.cpp:674-692) expects one
// PSVComputeMaskDwordsFromVectors(SigOutputVectors[i]) dword mask per
// non-empty output stream before the input-to-output tables, and requires
// the part to end exactly there. The module's dx.viewIdState metadata has
// the same layout and must grow the same mask, or the content verifier
// reports a ViewIDState mismatch.
func TestViewIndexPSV0DependencyTable(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"vertex", viewIndexVS},
		{"fragment", viewIndexFS},
		{"fragment partial mask", viewIndexPartialFS},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			blob, _, err := Compile(lowerSource(t, c.src), sm61(t))
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

// TestViewIDDependentOutputs (P2-DXIL-109) pins the mask content: only the
// output components whose value flows from @builtin(view_index) carry a
// bit, in the packed component space (row*4+col) that PSV0 and
// dx.viewIdState share. This is what dxc produces for the equivalent HLSL
// (viewIDMask 0x1 for the vertex shader above, 0xF for the fragment).
func TestViewIDDependentOutputs(t *testing.T) {
	cases := []struct {
		name, src, entry string
		wantMask         uint32
	}{
		{"vertex position.x", viewIndexVS, "main", 0x1},
		{"fragment color", viewIndexFS, "main", 0xF},
		{"fragment partial", viewIndexPartialFS, "main", 0xF},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := lowerSource(t, c.src)
			ep := findEntryPoint(t, m, c.entry)
			types, err := typifier.Function(m, &ep.Function)
			if err != nil {
				t.Fatal(err)
			}
			deps := analyzePSVViewID(m, ep, types, buildGraphicsPSVSigs(m, ep))
			if deps == nil {
				t.Fatal("analyzePSVViewID returned nil")
			}
			wantDwords := viewidMaskDwords(deps)
			if len(deps.ViewIDDependentOutputs) != int(wantDwords) {
				t.Fatalf("mask dwords: got %d, want %d", len(deps.ViewIDDependentOutputs), wantDwords)
			}
			if got := deps.ViewIDDependentOutputs[0]; got != c.wantMask {
				t.Errorf("mask: got %#x, want %#x", got, c.wantMask)
			}
			for i := 1; i < len(deps.ViewIDDependentOutputs); i++ {
				if deps.ViewIDDependentOutputs[i] != 0 {
					t.Errorf("mask dword %d: got %#x, want 0", i, deps.ViewIDDependentOutputs[i])
				}
			}
		})
	}
}

// TestViewIDMaskAbsentWithoutViewIndex checks the flip side: a shader that
// does not read the view index produces an all-zero mask, so no output is
// reported as view dependent.
func TestViewIDMaskAbsentWithoutViewIndex(t *testing.T) {
	const src = `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
	m := lowerSource(t, src)
	ep := findEntryPoint(t, m, "main")
	types, err := typifier.Function(m, &ep.Function)
	if err != nil {
		t.Fatal(err)
	}
	deps := analyzePSVViewID(m, ep, types, buildGraphicsPSVSigs(m, ep))
	if deps == nil {
		t.Fatal("analyzePSVViewID returned nil")
	}
	for i := range deps.ViewIDDependentOutputs {
		if deps.ViewIDDependentOutputs[i] != 0 {
			t.Errorf("mask dword %d: got %#x, want 0", i, deps.ViewIDDependentOutputs[i])
		}
	}
}

// viewidMaskDwords is the dword count of the ViewID mask as the PSV0
// writer sizes it: MaskDwords(SigOutputVectors).
func viewidMaskDwords(deps *viewid.Deps) uint32 {
	return (deps.SigOutputVectors + 7) >> 3
}
