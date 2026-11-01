// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package naga

import (
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// KeepUnused selects whether Compact keeps the functions, global variables
// and named overrides nothing uses, as Rust naga's compact::KeepUnused.
type KeepUnused bool

const (
	// KeepUnusedNo removes every function and global variable no entry point
	// reaches and every override nothing uses (Rust KeepUnused::No), as
	// pipeline-constant processing needs: an override no entry point uses
	// needs no value.
	KeepUnusedNo KeepUnused = false
	// KeepUnusedYes keeps every function, global variable and named override
	// (Rust KeepUnused::Yes), as after lowering.
	KeepUnusedYes KeepUnused = true
)

// Compact removes the objects module does not use and renumbers the handles
// of what is left, as Rust naga's compact::compact does. module must be
// valid.
//
// With either keep value it removes the function expressions nothing refers
// to, the constants, overrides and global expressions nothing uses (named
// constants of a concrete type always stay), the abstract types and the
// anonymous types nothing refers to any more. With KeepUnusedNo it first
// removes the global variables and functions no entry point reaches, and the
// named overrides nothing uses. Entry points, special types and named types
// always stay; Rust also removes an unused named type under KeepUnused::No.
//
// Compact modifies module in place: nothing else may read or write module
// while it runs, and handles taken from module before the call are not valid
// after it. Calls on different modules may run concurrently.
func Compact(module *ir.Module, keep KeepUnused) {
	irutil.Compact(module, irutil.KeepUnused(keep))
}
