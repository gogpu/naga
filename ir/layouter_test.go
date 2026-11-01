package ir

import "testing"

// layoutOf lays out m with a new Layouter and returns the layout of h.
func layoutOf(t *testing.T, m *Module, h TypeHandle) TypeLayout {
	t.Helper()
	var l Layouter
	if err := l.Update(m); err != nil {
		t.Fatalf("Layouter.Update: %v", err)
	}
	return l.Layout(h)
}

// TestLayouterUpdateIsIncremental: a later Update lays out only the types
// added since, as Rust's Layouter::update; the zero Layouter has no layouts.
func TestLayouterUpdateIsIncremental(t *testing.T) {
	f32 := ScalarType{Kind: ScalarFloat, Width: 4}
	m := &Module{Types: []Type{{Inner: f32}}}

	var l Layouter
	if got := l.Layout(0); got != (TypeLayout{Size: 0, Alignment: 1}) {
		t.Errorf("zero Layouter: Layout(0) = %+v, want {0 1}", got)
	}
	if err := l.Update(m); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := l.Layout(0); got != (TypeLayout{Size: 4, Alignment: 4}) {
		t.Errorf("Layout(f32) = %+v, want {4 4}", got)
	}

	n := uint32(3)
	m.Types = append(m.Types,
		Type{Inner: VectorType{Size: Vec3, Scalar: f32}},
		Type{Inner: ArrayType{Base: 1, Size: ArraySize{Constant: &n}, Stride: 16}})
	if err := l.Update(m); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	want := []TypeLayout{{Size: 4, Alignment: 4}, {Size: 12, Alignment: 16}, {Size: 48, Alignment: 16}}
	for h, w := range want {
		if got := l.Layout(TypeHandle(h)); got != w {
			t.Errorf("Layout(%d) = %+v, want %+v", h, got, w)
		}
	}
}

// TestLayouterForwardReference: a type may refer to a later one, which our
// type arenas do not forbid (Rust requires the element type first).
func TestLayouterForwardReference(t *testing.T) {
	m := &Module{Types: []Type{
		{Inner: StructType{Span: 16, Members: []StructMember{{Type: 1}}}},
		{Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
	}}
	if got := layoutOf(t, m, 0); got != (TypeLayout{Size: 16, Alignment: 16}) {
		t.Errorf("Layout(struct) = %+v, want {16 16}", got)
	}
}

// TestLayouterErrors: a reference outside the arena or back to the type
// itself fails, and a failed Update leaves the Layouter as it was. Type 0 is
// laid out by a first Update; the type under test is type 1.
func TestLayouterErrors(t *testing.T) {
	f32 := Type{Inner: ScalarType{Kind: ScalarFloat, Width: 4}}
	tests := []struct {
		name string
		ty   Type
		want string
	}{
		{"array element", Type{Inner: ArrayType{Base: 8, Stride: 4}}, "layout of type 1: array element type 8 does not exist"},
		{"binding array element", Type{Inner: BindingArrayType{Base: 8}}, "layout of type 1: binding array element type 8 does not exist"},
		{"struct member", Type{Inner: StructType{Members: []StructMember{{Type: 0}, {Type: 9}}}}, "layout of type 1: struct member[1] type 9 does not exist"},
		{"self reference", Type{Inner: StructType{Members: []StructMember{{Type: 1}}}}, "layout of type 1: the type contains itself"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l Layouter
			if err := l.Update(&Module{Types: []Type{f32}}); err != nil {
				t.Fatalf("Update: %v", err)
			}
			err := l.Update(&Module{Types: []Type{f32, tt.ty}})
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Update error = %v, want %q", err, tt.want)
			}
			if got := l.Layout(1); got != (TypeLayout{Size: 0, Alignment: 1}) {
				t.Errorf("after the failed Update, Layout(1) = %+v, want {0 1}", got)
			}
			if got := l.Layout(0); got != (TypeLayout{Size: 4, Alignment: 4}) {
				t.Errorf("after the failed Update, Layout(0) = %+v, want {4 4}", got)
			}
		})
	}
}
