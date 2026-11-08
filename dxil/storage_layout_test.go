package dxil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
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

// A 64-bit field of a struct of mixed types is loaded and stored with
// dx.op.rawBufferLoad/Store of its own type, as DXC does for shader model
// 6.3. It was loaded with dx.op.bufferLoad.i64 and .f64, which have no
// such overloads, and dx.op.bufferStore.i64 and .f64 were declared
// without a call, which the validator refuses too.
func TestStorageBuffer64BitFieldsInMixedStructs(t *testing.T) {
	mod, fn := emitMainModule(t, `
struct T { a: f32, b: u32, c: u64, d: f64 }
@group(0) @binding(0) var<storage, read_write> s: array<T>;
@compute @workgroup_size(1) fn main() {
	s[1] = s[0];
}
`)
	for _, f := range mod.Functions {
		if slices.Contains([]string{"dx.op.bufferLoad.i64", "dx.op.bufferLoad.f64", "dx.op.bufferStore.i64", "dx.op.bufferStore.f64"}, f.Name) {
			t.Errorf("%s is declared", f.Name)
		}
	}
	for _, name := range []string{"dx.op.rawBufferLoad.i64", "dx.op.rawBufferLoad.f64", "dx.op.rawBufferStore.i64", "dx.op.rawBufferStore.f64"} {
		if n := len(callOperands(fn, name, 0)); n != 1 {
			t.Errorf("%d calls of %s, want 1", n, name)
		}
	}
}

// EnableDoubleExtensions is set for the double operations DXC sets it for,
// a division here, and not for others, a multiplication: the validator
// refused the flag on a shader that only multiplies doubles ("Flags must
// match usage").
// A storage buffer element of more than four components is read and written
// with dx.op.bufferLoad and dx.op.bufferStore: the DXIL spec gives a RawBuffer
// one active coordinate, c0 in bytes, and bufferLoad/bufferStore carry four
// components per call from shader model 5.1, so the access is split into one
// call per four components. The emitter read such an element with
// dx.op.rawBufferLoad, whose mask and alignment operands are 6.2, and Compile
// refused the shader; DXC compiles the same storage buffer at cs_6_0
// (P1-DXIL-072).
func TestStorageBufferLargeElementBelowSM62(t *testing.T) {
	tests := []struct{ name, src string }{
		{"five u32 components, read and written", `
struct S { a: u32, b: u32, c: u32, d: u32, e: u32 }
@group(0) @binding(0) var<storage, read_write> s: array<S>;
@compute @workgroup_size(1) fn main() { s[1] = s[0]; }`},
		{"six f32 components, one read per member", `
struct S { a: vec2<f32>, b: vec3<f32>, c: f32 }
@group(0) @binding(0) var<storage, read_write> s: array<S>;
@compute @workgroup_size(1) fn main() { s[1].b.y = s[0].a.x; s[1].c = s[0].b.z; }`},
		{"nine f32 components, three calls", `
struct S { a: vec4<f32>, b: vec4<f32>, c: f32 }
@group(0) @binding(0) var<storage, read> s: array<S>;
@group(0) @binding(1) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() { let v = s[0]; o[0] = v.a.x + v.b.w + v.c; }`},
		{"matrix element of sixteen components", `
@group(0) @binding(0) var<storage, read_write> m: array<mat4x4<f32>>;
@compute @workgroup_size(1) fn main() { m[1] = m[0]; }`},
	}
	for _, sm := range []backend.ShaderModel{{Major: 6, Minor: 0}, {Major: 6, Minor: 1}} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s at 6.%d", tt.name, sm.Minor), func(t *testing.T) {
				blob, _, err := Compile(lowerForTest(t, tt.src), Options{ShaderModel: sm})
				if err != nil {
					t.Fatalf("Compile: %v", err)
				}
				if err := Validate(blob, ValidateFull); err != nil {
					t.Fatalf("Validate: %v", err)
				}
				// The container must claim the requested model: a bufferLoad
				// is legal at 6.0, a rawBufferLoad would not be.
				stat := dxbcParts(t, blob)["STAT"]
				if len(stat) < 4 {
					t.Fatal("container has no STAT part")
				}
				v := binary.LittleEndian.Uint32(stat[:4])
				if major, minor := (v>>4)&0xF, v&0xF; major != 6 || minor != uint32(sm.Minor) {
					t.Errorf("container shader model %d.%d, want 6.%d", major, minor, sm.Minor)
				}
			})
		}
	}
}

