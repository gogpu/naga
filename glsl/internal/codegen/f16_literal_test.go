package codegen

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// GLSL has no 16-bit float type. Rust naga's GLSL writer rejects an f16
// literal (back/glsl/writer.rs, write_possibly_const_expr:
// Literal::F16 => Error::Custom("GLSL has no 16-bit float type")) instead of
// writing a value. We wrote "0" (or, earlier, the f16 bits printed as a
// double), silently changing the value. Each place a literal can be written
// must return the error: a named module constant, a var<private>
// initializer and a literal in a function body.
func TestF16LiteralIsRejected(t *testing.T) {
	const wantErr = "GLSL has no 16-bit float type"
	tests := []struct {
		name string
		src  string
	}{
		{"module constant -65504h", `enable f16;
const MIN_F16: f16 = -65504h;
@compute @workgroup_size(1) fn main() { var x = MIN_F16; }
`},
		{"module constant 65504h", `enable f16;
const MAX_F16: f16 = 65504h;
@compute @workgroup_size(1) fn main() { var x = MAX_F16; }
`},
		{"private initializer 15.2h", `enable f16;
var<private> p: f16 = 15.2h;
@compute @workgroup_size(1) fn main() { p = p * p; }
`},
		{"function literal 15.2h", `enable f16;
@compute @workgroup_size(1) fn main() { var x: f16; x = x + 15.2h; }
`},
	}
	// The literal helpers themselves, for the three values of the f16 corpus
	// shaders (-65504, 65504, 15.2), as a literal and as an f16 constant's
	// init expression.
	for _, v := range []float32{-65504, 65504, 15.2} {
		module := &ir.Module{
			Types:             []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}},
			GlobalExpressions: []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF16(v)}}},
		}
		w := newWriter(module, &Options{LangVersion: Version330})
		if got, err := w.formatLiteral(ir.Literal{Value: ir.LiteralF16(v)}); !errors.Is(err, errNoFloat16) {
			t.Errorf("formatLiteral(F16(%v)) = %q, %v; want errNoFloat16", v, got, err)
		}
		if got, err := w.writeLiteral(ir.Literal{Value: ir.LiteralF16(v)}); !errors.Is(err, errNoFloat16) {
			t.Errorf("writeLiteral(F16(%v)) = %q, %v; want errNoFloat16", v, got, err)
		}
		if got, err := w.writeConstExpr(0); !errors.Is(err, errNoFloat16) {
			t.Errorf("writeConstExpr(F16(%v)) = %q, %v; want errNoFloat16", v, got, err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GLSL 4.30: the corpus shaders have compute entry points.
			out, _, err := compileWGSLHelper(tt.src, Options{LangVersion: Version430})
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("err = %v, want %q; output:\n%s", err, wantErr, out)
			}
			if out != "" {
				t.Errorf("output on error:\n%s", out)
			}
		})
	}
}
