// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
)

// dot4I8Packed and dot4U8Packed compile (P1-DXIL-067): they were refused
// ("unsupported binary math function: 34"). From SM 6.4 they are
// dx.op.dot4AddPacked (Dot4AddI8Packed, Dot4AddU8Packed) with a zero
// accumulator; below it the unpack-and-dot polyfill the HLSL writer uses,
// which DXC compiles to byte extraction and a mul + IMad / UMad chain.
func TestDot4PackedValidates(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read> a: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> o: array<i32, 2>;
@compute @workgroup_size(1) fn main() {
    o[0] = dot4I8Packed(a[0], a[1]);
    o[1] = i32(dot4U8Packed(a[0], a[1]));
}
`
	for _, minor := range []uint8{0, 4} {
		opts := DefaultOptions()
		opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: minor}
		blob, _, err := Compile(lowerSource(t, src), opts)
		if err != nil {
			t.Fatalf("SM 6.%d: Compile: %v", minor, err)
		}
		if err := Validate(blob, ValidateFull); err != nil {
			t.Fatalf("SM 6.%d: Validate: %v", minor, err)
		}
	}
}

// From SM 6.4 the packed dot products are one dx.op.dot4AddPacked each, with
// opcodes 163 (signed) and 164 (unsigned); below it no such call is made.
func TestDot4PackedIntrinsicFromSM64(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read> a: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> o: array<i32, 2>;
@compute @workgroup_size(1) fn main() {
    o[0] = dot4I8Packed(a[0], a[1]);
    o[1] = i32(dot4U8Packed(a[0], a[1]));
}
`
	for _, tt := range []struct {
		minor uint8
		want  []int64
	}{{4, []int64{163, 164}}, {0, nil}} {
		opts := DefaultOptions()
		opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: tt.minor}
		mod, fn := emitMainModuleOpts(t, src, opts)
		consts := map[int]int64{}
		for _, c := range mod.Constants {
			if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
				consts[c.ValueID] = c.IntValue
			}
		}
		var got []int64
		for _, bb := range fn.BasicBlocks {
			for _, in := range bb.Instructions {
				if in.Kind == module.InstrCall && in.CalledFunc != nil && in.CalledFunc.Name == "dx.op.dot4AddPacked.i32" {
					got = append(got, consts[in.Operands[0]])
				}
			}
		}
		if len(got) != len(tt.want) || (len(got) == 2 && (got[0] != tt.want[0] || got[1] != tt.want[1])) {
			t.Errorf("SM 6.%d: dot4AddPacked opcodes %v, want %v", tt.minor, got, tt.want)
		}
	}
}

// exp and log compile, lowered as DXC lowers them (P1-DXIL-078): DXIL's Exp
// and Log are base 2, so exp(x) = Exp(x * log2(e)) and log(x) = Log(x) *
// ln(2) (HLOperationLower.cpp TranslateExp, TranslateLog). They were refused
// ("unsupported unary math function: 28").
func TestExpLogValidate(t *testing.T) {
	const src = `
enable f16;
@group(0) @binding(0) var<storage, read_write> o: array<f32, 8>;
@compute @workgroup_size(1) fn main() {
    o[0] = exp(o[1]);
    o[2] = log(o[3]);
    let v = exp(vec2(o[4], o[5])) + log(vec2(o[6], o[7]));
    o[4] = v.x + v.y;
    o[5] = f32(exp(f16(o[6])) + log(f16(o[7])));
}
`
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 2}
	blob, _, err := Compile(lowerSource(t, src), opts)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// exp multiplies by log2(e) before Exp, log multiplies Log's result by
// ln(2): the constants are the f32 values DXC writes (0x3FF7154760000000 and
// 0x3FE62E4300000000 as doubles in its output).
func TestExpLogConstants(t *testing.T) {
	mod, fn := emitMainModule(t, `
@group(0) @binding(0) var<storage, read_write> o: array<f32, 4>;
@compute @workgroup_size(1) fn main() {
    o[0] = exp(o[1]);
    o[2] = log(o[3]);
}
`)
	floats := map[int]float64{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeFloat {
			floats[c.ValueID] = c.FloatValue
		}
	}
	var muls []float32
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrBinOp && len(in.Operands) == 3 && in.Operands[2] == int(emit.BinOpFMul) {
				for _, op := range in.Operands[:2] {
					if v, ok := floats[op]; ok {
						muls = append(muls, float32(v))
					}
				}
			}
		}
	}
	want := []float32{1.442695, 0.6931472}
	if len(muls) != 2 || muls[0] != want[0] || muls[1] != want[1] {
		t.Errorf("fmul constants %v, want %v", muls, want)
	}
}
