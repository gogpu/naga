// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

// textureSamples is GLSL 4.00's sample-count query (GLSL 4.50 §8.9): one
// overload takes a gsampler2DMS and one a gsampler2DMSArray, and both return
// the number of samples of a multisampled image as an int. The frontend lowers
// it to the IR's ImageQueryNumSamples, the query every backend already writes
// (card P2-GLSLIN-014). These tests start from GLSL source and read the IR the
// frontend produced, which is what the end-to-end matrix in snapshot/ then
// drives through glslangValidator, spirv-val and the text backends.

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// TestTextureSamplesLowersToNumSamples parses a fragment shader that asks the
// sample count of a multisampled sampler and checks the IR: exactly one
// ImageQueryNumSamples of the declared texture, converted to the int the GLSL
// builtin returns.
func TestTextureSamplesLowersToNumSamples(t *testing.T) {
	tests := []struct {
		name    string
		declare string
		arrayed bool
	}{
		{"sampler2DMS", "sampler2DMS", false},
		{"sampler2DMSArray", "sampler2DMSArray", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := fmt.Sprintf(`#version 450
layout(set = 0, binding = 0) uniform %s s;
layout(location = 0) out ivec4 count;
void main() { count = ivec4(textureSamples(s), 0, 0, 0); }
`, tt.declare)
			module, _, err := glsl.Parse(source, glsl.ParseOptions{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v\n\nsource:\n%s", err, source)
			}
			if _, verr := valid.Validate(module, valid.Options{}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}

			fn, query, queryHandle := findNumSamplesQuery(t, module)
			want := ir.ImageType{Dim: ir.Dim2D, Arrayed: tt.arrayed, Multisampled: true,
				Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}
			if image := imageOfExpression(t, module, fn, query.Image); image != want {
				t.Fatalf("the query's image is %+v, want %+v", image, want)
			}

			// The builtin returns int while the IR query yields uint, so the
			// frontend converts the result to a signed 32-bit integer (Rust
			// naga's call_as_int).
			if !convertedFromQuery(fn, queryHandle) {
				t.Fatal("the sample-count query is not converted to a signed integer")
			}
		})
	}
}

// TestTextureSamplesBothFormsInOneShader drives both overloads through one
// declaration: the variants are injected incrementally (the 2D multisampled
// array is requested by its argument, builtins.rs:2313-2365), so a shader that
// asks both must find both.
func TestTextureSamplesBothFormsInOneShader(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform sampler2DMS s;
layout(set = 0, binding = 1) uniform sampler2DMSArray sa;
layout(location = 0) out ivec4 count;
void main() { count = ivec4(textureSamples(s) + textureSamples(sa), 0, 0, 0); }
`
	module, _, err := glsl.Parse(source, glsl.ParseOptions{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v\n\nsource:\n%s", err, source)
	}
	if _, verr := valid.Validate(module, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}

	arrayed := map[bool]int{}
	var kinds []string
	for _, fn := range moduleFunctions(module) {
		for _, expr := range fn.Expressions {
			kinds = append(kinds, fmt.Sprintf("%T", expr.Kind))
			query, ok := expr.Kind.(ir.ExprImageQuery)
			if !ok {
				continue
			}
			if _, isSamples := query.Query.(ir.ImageQueryNumSamples); !isSamples {
				continue
			}
			arrayed[imageOfExpression(t, module, fn, query.Image).Arrayed]++
		}
	}
	if arrayed[false] != 1 || arrayed[true] != 1 {
		t.Fatalf("the sample-count queries are %v, want one of a 2DMS image and one of a 2DMS array; expressions: %v",
			arrayed, kinds)
	}
}

// findNumSamplesQuery returns the module's single ImageQueryNumSamples
// expression, the function holding it and its handle.
func findNumSamplesQuery(t *testing.T, module *ir.Module) (*ir.Function, ir.ExprImageQuery, ir.ExpressionHandle) {
	t.Helper()
	var (
		found  *ir.Function
		query  ir.ExprImageQuery
		handle ir.ExpressionHandle
	)
	count := 0
	for _, fn := range moduleFunctions(module) {
		for i, expr := range fn.Expressions {
			kind, ok := expr.Kind.(ir.ExprImageQuery)
			if !ok {
				continue
			}
			if _, isSamples := kind.Query.(ir.ImageQueryNumSamples); !isSamples {
				continue
			}
			count++
			found, query, handle = fn, kind, ir.ExpressionHandle(i)
		}
	}
	if count != 1 {
		t.Fatalf("the module has %d ImageQueryNumSamples expressions, want exactly one", count)
	}
	return found, query, handle
}

// moduleFunctions lists every function body of the module: the entry points,
// which the IR keeps inline, and the regular functions.
func moduleFunctions(module *ir.Module) []*ir.Function {
	out := make([]*ir.Function, 0, len(module.EntryPoints)+len(module.Functions))
	for i := range module.EntryPoints {
		out = append(out, &module.EntryPoints[i].Function)
	}
	for i := range module.Functions {
		out = append(out, &module.Functions[i])
	}
	return out
}

// imageOfExpression resolves the image type an expression of fn denotes: a
// global variable's type, the shape of a sampled-image operand.
func imageOfExpression(t *testing.T, module *ir.Module, fn *ir.Function, expr ir.ExpressionHandle) ir.ImageType {
	t.Helper()
	if int(expr) >= len(fn.Expressions) {
		t.Fatalf("expression %d is out of range", expr)
	}
	global, ok := fn.Expressions[expr].Kind.(ir.ExprGlobalVariable)
	if !ok {
		t.Fatalf("the query's image expression %d is %T, want a global variable", expr, fn.Expressions[expr].Kind)
	}
	image, ok := module.Types[module.GlobalVariables[global.Variable].Type].Inner.(ir.ImageType)
	if !ok {
		t.Fatalf("global %q is not an image", module.GlobalVariables[global.Variable].Name)
	}
	return image
}

// convertedFromQuery reports whether a width-4 conversion to sint reads the
// expression at query.
func convertedFromQuery(fn *ir.Function, query ir.ExpressionHandle) bool {
	for _, expr := range fn.Expressions {
		convert, ok := expr.Kind.(ir.ExprAs)
		if !ok || convert.Kind != ir.ScalarSint || convert.Convert == nil || *convert.Convert != 4 {
			continue
		}
		if convert.Expr == query {
			return true
		}
	}
	return false
}
