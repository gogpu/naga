// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"maps"

	"github.com/gogpu/naga/ir"
)

// depthArg is an argument of a function: the function's handle and the
// argument's index.
type depthArg struct {
	function ir.FunctionHandle
	index    uint32
}

// depthUse is a sample or load of a texture: the texture, and whether the
// sample compares against a reference.
type depthUse struct {
	image   samplingOperand
	compare bool
}

// scanDepthTextures decides how the selected entry point declares its depth
// textures. One it samples with a comparison is a shadow sampler, such as
// sampler2DShadow; any other is the sampler of a float texture, such as
// sampler2D, which a sample without a comparison and texelFetch take and
// which reads the depth in its first component. GLSL has no type that does
// both, so a depth texture that is sampled with a comparison and also
// sampled without one or loaded is an error. A function argument has the
// type of the textures the entry point's calls pass to it, which must be of
// one kind; one that no call passes a texture to, of a function written for
// the entry point all the same, is typed by its own uses.
func (w *Writer) scanDepthTextures() error {
	ep := w.getSelectedEntryPoint()
	if ep == nil {
		return nil
	}
	memo := make(map[ir.FunctionHandle][]depthUse)
	compared := make(map[samplingOperand]bool)
	plain := make(map[samplingOperand]bool)
	for _, u := range w.functionDepthUses(&ep.Function, memo) {
		if !u.image.argument {
			compared[u.image] = compared[u.image] || u.compare
			plain[u.image] = plain[u.image] || !u.compare
		}
	}
	w.plainDepth = make(map[ir.GlobalVariableHandle]bool)
	for h, global := range w.module.GlobalVariables {
		g := samplingOperand{index: uint32(h)}
		if !w.isSingleSampledDepth(global.Type) {
			continue
		}
		if compared[g] && plain[g] {
			return fmt.Errorf("depth texture %q is sampled with a comparison and also sampled without one or loaded, which GLSL declares as different sampler types", global.Name)
		}
		w.plainDepth[ir.GlobalVariableHandle(h)] = !compared[g]
	}

	w.plainDepthArgs = make(map[depthArg]bool)
	reach := w.depthArgGlobals(ep)
	for h := range w.module.Functions {
		fn := &w.module.Functions[h]
		uses := w.functionDepthUses(fn, memo)
		for i, arg := range fn.Arguments {
			if !w.isSingleSampledDepth(arg.Type) {
				continue
			}
			key := depthArg{function: ir.FunctionHandle(h), index: uint32(i)}
			kinds := make(map[bool]bool)
			for g := range reach[key] {
				kinds[w.plainDepth[g]] = true
			}
			if len(kinds) == 0 {
				for _, u := range uses {
					if u.image == (samplingOperand{argument: true, index: uint32(i)}) {
						kinds[!u.compare] = true
					}
				}
			}
			if len(kinds) > 1 {
				return fmt.Errorf("function %q takes depth textures declared as different sampler types as argument %d", fn.Name, i)
			}
			w.plainDepthArgs[key] = kinds[true]
		}
	}
	return nil
}

// isSingleSampledDepth reports whether t is a depth texture that isn't
// multisampled, which GLSL declares as a shadow sampler or not.
func (w *Writer) isSingleSampledDepth(t ir.TypeHandle) bool {
	if int(t) >= len(w.module.Types) {
		return false
	}
	img, ok := w.module.Types[t].Inner.(ir.ImageType)
	return ok && img.Class == ir.ImageClassDepth && !img.Multisampled
}

// functionDepthUses returns the samples and loads of fn and of the functions
// it calls, with the textures of a callee's mapped through the call's
// arguments, as functionSamplings maps samplings. memo holds the uses of
// the functions already done.
func (w *Writer) functionDepthUses(fn *ir.Function, memo map[ir.FunctionHandle][]depthUse) []depthUse {
	var out []depthUse
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprImageSample:
			if image, ok := samplingOperandOf(fn, e.Image); ok {
				out = append(out, depthUse{image: image, compare: e.DepthRef != nil})
			}
		case ir.ExprImageLoad:
			if image, ok := samplingOperandOf(fn, e.Image); ok {
				out = append(out, depthUse{image: image})
			}
		}
	}
	forEachCall(fn.Body, func(call ir.StmtCall) {
		if int(call.Function) >= len(w.module.Functions) {
			return
		}
		callee, done := memo[call.Function]
		if !done {
			callee = w.functionDepthUses(&w.module.Functions[call.Function], memo)
			memo[call.Function] = callee
		}
		for _, u := range callee {
			if !u.image.argument {
				out = append(out, u)
				continue
			}
			if int(u.image.index) >= len(call.Arguments) {
				continue
			}
			if image, ok := samplingOperandOf(fn, call.Arguments[u.image.index]); ok {
				out = append(out, depthUse{image: image, compare: u.compare})
			}
		}
	})
	return out
}

// depthArgGlobals returns, for each argument of the functions ep calls
// directly or not, the texture globals its calls pass to it.
func (w *Writer) depthArgGlobals(ep *ir.EntryPoint) map[depthArg]map[ir.GlobalVariableHandle]bool {
	reach := make(map[depthArg]map[ir.GlobalVariableHandle]bool)
	visit := func(fn *ir.Function, self ir.FunctionHandle, isEntry bool) bool {
		grew := false
		forEachCall(fn.Body, func(call ir.StmtCall) {
			if int(call.Function) >= len(w.module.Functions) {
				return
			}
			for i, a := range call.Arguments {
				op, ok := samplingOperandOf(fn, a)
				if !ok {
					continue
				}
				passed := map[ir.GlobalVariableHandle]bool{ir.GlobalVariableHandle(op.index): true}
				if op.argument {
					if isEntry {
						continue
					}
					passed = reach[depthArg{function: self, index: op.index}]
				}
				key := depthArg{function: call.Function, index: uint32(i)}
				if reach[key] == nil {
					reach[key] = make(map[ir.GlobalVariableHandle]bool)
				}
				n := len(reach[key])
				maps.Copy(reach[key], passed)
				grew = grew || len(reach[key]) > n
			}
		})
		return grew
	}
	visit(&ep.Function, 0, true)
	for grew := true; grew; {
		grew = false
		for h := range w.module.Functions {
			if visit(&w.module.Functions[h], ir.FunctionHandle(h), false) {
				grew = true
			}
		}
	}
	return reach
}

// isPlainDepth reports whether image, an expression of the function being
// written, is a depth texture declared as the sampler of a float texture.
func (w *Writer) isPlainDepth(image ir.ExpressionHandle) bool {
	op, ok := samplingOperandOf(w.currentFunction, image)
	if !ok {
		return false
	}
	if op.argument {
		return w.plainDepthArgs[depthArg{function: w.currentFuncHandle, index: op.index}]
	}
	return w.plainDepth[ir.GlobalVariableHandle(op.index)]
}

// isDepthImage reports whether image, an expression of the function being
// written, is a depth texture, multisampled or not.
func (w *Writer) isDepthImage(image ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(image) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}
	t, ok := w.resolveTypeInner(&w.currentFunction.ExpressionTypes[image], image).(ir.ImageType)
	return ok && t.Class == ir.ImageClassDepth
}

// depthImageTypeName returns the GLSL type of a texture of type t, which is
// the sampler of a float texture for a depth texture when plain is set.
func (w *Writer) depthImageTypeName(t ir.ImageType, plain bool) string {
	if plain && t.Class == ir.ImageClassDepth {
		t = ir.ImageType{Dim: t.Dim, Arrayed: t.Arrayed, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
	}
	return w.imageToGLSL(t)
}
