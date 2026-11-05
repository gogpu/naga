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

// TestFragmentSubgroupBuiltins (P1-DXIL-087): subgroup built-ins on a
// fragment entry point are wave intrinsics (WaveGetLaneCount,
// WaveGetLaneIndex), not input signature elements: DXC lowers them to
// dx.op calls and leaves the input signature and PSV0 to the real inputs.
func TestFragmentSubgroupBuiltins(t *testing.T) {
	cases := map[string]string{
		"subgroup_size alone": `
enable subgroups;
@fragment
fn fs(@builtin(subgroup_size) size: u32) -> @location(0) vec4f {
    return vec4f(f32(size) / 255.0);
}`,
		"with position and quad ops": `
enable subgroups;
@fragment
fn fs(
    @builtin(position) pos: vec4f,
    @builtin(subgroup_size) size: u32,
    @builtin(subgroup_invocation_id) lane: u32,
) -> @location(0) vec4f {
    let x = quadSwapX(pos.x);
    let y = quadBroadcast(pos.y, 1u);
    return vec4f(x, y, f32(size), f32(lane));
}`,
		"in a struct": `
enable subgroups;
struct In {
    @builtin(position) pos: vec4f,
    @location(0) uv: vec2f,
    @builtin(subgroup_size) size: u32,
}
@fragment
fn fs(i: In) -> @location(0) vec4f {
    return vec4f(i.uv, f32(i.size), i.pos.x);
}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) { compileValidateEach(t, src) })
	}
}

// TestComputeSubgroupStructMember: a subgroup_invocation_id carried in a
// compute input struct calls dx.op.waveGetLaneIndex, whose symbol has no
// type suffix (WaveGetLaneIndex has no overload); the struct path declared
// dx.op.waveGetLaneIndex.i32, which the validator rejects.
func TestComputeSubgroupStructMember(t *testing.T) {
	compileValidateEach(t, `
enable subgroups;
struct In {
    @builtin(subgroup_invocation_id) lane: u32,
    @builtin(subgroup_size) size: u32,
}
@group(0) @binding(0) var<storage, read_write> r: array<u32, 64>;
@compute @workgroup_size(64)
fn main(i: In) {
    r[i.lane] = i.size;
}`)
}
