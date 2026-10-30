// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// The range of the frag_depth clamp is in the fragment stage's immediates
// uniform, of struct type immediatesRangeType: the entry point's own
// immediate data, if any, in member immediatesData, then fragDepthMin and
// fragDepthMax.
const (
	fragImmediates      = "_immediates_binding_fs"
	immediatesRangeType = "naga_immediates"
	immediatesData      = "data"
	fragDepthMin        = "naga_frag_depth_min"
	fragDepthMax        = "naga_frag_depth_max"
)

// prepareFragDepthRange finds where the range of the frag_depth clamp
// goes, when Options.ClampFragDepth asks for it and the selected entry
// point returns frag_depth: in the immediates uniform with the entry
// point's own immediate data, or in one of its own, declared here, when
// the entry point has none.
func (w *Writer) prepareFragDepthRange() error {
	clamp := w.options.ClampFragDepth
	ep := w.getSelectedEntryPoint()
	if clamp == nil || ep == nil || ep.Stage != ir.StageFragment || !w.writesFragDepth(ep.Function.Result) {
		return nil
	}
	if clamp.Offset%4 != 0 {
		return fmt.Errorf("glsl: ClampFragDepth.Offset %d is not a multiple of 4", clamp.Offset)
	}
	w.clampFragDepth = true
	for handle, global := range w.module.GlobalVariables {
		h := ir.GlobalVariableHandle(handle)
		if global.Space != ir.SpaceImmediate || w.reachable != nil && !w.reachable.hasGlobal(h) {
			continue
		}
		if size := ir.TypeSize(w.module, global.Type); clamp.Offset < size {
			return fmt.Errorf("glsl: the frag_depth range at offset %d overlaps the %d bytes of immediate data of entry point %q",
				clamp.Offset, size, ep.Name)
		}
		w.rangeImmediates = &h
		return nil
	}
	w.writeImmediatesRangeType(nil)
	w.WriteLine("uniform %s %s;", immediatesRangeType, fragImmediates)
	w.WriteLine("")
	return nil
}

// carriesFragDepthRange reports whether the immediates uniform of h holds
// the range of the frag_depth clamp next to h's data.
func (w *Writer) carriesFragDepthRange(h ir.GlobalVariableHandle) bool {
	return w.rangeImmediates != nil && *w.rangeImmediates == h
}

// writeImmediatesRangeType declares the struct of the immediates uniform
// that holds the range of the frag_depth clamp, after the immediate data of
// type data when there is any.
func (w *Writer) writeImmediatesRangeType(data *ir.TypeHandle) {
	precision := ""
	if w.options.LangVersion.ES {
		precision = "highp "
	}
	w.WriteLine("struct %s {", immediatesRangeType)
	w.PushIndent()
	if data != nil {
		w.WriteLine("%s %s%s;", w.getBaseTypeName(*data), immediatesData, w.getArraySuffix(*data))
	}
	w.WriteLine("%sfloat %s;", precision, fragDepthMin)
	w.WriteLine("%sfloat %s;", precision, fragDepthMax)
	w.PopIndent()
	w.WriteLine("};")
}

// writesFragDepth reports whether result is frag_depth or has it as a
// member.
func (w *Writer) writesFragDepth(result *ir.FunctionResult) bool {
	if result == nil {
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

// outputValue is value, about to be written to the output with binding b,
// clamped to the range of the frag_depth clamp when it is frag_depth and
// the entry point clamps it.
func (w *Writer) outputValue(b *ir.Binding, value string) string {
	if !w.clampFragDepth || !isFragDepth(b) {
		return value
	}
	return fmt.Sprintf("clamp(%s, %s.%s, %s.%s)", value, fragImmediates, fragDepthMin, fragImmediates, fragDepthMax)
}
