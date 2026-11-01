package valid

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
)

// TestValidateRejectsSSAKinds checks that a module holding one of the DXIL
// backend's SSA kinds (package ssa) fails validation with an unknown
// expression kind instead of being accepted or crashing the validator. No
// public path creates such a module: the kinds exist only in the DXIL
// backend's own copy of a module (snapshot TestSSAKindsStayInDXIL). Should
// one leak, the validator says so.
func TestValidateRejectsSSAKinds(t *testing.T) {
	tests := []struct {
		name string
		kind ir.ExpressionKind
		want string
	}{
		{"alias", ssa.Alias{Source: 0}, "unknown expression kind ssa.Alias"},
		{"phi", ssa.Phi{Incoming: []ssa.Incoming{
			{PredKey: ssa.PredIfAccept, Value: 0},
			{PredKey: ssa.PredIfReject, Value: 1},
		}}, "unknown expression kind ssa.Phi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := newValidModule()
			fn := &module.Functions[0]
			fn.Expressions = append(fn.Expressions, ir.Expression{Kind: tt.kind})
			fn.Body = ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}}}

			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Validate panicked on %T: %v", tt.kind, r)
					}
				}()
				_, err = Validate(module, Options{Capabilities: CapAll})
			}()
			if err == nil {
				t.Fatalf("Validate accepted a module holding %T", tt.kind)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate error %q does not contain %q", err, tt.want)
			}
		})
	}
}

// TestValidateKnowsResultKinds checks that the kinds the expression switch
// accepts without a check of their own are not taken for unknown ones.
func TestValidateKnowsResultKinds(t *testing.T) {
	for _, kind := range []ir.ExpressionKind{
		ir.ExprOverride{Override: 0},
		ir.ExprAtomicResult{Ty: 2},
		ir.ExprWorkGroupUniformLoadResult{Ty: 2},
	} {
		module := newValidModule()
		fn := &module.Functions[0]
		fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
		_, err := Validate(module, Options{Capabilities: CapAll})
		if err != nil && strings.Contains(err.Error(), "unknown expression kind") {
			t.Errorf("Validate took %T for an unknown kind: %v", kind, err)
		}
	}
}
