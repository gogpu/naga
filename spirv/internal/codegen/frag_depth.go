// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// declareFragDepthRange declares, once, the push constant block that holds
// the range of the frag_depth clamp, when the fragment entry point ep writes
// frag_depth and Options.ClampFragDepth is set.
func (b *Backend) declareFragDepthRange(ep *ir.EntryPoint) error {
	if b.options.ClampFragDepth == nil || ep.Stage != ir.StageFragment || !b.entryPointWritesFragDepth(*ep) {
		return nil
	}
	for _, h := range b.collectUsedGlobalVars(&ep.Function) {
		if space := b.module.GlobalVariables[h].Space; space == ir.SpacePushConstant || space == ir.SpaceImmediate {
			return fmt.Errorf("fragment entry point %q writes frag_depth, whose clamp needs the push constants, and has push constants of its own", ep.Name)
		}
	}
	if b.fragDepthRange != 0 {
		return nil
	}
	f32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if err != nil {
		return err
	}
	block := b.builder.AddTypeStruct(f32TypeID, f32TypeID)
	b.builder.AddDecorate(block, DecorationBlock)
	b.builder.AddMemberDecorate(block, 0, DecorationOffset, b.options.ClampFragDepth.Offset)
	b.builder.AddMemberDecorate(block, 1, DecorationOffset, b.options.ClampFragDepth.Offset+4)
	b.fragDepthRange = b.builder.AddVariable(b.emitPointerType(StorageClassPushConstant, block), StorageClassPushConstant)
	if b.options.Debug {
		b.builder.AddName(block, "FragDepthRange")
		b.builder.AddMemberName(block, 0, "min")
		b.builder.AddMemberName(block, 1, "max")
		b.builder.AddName(b.fragDepthRange, "frag_depth_range")
	}
	return nil
}

// clampFragDepth returns value, about to be stored to output variable varID,
// clamped to the range in the push constants when varID is a FragDepth
// output, as WebGPU clamps frag_depth to the viewport's depth range.
func (e *ExpressionEmitter) clampFragDepth(varID, value uint32) (uint32, error) {
	b := e.backend
	if b.fragDepthRange == 0 || !b.fragDepthVars[varID] {
		return value, nil
	}
	f32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if err != nil {
		return 0, err
	}
	ptrTypeID := b.emitPointerType(StorageClassPushConstant, f32TypeID)
	bound := func(member uint32) (uint32, error) {
		index, err := e.u32Constant(member)
		if err != nil {
			return 0, err
		}
		return b.builder.AddLoad(f32TypeID, b.builder.AddAccessChain(ptrTypeID, b.fragDepthRange, index)), nil
	}
	low, err := bound(0)
	if err != nil {
		return 0, err
	}
	high, err := bound(1)
	if err != nil {
		return 0, err
	}
	return b.builder.AddExtInst(f32TypeID, b.glslExtID, GLSLstd450FClamp, value, low, high), nil
}
