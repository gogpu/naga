package front

// Contracts of the constructor shapes and builtin expansions the earlier tests
// did not reach: Rust naga's constructor_many matrix arm
// (front/glsl/functions.rs:414-514), vector_constructor with several arguments
// (358-412), matrix_one_arg's fall-through (211-356), and the macroCall arms of
// builtins.rs:1585-2060 that GLSL reaches through relational, bit and texture
// builtins.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestConstructorMatrixFromVectors is constructor_many's matrix arm
// (functions.rs:415-450): vector arguments flatten into the lanes and the
// matrix is rebuilt column by column.
func TestConstructorMatrixFromVectors(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec2 a; vec2 b; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m = mat2(u.a, u.b);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	if len(matrix.Components) != 2 {
		t.Fatalf("the matrix has %d columns, want 2", len(matrix.Components))
	}
	for i, column := range matrix.Components {
		compose, ok := exprKindOf(t, main, column).(ir.ExprCompose)
		if !ok || len(compose.Components) != 2 {
			t.Fatalf("column %d is %#v, want a vec2 compose", i, exprKindOf(t, main, column))
		}
	}
}

// TestConstructorMatrixFromScalars is the scalar lane of the same arm
// (functions.rs:452-470): four scalars fill a mat2 column by column.
func TestConstructorMatrixFromScalars(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { float x; float y; float z; float w; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m = mat2(u.x, u.y, u.z, u.w);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	lanes := 0
	for _, column := range matrix.Components {
		compose, ok := exprKindOf(t, main, column).(ir.ExprCompose)
		if !ok {
			t.Fatalf("a column is %#v, want a vec2 compose", exprKindOf(t, main, column))
		}
		lanes += len(compose.Components)
	}
	if lanes != 4 {
		t.Fatalf("the matrix has %d lanes, want the four scalar arguments", lanes)
	}
}

// TestConstructorVectorFromMixedArguments is vector_constructor's scalar and
// truncation arms (functions.rs:358-412): a vector argument flattens, a scalar
// contributes one lane, and anything past the target size is dropped.
func TestConstructorVectorFromMixedArguments(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec4 v; float x; float y; } u;
layout(location = 0) out vec4 o;
void main() {
    vec2 pair = vec2(u.x, u.y);
    vec4 four = vec4(u.v, u.x);
    o = four + vec4(pair, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	// The vec4 takes exactly four lanes from the five written arguments.
	found := false
	for _, expr := range main.Expressions {
		compose, ok := expr.Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		if vector, ok := mod.Types[compose.Type].Inner.(ir.VectorType); ok && vector.Size == ir.Vec4 && len(compose.Components) == 4 {
			found = true
		}
	}
	if !found {
		t.Fatal("no vec4 compose of four lanes: the vector argument or the truncation is missing")
	}
}

// TestConstructorMatrixFromVectorSource is matrix_one_arg's fall-through
// (functions.rs:330-345): a source that is neither a scalar nor a matrix fills
// every column with the expression, as Rust does.
func TestConstructorMatrixFromVectorSource(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform U { vec4 v; } u;
layout(location = 0) out vec4 o;
void main() {
    mat2 m = mat2(u.v);
    o = vec4(m[0], m[1]);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	matrix, _ := composeOfType(t, mod, main, ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32Scalar})
	if len(matrix.Components) != 2 {
		t.Fatalf("the matrix has %d columns, want 2", len(matrix.Components))
	}
	if matrix.Components[0] != matrix.Components[1] {
		t.Fatalf("the columns are %d and %d, want the same source expression", matrix.Components[0], matrix.Components[1])
	}
}

// TestConstructorCombinedSamplerCall is functionOrConstructorCall's combined
// sampler arm (functions.rs:33-38 with design §11): sampler2D(t, s) is a call
// of the sampler builtin over an image and a sampler, and the pair reaches the
// texture call through ctx.samplers.
func TestConstructorCombinedSamplerCall(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform texture2D t;
layout(set = 0, binding = 1) uniform sampler s;
layout(location = 0) out vec4 o;
void main() {
    o = texture(sampler2D(t, s), vec2(0.5));
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	main := functionNamed(t, mod, "main")
	found := false
	for _, expr := range main.Expressions {
		if _, ok := expr.Kind.(ir.ExprImageSample); ok {
			found = true
		}
	}
	if !found {
		t.Fatal("no image sample: the sampler constructor did not reach texture()")
	}
}

// TestMacroCallSingleArgumentExpansions covers the macroCall arms that need one
// or two operands (builtins.rs:1585-2060): relational, unary, binary, mod,
// bit cast and derivative.
func TestMacroCallSingleArgumentExpansions(t *testing.T) {
	tests := []struct {
		name  string
		macro macroCall
		args  []ir.TypeInner
		check func(t *testing.T, c *Context, h ir.ExpressionHandle)
	}{
		{
			name:  "relational",
			macro: macroCall{kind: macroRelational, relational: ir.RelationalAll},
			args:  []ir.TypeInner{f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				rel := exprAs[ir.ExprRelational](t, c, h, "the relational expansion")
				if rel.Fun != ir.RelationalAll {
					t.Fatalf("relational function = %v, want All", rel.Fun)
				}
			},
		},
		{
			name:  "unary",
			macro: macroCall{kind: macroUnary, unary: ir.UnaryBitwiseNot},
			args:  []ir.TypeInner{i32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				unary := exprAs[ir.ExprUnary](t, c, h, "the unary expansion")
				if unary.Op != ir.UnaryBitwiseNot {
					t.Fatalf("unary operator = %v, want BitwiseNot", unary.Op)
				}
			},
		},
		{
			name:  "binary",
			macro: macroCall{kind: macroBinary, binary: ir.BinaryMultiply},
			args:  []ir.TypeInner{f32Scalar, f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				binary := exprAs[ir.ExprBinary](t, c, h, "the binary expansion")
				if binary.Op != ir.BinaryMultiply {
					t.Fatalf("binary operator = %v, want Multiply", binary.Op)
				}
			},
		},
		{
			name:  "mod",
			macro: macroCall{kind: macroMod},
			args:  []ir.TypeInner{f32Scalar, f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				sub := exprAs[ir.ExprBinary](t, c, h, "the mod expansion")
				if sub.Op != ir.BinarySubtract {
					t.Fatalf("mod result = %v, want the final subtraction", sub.Op)
				}
			},
		},
		{
			name:  "bit cast",
			macro: macroCall{kind: macroBitCast, cast: ir.ScalarSint},
			args:  []ir.TypeInner{f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				as := exprAs[ir.ExprAs](t, c, h, "the bit cast")
				if as.Kind != ir.ScalarSint {
					t.Fatalf("cast kind = %v, want sint", as.Kind)
				}
			},
		},
		{
			name:  "derivative",
			macro: macroCall{kind: macroDerivate, axis: ir.DerivativeX, control: ir.DerivativeNone},
			args:  []ir.TypeInner{f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				derivative := exprAs[ir.ExprDerivative](t, c, h, "the derivative")
				if derivative.Axis != ir.DerivativeX {
					t.Fatalf("axis = %v, want X", derivative.Axis)
				}
			},
		},
		{
			name:  "smooth step",
			macro: macroCall{kind: macroSmoothStep},
			args:  []ir.TypeInner{f32Scalar, f32Scalar, f32Scalar},
			check: func(t *testing.T, c *Context, h ir.ExpressionHandle) {
				math := exprAs[ir.ExprMath](t, c, h, "the smoothstep")
				if math.Fun != ir.MathSmoothStep {
					t.Fatalf("math function = %v, want SmoothStep", math.Fun)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe, c := newTestContext(t)
			args := make([]loweredArg, len(tt.args))
			for i, inner := range tt.args {
				args[i] = loweredArg{expr: dynamicArg(t, c, fe.registerType(inner)), span: ir.Span{}}
			}
			macro := tt.macro
			h, ok, err := macro.call(c, args, ir.Span{})
			if err != nil {
				t.Fatalf("macro call: %v", err)
			}
			if !ok {
				t.Fatal("the expansion reported no value")
			}
			tt.check(t, c, h)
		})
	}
}

// TestMacroCallBarrier is the statement-shaped expansion of barrier()
// (builtins.rs:2030-2045): no value, one control-barrier statement.
func TestMacroCallBarrier(t *testing.T) {
	_, c := newTestContext(t)
	macro := macroCall{kind: macroBarrier}
	_, ok, err := macro.call(c, nil, ir.Span{})
	if err != nil {
		t.Fatalf("macro call: %v", err)
	}
	if ok {
		t.Fatal("barrier reported a value")
	}
	found := false
	for _, stmt := range c.body {
		if barrier, isBarrier := stmt.Kind.(ir.StmtControlBarrier); isBarrier {
			found = true
			if barrier.Flags != barrierAll {
				t.Fatalf("barrier flags = %v, want every class", barrier.Flags)
			}
		}
	}
	if !found {
		t.Fatal("barrier emitted no control-barrier statement")
	}
}

// TestMacroCallTooFewArguments is the arity guard of macroCall (builtins.rs
// indexes its slice directly; Go must report instead of panicking).
func TestMacroCallTooFewArguments(t *testing.T) {
	_, c := newTestContext(t)
	macro := macroCall{kind: macroBinary}
	_, _, err := macro.call(c, []loweredArg{{expr: 0, span: ir.Span{}}}, ir.Span{})
	if err == nil || err.Message != "Internal error: builtin called with too few arguments" {
		t.Fatalf("error = %v, want the too-few-arguments error", err)
	}
}

// TestMacroCallTextureSizeNonArrayed is callTextureSize's non-arrayed path
// (builtins.rs:1719-1783): the size query converted to a signed 32-bit int.
func TestMacroCallTextureSizeNonArrayed(t *testing.T) {
	fe, c := newTestContext(t)
	imageTy := fe.registerType(sampledImage(ir.Dim2D, false, false, ir.ScalarFloat))
	image := declareImageGlobal(t, fe, c, "tex", imageTy)
	macro := macroCall{kind: macroTextureSize}
	h, ok, err := macro.call(c, []loweredArg{{expr: image, span: ir.Span{}}}, ir.Span{})
	if err != nil {
		t.Fatalf("macro call: %v", err)
	}
	if !ok {
		t.Fatal("textureSize reported no value")
	}
	as := exprAs[ir.ExprAs](t, c, h, "the converted size")
	if as.Kind != ir.ScalarSint {
		t.Fatalf("conversion kind = %v, want sint", as.Kind)
	}
	if _, ok := c.Expression(as.Expr).(ir.ExprImageQuery); !ok {
		t.Fatalf("converted expression is %T, want the size query", c.Expression(as.Expr))
	}
}

// TestMacroCallImageLoadMultisampled is callImageLoad's multisampled arm
// (builtins.rs:1802-1819): the third argument is a sample index, not a level.
func TestMacroCallImageLoadMultisampled(t *testing.T) {
	fe, c := newTestContext(t)
	imageTy := fe.registerType(sampledImage(ir.Dim2D, false, true, ir.ScalarFloat))
	image := declareImageGlobal(t, fe, c, "tex", imageTy)
	coord := dynamicArg(t, c, fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: i32Scalar}))
	sample := dynamicArg(t, c, fe.registerType(i32Scalar))
	macro := macroCall{kind: macroImageLoad, multi: true}
	h, ok, err := macro.call(c, []loweredArg{{expr: image}, {expr: coord}, {expr: sample}}, ir.Span{})
	if err != nil {
		t.Fatalf("macro call: %v", err)
	}
	if !ok {
		t.Fatal("imageLoad reported no value")
	}
	load := exprAs[ir.ExprImageLoad](t, c, h, "the image load")
	if load.Sample == nil || *load.Sample != sample {
		t.Fatalf("image load sample = %v, want the sample argument", load.Sample)
	}
	if load.Level != nil {
		t.Fatalf("image load level = %v, want none for a multisampled image", *load.Level)
	}
}
