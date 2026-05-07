package backend

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// NewMemberInterfaceKey — classification of binding types
// ---------------------------------------------------------------------------

func TestNewMemberInterfaceKey(t *testing.T) {
	tests := []struct {
		name    string
		binding *ir.Binding
		want    MemberInterfaceKey
	}{
		{
			name:    "nil binding -> MemberOther",
			binding: nil,
			want:    MemberInterfaceKey{Kind: MemberOther},
		},
		{
			name:    "location(3) -> MemberLocation with index",
			binding: bindingPtr(ir.LocationBinding{Location: 3}),
			want:    MemberInterfaceKey{Kind: MemberLocation, Location: 3},
		},
		{
			name:    "location(0) -> MemberLocation zero index",
			binding: bindingPtr(ir.LocationBinding{Location: 0}),
			want:    MemberInterfaceKey{Kind: MemberLocation, Location: 0},
		},
		{
			name:    "builtin(position) -> MemberBuiltin",
			binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}),
			want:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
		},
		{
			name:    "builtin(frag_depth) -> MemberBuiltin",
			binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinFragDepth}),
			want:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinFragDepth},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMemberInterfaceKey(tt.binding)
			if got != tt.want {
				t.Errorf("NewMemberInterfaceKey() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MemberInterfaceLess — the sort contract: Location < Builtin < Other
// ---------------------------------------------------------------------------

func TestMemberInterfaceLess(t *testing.T) {
	tests := []struct {
		name string
		a, b MemberInterfaceKey
		want bool
	}{
		// Cross-kind ordering: Location < Builtin < Other.
		{"location < builtin", key(MemberLocation, 99, 0), key(MemberBuiltin, 0, 0), true},
		{"builtin > location", key(MemberBuiltin, 0, 0), key(MemberLocation, 0, 0), false},
		{"location < other", key(MemberLocation, 0, 0), key(MemberOther, 0, 0), true},
		{"builtin < other", key(MemberBuiltin, 0, 0), key(MemberOther, 0, 0), true},
		{"other > location", key(MemberOther, 0, 0), key(MemberLocation, 0, 0), false},

		// Within-kind: locations sorted by ascending index.
		{"loc(1) < loc(3)", key(MemberLocation, 1, 0), key(MemberLocation, 3, 0), true},
		{"loc(5) > loc(1)", key(MemberLocation, 5, 0), key(MemberLocation, 1, 0), false},
		{"loc(2) == loc(2) -> false", key(MemberLocation, 2, 0), key(MemberLocation, 2, 0), false},

		// Within-kind: builtins sorted by enum value (Position=0 < FragDepth=4).
		{"position < frag_depth", key(MemberBuiltin, 0, ir.BuiltinPosition), key(MemberBuiltin, 0, ir.BuiltinFragDepth), true},
		{"frag_depth > position", key(MemberBuiltin, 0, ir.BuiltinFragDepth), key(MemberBuiltin, 0, ir.BuiltinPosition), false},
		{"same builtin -> false", key(MemberBuiltin, 0, ir.BuiltinPosition), key(MemberBuiltin, 0, ir.BuiltinPosition), false},

		// Other vs Other: always false (stable sort preserves input order).
		{"other == other -> false", key(MemberOther, 0, 0), key(MemberOther, 0, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MemberInterfaceLess(tt.a, tt.b); got != tt.want {
				t.Errorf("MemberInterfaceLess(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SortedArgIndices — DXIL ISG1 register assignment order for function args
// ---------------------------------------------------------------------------

func TestSortedArgIndices_Empty(t *testing.T) {
	if got := SortedArgIndices(nil); got != nil {
		t.Fatalf("SortedArgIndices(nil) = %v, want nil", got)
	}
	if got := SortedArgIndices([]ir.FunctionArgument{}); got != nil {
		t.Fatalf("SortedArgIndices(empty) = %v, want nil", got)
	}
}

// TestSortedArgIndices_FragmentShaderInputs simulates a fragment shader
// with two struct-flattened inputs: VertexOutput (position builtin) and
// NoteInstance (location). DXC orders locations before builtins in
// fragment input signatures. This is the msl-varyings test case that
// first surfaced the cross-argument ordering bug.
func TestSortedArgIndices_FragmentShaderInputs(t *testing.T) {
	args := []ir.FunctionArgument{
		{Name: "pos", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "loc1", Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "loc0", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedArgIndices(args)
	// DXC order: loc(0) first, loc(1) second, builtin(position) last.
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedArgIndices fragment inputs = %v, want %v", got, want)
	}
}

func TestSortedArgIndices_NilBindingsLast(t *testing.T) {
	args := []ir.FunctionArgument{
		{Name: "noBinding", Binding: nil},
		{Name: "loc0", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedArgIndices(args)
	want := []int{1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedArgIndices nil binding = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// SortedMemberIndices — DXIL OSG1 output signature element ordering
// ---------------------------------------------------------------------------

func TestSortedMemberIndices_Empty(t *testing.T) {
	if got := SortedMemberIndices(nil); got != nil {
		t.Fatalf("SortedMemberIndices(nil) = %v, want nil", got)
	}
}

// TestSortedMemberIndices_VertexOutputStruct simulates a typical vertex
// shader output struct:
//
//	struct VertexOutput {
//	    @builtin(position) pos: vec4<f32>,   // idx 0
//	    @location(1) uv: vec2<f32>,          // idx 1
//	    @location(0) color: vec4<f32>,        // idx 2
//	}
//
// DXC signature order: loc(0), loc(1), builtin(position).
// Getting this wrong causes IDxcValidator: "Not all elements of output
// SV_Position were written".
func TestSortedMemberIndices_VertexOutputStruct(t *testing.T) {
	members := []ir.StructMember{
		{Name: "pos", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "uv", Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "color", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedMemberIndices(members)
	// color(idx 2) at loc(0), uv(idx 1) at loc(1), pos(idx 0) builtin last.
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices = %v, want %v", got, want)
	}
}

// TestSortedMemberIndices_MultipleBuiltinsSortByEnum verifies that when
// a struct has multiple builtins (e.g., @builtin(vertex_index) and
// @builtin(instance_index)), they sort by the BuiltinValue enum.
func TestSortedMemberIndices_MultipleBuiltinsSortByEnum(t *testing.T) {
	members := []ir.StructMember{
		{Name: "inst", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinInstanceIndex})}, // enum=2
		{Name: "vert", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})},   // enum=1
		{Name: "loc", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedMemberIndices(members)
	// loc first, then vert_idx (enum 1), then inst_idx (enum 2).
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices builtins by enum = %v, want %v", got, want)
	}
}

// TestSortedMemberIndices_NilBindingsLast verifies that members without
// bindings sort after all locations and builtins.
func TestSortedMemberIndices_NilBindingsLast(t *testing.T) {
	members := []ir.StructMember{
		{Name: "noBinding"},
		{Name: "loc", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
		{Name: "builtin", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})},
	}
	got := SortedMemberIndices(members)
	want := []int{1, 2, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices nil bindings = %v, want %v", got, want)
	}
}

// TestSortedMemberIndices_StableForEqualKeys verifies SliceStable
// preserves declaration order for members with identical sort keys.
func TestSortedMemberIndices_StableForEqualKeys(t *testing.T) {
	members := []ir.StructMember{
		{Name: "first"},
		{Name: "second"},
		{Name: "third"},
	}
	got := SortedMemberIndices(members)
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stable order = %v, want %v", got, want)
	}
}

// SortFlatBindings tests are in sig_pack_test.go (pre-existing).
// They cover: LocationsBeforeBuiltins, VSInputPreservesOrder,
// MultipleLocations — the core DXC fragment/vertex ordering contract.

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// bindingPtr returns a pointer to a Binding interface value.
func bindingPtr(b ir.Binding) *ir.Binding {
	return &b
}

// key builds a MemberInterfaceKey for test readability.
func key(kind MemberInterfaceKind, loc uint32, builtin ir.BuiltinValue) MemberInterfaceKey {
	return MemberInterfaceKey{Kind: kind, Location: loc, Builtin: builtin}
}
