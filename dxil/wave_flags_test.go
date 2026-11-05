// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

// compileValidateEach compiles every entry point of src and validates it
// with IDxcValidator.
func compileValidateEach(t *testing.T, src string) {
	t.Helper()
	m := lowerSource(t, src)
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		opts := DefaultOptions()
		opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
		blob, _, err := Compile(m, opts)
		if err != nil {
			t.Fatalf("%s: Compile: %v", ep.Name, err)
		}
		if err := Validate(blob, ValidateFull); err != nil {
			t.Errorf("%s: Validate: %v", ep.Name, err)
		}
	}
}

// TestWaveOpsFlagFollowsEmittedCalls (P1-DXIL-086): the WaveOps shader flag
// is set when the module calls a wave or quad dx.op, and only then. DXC
// derives it from the calls (DxilShaderFlags.cpp:587, OP::IsDxilOpWave at
// DxilOperations.cpp:3365: opcodes 110-123, 135-136, 165-167, 222). We set
// it from the declared subgroup built-ins and subgroup statements, so an
// unused @builtin(subgroup_size) and a subgroupBarrier (a plain
// dx.op.barrier) gave "Flags must match usage".
func TestWaveOpsFlagFollowsEmittedCalls(t *testing.T) {
	cases := map[string]string{
		"subgroupBarrier": `
enable subgroups;
@compute @workgroup_size(1)
fn main() {
    subgroupBarrier();
}`,
		"unused subgroup_size": `
enable subgroups;
@group(0) @binding(0) var<storage, read_write> r: array<u32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(subgroup_size) size: u32) {
    r[0] = 1u;
}`,
		"used subgroup_size": `
enable subgroups;
@group(0) @binding(0) var<storage, read_write> r: array<u32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(subgroup_size) size: u32) {
    r[0] = size;
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) { compileValidateEach(t, src) })
	}
}
