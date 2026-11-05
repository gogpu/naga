// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package unsupported

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

func TestCapabilities(t *testing.T) {
	f64 := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}
	module := &ir.Module{Types: []ir.Type{f64}}

	if err := Capabilities("x", module, valid.CapFloat64); err != nil {
		t.Errorf("a supported capability is refused: %v", err)
	}
	if err := Capabilities("x", nil, 0); err != nil {
		t.Errorf("a nil module is refused: %v", err)
	}
	err := Capabilities("x", module, 0)
	if err == nil {
		t.Fatal("a module that needs FLOAT64 is accepted by a backend without it")
	}
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("error %q does not wrap backend.ErrUnsupported", err)
	}
	const want = "x: the module needs Capabilities(FLOAT64), which the x backend cannot write"
	if err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}
