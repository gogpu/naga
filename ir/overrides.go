package ir

import (
	"errors"
	"fmt"
)

// ErrUnresolvedOverride is the error of a backend given a module that still
// uses pipeline-overridable constants (Rust naga's backend Error::Override).
// Run naga.ProcessOverrides first.
var ErrUnresolvedOverride = errors.New("overrides should not be present at this stage")

// CheckWorkgroupSizesResolved reports, wrapping ErrUnresolvedOverride, an
// entry point whose workgroup size is still given by an override: a backend
// would otherwise write the placeholder size 1.
func CheckWorkgroupSizesResolved(m *Module) error {
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		if ep.WorkgroupSizeOverrides != [3]*ExpressionHandle{} {
			return fmt.Errorf("entry point %q: workgroup size: %w", ep.Name, ErrUnresolvedOverride)
		}
	}
	return nil
}

// IsRuntimeSized reports an array whose element count is known only at run
// time (Rust ArraySize::Dynamic); a pending count is not.
func (s ArraySize) IsRuntimeSized() bool { return s.Constant == nil && s.Pending == nil }

// Resolve returns the element count of s (Rust ArraySize::resolve): for a
// pending count, the value of the override's initializer, which must be a
// fully evaluated positive integer, as naga.ProcessOverrides leaves it.
func (s ArraySize) Resolve(m *Module) (IndexableLength, error) {
	switch {
	case s.Constant != nil:
		return IndexableLength{Known: *s.Constant}, nil
	case s.Pending == nil:
		return IndexableLength{Dynamic: true}, nil
	}
	o := *s.Pending
	if int(o) >= len(m.Overrides) || m.Overrides[o].Init == nil || int(*m.Overrides[o].Init) >= len(m.GlobalExpressions) {
		return IndexableLength{}, errArraySizeUnresolved
	}
	lit, ok := m.GlobalExpressions[*m.Overrides[o].Init].Kind.(Literal)
	if !ok {
		return IndexableLength{}, errArraySizeUnresolved
	}
	var n int64
	switch v := lit.Value.(type) {
	case LiteralU32:
		n = int64(v)
	case LiteralI32:
		n = int64(v)
	default:
		return IndexableLength{}, errArraySizeUnresolved
	}
	if n <= 0 {
		return IndexableLength{}, errArraySizeNotPositive
	}
	return IndexableLength{Known: uint32(n)}, nil
}

// Rust ResolveArraySizeError.
var (
	errArraySizeNotPositive = errors.New("array element count must be positive (> 0)")
	errArraySizeUnresolved  = fmt.Errorf("array size override has not been resolved: %w", ErrUnresolvedOverride)
)

// ResolvePendingArraySizes returns m with every pending array size replaced
// by its count, as each Rust backend does through ArraySize::resolve. Type
// handles are unchanged; m is not modified (the result shares everything but
// its types). A module without pending sizes is returned as it is.
func ResolvePendingArraySizes(m *Module) (*Module, error) {
	pending := false
	for i := range m.Types {
		if at, ok := m.Types[i].Inner.(ArrayType); ok && at.Size.Pending != nil {
			pending = true
			break
		}
	}
	if !pending {
		return m, nil
	}
	types := make([]Type, len(m.Types))
	copy(types, m.Types)
	for i := range types {
		at, ok := types[i].Inner.(ArrayType)
		if !ok || at.Size.Pending == nil {
			continue
		}
		l, err := at.Size.Resolve(m)
		if err != nil {
			return nil, fmt.Errorf("array type %d: %w", i, err)
		}
		n := l.Known
		at.Size = ArraySize{Constant: &n}
		types[i].Inner = at
	}
	resolved := *m
	resolved.Types = types
	return &resolved, nil
}
