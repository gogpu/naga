// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
)

// maxWorkgroupVariableBytes is the most thread group shared memory (TGSM,
// WGSL's workgroup storage) D3D12 accepts for a compute, task or mesh entry
// point: 32768 bytes. DXC's validator takes the limit from DXIL::kMaxTGSMSize
// (include/dxc/DXIL/DxilConstants.h:130, "8192 * 4") for compute, amplification
// and node shaders and from kMaxMSSMSize (...:147, "1024 * 28") for mesh
// shaders (lib/DxilValidation/DxilValidation.cpp:3649-3656), sums the size of
// every groupshared global the entry point reaches
// (DxilValidation.cpp:3736) and reports
// SM.MAXTGSMSIZEONENTRY, "Total Thread Group Shared Memory used by entry must
// not exceed maximum for shader model" (docs/DXIL.rst:3345), when the sum is
// greater (DxilValidation.cpp:3763).
const maxWorkgroupVariableBytes = 8192 * 4

// maxMeshWorkgroupVariableBytes is DXC's TGSM limit for a mesh entry point,
// DXIL::kMaxMSSMSize (include/dxc/DXIL/DxilConstants.h:147, "1024 * 28").
const maxMeshWorkgroupVariableBytes = 1024 * 28

// maxWorkgroupBytes returns the TGSM limit of a stage, and false for a stage
// DXIL allows no thread group shared memory in at all.
//
// DXC's getMaxTGSM first honors a per-function Props.groupSharedLimitBytes
// override (newer shader models); naga never writes that attribute, so the
// fixed limits are right today. Whoever adds it moves this check with it.
func maxWorkgroupBytes(stage ir.ShaderStage) (uint64, bool) {
	switch stage {
	case ir.StageCompute, ir.StageTask:
		return maxWorkgroupVariableBytes, true
	case ir.StageMesh:
		return maxMeshWorkgroupVariableBytes, true
	}
	return 0, false
}

// workgroupVariableBytes is the size of the groupshared memory the entry
// point ep reaches, as the emitter writes it and the validator measures it:
// the sum of the workgroup variables the entry point's body keeps, so a struct
// counts its members and an array of vectors its scalars, not the padding of
// the source types. The sum saturates at variableBytesCap, which is above
// every limit here, so it cannot wrap. A mesh shader's output variable is not
// counted: D3D12 holds mesh outputs in the node record, not in TGSM, and the
// emitter writes them with dx.op stores instead of groupshared globals.
func workgroupVariableBytes(m *ir.Module, ep *ir.EntryPoint) uint64 {
	var total uint64
	var meshOutput *ir.GlobalVariableHandle
	if ep.MeshInfo != nil {
		meshOutput = &ep.MeshInfo.OutputVariable
	}
	for h := range reachableGlobalsForEntry(m, ep) {
		if meshOutput != nil && h == *meshOutput {
			continue
		}
		gv := &m.GlobalVariables[h]
		if gv.Space != ir.SpaceWorkGroup {
			continue
		}
		total = min(total+variableBytes(m, gv.Type), variableBytesCap)
	}
	return total
}

// checkWorkgroupVariableBytes refuses entry point ep when its workgroup
// variables take more shared memory than D3D12 gives a thread group, which
// DXC's validator reports as SM.MAXTGSMSIZEONENTRY, instead of writing a
// container every D3D12 runtime refuses. WGSL's own
// maxComputeWorkgroupStorageSize (16384 by default) is a device limit the API
// checks; this one is the hardware's, which no device setting raises.
func checkWorkgroupVariableBytes(m *ir.Module, ep *ir.EntryPoint) error {
	total := workgroupVariableBytes(m, ep)
	if total == 0 {
		return nil
	}
	maxBytes, ok := maxWorkgroupBytes(ep.Stage)
	if !ok {
		return unsupported.Errorf("dxil: entry point %q: workgroup variables are not supported in a %s entry point (DXIL has thread group shared memory only in compute, task and mesh shaders)",
			ep.Name, ep.Stage)
	}
	if total > maxBytes {
		return unsupported.Errorf("dxil: entry point %q declares more than %d bytes of workgroup (groupshared) variables, the most D3D12 accepts in a %s entry point",
			ep.Name, maxBytes, ep.Stage)
	}
	return nil
}
