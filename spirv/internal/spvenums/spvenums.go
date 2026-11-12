// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package spvenums holds the SPIR-V enumerations the SPIR-V backend writes and
// the input frontend reads — opcodes, capabilities, decorations, built-ins,
// execution models and modes, storage classes, the memory and image kinds and
// the GLSL.std.450 extended instructions. The declarations are generated from
// the vendored SPIR-V grammar, so they cannot drift from the specification or
// from each other, and the drift test regenerates them and compares the bytes.
//
// The package is internal: it is a shared implementation detail of
// spirv/internal/codegen and spirv/internal/front, not public API.
//
//go:generate go run gen.go -core ../../../testdata/upstream/github.com/KhronosGroup/SPIRV-Headers/spirv.core.grammar.json -extinst ../../../testdata/upstream/github.com/KhronosGroup/SPIRV-Headers/extinst.glsl.std.450.grammar.json -out enums.go
package spvenums
