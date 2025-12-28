// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"fmt"
	"strings"
)

// namer generates unique identifiers for HLSL output.
// It tracks used names to ensure uniqueness and handles
// HLSL's case-insensitive keyword matching.
type namer struct {
	// usedNames tracks names that have been generated (stored in lowercase for case-insensitive comparison).
	usedNames map[string]struct{}

	// counter is used to generate unique suffixes.
	counter uint32
}

// newNamer creates a new namer instance.
func newNamer() *namer {
	n := &namer{
		usedNames: make(map[string]struct{}),
	}

	// Pre-register all naga helper function names to avoid conflicts
	helperNames := []string{
		NagaModfFunction,
		NagaFrexpFunction,
		NagaExtractBitsFunction,
		NagaInsertBitsFunction,
		SamplerHeapVar,
		ComparisonSamplerHeapVar,
		SampleExternalTextureFunction,
		NagaAbsFunction,
		NagaDivFunction,
		NagaModFunction,
		NagaNegFunction,
		NagaF2I32Function,
		NagaF2U32Function,
		NagaF2I64Function,
		NagaF2U64Function,
		ImageLoadExternalFunction,
		ImageSampleBaseClampToEdgeFunc,
		DynamicBufferOffsetsPrefix,
		ImageStorageLoadScalarWrapper,
	}

	for _, name := range helperNames {
		n.usedNames[strings.ToLower(name)] = struct{}{}
	}

	return n
}

// call generates a unique name based on the given base.
// It escapes reserved keywords and adds numeric suffixes if needed.
func (n *namer) call(base string) string {
	// Handle empty base name
	if base == "" {
		base = UnnamedIdentifier
	}

	// Escape reserved words first
	escaped := Escape(base)

	// Check if this name is available (case-insensitive for HLSL)
	lowerEscaped := strings.ToLower(escaped)
	if !n.isUsedLower(lowerEscaped) {
		n.usedNames[lowerEscaped] = struct{}{}
		return escaped
	}

	// Add numeric suffix to make unique
	for {
		n.counter++
		candidate := fmt.Sprintf("%s_%d", escaped, n.counter)
		lowerCandidate := strings.ToLower(candidate)
		if !n.isUsedLower(lowerCandidate) {
			n.usedNames[lowerCandidate] = struct{}{}
			return candidate
		}
	}
}

// isUsed checks if a name has already been used (case-insensitive).
func (n *namer) isUsed(name string) bool {
	return n.isUsedLower(strings.ToLower(name))
}

// isUsedLower checks if a lowercase name has already been used.
func (n *namer) isUsedLower(lowerName string) bool {
	_, used := n.usedNames[lowerName]
	return used
}

// reserve marks a name as used without returning it.
// This is useful for reserving names that are used externally.
func (n *namer) reserve(name string) {
	n.usedNames[strings.ToLower(name)] = struct{}{}
}

// reset clears all tracked names and resets the counter.
// Primarily useful for testing.
func (n *namer) reset() {
	n.usedNames = make(map[string]struct{})
	n.counter = 0
}

// count returns the number of unique names tracked.
func (n *namer) count() int {
	return len(n.usedNames)
}

// callWithPrefix generates a unique name with a specific prefix.
// The prefix is not escaped but the resulting name is checked for uniqueness.
func (n *namer) callWithPrefix(prefix, base string) string {
	combined := prefix + base

	// Check if escaped name is available
	escaped := Escape(combined)
	lowerEscaped := strings.ToLower(escaped)

	if !n.isUsedLower(lowerEscaped) {
		n.usedNames[lowerEscaped] = struct{}{}
		return escaped
	}

	// Add numeric suffix
	for {
		n.counter++
		candidate := fmt.Sprintf("%s_%d", escaped, n.counter)
		lowerCandidate := strings.ToLower(candidate)
		if !n.isUsedLower(lowerCandidate) {
			n.usedNames[lowerCandidate] = struct{}{}
			return candidate
		}
	}
}
