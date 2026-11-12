// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// fragDepthRangeAt is where the range of a frag_depth clamp is: members
// member (min) and member+1 (max) of the push constant block variable.
type fragDepthRangeAt struct {
	variable, member uint32
}

// immediateGlobal returns the push constant global ep uses, if any.
func (b *Backend) immediateGlobal(ep *ir.EntryPoint) (ir.GlobalVariableHandle, bool) {
	for _, h := range b.collectUsedGlobalVars(&ep.Function) {
		if space := b.module.GlobalVariables[h].Space; space == ir.SpaceImmediate {
			return h, true
		}
	}
	return 0, false
}

// carriesFragDepthRange reports whether the push constant global h carries
// the range of the frag_depth clamp after its own data, because a fragment
// entry point that writes frag_depth uses it. Offset has to be past that
// data.
func (b *Backend) carriesFragDepthRange(h ir.GlobalVariableHandle) (bool, error) {
	if b.options.ClampFragDepth == nil {
		return false, nil
	}
	if space := b.module.GlobalVariables[h].Space; space != ir.SpaceImmediate {
		return false, nil
	}
	for i := range b.module.EntryPoints {
		ep := &b.module.EntryPoints[i]
		if !b.writesEntryPoint(i) || ep.Stage != ir.StageFragment || !b.entryPointWritesFragDepth(*ep) {
			continue
		}
		if g, ok := b.immediateGlobal(ep); !ok || g != h {
			continue
		}
		global := b.module.GlobalVariables[h]
		var layouter ir.Layouter
		if err := layouter.Update(b.module); err != nil {
			return false, err
		}
		if size := layouter.Layout(global.Type).Size; b.options.ClampFragDepth.Offset < size {
			return false, fmt.Errorf("the frag_depth range at push constant offset %d overlaps %s, which takes %d bytes", b.options.ClampFragDepth.Offset, global.Name, size)
		}
		return true, nil
	}
	return false, nil
}

// declareFragDepthRange finds the range of the frag_depth clamp of the
// fragment entry point ep, when it writes frag_depth and
// Options.ClampFragDepth is set: after the data of its push constant block
// when it has one, else in a push constant block of the range alone,
// declared once.
func (b *Backend) declareFragDepthRange(epIdx int, ep *ir.EntryPoint, output *entryPointOutput) error {
	if b.options.ClampFragDepth == nil || ep.Stage != ir.StageFragment || !b.entryPointWritesFragDepth(*ep) {
		return nil
	}
	var at fragDepthRangeAt
	if h, ok := b.immediateGlobal(ep); ok {
		if !b.fragDepthRangeGlobals[h] {
			return fmt.Errorf("fragment entry point %q writes frag_depth, and its push constants don't carry the clamp's range", ep.Name)
		}
		at = fragDepthRangeAt{variable: b.globalIDs[h], member: 1}
	} else {
		if b.fragDepthRange == 0 {
			f32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
			if err != nil {
				return err
			}
			block := b.builder.AddTypeStruct(f32TypeID, f32TypeID)
			b.builder.AddDecorate(block, spvenums.DecorationBlock)
			b.builder.AddMemberDecorate(block, 0, spvenums.DecorationOffset, b.options.ClampFragDepth.Offset)
			b.builder.AddMemberDecorate(block, 1, spvenums.DecorationOffset, b.options.ClampFragDepth.Offset+4)
			b.fragDepthRange = b.builder.AddVariable(b.emitPointerType(spvenums.StorageClassPushConstant, block), spvenums.StorageClassPushConstant)
			if b.options.Debug {
				b.builder.AddName(block, "FragDepthRange")
				b.builder.AddMemberName(block, 0, "min")
				b.builder.AddMemberName(block, 1, "max")
				b.builder.AddName(b.fragDepthRange, "frag_depth_range")
			}
		}
		at = fragDepthRangeAt{variable: b.fragDepthRange}
		b.fragDepthRangeEPs[epIdx] = true
	}
	for _, varID := range append([]uint32{output.singleVarID}, output.memberVarIDs...) {
		if b.fragDepthVars[varID] {
			b.fragDepthRanges[varID] = at
		}
	}
	return nil
}

// flipPositionY negates the y of output variable varID, just stored, when it
// is a Position output and Options.AdjustCoordinateSpace is set: Rust naga's
// write_epilogue_position_y_flip (back/spv/block.rs), for a Vulkan device
// that ignores a negative viewport height.
func (e *ExpressionEmitter) flipPositionY(varID uint32) error {
	b := e.backend
	if !b.options.AdjustCoordinateSpace || !b.positionVars[varID] {
		return nil
	}
	f32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if err != nil {
		return err
	}
	index, err := e.u32Constant(1)
	if err != nil {
		return err
	}
	y := b.builder.AddAccessChain(b.emitPointerType(spvenums.StorageClassOutput, f32TypeID), varID, index)
	b.builder.AddStore(y, b.builder.AddUnaryOp(spvenums.OpFNegate, f32TypeID, b.builder.AddLoad(f32TypeID, y)))
	return nil
}

// clampFragDepth returns value, about to be stored to output variable varID,
// clamped to the range in the push constants when varID is a FragDepth
// output of an entry point that clamps it, as WebGPU clamps frag_depth to
// the viewport's depth range.
func (e *ExpressionEmitter) clampFragDepth(varID, value uint32) (uint32, error) {
	b := e.backend
	at, ok := b.fragDepthRanges[varID]
	if !ok {
		return value, nil
	}
	f32TypeID, err := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if err != nil {
		return 0, err
	}
	ptrTypeID := b.emitPointerType(spvenums.StorageClassPushConstant, f32TypeID)
	bound := func(member uint32) (uint32, error) {
		index, err := e.u32Constant(member)
		if err != nil {
			return 0, err
		}
		return b.builder.AddLoad(f32TypeID, b.builder.AddAccessChain(ptrTypeID, at.variable, index)), nil
	}
	low, err := bound(at.member)
	if err != nil {
		return 0, err
	}
	high, err := bound(at.member + 1)
	if err != nil {
		return 0, err
	}
	return b.builder.AddExtInst(f32TypeID, b.glslExtID, uint32(spvenums.GlslStd450FClamp), value, low, high), nil
}