// The calls a 32-bit element of more than four components is read and
// written with: bufferLoad/bufferStore, one per four components, and never
// their rawBuffer twins, whose mask and alignment operands are 6.2. Nine
// components are 4 + 4 + 1.
func TestStorageBufferLargeElementCalls(t *testing.T) {
	_, fn := emitMainModule(t, `
struct S { a: vec4<f32>, b: vec4<f32>, c: f32 }
@group(0) @binding(0) var<storage, read_write> s: array<S>;
@compute @workgroup_size(1) fn main() { let v = s[0]; s[1] = v; }
`)
	loads, stores, raw := 0, 0, []string{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind != module.InstrCall || in.CalledFunc == nil {
				continue
			}
			name := in.CalledFunc.Name
			switch {
			case strings.HasPrefix(name, "dx.op.bufferLoad"):
				loads++
			case strings.HasPrefix(name, "dx.op.bufferStore"):
				stores++
			case strings.HasPrefix(name, "dx.op.rawBuffer"):
				raw = append(raw, name)
			}
		}
	}
	if loads != 3 || stores != 3 {
		t.Errorf("bufferLoad/bufferStore calls = %d/%d, want 3/3 for nine components", loads, stores)
	}
	if len(raw) != 0 {
		t.Errorf("raw buffer calls at a 32-bit element: %v", raw)
	}
}

// A 64-bit storage buffer element is the one raw buffer access that stays a
// shader model requirement: dx.op.rawBufferLoad/Store take i64/f64 overloads
// only from 6.3 (DXIL error SM.64BITRAWBUFFERLOADSTORE), and the emitter
// writes native 64-bit values there. DXC reaches cs_6_0 by rewriting every
// 64-bit operation as a pair of 32-bit ones, which we do not do, so the model
// is a real requirement of this code generation and Compile refuses below it.
func TestStorageBufferInt64RequiresSM63(t *testing.T) {
	m := lowerForTest(t, `
@group(0) @binding(0) var<storage, read_write> s: array<u64>;
@compute @workgroup_size(1) fn main() { s[1] = s[0] + 1; }
`)
	for _, sm := range []backend.ShaderModel{{Major: 6, Minor: 0}, {Major: 6, Minor: 2}} {
		_, _, err := Compile(m, Options{ShaderModel: sm})
		var vErr *backend.VersionError
		if !errors.As(err, &vErr) || vErr.Space != backend.VersionShaderModel ||
			vErr.Required != (backend.Version{Major: 6, Minor: 3}) {
			t.Fatalf("SM 6.%d: err = %v, want a shader model VersionError requiring 6.3", sm.Minor, err)
		}
	}
	if _, _, err := Compile(m, Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 3}}); err != nil {
		t.Errorf("SM 6.3: %v", err)
	}
}

func TestDoubleExtensionsOnlyForTheirOperations(t *testing.T) {
	for op, want := range map[string]bool{"*": false, "/": true} {
		blob, _, err := Compile(lowerForTest(t, `
@group(0) @binding(0) var<storage, read_write> s: array<f64>;
@compute @workgroup_size(1) fn main() {
	s[1] = s[0] `+op+` 3.0lf;
}
`), Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 3}}) // 64-bit buffer elements (P1-DXIL-072)
		if err != nil {
			t.Fatal(err)
		}
		features := binary.LittleEndian.Uint64(dxbcParts(t, blob)["SFI0"])
		if features&0x1 == 0 || (features&0x20 != 0) != want {
			t.Errorf("%s: features %#x, want Doubles with DoubleExtensions %v", op, features, want)
		}
	}
}
