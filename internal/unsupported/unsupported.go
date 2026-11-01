// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package unsupported builds the backends' refusals: errors that wrap
// backend.ErrUnsupported while keeping their message as written.
package unsupported

import (
	"fmt"

	"github.com/gogpu/naga/backend"
)

// Errorf formats an error as fmt.Errorf does (%w included) and makes it
// match backend.ErrUnsupported with errors.Is; the text is unchanged.
func Errorf(format string, a ...any) error {
	return &refusal{err: fmt.Errorf(format, a...)}
}

type refusal struct{ err error }

func (r *refusal) Error() string { return r.err.Error() }

func (r *refusal) Unwrap() []error { return []error{r.err, backend.ErrUnsupported} }
