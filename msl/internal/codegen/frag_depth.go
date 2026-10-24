// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// fragDepthRangeName is the entry point argument that holds the range of
// the frag_depth clamp.
const fragDepthRangeName = "_frag_depth_range"

// writesFragDepth reports whether ep returns frag_depth, as its result or
// a member of it.
func (w *Writer) writesFragDepth(ep *ir.EntryPoint) bool {
	result := ep.Function.Result
	if ep.Stage != ir.StageFragment || result == nil {
		return false
	}
	if isFragDepth(result.Binding) {
		return true
	}
	if int(result.Type) < len(w.module.Types) {
		if st, ok := w.module.Types[result.Type].Inner.(ir.StructType); ok {
			for _, m := range st.Members {
				if isFragDepth(m.Binding) {
					return true
				}
			}
		}
	}
	return false
}

func isFragDepth(b *ir.Binding) bool {
	if b == nil {
		return false
	}
	bb, ok := (*b).(ir.BuiltinBinding)
	return ok && bb.Builtin == ir.BuiltinFragDepth
}

// writeFragDepthRangeParam adds the argument that holds the range of the
// frag_depth clamp to the entry point ep, when Options.ClampFragDepth asks
// for it and ep returns frag_depth.
func (w *Writer) writeFragDepthRangeParam(ep *ir.EntryPoint, paramCount *int) {
	if w.options.ClampFragDepth == nil || !w.writesFragDepth(ep) {
		return
	}
	w.writeEntryPointParam(*paramCount, fmt.Sprintf("constant %sfloat2& %s [[buffer(%d)]]",
		Namespace, fragDepthRangeName, w.options.ClampFragDepth.Buffer))
	*paramCount++
	w.clampFragDepth = true
}

// writeOutputMember writes value, the output member or result with binding
// b, clamped to the range of the frag_depth clamp when it is frag_depth and
// the entry point clamps it, and ANDed with Options.FixedSampleMask when it
// is sample_mask and the entry point applies the mask.
func (w *Writer) writeOutputMember(b *ir.Binding, value func() error) error {
	if w.options.FixedSampleMask != nil && w.entryPointStage == ir.StageFragment && isSampleMask(b) {
		w.write("(")
		if err := value(); err != nil {
			return err
		}
		w.write(" & %du)", *w.options.FixedSampleMask)
		return nil
	}
	if !w.clampFragDepth || !isFragDepth(b) {
		return value()
	}
	w.write("%sclamp(", Namespace)
	if err := value(); err != nil {
		return err
	}
	w.write(", %s.x, %s.y)", fragDepthRangeName, fragDepthRangeName)
	return nil
}
