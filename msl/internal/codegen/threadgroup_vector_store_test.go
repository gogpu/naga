package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// P1-MSL-106: the Apple Paravirtual Metal device loses a run-time component
// store into a threadgroup vector written as `v[i] = x`. The writer emits the
// component through a scalar pointer instead; production is the corpus case
// testdata/readback/vars/workgroup-split-rt.wgsl, whose `c` reads 0 in every
// lane on that device with the plain form.
//
// These tests pin the emitted MSL shape. The device behavior itself is
// verified by wgpu's macOS readback run, not here.

// mslLineContaining returns the trimmed first line of msl that contains
// marker, failing the test when there is none.
func mslLineContaining(t *testing.T, msl, marker string) string {
	t.Helper()
	for _, line := range strings.Split(msl, "\n") {
		if strings.Contains(line, marker) {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no line contains %q in MSL:\n%s", marker, msl)
	return ""
}

// threadgroupVectorStoreWGSL is the readback case reduced to its store and
// load: a dynamic component store into a workgroup vec4 by every invocation,
// a barrier, then a component load by another index.
const threadgroupVectorStoreWGSL = `
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

var<workgroup> c: vec4<u32>;

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) i: u32) {
    c[i] = i * 7u;
    workgroupBarrier();
    dst[i] = c[(i + 1u) % 4u];
}
`

// TestMSLThreadgroupVectorDynamicStore covers the store alone: the component
// is written through a threadgroup scalar pointer, and the bounds-check
// rendering of the chain (the ReadZeroSkipWrite condition of the default
// options, the Restrict clamp of wgpu's readback harness) is preserved.
func TestMSLThreadgroupVectorDynamicStore(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts Options
		want string
	}{
		{"read zero skip write", DefaultOptions(), "((threadgroup uint*)&c)[i] = i * 7u;"},
		{"restrict", optionsWithPolicy(backend.BoundsCheckRestrict),
			"((threadgroup uint*)&c)[metal::min(unsigned(i), 3u)] = i * 7u;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			msl := compileWGSLWithOpts(t, threadgroupVectorStoreWGSL, tt.opts)
			store := mslLineContaining(t, msl, "= i * 7u;")
			if store != tt.want {
				t.Errorf("dynamic threadgroup vector store:\n got %q\nwant %q", store, tt.want)
			}
		})
	}
}

// TestMSLThreadgroupVectorLoadUnchanged covers the load: it keeps the plain
// subscript form. Only stores are suspected by the device bug, and only stores
// are worked around.
func TestMSLThreadgroupVectorLoadUnchanged(t *testing.T) {
	msl := compileWGSL(t, threadgroupVectorStoreWGSL)

	load := mslLineContaining(t, msl, "? c[")
	if strings.Contains(load, "*)&") {
		t.Errorf("load was rewritten as a pointer cast: %q", load)
	}
}

// TestMSLThreadgroupVectorStaticStoreUnchanged covers a static component
// store: MSL keeps the swizzle form and no pointer cast is introduced.
func TestMSLThreadgroupVectorStaticStoreUnchanged(t *testing.T) {
	const src = `
var<workgroup> c: vec4<u32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    c[1] = 5u;
    c.y = c[0];
}
`
	msl := compileWGSL(t, src)

	line := mslLineContaining(t, msl, "= 5u;")
	if strings.Contains(line, "*)&") {
		t.Errorf("static component store was rewritten as a pointer cast: %q", line)
	}
	if !strings.HasPrefix(line, "c.y = ") {
		t.Errorf("static component store changed form: %q", line)
	}
}

// TestMSLThreadgroupMatrixColumnDynamicStore covers the second matched shape,
// a pointer to a matrix column (ValuePointerType with a size): the column's
// component store also goes through a scalar pointer.
func TestMSLThreadgroupMatrixColumnDynamicStore(t *testing.T) {
	const src = `
var<workgroup> mm: mat2x2<f32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32, @builtin(local_invocation_id) j: vec3<u32>) {
    mm[i][j.x] = 1.0;
}
`
	msl := compileWGSL(t, src)

	line := mslLineContaining(t, msl, "= 1.0;")
	if !strings.Contains(line, "((threadgroup float*)&mm[i])[") {
		t.Errorf("dynamic workgroup matrix column store does not go through a scalar pointer: %q", line)
	}
}

// TestMSLNonThreadgroupVectorDynamicStoreUnchanged covers the address spaces
// the workaround must leave alone: a private vector, a function-local one and a
// storage buffer one (the case globals.wgsl exercises) keep the plain subscript
// store.
func TestMSLNonThreadgroupVectorDynamicStoreUnchanged(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read_write> sv: vec4<u32>;

var<private> pv: vec4<u32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    pv[i] = 1u;
    var lv: vec4<u32>;
    lv[i] = 2u;
    sv[i] = 3u;
}
`
	msl := compileWGSL(t, src)

	privateStore := mslLineContaining(t, msl, "= 1u;")
	if strings.Contains(privateStore, "*)&") {
		t.Errorf("private vector store was rewritten as a pointer cast: %q", privateStore)
	}
	localStore := mslLineContaining(t, msl, "= 2u;")
	if strings.Contains(localStore, "*)&") {
		t.Errorf("function-local vector store was rewritten as a pointer cast: %q", localStore)
	}
	storageStore := mslLineContaining(t, msl, "= 3u;")
	if strings.Contains(storageStore, "*)&") {
		t.Errorf("storage vector store was rewritten as a pointer cast: %q", storageStore)
	}
}

// TestMSLThreadgroupArrayDynamicStoreUnchanged covers a workgroup array
// element: the resolved base is a workgroup pointer to an array, not to a
// vector, so the workaround must not fire.
func TestMSLThreadgroupArrayDynamicStoreUnchanged(t *testing.T) {
	const src = `
var<workgroup> arr: array<u32, 4>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    arr[i] = 4u;
}
`
	msl := compileWGSL(t, src)

	line := mslLineContaining(t, msl, "= 4u;")
	if strings.Contains(line, "*)&") {
		t.Errorf("workgroup array store was rewritten as a pointer cast: %q", line)
	}
}

// TestMSLPointerValueVectorStoreUnchanged covers the pointer-value shape: the
// base resolves to a ValuePointerType whose space is not workgroup (and whose
// size is nil), so the workaround must not fire.
func TestMSLPointerValueVectorStoreUnchanged(t *testing.T) {
	const src = `
var<private> pv: vec4<u32>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    let p = &pv;
    (*p)[i] = 5u;
}
`
	msl := compileWGSL(t, src)

	line := mslLineContaining(t, msl, "= 5u;")
	if strings.Contains(line, "*)&") {
		t.Errorf("pointer-value vector store was rewritten as a pointer cast: %q", line)
	}
}

// TestThreadgroupVectorComponentStoreGuards covers the defensive branches of
// the detector with a bare writer: no current function, and a pointer handle
// outside the expression arena.
func TestThreadgroupVectorComponentStoreGuards(t *testing.T) {
	if _, _, ok := (&Writer{}).threadgroupVectorComponentStore(0); ok {
		t.Error("a writer without a current function matched")
	}
	w := &Writer{currentFunction: &ir.Function{}}
	if _, _, ok := w.threadgroupVectorComponentStore(0); ok {
		t.Error("an out-of-range pointer matched")
	}
	// A base expression with no resolved type must report false instead of
	// assuming a shape.
	unresolved := &Writer{currentFunction: &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.ExprAccess{Base: 0, Index: 1}},
	}}}
	if _, _, ok := unresolved.threadgroupVectorComponentStore(0); ok {
		t.Error("an expression without a resolved type matched")
	}
}
