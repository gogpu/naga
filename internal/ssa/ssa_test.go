package ssa

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// kindName names the case of a type switch over ir kinds that kind takes.
func kindName(kind ir.ExpressionKind) string {
	switch kind.(type) {
	case ir.ExprRayQueryProceedResult:
		return "ir.ExprRayQueryProceedResult"
	case Alias:
		return "ssa.Alias"
	case Phi:
		return "ssa.Phi"
	default:
		return "other"
	}
}

// TestKindsAreNotTheirEmbeddedKind proves that embedding
// ir.ExprRayQueryProceedResult only seals the ssa kinds into
// ir.ExpressionKind: a type switch that has a case for the embedded kind
// does not take it for an Alias or a Phi, and an ir code path matching
// ray-query results never mistakes one of them for a ray-query result.
func TestKindsAreNotTheirEmbeddedKind(t *testing.T) {
	tests := []struct {
		kind ir.ExpressionKind
		want string
	}{
		{Alias{Source: 3}, "ssa.Alias"},
		{Phi{Incoming: []Incoming{{PredKey: PredIfAccept, Value: 1}}}, "ssa.Phi"},
		{ir.ExprRayQueryProceedResult{}, "ir.ExprRayQueryProceedResult"},
	}
	for _, tt := range tests {
		if got := kindName(tt.kind); got != tt.want {
			t.Errorf("%T takes the %s case, want %s", tt.kind, got, tt.want)
		}
		_, isRayQuery := tt.kind.(ir.ExprRayQueryProceedResult)
		if want := tt.want == "ir.ExprRayQueryProceedResult"; isRayQuery != want {
			t.Errorf("%T asserts to ir.ExprRayQueryProceedResult: %v, want %v", tt.kind, isRayQuery, want)
		}
	}

	// A switch with no case for the ssa kinds sends them to its default,
	// which is where a public consumer reports an unknown kind.
	for _, kind := range []ir.ExpressionKind{Alias{}, Phi{}} {
		switch kind.(type) {
		case ir.ExprRayQueryProceedResult, ir.ExprLoad, ir.Literal:
			t.Errorf("%T matched an ir kind's case", kind)
		default:
		}
	}
}

// TestPredKeyValues pins the values of the predecessor keys, the same as
// those of the ir.PhiPred* constants they replaced.
func TestPredKeyValues(t *testing.T) {
	keys := []PredKey{PredIfAccept, PredIfReject, PredLoopInit, PredLoopBackEdge, PredSwitchCase, PredFallThrough}
	for i, k := range keys {
		if int(k) != i {
			t.Errorf("predecessor key #%d has value %d", i, k)
		}
	}
}
