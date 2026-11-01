// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

// IndexableLength is the number of elements of an indexable type, for
// bounds checks. Mirrors Rust naga's proc::IndexableLength.
type IndexableLength struct {
	// Known is the number of elements when it is fixed by the type.
	Known uint32
	// Dynamic reports a length known only at run time: a runtime-sized
	// array or an unbounded binding array.
	Dynamic bool
}
