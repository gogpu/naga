package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// The error paths of the runtime-sized array bound (P0-MSL-100, P2-TEST-015).
// A valid module never reaches them: the validator rejects a storage pointer
// parameter (Rust naga's FunctionError::InvalidArgumentPointerSpace), so every
// runtime-sized array access originates from a global. The modules below are
// lowered without validation, or edited after lowering, to make sure each
// caller of the bound returns its error instead of writing a partial or an
// unchecked access.

// compileMSLError compiles module and returns the error; it fails the test
// when compilation succeeds or returns any MSL text with the error.
func compileMSLError(t *testing.T, module *ir.Module, opts Options) error {
	t.Helper()
	out, _, err := Compile(module, opts)
	if err == nil {
		t.Fatalf("Compile succeeded, want an error; output:\n%s", out)
	}
	if out != "" {
		t.Fatalf("Compile returned %d bytes of MSL with error %v", len(out), err)
	}
	return err
}

func optionsWithPolicy(policy backend.BoundsCheckPolicy) Options {
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Index = policy
	opts.BoundsCheckPolicies.Buffer = policy
	return opts
}

// TestRuntimeArrayBoundNotFromGlobal: a runtime-sized array reached through a
// storage pointer parameter has no originating global, so no _buffer_sizes
// field bounds it. Every way the bound is written reports that.
func TestRuntimeArrayBoundNotFromGlobal(t *testing.T) {
	const want = "runtime-sized array access does not originate from a global variable"
	const prelude = `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@group(0) @binding(1) var<storage, read_write> c: array<atomic<u32>>;
fn g(q: ptr<storage, u32, read_write>) { *q = 1u; }
`
	tests := []struct {
		name   string
		body   string
		policy backend.BoundsCheckPolicy
	}{
		{"load", `fn f(p: ptr<storage, array<u32>, read_write>, i: u32) -> u32 { return (*p)[i]; }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"store", `fn f(p: ptr<storage, array<u32>, read_write>, i: u32) { (*p)[i] = 2u; }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"pointer argument, dynamic index", `fn f(p: ptr<storage, array<u32>, read_write>, i: u32) { g(&(*p)[i]); }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"pointer argument, constant index", `fn f(p: ptr<storage, array<u32>, read_write>) { g(&(*p)[3]); }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"atomic fetch", `fn f(p: ptr<storage, array<atomic<u32>>, read_write>, i: u32) -> u32 { return atomicAdd(&(*p)[i], 1u); }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"atomic compare exchange", `fn f(p: ptr<storage, array<atomic<u32>>, read_write>, i: u32) -> bool {
    return atomicCompareExchangeWeak(&(*p)[i], 1u, 2u).exchanged; }`,
			backend.BoundsCheckReadZeroSkipWrite},
		{"restrict", `fn f(p: ptr<storage, array<u32>, read_write>, i: u32) -> u32 { return (*p)[i]; }`,
			backend.BoundsCheckRestrict},
		{"arrayLength", `fn f(p: ptr<storage, array<u32>, read_write>) -> u32 { return arrayLength(p); }`,
			backend.BoundsCheckUnchecked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := lowerWGSLForTest(t, prelude+tt.body)
			err := compileMSLError(t, module, optionsWithPolicy(tt.policy))
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q, want one containing %q", err, want)
			}
		})
	}
}

// runtimeArrayStructModule lowers a module reading s.d[i], the runtime-sized
// last member of a storage struct, and returns it with the struct's handle.
func runtimeArrayStructModule(t *testing.T) (*ir.Module, ir.TypeHandle) {
	t.Helper()
	module := lowerWGSLForTest(t, `
struct S { n: u32, d: array<u32> }
@group(0) @binding(0) var<storage, read_write> s: S;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    s.n = s.d[gid.x]; }
`)
	for h := range module.Types {
		if module.Types[h].Name == "S" {
			return module, ir.TypeHandle(h)
		}
	}
	t.Fatal("struct S not found")
	return nil, 0
}

// scalarTypeHandle is the handle of the first scalar type of m.
func scalarTypeHandle(m *ir.Module) ir.TypeHandle {
	for h := range m.Types {
		if _, ok := m.Types[h].Inner.(ir.ScalarType); ok {
			return ir.TypeHandle(h)
		}
	}
	return ir.TypeHandle(len(m.Types))
}

// TestRuntimeArrayBoundBadGlobalType: the bound reads the originating
// global's type for the array's offset and stride; a type that has no
// runtime-sized array where the access says there is one is an error, not a
// made-up bound.
func TestRuntimeArrayBoundBadGlobalType(t *testing.T) {
	// A member after the runtime-sized one: the access chain (s.d) still
	// types, but the bound takes the struct's last member, as the WGSL rule
	// (the runtime-sized array is last) lets it.
	addLastMember := func(t *testing.T, ty func(*ir.Module) ir.TypeHandle) *ir.Module {
		t.Helper()
		module, s := runtimeArrayStructModule(t)
		st := module.Types[s].Inner.(ir.StructType)
		members := append([]ir.StructMember(nil), st.Members...)
		members = append(members, ir.StructMember{Name: "m", Type: ty(module), Offset: 8})
		st.Members = members
		module.Types[s].Inner = st
		return module
	}
	for _, tc := range []struct {
		name, want string
		ty         func(*ir.Module) ir.TypeHandle
	}{
		{"last member not an array", "expected a runtime-sized array, got ir.ScalarType", scalarTypeHandle},
		{"last member type out of range", "invalid array type handle",
			func(m *ir.Module) ir.TypeHandle { return ir.TypeHandle(len(m.Types) + 7) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module := addLastMember(t, tc.ty)
			err := compileMSLError(t, module, DefaultOptions())
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want one containing %q", err, tc.want)
			}
		})
	}
}

// TestRuntimeArrayMaxIndexGlobalType covers the checks of
// writeRuntimeArrayMaxIndex that Compile cannot reach because the typifier
// rejects the module first (a global whose type handle is out of range, an
// empty struct, a global of a type that holds no runtime-sized array). They
// stay as errors so a future caller with untyped input cannot index out of
// range or write a made-up bound.
func TestRuntimeArrayMaxIndexGlobalType(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	for _, tc := range []struct {
		name   string
		types  []ir.Type
		global ir.TypeHandle
		want   string
	}{
		{"type handle out of range", []ir.Type{{Inner: u32}}, 5, "global 0 has an invalid type handle 5"},
		{"struct without members", []ir.Type{{Inner: ir.StructType{}}}, 0, "global 0: struct has no members"},
		{"not an array type", []ir.Type{{Inner: u32}}, 0, "global 0: expected a type with a runtime-sized array, got ir.ScalarType"},
		{"binding array", []ir.Type{{Inner: u32}, {Inner: ir.BindingArrayType{Base: 0}}}, 1,
			"global 0: bounds check of a runtime-sized array inside binding_array is not supported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module := &ir.Module{
				Types:           tc.types,
				GlobalVariables: []ir.GlobalVariable{{Name: "g", Space: ir.SpaceStorage, Type: tc.global}},
			}
			w := &Writer{module: module, currentFunction: &ir.Function{
				Expressions: []ir.Expression{{Kind: ir.ExprGlobalVariable{Variable: 0}}},
			}}
			err := w.writeRuntimeArrayMaxIndex(0)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
			if w.Out.Len() != 0 {
				t.Fatalf("wrote %q with the error", w.Out.String())
			}
		})
	}
}

// TestRuntimeArrayBoundIndexError: an index the writer cannot write fails
// the bounds condition instead of leaving `uint() < …` in the output.
func TestRuntimeArrayBoundIndexError(t *testing.T) {
	module := lowerWGSLForTest(t, `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    a[0] = a[gid.x + 1u]; }
`)
	fn := &module.EntryPoints[0].Function
	replaced := false
	for i := range fn.Expressions {
		if bin, ok := fn.Expressions[i].Kind.(ir.ExprBinary); ok && bin.Op == ir.BinaryAdd {
			// A kind this backend never writes inline: its error must surface.
			fn.Expressions[i].Kind = ir.ExprWorkGroupUniformLoadResult{Ty: 0}
			replaced = true
		}
	}
	if !replaced {
		t.Fatal("index expression not found")
	}
	err := compileMSLError(t, module, DefaultOptions())
	if !strings.Contains(err.Error(), "unsupported expression kind") {
		t.Fatalf("error %q, want the index's error", err)
	}
}

// TestRZSWLoadOutsideEmitIsParenthesized: rendersAsRZSWTernary keeps its Load
// case although the writer bakes every emitted Load (Rust's bake_ref_count
// of 1). A Load that no Emit covers is written where it is used, through
// writeLoad, as a ReadZeroSkipWrite ternary; as an operand of a binary
// operator it needs the parentheses Rust's put_bounds_checks opens when
// !is_scoped. Without the case it printed `x + uint(i) < 4 ? … : …`.
func TestRZSWLoadOutsideEmitIsParenthesized(t *testing.T) {
	module := lowerWGSLForTest(t, `
@group(0) @binding(0) var<storage, read_write> b: array<u32, 4>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    b[0] = gid.y + b[gid.x]; }
`)
	fn := &module.EntryPoints[0].Function
	load := -1
	for i := range fn.Expressions {
		if _, ok := fn.Expressions[i].Kind.(ir.ExprLoad); ok {
			if acc, ok := fn.Expressions[fn.Expressions[i].Kind.(ir.ExprLoad).Pointer].Kind.(ir.ExprAccess); ok {
				_ = acc
				load = i
			}
		}
	}
	if load < 0 {
		t.Fatal("load of b[gid.x] not found")
	}
	fn.Body = withoutEmitOf(fn.Body, ir.ExpressionHandle(load))
	out, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	mustContainMSL(t, out, "b.inner[0] = gid.y + (uint(_e5) < 4 ? b.inner[_e5] : DefaultConstructible());")
}

// withoutEmitOf splits every Emit range of block that covers h so that h is
// in none.
func withoutEmitOf(block ir.Block, h ir.ExpressionHandle) ir.Block {
	out := make(ir.Block, 0, len(block)+1)
	for _, st := range block {
		emit, ok := st.Kind.(ir.StmtEmit)
		if !ok || h < emit.Range.Start || h >= emit.Range.End {
			out = append(out, st)
			continue
		}
		if emit.Range.Start < h {
			out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: emit.Range.Start, End: h}}})
		}
		if h+1 < emit.Range.End {
			out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: h + 1, End: emit.Range.End}}})
		}
	}
	return out
}
