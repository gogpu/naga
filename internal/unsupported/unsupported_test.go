// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package unsupported

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
)

func TestErrorf(t *testing.T) {
	inner := errors.New("inner")
	err := Errorf("msl: %s is not supported: %w", "x", inner)
	if got, want := err.Error(), "msl: x is not supported: inner"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Error("errors.Is(err, backend.ErrUnsupported) = false")
	}
	if !errors.Is(err, inner) {
		t.Error("the %w operand is not wrapped")
	}
	if errors.Is(err, backend.ErrMissingBinding) {
		t.Error("matches ErrMissingBinding")
	}
}
