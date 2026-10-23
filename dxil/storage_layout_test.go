package dxil

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// Loads, stores and atomics of a storage buffer address the struct members,
// array elements, matrix columns and vector components they index where
// WGSL lays them out, however the indices nest and whichever of them are
// run-time ones. The chains of constant indices into a vector read the
// element instead, or the element at the vector's index, or failed.
func TestStorageBufferAddresses(t *testing.T) {
	const decls = `
struct S { a: u32, v: array<vec4<f32>> }
struct T { m: mat4x4<f32>, x: vec4<f32> }
struct A { total: atomic<u32>, counts: array<atomic<u32>> }
@group(0) @binding(0) var<storage, read> data: array<vec4<f32>>;
@group(0) @binding(1) var<storage, read> s: S;
@group(0) @binding(2) var<storage, read_write> t: array<T>;
@group(0) @binding(3) var<storage, read_write> a: A;
`
	for _, tc := range []struct {
		body, call, want string
	}{
		{"t[0].x.x = data[3].z;", "dx.op.bufferLoad", "56"},
		{"t[0].x.x = s.v[3].z;", "dx.op.bufferLoad", "72"},
		{"t[0].x.x = t[2].m[1].z;", "dx.op.bufferLoad", "184"},
		{"t[2].x.y = 1.0;", "dx.op.bufferStore", "228"},
		{"t[0].x.x = data[id.x].z;", "dx.op.bufferLoad", "add(shl(tid(0),4),8)"},
		{"t[0].x.x = data[3][id.y];", "dx.op.bufferLoad", "add(shl(tid(1),2),48)"},
		{"t[0].x.x = data[id.x][id.y];", "dx.op.bufferLoad", "add(shl(tid(0),4),shl(tid(1),2))"},
		{"t[id.x].m[id.y].w = 1.0;", "dx.op.bufferStore", "add(add(mul(tid(0),80),shl(tid(1),4)),12)"},
		{"atomicAdd(&a.counts[id.x], 1u);", "dx.op.atomicBinOp", "add(shl(tid(0),2),4)"},
	} {
		mod, fn := emitMainModule(t, decls+"@compute @workgroup_size(1) fn main(@builtin(local_invocation_id) id: vec3<u32>) {\n\t"+tc.body+"\n}\n")
		operand := 2
		if tc.call == "dx.op.atomicBinOp" {
			operand = 3
		}
		var got []string
		for _, id := range callOperands(fn, tc.call, operand) {
			got = append(got, valueTree(mod, fn, id))
		}
		if !slices.Equal(got, []string{tc.want}) {
			t.Errorf("%s: %s addresses %v, want [%s]", tc.body, tc.call, got, tc.want)
		}
	}
}

// valueTree renders the integer arithmetic that computes value id in fn:
// constants as their values, a thread ID in group as tid(component), and
// the add, mul and shl it is built from.
func valueTree(mod *module.Module, fn *module.Function, id int) string {
	for _, c := range mod.Constants {
		if c.ValueID == id && c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			return fmt.Sprint(c.IntValue)
		}
	}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if !in.HasValue || in.ValueID != id {
				continue
			}
			switch {
			case in.Kind == module.InstrBinOp:
				op := map[int]string{0: "add", 2: "mul", 7: "shl"}[in.Operands[2]]
				return op + "(" + valueTree(mod, fn, in.Operands[0]) + "," + valueTree(mod, fn, in.Operands[1]) + ")"
			case in.Kind == module.InstrCall && strings.HasPrefix(in.CalledFunc.Name, "dx.op.threadIdInGroup"):
				return "tid(" + valueTree(mod, fn, in.Operands[1]) + ")"
			}
		}
	}
	return "?"
}

// A constant index into a read-only storage buffer leaves the buffer to the
// load, as one into a read-write buffer does. It indexed the handle of the
// buffer with a GEP, which IDxcValidator refuses.
func TestReadOnlyStorageIndexEmitsNoGEP(t *testing.T) {
	_, fn := emitMainModule(t, `
struct P { a: u32, v: array<u32> }
@group(0) @binding(0) var<storage, read> p: P;
@group(0) @binding(1) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn main() {
	o[0] = p.v[0];
}
`)
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrGEP {
				t.Errorf("GEP %v in a shader that only loads and stores storage buffers", in.Operands)
			}
		}
	}
}

// A load or store of a whole value whose scalars have padding between them,
// a struct of two vec3s or a mat3x3, reads and writes each scalar at its
// WGSL offset. It read and wrote them one after the other from the first,
// the padding included, and matrix columns 16 bytes apart whatever their
// rows.
func TestStorageBufferPaddedValues(t *testing.T) {
	const decls = `
struct P { pos: vec3<f32>, vel: vec3<f32> }
struct M { a: f32, m: mat3x3<f32>, n: mat2x2<f32> }
@group(0) @binding(0) var<storage, read_write> ps: array<P>;
@group(0) @binding(1) var<storage, read_write> s: M;
@group(0) @binding(2) var<storage, read_write> out: array<vec4<f32>>;
`
	for _, tc := range []struct {
		body, call string
		want       []int64
	}{
		{"let p = ps[1]; out[0] = vec4(p.vel, p.pos.x);", "dx.op.bufferLoad", []int64{32, 36, 40, 48, 52, 56}},
		{"ps[1] = P(vec3(1.0), vec3(2.0));", "dx.op.bufferStore", []int64{32, 36, 40, 48, 52, 56}},
		{"let m = s.m; out[0] = vec4(m[2], 1.0);", "dx.op.bufferLoad", []int64{16, 20, 24, 32, 36, 40, 48, 52, 56}},
		{"s.n = mat2x2<f32>(1.0, 2.0, 3.0, 4.0);", "dx.op.bufferStore", []int64{64}},
	} {
		mod, fn := emitMainModule(t, decls+"@compute @workgroup_size(1) fn main() {\n\t"+tc.body+"\n}\n")
		var got []int64
		for _, id := range callOperands(fn, tc.call, 2) {
			v, ok := constValue(mod, fn, id)
			if !ok {
				t.Fatalf("%s: %s address %s isn't a constant", tc.body, tc.call, valueTree(mod, fn, id))
			}
			got = append(got, v)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: %s addresses %v, want %v", tc.body, tc.call, got, tc.want)
		}
	}
}

// constValue evaluates value id in fn when it is an integer constant or
// adds, muls and shls of them.
func constValue(mod *module.Module, fn *module.Function, id int) (int64, bool) {
	for _, c := range mod.Constants {
		if c.ValueID == id && c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			return c.IntValue, true
		}
	}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind != module.InstrBinOp || in.ValueID != id {
				continue
			}
			a, okA := constValue(mod, fn, in.Operands[0])
			b, okB := constValue(mod, fn, in.Operands[1])
			switch in.Operands[2] {
			case 0:
				return a + b, okA && okB
			case 2:
				return a * b, okA && okB
			case 7:
				return a << b, okA && okB
			}
		}
	}
	return 0, false
}
