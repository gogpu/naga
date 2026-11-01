// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package naga_test

import (
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/valid"
)

const compactShader = `
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@group(0) @binding(1) var<storage, read> unused_buf: array<u32>;
override unused_override: u32 = 1u;

fn unused_helper() -> u32 { return unused_buf[0]; }

@compute @workgroup_size(1)
fn main() { out[0] = 7u; }
`

// TestCompactKeepUnused pins the two modes of the public Compact, as Rust's
// compact::KeepUnused: Yes keeps the function, global and named override no
// entry point uses; No removes them. Both leave a module that validates.
func TestCompactKeepUnused(t *testing.T) {
	for _, tt := range []struct {
		keep                       naga.KeepUnused
		functions, globals, overds int
	}{
		{naga.KeepUnusedYes, 1, 2, 1},
		{naga.KeepUnusedNo, 0, 1, 0},
	} {
		m := lowerSource(t, compactShader)
		naga.Compact(m, tt.keep)
		if len(m.Functions) != tt.functions || len(m.GlobalVariables) != tt.globals || len(m.Overrides) != tt.overds {
			t.Errorf("Compact(%v): %d functions, %d globals, %d overrides; want %d, %d, %d", tt.keep,
				len(m.Functions), len(m.GlobalVariables), len(m.Overrides), tt.functions, tt.globals, tt.overds)
		}
		if _, err := valid.Validate(m, valid.Options{}); err != nil {
			t.Errorf("Compact(%v): module does not validate: %v", tt.keep, err)
		}
	}
}
