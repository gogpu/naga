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
