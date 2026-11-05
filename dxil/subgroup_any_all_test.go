// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

// subgroupAny and subgroupAll return a bool, the i1 that dx.op.waveAnyTrue
// and dx.op.waveAllTrue return (P1-DXIL-085). The emitter widened the result
// to an i32, so a select on it was "Invalid type for value" and a `!` of it
// (an xor with an i1 true) "Invalid record". DXC keeps the i1 and widens it
// only where the HLSL converts it (`zext i1 to i32`).
func TestSubgroupAnyAllValidate(t *testing.T) {
	const prelude = `
enable subgroups;
@group(0) @binding(0) var<storage, read_write> r: array<u32, 192>;
`
	tests := []struct{ name, body string }{
		{"select on subgroupAny", `r[lid] = select(0u, 1u, subgroupAny(lid == 5u));`},
		{"select on subgroupAll", `r[lid] = select(0u, 1u, subgroupAll(true));`},
		{"negated subgroupAll", `r[lid] = u32(!subgroupAll(sid != 0u));`},
		{"subgroupAny in a condition", `if subgroupAny(lid > 3u) { r[lid + 64u] = 2u; }`},
	}
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 0}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := prelude + "@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) lid: u32, " +
				"@builtin(subgroup_invocation_id) sid: u32) {" + tt.body + "\n}\n"
			blob, _, err := Compile(lowerSource(t, src), opts)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}
