// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestE2E_DeeplyNestedStorageStore is the regression test for P2-HLSL-036: a
// storage store's emitted indentation is not a recursion depth. WGSL allows
// brace nesting up to 127, so a store 30 statements deep must compile.
func TestE2E_DeeplyNestedStorageStore(t *testing.T) {
	const depth = 30

	var src strings.Builder
	src.WriteString("struct Buf { value: f32 }\n")
	src.WriteString("@group(0) @binding(0) var<storage, read_write> buf: Buf;\n")
	src.WriteString("@compute @workgroup_size(1)\n")
	src.WriteString("fn main(@builtin(global_invocation_id) gid: vec3<u32>) {\n")
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&src, "if gid.x > %du {\n", i)
	}
	src.WriteString("buf.value = 1.0;\n")
	for i := 0; i < depth; i++ {
		src.WriteString("}\n")
	}
	src.WriteString("}\n")

	code := compileWGSLToHLSL(t, src.String())

	assertContains(t, code, "buf.Store(")
}
