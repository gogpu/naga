// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/emit"
)

const (
	legacyLayout = "e-m:e-p:32:32-i1:32-i8:32-i16:32-i32:32-i64:64-f16:32-f32:32-f64:64-n8:16:32:64"
	nativeLayout = "e-m:e-p:32:32-i1:32-i8:8-i16:16-i32:32-i64:64-f16:16-f32:32-f64:64-n8:16:32:64"

	flagLowPrecisionPresent   = 0x20
	flagUseNativeLowPrecision = 0x800000
)

// TestNativeLowPrecisionModule (P2-DXIL-098): a module with 16-bit types
// is compiled as DXC compiles it with -enable-16bit-types (what wgpu-hal
// passes for SHADER_F16 / SHADER_I16): the native data layout
// (kNewLayoutString, chosen by HLOperationLower.cpp:62-66 when the module
// does not use min precision) and UseNativeLowPrecision (bit 23, set from
// !UseMinPrecision for the whole module, DxilShaderFlags.cpp:412) on every
// entry point, LowPrecisionPresent (bit 5) only where a 16-bit value is
// used (DxilShaderFlags.cpp:829). We wrote the legacy layout always and both
// flags only for an entry point that uses a 16-bit value; DXC's golden for
// conversion-float-to-int (f16 consts folded away) differs in those two
// lines. A module without 16-bit types keeps the legacy layout and no flag.
func TestNativeLowPrecisionModule(t *testing.T) {
	cases := []struct {
		name, src    string
		layout       string
		flags, clear uint64
	}{
		{"f16 declared, not used by the entry point", `
enable f16;
const H = 1.5h;
@group(0) @binding(0) var<storage, read_write> out: array<i32>;
@compute @workgroup_size(1)
fn main() { out[0] = i32(H); }`, nativeLayout, flagUseNativeLowPrecision, flagLowPrecisionPresent},
		{"f16 used", `
enable f16;
@group(0) @binding(0) var<storage, read_write> out: array<f32>;
@compute @workgroup_size(1)
fn main() { var h = f16(out[1]); h = h * 2.0h; out[0] = f32(h); }`, nativeLayout, flagUseNativeLowPrecision | flagLowPrecisionPresent, 0},
		{"no 16-bit types", `
@group(0) @binding(0) var<storage, read_write> out: array<f32>;
@compute @workgroup_size(1)
fn main() { out[0] = 1.0; }`, legacyLayout, 0, flagUseNativeLowPrecision | flagLowPrecisionPresent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 2}
			irMod, err := prepareModule(lowerSource(t, c.src))
			if err != nil {
				t.Fatal(err)
			}
			if err := prepareEntryPoint(irMod, &irMod.EntryPoints[0], opts); err != nil {
				t.Fatal(err)
			}
			if _, err := lowerForEmit(irMod, opts); err != nil {
				t.Fatal(err)
			}
			mod, flags, err := emit.EmitWithFlags(irMod, emit.EmitOptions{
				ShaderModelMajor: 6, ShaderModelMinor: 2,
				ReachableGlobals: reachableGlobalsForEntry(irMod, &irMod.EntryPoints[0]),
			})
			if err != nil {
				t.Fatal(err)
			}
			if mod.DataLayout != c.layout {
				t.Errorf("datalayout %q, want %q", mod.DataLayout, c.layout)
			}
			if flags&c.flags != c.flags || flags&c.clear != 0 {
				t.Errorf("shader flags %#x: want %#x set and %#x clear", flags, c.flags, c.clear)
			}

			m := lowerSource(t, c.src)
			blob, _, err := Compile(m, opts)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}
}
