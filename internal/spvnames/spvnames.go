// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package spvnames names SPIR-V capabilities by their number in the SPIR-V
// specification, and numbers them by name.
//
// The tools that print or read capability names — cmd/spvdis and the snapshot
// test diagnostics — share this one table. Two copies of it, as there were
// before, drift apart from the specification independently: one of them goes
// wrong and nothing compares the other with it.
//
// The table covers every capability of the SPIR-V headers; gen.go generates it
// from the SPIR-V grammar and checks it against spirv.h. Aliases are the
// further names the specification gives a capability, so a name either form
// resolves.
package spvnames

// capability is one entry of the SPIR-V capability table: the capability's
// number, its canonical name in the specification, and the alias names the
// specification gives it.
type capability struct {
	value   uint32
	name    string
	aliases []string
}

// CapabilityName returns the SPIR-V name of capability v and whether the
// specification defines that capability.
func CapabilityName(v uint32) (string, bool) {
	name, ok := capabilityNames[v]
	return name, ok
}

// CapabilityByName returns the number of the SPIR-V capability called name and
// whether the specification defines it. An alias of a capability resolves to
// the same number as its canonical name.
func CapabilityByName(name string) (uint32, bool) {
	v, ok := capabilityNumbers[name]
	return v, ok
}

// capabilityNames maps a capability number to its canonical name.
var capabilityNames = indexNames(capabilityTable)

// capabilityNumbers maps every name of a capability, canonical and alias, to
// its number.
var capabilityNumbers = indexNumbers(capabilityTable)

// indexNames builds the number-to-name index of a capability table.
func indexNames(table []capability) map[uint32]string {
	names := make(map[uint32]string, len(table))
	for _, c := range table {
		names[c.value] = c.name
	}
	return names
}

// indexNumbers builds the name-to-number index of a capability table, aliases
// included.
func indexNumbers(table []capability) map[string]uint32 {
	numbers := make(map[string]uint32, len(table))
	for _, c := range table {
		numbers[c.name] = c.value
		for _, alias := range c.aliases {
			numbers[alias] = c.value
		}
	}
	return numbers
}
