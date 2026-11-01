package irutil

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// CheckWorkgroupSizesResolved reports, wrapping ir.ErrUnresolvedOverride, an
// entry point whose workgroup size is still given by an override: a backend
// would otherwise write the placeholder size 1.
func CheckWorkgroupSizesResolved(m *ir.Module) error {
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		if ep.WorkgroupSizeOverrides != [3]*ir.ExpressionHandle{} {
			return fmt.Errorf("entry point %q: workgroup size: %w", ep.Name, ir.ErrUnresolvedOverride)
		}
	}
	return nil
}

// ResolvePendingArraySizes returns m with every pending array size replaced
// by its count, as each Rust backend does through ArraySize::resolve. Type
// handles are unchanged; m is not modified (the result shares everything but
// its types). A module without pending sizes is returned as it is.
func ResolvePendingArraySizes(m *ir.Module) (*ir.Module, error) {
	pending := false
	for i := range m.Types {
		if at, ok := m.Types[i].Inner.(ir.ArrayType); ok && at.Size.Pending != nil {
			pending = true
			break
		}
	}
	if !pending {
		return m, nil
	}
	types := make([]ir.Type, len(m.Types))
	copy(types, m.Types)
	for i := range types {
		at, ok := types[i].Inner.(ir.ArrayType)
		if !ok || at.Size.Pending == nil {
			continue
		}
		l, err := at.Size.Resolve(m)
		if err != nil {
			return nil, fmt.Errorf("array type %d: %w", i, err)
		}
		n := l.Known
		at.Size = ir.ArraySize{Constant: &n}
		types[i].Inner = at
	}
	resolved := *m
	resolved.Types = types
	return &resolved, nil
}
