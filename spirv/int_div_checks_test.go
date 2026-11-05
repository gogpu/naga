// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package spirv_test

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// SPIR-V opcodes the test counts (SPIR-V spec, section 3.52).
const (
	opFunction     = 54
	opFunctionCall = 57
	opUDiv         = 134
	opSDiv         = 135
	opUMod         = 137
	opSRem         = 138
)

func opcodeCounts(t *testing.T, spv []byte) map[uint32]int {
	t.Helper()
	const headerBytes = 20
	counts := map[uint32]int{}
	for off := headerBytes; off+4 <= len(spv); {
		word := binary.LittleEndian.Uint32(spv[off:])
		n := int(word >> 16)
		if n == 0 || off+n*4 > len(spv) {
			t.Fatalf("malformed SPIR-V instruction at byte %d", off)
		}
		counts[word&0xFFFF]++
		off += n * 4
	}
	return counts
}

// TestSkipIntDivChecks: Options.SkipIntDivChecks drops the guard helpers of
// integer `/` and `%`, as Rust naga's emit_int_div_checks = false does
// (back/spv/mod.rs, back/spv/writer.rs write_wrapped_functions): OpSDiv,
// OpUDiv and OpUMod are written raw. Signed `%` keeps its helper whatever the
// option says, because OpSRem is poison for negative operands in Vulkan
// without VK_KHR_maintenance8 (gfx-rs/wgpu#8191); the helper then computes
// a - b * (a / b), so OpSRem is never written.
func TestSkipIntDivChecks(t *testing.T) {
	const prelude = `
struct Data { a: array<i32, 2>, u: array<u32, 2>, v: array<vec2<i32>, 2>, r: array<i32, 4> }
@group(0) @binding(0) var<storage, read_write> d: Data;
`
	for _, tc := range []struct {
		name string
		body string
		// default options and SkipIntDivChecks: OpFunction (main plus
		// helpers), OpFunctionCall, then the raw opcodes.
		def, skip counts
	}{
		{
			name: "u32 div and mod",
			body: `d.u[0] = d.u[0] / d.u[1]; d.u[1] = d.u[0] % d.u[1];`,
			def:  counts{functions: 3, calls: 2, udiv: 1, umod: 1},
			skip: counts{functions: 1, calls: 0, udiv: 1, umod: 1},
		},
		{
			name: "i32 div",
			body: `d.a[0] = d.a[0] / d.a[1];`,
			def:  counts{functions: 2, calls: 1, sdiv: 1},
			skip: counts{functions: 1, calls: 0, sdiv: 1},
		},
		{
			name: "vec2<i32> div",
			body: `d.v[0] = d.v[0] / d.v[1];`,
			def:  counts{functions: 2, calls: 1, sdiv: 1},
			skip: counts{functions: 1, calls: 0, sdiv: 1},
		},
		{
			name: "i32 mod keeps its helper",
			body: `d.a[0] = d.a[0] % d.a[1];`,
			def:  counts{functions: 2, calls: 1, sdiv: 1},
			skip: counts{functions: 2, calls: 1, sdiv: 1},
		},
		{
			name: "i32 div and mod",
			body: `d.r[0] = d.a[0] / d.a[1]; d.r[1] = d.a[0] % d.a[1];`,
			def:  counts{functions: 3, calls: 2, sdiv: 2},
			skip: counts{functions: 2, calls: 1, sdiv: 2},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, _, err := wgsl.Parse(prelude+`@compute @workgroup_size(1) fn main() { `+tc.body+` }`, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, run := range []struct {
				name string
				opts spirv.Options
				want counts
			}{
				{"default", spirv.Options{}, tc.def},
				{"SkipIntDivChecks", spirv.Options{SkipIntDivChecks: true}, tc.skip},
			} {
				spv, _, err := spirv.Compile(module, run.opts)
				if err != nil {
					t.Fatalf("%s: %v", run.name, err)
				}
				c := opcodeCounts(t, spv)
				got := counts{int(c[opFunction]), int(c[opFunctionCall]), int(c[opUDiv]), int(c[opSDiv]), int(c[opUMod])}
				if got != run.want {
					t.Errorf("%s: %+v, want %+v", run.name, got, run.want)
				}
				if c[opSRem] != 0 {
					t.Errorf("%s: %d OpSRem, want none", run.name, c[opSRem])
				}
			}
		})
	}
}

// counts is a row of opcode counts, in the order TestSkipIntDivChecks reads them.
type counts struct{ functions, calls, udiv, sdiv, umod int }
