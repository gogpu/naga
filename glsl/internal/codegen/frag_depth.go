// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// writeFragDepthRangeUniform declares the uniform that holds the range of
// the frag_depth clamp, when Options.ClampFragDepth asks for it and the
// selected entry point returns frag_depth.
func (w *Writer) writeFragDepthRangeUniform() {
	ep := w.getSelectedEntryPoint()
	if !w.options.ClampFragDepth || ep == nil || ep.Stage != ir.StageFragment || !w.writesFragDepth(ep.Function.Result) {
		return
	}
	precision := ""
	if w.options.LangVersion.ES {
		precision = "highp "
	}
	w.WriteLine("uniform %svec2 %s;", precision, FragDepthRangeUniform)
	w.WriteLine("")
	w.clampFragDepth = true
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
	return fmt.Sprintf("clamp(%s, %s.x, %s.y)", value, FragDepthRangeUniform, FragDepthRangeUniform)
}
