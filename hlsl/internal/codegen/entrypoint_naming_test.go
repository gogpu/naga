// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestTwoSubgroupEntryPointsNameTheirOwnGroupIndexMember: each subgroup compute
// entry point gets a synthetic SV_GroupIndex member, and the namer numbers the
// second one. The member's name belongs to the entry point that declares it:
// one Writer-wide name left every earlier entry point reading a member its own
// interface struct does not have, which is invalid HLSL.
func TestTwoSubgroupEntryPointsNameTheirOwnGroupIndexMember(t *testing.T) {
	subgroupID := bindingPtr(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupID})
	newEntryPoint := func(name string) ir.EntryPoint {
		return ir.EntryPoint{
			Name:      name,
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Name: name,
				Arguments: []ir.FunctionArgument{
					{Binding: subgroupID, Type: 0},
				},
				Body: ir.Block{{Kind: ir.StmtReturn{}}},
			},
		}
	}
	module := &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		EntryPoints: []ir.EntryPoint{
			newEntryPoint("first"),
			newEntryPoint("second"),
		},
	}
	code, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// struct <type> { uint <member> : SV_GroupIndex; };
	structRE := regexp.MustCompile(`struct (\w+) \{\n\s+uint (\w+) : SV_GroupIndex;`)
	member := map[string]string{}
	for _, match := range structRE.FindAllStringSubmatch(code, -1) {
		member[match[1]] = match[2]
	}
	if len(member) != 2 {
		t.Fatalf("found %d SV_GroupIndex structs, want 2:\n%s", len(member), code)
	}

	// void <entry>(<type> <arg>) { ... <arg>.<member> / WaveGetLaneCount() ... }
	entryRE := regexp.MustCompile(`void (\w+)\((\w+) (\w+)\)`)
	entries := entryRE.FindAllStringSubmatch(code, -1)
	if len(entries) != 2 {
		t.Fatalf("found %d entry points, want 2:\n%s", len(entries), code)
	}
	for _, entry := range entries {
		typeName, argName := entry[2], entry[3]
		want := argName + "." + member[typeName] + " / WaveGetLaneCount()"
		if !strings.Contains(code, want) {
			t.Errorf("entry point %s does not read %q — it names a member its own struct does not declare:\n%s",
				entry[1], want, code)
		}
	}
}
