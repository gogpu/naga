package dxil

import (
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
)

// maxInlinedEntryPointSize bounds the size of an entry point after every
// user function is inlined into it (prepareModule), counted as expressions
// plus statements by irutil.InlinedEntryPointSizes. Full inlining multiplies
// a helper's body by the number of paths that reach it: a chain of n helpers
// each calling the next twice inlines into 2^n copies of the last from
// source of linear size, so a crafted shader of about one kilobyte (n=18)
// kept Compile busy for over a minute, and one of two kilobytes would not
// finish. DXC inlines every function too (AlwaysInliner, DxilLinker.cpp)
// and has no such bound either. The largest entry point of the snapshot
// corpora and of gg's production shaders, also of the g3d, ui and wgpu
// shaders, is gg's coarse.wgsl cs_coarse at 1913 (measured 2026-11-06);
// this allows over thirty times that. Past the inlining, Compile's time
// grows linearly with this size: the fan-out chain's largest accepted
// entry point (n=11, 38912) compiles in 0.4 s, and n=12 (77824) is refused.
const maxInlinedEntryPointSize = 65536

// checkInlinedSize refuses an entry point whose body full inlining would
// make larger than maxInlinedEntryPointSize, before the inlining is done.
func checkInlinedSize(m *ir.Module) error {
	sizes := irutil.InlinedEntryPointSizes(m, maxInlinedEntryPointSize+1)
	for i, size := range sizes {
		if size > maxInlinedEntryPointSize {
			return unsupported.Errorf("dxil: entry point %q is larger than %d expressions and statements once its function calls are inlined, the most this backend supports",
				m.EntryPoints[i].Name, maxInlinedEntryPointSize)
		}
	}
	return nil
}
