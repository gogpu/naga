// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"runtime"
	"strings"
	"testing"
)

// clampOptions clamps frag_depth to the range at offset of the immediates
// buffer at slot 5 of fs_main.
func clampOptions(offset uint32) Options {
	slot := uint8(5)
	opts := DefaultOptions()
	opts.PerEntryPointMap = map[string]EntryPointResources{"fs_main": {ImmediatesBuffer: &slot}}
	opts.ClampFragDepth = &ClampFragDepth{Offset: offset}
	return opts
}

// TestClampFragDepth: with ClampFragDepth, a fragment entry point that
// returns frag_depth, alone or in a struct, takes its immediates buffer and
// returns frag_depth clamped to the range at the offset given in it, on
// every return. Without the option, or in an entry point that doesn't write
// frag_depth, nothing changes.
func TestClampFragDepth(t *testing.T) {
	const (
		param = "constant char* _immediates [[buffer(5)]]"
		rng   = "constant float* _frag_depth_range = reinterpret_cast<constant float*>(_immediates + 8);"
	)
	for name, tc := range map[string]struct {
		source string
		values []string
	}{
		"result": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 {
	if p.x > 2.0 { return 0.25; }
	return p.z * 2.0;
}`, []string{"0.25", "p.z * 2.0"}},
		"struct": {`
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z * 2.0); }`, []string{"_tmp.depth"}},
		"none": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`, nil},
	} {
		for _, clamp := range []bool{false, true} {
			opts := DefaultOptions()
			if clamp {
				opts = clampOptions(8)
			}
			got := compileWGSLWithOpts(t, tc.source, opts)
			want := clamp && tc.values != nil
			for _, line := range []string{param, rng} {
				if strings.Contains(got, line) != want {
					t.Errorf("%s, clamp %v: %q: %v, want %v:\n%s", name, clamp, line, !want, want, got)
				}
			}
			for _, v := range tc.values {
				clamped := "metal::clamp(" + v + ", _frag_depth_range[0], _frag_depth_range[1])"
				if strings.Contains(got, clamped) != want {
					t.Errorf("%s, clamp %v: %q: %v, want %v:\n%s", name, clamp, clamped, !want, want, got)
				}
			}
			if runtime.GOOS == "darwin" {
				verifyMSLWithXcrun(t, got)
			}
		}
	}
}

// TestClampFragDepthWithImmediates: an entry point with immediate data of its
// own finds the range in that buffer past the data, through the address of
// its immediates argument, as the buffer slot holds one argument. An offset
// inside the data, one that isn't a multiple of 4, or no immediates buffer
// is an error.
func TestClampFragDepthWithImmediates(t *testing.T) {
	const src = `
struct Immediates { tint: vec4<f32> }
var<immediate> im: Immediates;
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 { return p.z * im.tint.x; }`
	got := compileWGSLWithOpts(t, src, clampOptions(16))
	for _, want := range []string{
		"constant Immediates& im [[buffer(5)]]",
		"constant float* _frag_depth_range = reinterpret_cast<constant float*>(reinterpret_cast<constant char*>(&im) + 16);",
		"metal::clamp(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q not in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "_immediates") {
		t.Errorf("a second argument at the immediates slot:\n%s", got)
	}
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, got)
	}

	noBuffer := clampOptions(16)
	noBuffer.PerEntryPointMap = nil
	for name, tc := range map[string]struct {
		opts Options
		want string
	}{
		"inside the data":    {clampOptions(8), "overlaps"},
		"unaligned":          {clampOptions(18), "multiple of 4"},
		"no immediates slot": {noBuffer, "ImmediatesBuffer"},
	} {
		_, _, err := Compile(lowerWGSLForTest(t, src), tc.opts)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, want one with %q", name, err, tc.want)
		}
	}
}
