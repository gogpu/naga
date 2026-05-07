package backend

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestNewMemberInterfaceKey(t *testing.T) {
	tests := []struct {
		name    string
		binding *ir.Binding
		want    MemberInterfaceKey
	}{
		{
			name:    "nil binding",
			binding: nil,
			want:    MemberInterfaceKey{Kind: MemberOther},
		},
		{
			name:    "location binding",
			binding: bindingPtr(ir.LocationBinding{Location: 3}),
			want:    MemberInterfaceKey{Kind: MemberLocation, Location: 3},
		},
		{
			name:    "location zero",
			binding: bindingPtr(ir.LocationBinding{Location: 0}),
			want:    MemberInterfaceKey{Kind: MemberLocation, Location: 0},
		},
		{
			name:    "builtin position",
			binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition}),
			want:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
		},
		{
			name:    "builtin frag depth",
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

func TestMemberInterfaceLess(t *testing.T) {
	tests := []struct {
		name string
		a, b MemberInterfaceKey
		want bool
	}{
		{
			name: "location before builtin",
			a:    MemberInterfaceKey{Kind: MemberLocation, Location: 5},
			b:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
			want: true,
		},
		{
			name: "builtin after location",
			a:    MemberInterfaceKey{Kind: MemberBuiltin},
			b:    MemberInterfaceKey{Kind: MemberLocation},
			want: false,
		},
		{
			name: "location before other",
			a:    MemberInterfaceKey{Kind: MemberLocation},
			b:    MemberInterfaceKey{Kind: MemberOther},
			want: true,
		},
		{
			name: "builtin before other",
			a:    MemberInterfaceKey{Kind: MemberBuiltin},
			b:    MemberInterfaceKey{Kind: MemberOther},
			want: true,
		},
		{
			name: "location sorted by index ascending",
			a:    MemberInterfaceKey{Kind: MemberLocation, Location: 1},
			b:    MemberInterfaceKey{Kind: MemberLocation, Location: 3},
			want: true,
		},
		{
			name: "location same index is not less",
			a:    MemberInterfaceKey{Kind: MemberLocation, Location: 2},
			b:    MemberInterfaceKey{Kind: MemberLocation, Location: 2},
			want: false,
		},
		{
			name: "location higher index is not less",
			a:    MemberInterfaceKey{Kind: MemberLocation, Location: 5},
			b:    MemberInterfaceKey{Kind: MemberLocation, Location: 1},
			want: false,
		},
		{
			name: "builtin sorted by enum value",
			a:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
			b:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinFragDepth},
			want: true,
		},
		{
			name: "builtin same enum is not less",
			a:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
			b:    MemberInterfaceKey{Kind: MemberBuiltin, Builtin: ir.BuiltinPosition},
			want: false,
		},
		{
			name: "other vs other is not less",
			a:    MemberInterfaceKey{Kind: MemberOther},
			b:    MemberInterfaceKey{Kind: MemberOther},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MemberInterfaceLess(tt.a, tt.b); got != tt.want {
				t.Errorf("MemberInterfaceLess(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSortedArgIndices_Empty(t *testing.T) {
	if got := SortedArgIndices(nil); got != nil {
		t.Fatalf("SortedArgIndices(nil) = %v, want nil", got)
	}
	if got := SortedArgIndices([]ir.FunctionArgument{}); got != nil {
		t.Fatalf("SortedArgIndices(empty) = %v, want nil", got)
	}
}

func TestSortedArgIndices_LocationsFirst(t *testing.T) {
	args := []ir.FunctionArgument{
		{Name: "pos", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "loc1", Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "loc0", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedArgIndices(args)
	// Expected: loc0 (idx 2), loc1 (idx 1), position (idx 0)
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedArgIndices = %v, want %v", got, want)
	}
}

func TestSortedArgIndices_NilBindingsLast(t *testing.T) {
	args := []ir.FunctionArgument{
		{Name: "noBinding", Binding: nil},
		{Name: "loc0", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedArgIndices(args)
	// Location first (idx 1), then nil binding (idx 0)
	want := []int{1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedArgIndices = %v, want %v", got, want)
	}
}

func TestSortedArgIndices_SingleArg(t *testing.T) {
	args := []ir.FunctionArgument{
		{Name: "only", Binding: bindingPtr(ir.LocationBinding{Location: 7})},
	}
	got := SortedArgIndices(args)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedArgIndices = %v, want %v", got, want)
	}
}

func TestSortedMemberIndices_Empty(t *testing.T) {
	if got := SortedMemberIndices(nil); got != nil {
		t.Fatalf("SortedMemberIndices(nil) = %v, want nil", got)
	}
	if got := SortedMemberIndices([]ir.StructMember{}); got != nil {
		t.Fatalf("SortedMemberIndices(empty) = %v, want nil", got)
	}
}

func TestSortedMemberIndices_LocationsThenBuiltins(t *testing.T) {
	members := []ir.StructMember{
		{Name: "pos", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})},
		{Name: "uv", Binding: bindingPtr(ir.LocationBinding{Location: 1})},
		{Name: "color", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
	}
	got := SortedMemberIndices(members)
	// Expected: color (idx 2, loc 0), uv (idx 1, loc 1), pos (idx 0, builtin)
	want := []int{2, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices = %v, want %v", got, want)
	}
}

func TestSortedMemberIndices_NilBindingsLast(t *testing.T) {
	members := []ir.StructMember{
		{Name: "noBinding"},
		{Name: "loc", Binding: bindingPtr(ir.LocationBinding{Location: 0})},
		{Name: "builtin", Binding: bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})},
	}
	got := SortedMemberIndices(members)
	// loc first, builtin second, nil last
	want := []int{1, 2, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices = %v, want %v", got, want)
	}
}

func TestSortedMemberIndices_StableOrder(t *testing.T) {
	// Two members with identical keys (both nil binding) should keep original order.
	members := []ir.StructMember{
		{Name: "first"},
		{Name: "second"},
	}
	got := SortedMemberIndices(members)
	want := []int{0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortedMemberIndices stable order = %v, want %v", got, want)
	}
}

func TestSortFlatBindings_Empty(t *testing.T) {
	// No panic on empty/nil.
	SortFlatBindings(nil, nil, false)
	SortFlatBindings([]ir.Binding{}, []ir.TypeHandle{}, false)
}

func TestSortFlatBindings_SingleElement(t *testing.T) {
	bindings := []ir.Binding{ir.LocationBinding{Location: 0}}
	types := []ir.TypeHandle{42}
	SortFlatBindings(bindings, types, false)

	if lb, ok := bindings[0].(ir.LocationBinding); !ok || lb.Location != 0 {
		t.Errorf("single element should be unchanged, got %v", bindings[0])
	}
	if types[0] != 42 {
		t.Errorf("single element type should be unchanged, got %d", types[0])
	}
}

// bindingPtr returns a pointer to a Binding interface value.
func bindingPtr(b ir.Binding) *ir.Binding {
	return &b
}
