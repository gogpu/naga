// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import "testing"

// TestComputeVectorBuiltinStructMembers (P2-DXIL-093): the vec3 compute
// built-ins (local_invocation_id, global_invocation_id, workgroup_id,
// num_workgroups) as members of a compute input struct are read like the
// same built-ins as arguments, one dx.op call per component
// (threadIdInGroup, threadId, groupId; num_workgroups from its constant
// buffer). The struct path took scalars only and refused them
// ("compute builtin 7 (local_invocation_id vec3) not supported as struct
// member"), so 8820-multiple-local-invocation-index-id did not compile.
func TestComputeVectorBuiltinStructMembers(t *testing.T) {
	compileValidateEach(t, `
var<workgroup> wg_var: u32;
struct Input {
    @builtin(local_invocation_id) local_invocation_id: vec3<u32>,
    @builtin(local_invocation_index) local_invocation_index: u32,
    @builtin(global_invocation_id) global_id: vec3<u32>,
    @builtin(workgroup_id) group_id: vec3<u32>,
}
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(4, 2, 1)
fn main(input: Input) {
    wg_var = input.local_invocation_index * 2u;
    wg_var += input.local_invocation_id.x;
    out[input.global_id.x] = input.group_id.y + input.local_invocation_id.y + wg_var;
}`)
}
