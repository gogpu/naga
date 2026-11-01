package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// compactTypes must keep the types SpecialTypes names and renumber those
// handles, as Rust naga's compact traces them (trace_special_types) and
// adjusts them (adjust_special_types). It kept the named RayIntersection but
// left SpecialTypes pointing at its old index, so once an unused type before
// it was removed every backend indexed past the arena: ray query shaders
// panicked (P0-IR-025). An unnamed special type was dropped outright.
func TestCompactTypesKeepsAndRemapsSpecialTypes(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	th := func(h ir.TypeHandle) *ir.TypeHandle { return &h }
	m := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: u32}},                                                                  // [0] unused, removed
			{Name: "RayIntersection", Inner: ir.StructType{Members: []ir.StructMember{{Name: "kind", Type: 2}}, Span: 4}}, // [1]
			{Inner: u32}, // [2] member of [1]
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "a", Type: 2}}, Span: 4}}, // [3] unnamed, only a special type
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "b", Type: 2}}, Span: 4}}, // [4] unnamed, only a special type
		},
		SpecialTypes: ir.SpecialTypes{
			RayIntersection:                 th(1),
			ExternalTextureParams:           th(3),
			ExternalTextureTransferFunction: th(4),
		},
	}

	Compact(m, KeepUnusedNo)

	check := func(name string, h *ir.TypeHandle, wantMember string) {
		t.Helper()
		if h == nil {
			t.Fatalf("SpecialTypes.%s is nil", name)
		}
		if int(*h) >= len(m.Types) {
			t.Fatalf("SpecialTypes.%s = %d, past the %d types", name, *h, len(m.Types))
		}
		st, ok := m.Types[*h].Inner.(ir.StructType)
		if !ok || len(st.Members) != 1 || st.Members[0].Name != wantMember {
			t.Errorf("SpecialTypes.%s = %d names %+v, want the struct with member %q", name, *h, m.Types[*h], wantMember)
		}
	}
	check("RayIntersection", m.SpecialTypes.RayIntersection, "kind")
	check("ExternalTextureParams", m.SpecialTypes.ExternalTextureParams, "a")
	check("ExternalTextureTransferFunction", m.SpecialTypes.ExternalTextureTransferFunction, "b")
	if len(m.Types) != 4 {
		t.Errorf("%d types after compaction, want 4 (only the unused vector removed)", len(m.Types))
	}
}

// The output types of a mesh entry point are used by it, as Rust's compact
// traces MeshStageInfo; unnamed ones were dropped.
func TestCompactTypesKeepsMeshOutputTypes(t *testing.T) {
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	m := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: u32}}, // [0] unused
			{Inner: u32}, // [1]
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "v", Type: 1}}, Span: 4}}, // [2] vertex output
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "p", Type: 1}}, Span: 4}}, // [3] primitive output
		},
		EntryPoints: []ir.EntryPoint{{
			Name:     "ms",
			Stage:    ir.StageMesh,
			MeshInfo: &ir.MeshStageInfo{VertexOutputType: 2, PrimitiveOutputType: 3},
		}},
	}

	Compact(m, KeepUnusedNo)

	mi := m.EntryPoints[0].MeshInfo
	for name, h := range map[string]ir.TypeHandle{"v": mi.VertexOutputType, "p": mi.PrimitiveOutputType} {
		if int(h) >= len(m.Types) {
			t.Fatalf("mesh output %q handle %d past the %d types", name, h, len(m.Types))
		}
		st, ok := m.Types[h].Inner.(ir.StructType)
		if !ok || st.Members[0].Name != name {
			t.Errorf("mesh output handle %d is %+v, want the struct with member %q", h, m.Types[h], name)
		}
	}
}
