// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/wgsl"
)

// wgpu's indirect-dispatch validation shader: an if/else on an `||` chain
// whose arms both store. DXC's output for it has no block without a
// predecessor; ours had an extra block holding only a branch to the merge.
// Its two short-circuit locals are promoted through i32 phis (P2-DXIL-107):
// as `phi i1` (v0.21.3's candidate) WARP ran neither arm's stores, and as
// `alloca i1` D3D12's GPU-based validation did the same.
const orChainSource = `
@group(0) @binding(0) var<storage, read_write> dst: array<u32, 3>;
@group(0) @binding(1) var<uniform> src_index: SrcIndex;
@group(1) @binding(0) var<storage, read> src: array<u32>;

struct SrcIndex {
    inner: u32,
}

@compute @workgroup_size(1)
fn main() {
    let i = src_index.inner;
    let x = src[i];
    let y = src[i + 1u];
    let z = src[i + 2u];

    let limit = 65535u;
    if (x > limit || y > limit || z > limit) {
        dst[0] = 0u;
        dst[1] = 0u;
        dst[2] = 0u;
    } else {
        dst[0] = x;
        dst[1] = y;
        dst[2] = z;
    }
}
`

// unreachableBlocks returns the indices of the blocks of fn, other than the
// entry block, that no branch targets. A conditional branch carries the
// operands [trueBB, falseBB, cond], an unconditional one [bb].
func unreachableBlocks(fn *module.Function) []int {
	targeted := map[int]bool{}
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind != module.InstrBr {
				continue
			}
			switch len(in.Operands) {
			case 1:
				targeted[in.Operands[0]] = true
			case 3:
				targeted[in.Operands[0]] = true
				targeted[in.Operands[1]] = true
			}
		}
	}
	var out []int
	for i := 1; i < len(fn.BasicBlocks); i++ {
		if !targeted[i] {
			out = append(out, i)
		}
	}
	return out
}

// wgpuDX12Options are the options wgpu's DX12 backend compiles with: SM 6.5
// and Restrict bounds checks.
func wgpuDX12Options() Options {
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 5}
	opts.BoundsCheckPolicies = backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckRestrict,
		Buffer:       backend.BoundsCheckRestrict,
		ImageLoad:    backend.BoundsCheckRestrict,
		BindingArray: backend.BoundsCheckUnchecked,
	}
	return opts
}

// orChainOptions are the option sets the `||` chain tests compile with.
var orChainOptions = []struct {
	name string
	opts func() Options
}{
	{"default", DefaultOptions},
	{"wgpu-dx12", wgpuDX12Options},
}

// The if/else is the last statement and both its arms return (the IR's
// trailing return goes into each arm), so no branch reaches its merge block.
func TestOrChainNoUnreachableMerge(t *testing.T) {
	for _, tc := range orChainOptions {
		t.Run(tc.name, func(t *testing.T) {
			_, fn := emitMainModuleOpts(t, orChainSource, tc.opts())
			if blocks := unreachableBlocks(fn); len(blocks) != 0 {
				t.Errorf("blocks without a predecessor: %v", blocks)
			}
		})
	}
}

// cfgCorpora are the WGSL inputs the CFG gate compiles: the snapshot corpus
// (Rust's inputs and ours) and gg's production shaders.
var cfgCorpora = []string{
	"../snapshot/testdata/rust/in/wgsl",
	"../snapshot/testdata/own/in",
	"../testdata/upstream/github.com/gogpu/gg/internal/gpu/shaders",
	"../testdata/upstream/github.com/gogpu/gg/internal/gpu/tilecompute/shaders",
}

// TestNoUnreachableBlocks is a gate over every entry point of the corpora:
// no emitted function has a block that no branch reaches, as DXC's output
// never has one.
func TestNoUnreachableBlocks(t *testing.T) {
	checked, skipped := 0, 0
	for _, dir := range cfgCorpora {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("corpus %q: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wgsl") {
				continue
			}
			path := dir + "/" + entry.Name()
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			irMod, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
			if err != nil {
				skipped++ // parse failures are the snapshot tests' concern
				continue
			}
			for _, ep := range irMod.EntryPoints {
				opts := wgpuDX12Options()
				opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
				em, err := emitEntryModule(irMod, &opts)
				if err != nil {
					skipped++ // compile failures are TestDxilValSummary's concern
					continue
				}
				checked++
				for _, fn := range em.mod.Functions {
					if blocks := unreachableBlocks(fn); len(blocks) != 0 {
						t.Errorf("%s/%s: function %s has blocks without a predecessor: %v",
							path, ep.Name, fn.Name, blocks)
					}
				}
			}
		}
	}
	t.Logf("checked %d entry points, %d not compiled", checked, skipped)
	// The corpora hold well over 250 entry points; far fewer means the
	// inputs went missing, not that the gate passed.
	if checked < 250 {
		t.Errorf("only %d entry points checked", checked)
	}
}

// The short-circuit locals of the `||` chain are promoted, and their two
// phis are i32, as DXC's frontend emits them (P2-DXIL-107). The i1 shapes are
// what breaks: `phi i1` at these merges makes WARP, the D3D12 adapter of
// GPU-less CI runners, run neither arm's stores (gogpu/wgpu's
// TestDispatchIndirectReadsItsOwnArguments), and `alloca i1` makes D3D12's
// GPU-based validation drop the stores.
func TestOrChainBoolMergesThroughI32Phi(t *testing.T) {
	for _, tc := range orChainOptions {
		t.Run(tc.name, func(t *testing.T) {
			_, fn := emitMainModuleOpts(t, orChainSource, tc.opts())
			phi32 := 0
			for _, bb := range fn.BasicBlocks {
				for _, in := range bb.Instructions {
					if in.ResultType == nil || in.ResultType.Kind != module.TypeInteger {
						continue
					}
					switch {
					case in.ResultType.IntBits == 1 && in.Kind == module.InstrPhi:
						t.Errorf("phi i1 %%%d at the chain's merge: a bool local was merged as i1", in.ValueID)
					case in.ResultType.IntBits == 1 && in.Kind == module.InstrAlloca:
						t.Errorf("alloca i1 %%%d: a bool local kept an i1 alloca", in.ValueID)
					case in.ResultType.IntBits == 32 && in.Kind == module.InstrPhi:
						phi32++
					}
				}
			}
			// One phi per short-circuit local of the two-term chain.
			if phi32 < 2 {
				t.Errorf("%d i32 phis in the chain, want at least 2", phi32)
			}
		})
	}
}
