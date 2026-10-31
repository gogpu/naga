// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// TestDiscardEmitsDxOpDiscard (P0-DXIL-068): WGSL discard is dx.op.discard
// with a true condition (DXC's OC::Discard = 82; Mesa nir_to_dxil
// emit_discard). The emitter dropped the statement, so every fragment wrote
// its outputs: gg's convex, cover and sdf_render and g3d's standard shader
// discard. Static validators accept a shader without the call.
func TestDiscardEmitsDxOpDiscard(t *testing.T) {
	const src = `
@fragment fn main(@location(0) c: vec4<f32>) -> @location(0) vec4<f32> {
    if c.a < 0.5 { discard; }
    return c;
}`
	_, fn := emitMainModule(t, src)
	calls := 0
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrCall && in.CalledFunc != nil && in.CalledFunc.Name == "dx.op.discard" {
				calls++
			}
		}
	}
	if calls != 1 {
		t.Errorf("dx.op.discard calls = %d, want 1", calls)
	}
	compileWGSLToDXIL(t, src)
}
