// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package unsupported

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// Capabilities refuses a module that needs an optional capability outside
// supported, the SupportedCapabilities of the backend named by name. The
// error wraps backend.ErrUnsupported and names the capabilities as Rust
// prints them ("Capabilities(SHADER_FLOAT32_ATOMIC)"), the text wgpu matches
// a missing-capability error on. It returns nil when the module needs
// nothing the backend lacks.
func Capabilities(name string, module *ir.Module, supported valid.Capabilities) error {
	missing := valid.MissingCapabilities(module, supported)
	if missing == 0 {
		return nil
	}
	return Errorf("%s: the module needs %s, which the %s backend cannot write", name, missing, name)
}
