package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestRemapExpressionHandlesImageQueryLod: the level-of-detail query's sampler
// and coordinate are expression handles like its image, so a remap must
// rewrite all three. A walker that missed them would leave the query pointing
// at expressions a compaction has already moved.
func TestRemapExpressionHandlesImageQueryLod(t *testing.T) {
	remap := make([]ir.ExpressionHandle, 8)
	for i := range remap {
		remap[i] = ir.ExpressionHandle(i + 10)
	}
	kind := ir.ExprImageQuery{Image: 1, Query: ir.ImageQueryLod{Sampler: 2, Coordinate: 3}}

	remapped, ok := RemapExpressionHandles(kind, remap).(ir.ExprImageQuery)
	if !ok {
		t.Fatalf("RemapExpressionHandles returned %T, want ir.ExprImageQuery", remapped)
	}
	if remapped.Image != 11 {
		t.Errorf("Image = %d, want 11", remapped.Image)
	}
	query, ok := remapped.Query.(ir.ImageQueryLod)
	if !ok {
		t.Fatalf("Query is %T, want ir.ImageQueryLod", remapped.Query)
	}
	if query.Sampler != 12 || query.Coordinate != 13 {
		t.Errorf("Query = %+v, want Sampler 12 and Coordinate 13", query)
	}
}
