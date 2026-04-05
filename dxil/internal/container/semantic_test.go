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
		{"ClipDistance", ir.BuiltinClipDistance, "SV_ClipDistance", SVClipDistance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MapBuiltinToSemantic(tt.builtin)
			if m.SemanticName != tt.wantName {
				t.Errorf("name: got %q, want %q", m.SemanticName, tt.wantName)
			}
			if m.SystemValue != tt.wantSV {
				t.Errorf("system value: got %d, want %d", m.SystemValue, tt.wantSV)
			}
		})
	}
}

func TestMapLocationToInputSemantic(t *testing.T) {
	tests := []struct {
		loc       uint32
		wantName  string
		wantIndex uint32
	}{
		{0, "TEXCOORD", 0},
		{1, "TEXCOORD", 1},
		{5, "TEXCOORD", 5},
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
	if m.SemanticName != "TEXCOORD" {
		t.Errorf("name: got %q, want %q", m.SemanticName, "TEXCOORD")
	}
	if m.SystemValue != SVArbitrary {
		t.Errorf("system value: got %d, want SVArbitrary", m.SystemValue)
	}
}

func TestMapBindingToSemantic(t *testing.T) {
	// Builtin input.
	m := MapBindingToSemantic(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}, false, false)
	if m.SemanticName != "SV_Position" {
		t.Errorf("builtin input: got %q, want %q", m.SemanticName, "SV_Position")
	}

	// Location input.
	m2 := MapBindingToSemantic(ir.LocationBinding{Location: 3}, false, false)
	if m2.SemanticName != "TEXCOORD" || m2.SemanticIndex != 3 {
		t.Errorf("location input: got %q/%d, want TEXCOORD/3", m2.SemanticName, m2.SemanticIndex)
	}

	// Location output for fragment.
	m3 := MapBindingToSemantic(ir.LocationBinding{Location: 1}, true, true)
	if m3.SemanticName != "SV_Target" || m3.SemanticIndex != 1 {
		t.Errorf("fragment output: got %q/%d, want SV_Target/1", m3.SemanticName, m3.SemanticIndex)
	}

	// Location output for vertex.
	m4 := MapBindingToSemantic(ir.LocationBinding{Location: 0}, true, false)
	if m4.SemanticName != "TEXCOORD" || m4.SemanticIndex != 0 {
		t.Errorf("vertex output: got %q/%d, want TEXCOORD/0", m4.SemanticName, m4.SemanticIndex)
	}
}
