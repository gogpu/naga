// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/module"
)

const int64ShiftPrelude = `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 8>;
`

// 64-bit shifts and negation (P1-DXIL-080). An LLVM shift takes both operands
// in the type of the shifted value, but a WGSL shift amount is always u32: the
// emitter put an i32 amount on an i64 shl/lshr/ashr, and negated an i64 with
// an i32 zero, so the bitcode record was malformed ("Invalid record") and
// neither IDxcValidator nor dxc -dumpbin could read the module. DXC widens the
// amount with zext and masks it with 63 (HLSL, like WGSL, takes the amount
// modulo the bit width), and negates as `sub i64 0, x`.
var int64ShiftCases = []struct{ name, body string }{
	{"u64 shl constant", `
    let a = u64(data[0]) << 3u;
    data[1] = u32(a >> 32u);
    data[2] = u32(a);`},
	{"u64 shr constant", `
    let a = u64(data[0]) * 4000000000lu;
    data[1] = u32(a >> 32u);`},
	{"i64 shr constant", `
    let a = i64(i32(data[0])) >> 3u;
    data[1] = u32(a);`},
	{"i64 negate", `
    let a = -i64(data[0]);
    data[1] = u32(a);
    data[2] = u32(a >> 32u);`},
	{"u64 shl run-time amount", `
    let a = u64(data[0]) << data[3];
    data[1] = u32(a);
    data[2] = u32(u64(data[0]) >> data[4]);`},
	{"vec2<u64> shl run-time amount", `
    let a = vec2<u64>(u64(data[0]), u64(data[1])) << vec2<u32>(data[3], 7u);
    data[1] = u32(a.x);
    data[2] = u32(a.y);`},
}

func TestInt64ShiftAndNegateValidate(t *testing.T) {
	for _, tt := range int64ShiftCases {
		t.Run(tt.name, func(t *testing.T) {
			src := int64ShiftPrelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
			blob, _, err := Compile(lowerSource(t, src), DefaultOptions())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

// Every shift and every integer sub of the emitted function has both operands
// in the result's type, whatever platform runs the test: the operand types
// are what the bitcode reader checks.
func TestInt64ShiftOperandTypes(t *testing.T) {
	for _, tt := range int64ShiftCases {
		t.Run(tt.name, func(t *testing.T) {
			src := int64ShiftPrelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
			mod, fn := emitMainModule(t, src)
			types := valueTypes(mod, fn)
			for _, bb := range fn.BasicBlocks {
				for _, in := range bb.Instructions {
					if in.Kind != module.InstrBinOp || len(in.Operands) < 3 {
						continue
					}
					for _, op := range in.Operands[:2] {
						if ty, ok := types[op]; ok && ty != in.ResultType {
							t.Errorf("binop %d (%%%d): operand %%%d is %v, result is %v",
								in.Operands[2], in.ValueID, op, ty, in.ResultType)
						}
					}
				}
			}
		})
	}
}

// Integer abs, sign and the strength-reduced forms of 64-bit arithmetic
// validate (P1-DXIL-080, found on the Rust int64 shader). abs of an integer
// went to FAbs, whose overloads are float only ("DXIL intrinsic overload must
// be valid"), at any width; DXC lowers it to IMax(x, 0 - x), and an unsigned
// abs to x. sign, ~x, x * 2^N, x % 2^N and x - C used i32 constants with an
// i64 operand ("Invalid record").
func TestInt64MathValidates(t *testing.T) {
	const prelude = `
struct U { b: i32, i: i64, v: u64, i2: vec2<i64> }
@group(0) @binding(0) var<storage> s: U;
@group(0) @binding(1) var<storage, read_write> o: U;
`
	tests := []struct{ name, body string }{
		{"abs i32", `o.b = abs(s.b);`},
		{"abs i64", `o.i = abs(s.i);`},
		{"abs vec2<i64>", `o.i2 = abs(s.i2);`},
		{"abs u64", `o.v = abs(s.v);`},
		{"sign i64", `o.i = sign(s.i);`},
		{"sign vec2<i64>", `o.i2 = sign(s.i2);`},
		{"bitwise not u64", `o.v = ~s.v;`},
		{"multiply u64 by a power of two", `o.v = s.v * 8lu;`},
		{"modulo u64 by a power of two", `o.v = s.v % 8lu;`},
		{"subtract a u64 constant", `o.v = s.v - 8lu;`},
	}
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 3} // 64-bit storage elements
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := prelude + "@compute @workgroup_size(1) fn main() {" + tt.body + "\n}\n"
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

// valueTypes maps the constants and the instruction results of fn to their
// types.
func valueTypes(mod *module.Module, fn *module.Function) map[int]*module.Type {
	types := map[int]*module.Type{}
	for _, c := range mod.Constants {
		types[c.ValueID] = c.ConstType
	}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.HasValue {
				types[in.ValueID] = in.ResultType
			}
		}
	}
	return types
}
