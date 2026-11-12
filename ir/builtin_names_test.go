package ir

import (
	"strings"
	"testing"
)

// TestBuiltinValueNames pins the name table to the enum: one name per member,
// no blanks, no duplicates, and a number for a value outside the enum. An
// enum that grows without a name fails here.
func TestBuiltinValueNames(t *testing.T) {
	if got, want := len(builtinValueNames), int(BuiltinWorkGroupSize)+1; got != want {
		t.Fatalf("builtinValueNames has %d names, the enum has %d members", got, want)
	}
	seen := make(map[string]BuiltinValue, len(builtinValueNames))
	for b := BuiltinPosition; b <= BuiltinWorkGroupSize; b++ {
		name := b.String()
		if strings.TrimSpace(name) == "" {
			t.Errorf("built-in %d has an empty name", b)
			continue
		}
		if strings.HasPrefix(name, "BuiltinValue(") {
			t.Errorf("built-in %d has no table entry: String() = %q", b, name)
			continue
		}
		if prev, ok := seen[name]; ok {
			t.Errorf("built-ins %d and %d share the name %q", prev, b, name)
		}
		seen[name] = b
	}
	for _, b := range []BuiltinValue{BuiltinWorkGroupSize + 1, 255} {
		if got := b.String(); !strings.HasPrefix(got, "BuiltinValue(") {
			t.Errorf("String() for out-of-range value %d = %q, want a number", b, got)
		}
	}
	for b, want := range map[BuiltinValue]string{
		BuiltinPosition:      "position",
		BuiltinVertexIndex:   "vertex_index",
		BuiltinClipDistances: "clip_distances",
		BuiltinBaseInstance:  "base_instance",
		BuiltinWorkGroupSize: "workgroup_size",
	} {
		if got := b.String(); got != want {
			t.Errorf("String() = %q, want the WGSL or stable spelling %q", got, want)
		}
	}
}
