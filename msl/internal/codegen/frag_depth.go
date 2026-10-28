// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// fragDepthRangeName is the pointer to the range of the frag_depth clamp,
// min then max, and immediatesName the immediates buffer argument of an
// entry point without immediate data of its own.
const (
	fragDepthRangeName = "_frag_depth_range"
	immediatesName     = "_immediates"
)

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

// writeFragDepthRangeParam finds the range of the frag_depth clamp of the
// entry point ep, when Options.ClampFragDepth asks for it and ep returns
// frag_depth: at the offset given in ep's immediates buffer, past the
// immediate data that ep uses, if any, which is the argument at that buffer
// slot then. Without such data ep takes the buffer as an argument of its
// own. used holds the globals ep uses.
func (w *Writer) writeFragDepthRangeParam(ep *ir.EntryPoint, used map[uint32]struct{}, paramCount *int) error {
	clamp := w.options.ClampFragDepth
	if clamp == nil || !w.writesFragDepth(ep) {
		return nil
	}
	res, ok := w.options.PerEntryPointMap[ep.Name]
	if !ok || res.ImmediatesBuffer == nil {
		return fmt.Errorf("msl: entry point %q clamps frag_depth but has no ImmediatesBuffer for the range", ep.Name)
	}
	if clamp.Offset%4 != 0 {
		return fmt.Errorf("msl: ClampFragDepth.Offset %d is not a multiple of 4", clamp.Offset)
	}
	base := immediatesName
	for h := range used {
		global := &w.module.GlobalVariables[h]
		if global.Space != ir.SpaceImmediate {
			continue
		}
		if size := ir.TypeSize(w.module, global.Type); clamp.Offset < size {
			return fmt.Errorf("msl: the frag_depth range at offset %d overlaps the %d bytes of immediate data of entry point %q",
				clamp.Offset, size, ep.Name)
		}
		base = fmt.Sprintf("reinterpret_cast<constant char*>(&%s)", w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: h}))
	}
	if base == immediatesName {
		w.writeEntryPointParam(*paramCount, fmt.Sprintf("constant char* %s [[buffer(%d)]]", immediatesName, *res.ImmediatesBuffer))
		*paramCount++
	}
	w.fragDepthRange = fmt.Sprintf("reinterpret_cast<constant float*>(%s + %d)", base, clamp.Offset)
	return nil
}

// writeFragDepthRange declares the pointer to the range of the frag_depth
// clamp at the start of an entry point that clamps.
func (w *Writer) writeFragDepthRange() {
	if w.fragDepthRange != "" {
		w.WriteLine("constant float* %s = %s;", fragDepthRangeName, w.fragDepthRange)
	}
}

// writeOutputMember writes value, the output member or result with binding
// b, clamped to the range of the frag_depth clamp when it is frag_depth and
// the entry point clamps it.
func (w *Writer) writeOutputMember(b *ir.Binding, value func() error) error {
	if w.fragDepthRange == "" || !isFragDepth(b) {
		return value()
	}
	w.write("%sclamp(", Namespace)
	if err := value(); err != nil {
		return err
	}
	w.write(", %s[0], %s[1])", fragDepthRangeName, fragDepthRangeName)
	return nil
}
