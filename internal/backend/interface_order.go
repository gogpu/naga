package backend

import (
	"sort"

	"github.com/gogpu/naga/ir"
)

// EntryInterfaceOrder ranks an entry-point struct member's binding for
// graphics-pipeline output-signature register assignment.
//
// Convention (matches DXC HLSL frontend and naga's own HLSL backend wrapper):
//   - Location bindings come FIRST, sorted ascending by location index
//   - Builtin bindings come LAST, ordered by builtin enum value
//   - Anything else (no binding, etc.) is sorted to the end
//
// The DXIL backend must apply this order in EVERY place that encodes the
// output signature; otherwise the ordering drifts and the validator rejects
// the result. Known consumers:
//   - dxil/dxil.go buildSignatures (OSG1 / output signature element layout)
//   - dxil/internal/emit/statements.go emitStructReturn (storeOutput sigID)
//   - dxil/internal/emit/statements.go emitStructReturnFromLoad (likewise)
//   - dxil/internal/emit/emitter.go !dx.entryPoints metadata (output sig nodes)
//
// Without lockstep ordering across all consumers, IDxcValidator emits
// "Not all elements of output SV_Position were written" or similar — each
// individual store is well-typed but the OSG1 register slot the storeOutput
// targets does not match what the entry point metadata declares.
//
// Mirrors hlsl/functions.go:interfaceKey/Less which already sorts the HLSL
// `_vs_main`-style wrapper struct the same way before emission.

// MemberInterfaceKey is the per-member sort key used to order struct members
// for graphics output emission.
type MemberInterfaceKey struct {
	Kind     MemberInterfaceKind
	Location uint32
	Builtin  ir.BuiltinValue
}

// MemberInterfaceKind enumerates the binding categories in sort priority order.
type MemberInterfaceKind int

const (
	// MemberLocation is a @location(N) binding — sorted first by Location.
	MemberLocation MemberInterfaceKind = iota
	// MemberBuiltin is a @builtin(...) binding — sorted second by Builtin enum.
	MemberBuiltin
	// MemberOther is the catch-all (nil binding, unrecognized) — sorted last.
	MemberOther
)

// NewMemberInterfaceKey builds a sort key from a struct member binding.
func NewMemberInterfaceKey(binding *ir.Binding) MemberInterfaceKey {
	if binding == nil {
		return MemberInterfaceKey{Kind: MemberOther}
	}
	switch b := (*binding).(type) {
	case ir.LocationBinding:
		return MemberInterfaceKey{Kind: MemberLocation, Location: b.Location}
	case ir.BuiltinBinding:
		return MemberInterfaceKey{Kind: MemberBuiltin, Builtin: b.Builtin}
	default:
		return MemberInterfaceKey{Kind: MemberOther}
	}
}

// MemberInterfaceLess reports whether key a should come before key b in the
// graphics output ordering. Defined here so callers don't have to inline the
// three-way comparison.
func MemberInterfaceLess(a, b MemberInterfaceKey) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	switch a.Kind {
	case MemberLocation:
		return a.Location < b.Location
	case MemberBuiltin:
		return a.Builtin < b.Builtin
	default:
		return false
	}
}

// SortedArgIndices returns the indices into the args slice in sorted emission
// order (locations first, builtins last), without mutating the original slice.
// Used by the DXIL backend to iterate entry-point arguments in the same order
// as the ISG1 signature, so loadInput sigID values match the register layout.
func SortedArgIndices(args []ir.FunctionArgument) []int {
	if len(args) == 0 {
		return nil
	}
	type keyed struct {
		idx int
		key MemberInterfaceKey
	}
	tmp := make([]keyed, len(args))
	for i := range args {
		tmp[i] = keyed{idx: i, key: NewMemberInterfaceKey(args[i].Binding)}
	}
	sort.SliceStable(tmp, func(i, j int) bool {
		return MemberInterfaceLess(tmp[i].key, tmp[j].key)
	})
	out := make([]int, len(args))
	for i, k := range tmp {
		out[i] = k.idx
	}
	return out
}

// SortedMemberIndices returns the indices into the members slice in sorted
// emission order, without mutating the original slice. The DXIL backend uses
// these indices to walk both the signature builder and the storeOutput
// emitter in lockstep.
//
// Returned slice has length len(members). If members is empty, returns nil.
func SortedMemberIndices(members []ir.StructMember) []int {
	if len(members) == 0 {
		return nil
	}
	type keyed struct {
		idx int
		key MemberInterfaceKey
	}
	tmp := make([]keyed, len(members))
	for i := range members {
		tmp[i] = keyed{idx: i, key: NewMemberInterfaceKey(members[i].Binding)}
	}
	sort.SliceStable(tmp, func(i, j int) bool {
		return MemberInterfaceLess(tmp[i].key, tmp[j].key)
	})
	out := make([]int, len(members))
	for i, k := range tmp {
		out[i] = k.idx
	}
	return out
}
