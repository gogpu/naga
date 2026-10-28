// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"github.com/gogpu/naga/ir"
)

// appliesSampleMask reports whether ep applies Options.FixedSampleMask.
func (w *Writer) appliesSampleMask(ep *ir.EntryPoint) bool {
	return w.options.FixedSampleMask != nil && ep.Stage == ir.StageFragment
}

// writesSampleMask reports whether ep returns sample_mask, as its result or
// a member of it.
func (w *Writer) writesSampleMask(ep *ir.EntryPoint) bool {
	result := ep.Function.Result
	if result == nil {
		return false
	}
	if isSampleMask(result.Binding) {
		return true
	}
	if int(result.Type) < len(w.module.Types) {
		if st, ok := w.module.Types[result.Type].Inner.(ir.StructType); ok {
			for _, m := range st.Members {
				if isSampleMask(m.Binding) {
					return true
				}
			}
		}
	}
	return false
}

func isSampleMask(b *ir.Binding) bool {
	if b == nil {
		return false
	}
	bb, ok := (*b).(ir.BuiltinBinding)
	return ok && bb.Builtin == ir.BuiltinSampleMask
}

// writeSampleMaskMember adds the [[sample_mask]] member that returns
// Options.FixedSampleMask to the output struct of ep, when ep applies the
// mask and returns no sample_mask of its own.
func (w *Writer) writeSampleMaskMember(ep *ir.EntryPoint) {
	if !w.appliesSampleMask(ep) || w.writesSampleMask(ep) {
		return
	}
	w.sampleMaskOutput = w.namer.call("fixed_sample_mask")
	w.WriteLine("uint %s [[sample_mask]];", w.sampleMaskOutput)
}

// writeSampleMaskValue writes the fixed sample mask as the last value of
// an output struct, when the entry point returns it.
func (w *Writer) writeSampleMaskValue(first bool) {
	if w.sampleMaskOutput == "" {
		return
	}
	if !first {
		w.write(",")
	}
	w.write(" %du", *w.options.FixedSampleMask)
}

// mayReachEnd reports whether control can reach the end of block. Only a
// return ends it: metal::discard_fragment() is a call to C++, so a branch
// that discards still reaches the end of a function that returns a value.
func mayReachEnd(block ir.Block) bool {
	if len(block) == 0 {
		return true
	}
	switch s := block[len(block)-1].Kind.(type) {
	case ir.StmtReturn:
		return false
	case ir.StmtBlock:
		return mayReachEnd(s.Block)
	case ir.StmtIf:
		return mayReachEnd(s.Accept) || mayReachEnd(s.Reject)
	}
	return true
}

// writeVoidOutputReturn writes a return of the output struct a fragment
// entry point that returns nothing gets for the fixed sample mask.
func (w *Writer) writeVoidOutputReturn() {
	w.WriteIndent()
	w.write("return %s {", w.voidOutputStruct)
	w.writeSampleMaskValue(true)
	w.write(" };\n")
}
