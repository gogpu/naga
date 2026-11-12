package container

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestMapBuiltinToSemantic(t *testing.T) {
	tests := []struct {
		name     string
		builtin  ir.BuiltinValue
		wantName string
		wantSV   SystemValueKind
	}{
		{"Position", ir.BuiltinPosition, "SV_Position", SVPosition},
		{"VertexIndex", ir.BuiltinVertexIndex, "SV_VertexID", SVVertexID},
		{"InstanceIndex", ir.BuiltinInstanceIndex, "SV_InstanceID", SVInstanceID},
		{"FrontFacing", ir.BuiltinFrontFacing, "SV_IsFrontFace", SVIsFrontFace},
		{"FragDepth", ir.BuiltinFragDepth, "SV_Depth", SVDepth},
		{"SampleIndex", ir.BuiltinSampleIndex, "SV_SampleIndex", SVSampleIndex},
		{"ClipDistance", ir.BuiltinClipDistances, "SV_ClipDistance", SVClipDistance},
		{"CullDistance", ir.BuiltinCullDistance, "SV_CullDistance", SVCullDistance},
		{"SampleMask", ir.BuiltinSampleMask, "SV_Coverage", SVCoverage},
		{"PrimitiveIndex", ir.BuiltinPrimitiveIndex, "SV_PrimitiveID", SVPrimitiveID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := MapBuiltinToSemantic(tt.builtin)
			if !ok {
				t.Fatalf("MapBuiltinToSemantic(%v) = not mapped, want %q", tt.builtin, tt.wantName)
			}
			if m.SemanticName != tt.wantName {
				t.Errorf("name: got %q, want %q", m.SemanticName, tt.wantName)
			}
			if m.SystemValue != tt.wantSV {
				t.Errorf("system value: got %d, want %d", m.SystemValue, tt.wantSV)
			}
		})
	}
}

// TestMapBuiltinToSemanticUnmapped pins the default: a built-in without a
// signature system value, or one the emitter cannot express, is reported as
// unmapped, never named SV_Unknown.
func TestMapBuiltinToSemanticUnmapped(t *testing.T) {
	for _, b := range []ir.BuiltinValue{
		ir.BuiltinBaseInstance, ir.BuiltinBaseVertex, ir.BuiltinPointCoord, ir.BuiltinWorkGroupSize,
		ir.BuiltinLocalInvocationID,
	} {
		if m, ok := MapBuiltinToSemantic(b); ok {
			t.Errorf("MapBuiltinToSemantic(%v) = %+v, want not mapped", b, m)
		}
	}
}

func TestMapLocationToInputSemantic(t *testing.T) {
	tests := []struct {
		loc       uint32
		wantName  string
		wantIndex uint32
	}{
		{0, "LOC", 0},
		{1, "LOC", 1},
		{5, "LOC", 5},
	}

	for _, tt := range tests {
		m := MapLocationToInputSemantic(tt.loc)
		if m.SemanticName != tt.wantName {
			t.Errorf("loc %d: name: got %q, want %q", tt.loc, m.SemanticName, tt.wantName)
		}
		if m.SemanticIndex != tt.wantIndex {
			t.Errorf("loc %d: index: got %d, want %d", tt.loc, m.SemanticIndex, tt.wantIndex)
		}
		if m.SystemValue != SVArbitrary {
			t.Errorf("loc %d: system value: got %d, want SVArbitrary", tt.loc, m.SystemValue)
		}
	}
}

func TestMapLocationToOutputSemantic_Fragment(t *testing.T) {
	m := MapLocationToOutputSemantic(0, true)
	if m.SemanticName != "SV_Target" {
		t.Errorf("name: got %q, want %q", m.SemanticName, "SV_Target")
	}
	if m.SemanticIndex != 0 {
		t.Errorf("index: got %d, want 0", m.SemanticIndex)
	}
	if m.SystemValue != SVTarget {
		t.Errorf("system value: got %d, want %d", m.SystemValue, SVTarget)
	}

	m2 := MapLocationToOutputSemantic(2, true)
	if m2.SemanticIndex != 2 {
		t.Errorf("loc 2 index: got %d, want 2", m2.SemanticIndex)
	}
}

func TestMapLocationToOutputSemantic_Vertex(t *testing.T) {
	m := MapLocationToOutputSemantic(0, false)
	if m.SemanticName != "LOC" {
		t.Errorf("name: got %q, want %q", m.SemanticName, "LOC")
	}
	if m.SystemValue != SVArbitrary {
		t.Errorf("system value: got %d, want SVArbitrary", m.SystemValue)
	}
}

func TestMapBindingToSemantic(t *testing.T) {
	// Builtin input.
	m, ok := MapBindingToSemantic(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, false, false)
	if !ok || m.SemanticName != "SV_Position" {
		t.Errorf("builtin input: got %q/%v, want SV_Position/true", m.SemanticName, ok)
	}

	// Location input.
	m2, ok := MapBindingToSemantic(ir.LocationBinding{Location: 3}, false, false)
	if !ok || m2.SemanticName != "LOC" || m2.SemanticIndex != 3 {
		t.Errorf("location input: got %q/%d/%v, want LOC/3/true", m2.SemanticName, m2.SemanticIndex, ok)
	}

	// Location output for fragment.
	m3, ok := MapBindingToSemantic(ir.LocationBinding{Location: 1}, true, true)
	if !ok || m3.SemanticName != "SV_Target" || m3.SemanticIndex != 1 {
		t.Errorf("fragment output: got %q/%d/%v, want SV_Target/1/true", m3.SemanticName, m3.SemanticIndex, ok)
	}

	// Location output for vertex.
	m4, ok := MapBindingToSemantic(ir.LocationBinding{Location: 0}, true, false)
	if !ok || m4.SemanticName != "LOC" || m4.SemanticIndex != 0 {
		t.Errorf("vertex output: got %q/%d/%v, want LOC/0/true", m4.SemanticName, m4.SemanticIndex, ok)
	}

	// A built-in without a signature semantic is reported, not invented.
	if _, ok := MapBindingToSemantic(ir.BuiltinBinding{Builtin: ir.BuiltinBaseVertex}, false, false); ok {
		t.Error("builtin input BaseVertex: got mapped, want not mapped")
	}
}
