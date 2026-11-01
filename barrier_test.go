package naga

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// memoryBarrierModule is a compute shader whose barriers are memory
// barriers: WGSL has none, so the control barriers its frontend writes are
// turned into ir.StmtMemoryBarrier, as the SPIR-V frontend will write them.
func memoryBarrierModule(t *testing.T) *ir.Module {
	t.Helper()
	src := `@group(0) @binding(0) var<storage, read_write> buf: array<u32, 4>;
var<workgroup> tile: array<u32, 4>;
@compute @workgroup_size(4) fn main(@builtin(local_invocation_index) i: u32) {
    tile[i] = buf[i];
    storageBarrier();
    workgroupBarrier();
    buf[i] = tile[3u - i];
}`
	m, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	var walk func(b ir.Block)
	walk = func(b ir.Block) {
		for i := range b {
			switch s := b[i].Kind.(type) {
			case ir.StmtControlBarrier:
				b[i].Kind = ir.StmtMemoryBarrier(s)
				n++
			case ir.StmtBlock:
				walk(s.Block)
			}
		}
	}
	walk(m.EntryPoints[0].Function.Body)
	if n != 2 {
		t.Fatalf("turned %d control barriers into memory barriers, want 2", n)
	}
	return m
}

// TestMemoryBarrier: ir.StmtMemoryBarrier (ADR-010 E3, Rust's
// Statement::MemoryBarrier) orders memory without synchronizing execution,
// in every backend as Rust naga writes it; OpMemoryBarrier had no IR form.
func TestMemoryBarrier(t *testing.T) {
	m := memoryBarrierModule(t)
	// The zero-initialization polyfill adds a control barrier of its own.
	const noZeroInit = backend.ZeroInitWorkgroupNone

	t.Run("spirv", func(t *testing.T) {
		const opControlBarrier, opMemoryBarrier = 224, 225
		bin, err := spirvBinary(m, spirv.Options{ZeroInitializeWorkgroupMemory: noZeroInit})
		if err != nil {
			t.Fatal(err)
		}
		ops, _ := spirvOps(t, bin)
		if countOp(ops, opMemoryBarrier) != 2 || countOp(ops, opControlBarrier) != 0 {
			t.Errorf("OpMemoryBarrier %d, OpControlBarrier %d; want 2 and 0",
				countOp(ops, opMemoryBarrier), countOp(ops, opControlBarrier))
		}
	})
	t.Run("glsl", func(t *testing.T) {
		src, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.Version430, ZeroInitializeWorkgroupMemory: noZeroInit})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(src, "memoryBarrierBuffer();") || !strings.Contains(src, "memoryBarrierShared();") {
			t.Errorf("want memoryBarrierBuffer and memoryBarrierShared:\n%s", src)
		}
		// Without "Barrier" in the memory barriers' names, any "barrier();"
		// left is the execution barrier.
		if strings.Contains(strings.ReplaceAll(src, "Barrier", ""), "barrier();") {
			t.Errorf("an execution barrier() is written:\n%s", src)
		}
	})
	t.Run("hlsl", func(t *testing.T) {
		src, _, err := hlsl.Compile(m, hlsl.Options{ZeroInitializeWorkgroupMemory: noZeroInit})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(src, "DeviceMemoryBarrier();") || !strings.Contains(src, "GroupMemoryBarrier();") ||
			strings.Contains(src, "WithGroupSync") {
			t.Errorf("want DeviceMemoryBarrier and GroupMemoryBarrier without the group sync:\n%s", src)
		}
	})
	t.Run("msl", func(t *testing.T) {
		src, _, err := msl.Compile(m, msl.Options{})
		if err != nil {
			t.Fatal(err)
		}
		// Metal has no memory-only barrier; Rust writes threadgroup_barrier.
		if !strings.Contains(src, "threadgroup_barrier(metal::mem_flags::mem_device);") ||
			!strings.Contains(src, "threadgroup_barrier(metal::mem_flags::mem_threadgroup);") {
			t.Errorf("want the two threadgroup barriers:\n%s", src)
		}
	})
	t.Run("dxil", func(t *testing.T) {
		blob, err := dxilBlob(m, dxil.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := dxil.Validate(blob, dxil.ValidateFull); err != nil {
			t.Fatal(err)
		}
	})
}
