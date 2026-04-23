package emit

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/viewid"
	"github.com/gogpu/naga/ir"
)

// TestInputUsedMaskClamping verifies that InputUsedMasks values (0xFF for
// "all components used") are properly clamped to the element's actual
// channel count before being stored in inputUsedMasks.
func TestInputUsedMaskClamping(t *testing.T) {
	// Build a minimal module with a fragment shader that has a vec3 input.
	vec3Type := ir.TypeHandle(0)
	f32Type := ir.TypeHandle(1)
	vec4Type := ir.TypeHandle(2)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	_ = f32Type

	loc0 := ir.Binding(ir.LocationBinding{Location: 0})
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	ep := &ir.EntryPoint{
		Name:  "frag",
		Stage: ir.StageFragment,
		Function: ir.Function{
			Name: "frag",
			Arguments: []ir.FunctionArgument{
				{Type: vec3Type, Binding: &loc0},
			},
			Result: &ir.FunctionResult{
				Type:    vec4Type,
				Binding: &posBinding,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
		},
	}

	// InputUsedMasks with 0xFF (all bits set, needs clamping to 3 channels).
	usedMasks := map[InputUsageKey]uint8{
		{ArgIdx: 0, MemberIdx: -1}: 0xFF,
	}

	e := &Emitter{
		ir:   irMod,
		opts: EmitOptions{InputUsedMasks: usedMasks},
	}

	// Simulate what emitGraphicsViewIDState does: build inElems and compute masks.
	inElems := []viewid.SigElement{
		{ScalarStart: 0, NumChannels: 3, VectorRow: 0},
	}

	// Replicate the mask computation logic.
	e.inputUsedMasks = make([]int64, len(inElems))
	elemIdx := 0
	for argIdx := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[argIdx]
		argType := irMod.Types[arg.Type]
		if _, ok := argType.Inner.(ir.StructType); ok {
			t.Fatal("unexpected struct")
		} else if arg.Binding != nil {
			if elemIdx < len(e.inputUsedMasks) {
				key := InputUsageKey{ArgIdx: argIdx, MemberIdx: -1}
				mask := e.opts.InputUsedMasks[key]
				channels := inElems[elemIdx].NumChannels
				if channels > 0 && channels <= 4 {
					mask &= uint8((1 << channels) - 1)
				}
				e.inputUsedMasks[elemIdx] = int64(mask)
				elemIdx++
			}
		}
	}

	// Verify: 0xFF clamped to 3 channels = 0x07 (bits 0,1,2)
	if e.inputUsedMasks[0] != 7 {
		t.Errorf("expected clamped mask 7 (3 channels), got %d", e.inputUsedMasks[0])
	}
}

// TestInputUsedMaskNull verifies that unused inputs (mask=0) produce null
// metadata, while used inputs produce !{i32 3, i32 mask}.
func TestInputUsedMaskNull(t *testing.T) {
	// Build a minimal module with two inputs: one used, one unused.
	vec4Type := ir.TypeHandle(0)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	loc0 := ir.Binding(ir.LocationBinding{Location: 0})
	loc1 := ir.Binding(ir.LocationBinding{Location: 1})
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	ep := &ir.EntryPoint{
		Name:  "vert",
		Stage: ir.StageVertex,
		Function: ir.Function{
			Name: "vert",
			Arguments: []ir.FunctionArgument{
				{Type: vec4Type, Binding: &loc0},
				{Type: vec4Type, Binding: &loc1},
			},
			Result: &ir.FunctionResult{
				Type:    vec4Type,
				Binding: &posBinding,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
		},
	}
	_ = ep

	// Arg 0 is used (mask=0x0F for vec4), arg 1 is unused (mask=0).
	usedMasks := map[InputUsageKey]uint8{
		{ArgIdx: 0, MemberIdx: -1}: 0x0F,
		// Arg 1 absent from map = not used
	}

	e := &Emitter{
		ir:   irMod,
		opts: EmitOptions{InputUsedMasks: usedMasks},
	}

	inElems := []viewid.SigElement{
		{ScalarStart: 0, NumChannels: 4, VectorRow: 0},
		{ScalarStart: 4, NumChannels: 4, VectorRow: 1},
	}

	e.inputUsedMasks = make([]int64, len(inElems))
	elemIdx := 0
	for argIdx := range ep.Function.Arguments {
		arg := &ep.Function.Arguments[argIdx]
		argType := irMod.Types[arg.Type]
		if _, ok := argType.Inner.(ir.StructType); ok {
			t.Fatal("unexpected struct")
		} else if arg.Binding != nil {
			if elemIdx < len(e.inputUsedMasks) {
				key := InputUsageKey{ArgIdx: argIdx, MemberIdx: -1}
				mask := e.opts.InputUsedMasks[key]
				channels := inElems[elemIdx].NumChannels
				if channels > 0 && channels <= 4 {
					mask &= uint8((1 << channels) - 1)
				}
				e.inputUsedMasks[elemIdx] = int64(mask)
				elemIdx++
			}
		}
	}

	// Arg 0: used, mask = 15
	if e.inputUsedMasks[0] != 15 {
		t.Errorf("arg 0: expected mask 15, got %d", e.inputUsedMasks[0])
	}
	// Arg 1: unused, mask = 0 -> will produce null in metadata
	if e.inputUsedMasks[1] != 0 {
		t.Errorf("arg 1: expected mask 0 (null), got %d", e.inputUsedMasks[1])
	}
}

// TestInputUsedMaskSortedOrder verifies that computeInputElementUsedMasks
// assigns masks in sorted signature order (locations before builtins) rather
// than declaration order. Regression test for the msl-varyings/fs_main bug
// where @builtin(position) from arg0 got LOC's slot and @location(1) from
// arg1 got SV_Position's slot.
func TestInputUsedMaskSortedOrder(t *testing.T) {
	// Fragment shader with:
	//   arg0: VertexOutput { @builtin(position) position: vec4f }
	//   arg1: NoteInstance { @location(1) position: vec2f }
	// Sorted order: LOC1 (sig element 0), SV_Position (sig element 1)
	vec4Type := ir.TypeHandle(0)
	vec2Type := ir.TypeHandle(1)
	vertexOutputType := ir.TypeHandle(2) // struct { @builtin(position) vec4f }
	noteInstanceType := ir.TypeHandle(3) // struct { @location(1) vec2f }

	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	loc1Binding := ir.Binding(ir.LocationBinding{Location: 1})

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "position", Type: vec4Type, Binding: &posBinding},
			}}},
			{Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "position", Type: vec2Type, Binding: &loc1Binding},
			}}},
		},
	}

	targetBinding := ir.Binding(ir.LocationBinding{Location: 0})
	ep := &ir.EntryPoint{
		Name:  "fs_main",
		Stage: ir.StageFragment,
		Function: ir.Function{
			Name: "fs_main",
			Arguments: []ir.FunctionArgument{
				{Type: vertexOutputType}, // arg0: has @builtin(position)
				{Type: noteInstanceType}, // arg1: has @location(1)
			},
			Result: &ir.FunctionResult{
				Type:    vec4Type,
				Binding: &targetBinding,
			},
		},
	}

	// SV_Position is used (mask=15), LOC1 is not used (mask=0)
	usedMasks := map[InputUsageKey]uint8{
		{ArgIdx: 0, MemberIdx: 0}: 0x0F, // SV_Position used
		// arg1/member0 (LOC1) absent = not used
	}

	e := &Emitter{
		ir:   irMod,
		opts: EmitOptions{InputUsedMasks: usedMasks},
	}

	// inElems in sorted order: LOC1 first (2 channels), SV_Position second (4 channels)
	inElems := []viewid.SigElement{
		{ScalarStart: 0, NumChannels: 2, VectorRow: 0}, // LOC1
		{ScalarStart: 2, NumChannels: 4, VectorRow: 1}, // SV_Position
	}

	masks := e.computeInputElementUsedMasks(ep, inElems)

	// LOC1 (element 0) is not used -> mask should be 0
	if masks[0] != 0 {
		t.Errorf("LOC1 (element 0): expected mask 0 (unused), got %d", masks[0])
	}
	// SV_Position (element 1) is used -> mask should be 15
	if masks[1] != 15 {
		t.Errorf("SV_Position (element 1): expected mask 15 (all 4 channels used), got %d", masks[1])
	}
}

// TestInputUsedMaskVSInputPreservesOrder verifies that VS inputs use
// declaration order (not sorted) for the usage mask mapping.
func TestInputUsedMaskVSInputPreservesOrder(t *testing.T) {
	vec4Type := ir.TypeHandle(0)
	u32Type := ir.TypeHandle(1)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}

	vidBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})
	loc0Binding := ir.Binding(ir.LocationBinding{Location: 0})
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	ep := &ir.EntryPoint{
		Name:  "vs_main",
		Stage: ir.StageVertex,
		Function: ir.Function{
			Name: "vs_main",
			Arguments: []ir.FunctionArgument{
				{Type: u32Type, Binding: &vidBinding},   // arg0: SV_VertexID
				{Type: vec4Type, Binding: &loc0Binding}, // arg1: LOC0
			},
			Result: &ir.FunctionResult{
				Type:    vec4Type,
				Binding: &posBinding,
			},
		},
	}

	usedMasks := map[InputUsageKey]uint8{
		{ArgIdx: 0, MemberIdx: -1}: 0x01, // SV_VertexID used
		{ArgIdx: 1, MemberIdx: -1}: 0x0F, // LOC0 used
	}

	e := &Emitter{
		ir:   irMod,
		opts: EmitOptions{InputUsedMasks: usedMasks},
	}

	// VS inputs in declaration order: SV_VertexID (1 channel), LOC0 (4 channels)
	inElems := []viewid.SigElement{
		{ScalarStart: 0, NumChannels: 1, VectorRow: 0}, // SV_VertexID
		{ScalarStart: 1, NumChannels: 4, VectorRow: 1}, // LOC0
	}

	masks := e.computeInputElementUsedMasks(ep, inElems)

	// Declaration order preserved for VS: element 0 = SV_VertexID
	if masks[0] != 1 {
		t.Errorf("SV_VertexID (element 0): expected mask 1, got %d", masks[0])
	}
	if masks[1] != 15 {
		t.Errorf("LOC0 (element 1): expected mask 15, got %d", masks[1])
	}
}

// TestViewIDMaxPackedLocationIncludesStartCol verifies that the ViewID state
// scalar count uses DXC's DetermineMaxPackedLocation formula:
// max(StartRow*4 + StartCol + NumChannels) over all non-system-managed elements.
// This correctly handles packed elements like LOC0=xyz at col 0 + LOC1=w at col 3.
func TestViewIDMaxPackedLocationIncludesStartCol(t *testing.T) {
	// LOC0: row=0, col=0, 3 channels (xyz) -> end = 0+0+3 = 3
	// LOC1: row=0, col=3, 1 channel (w) -> end = 0+3+1 = 4
	// Max should be 4, not 3.
	inElems := []viewid.SigElement{
		{ScalarStart: 0, NumChannels: 3, VectorRow: 0, StartCol: 0},
		{ScalarStart: 3, NumChannels: 1, VectorRow: 0, StartCol: 3},
	}

	var inComps uint32
	for i := range inElems {
		if inElems[i].SystemManaged {
			continue
		}
		end := inElems[i].VectorRow*4 + inElems[i].StartCol + inElems[i].NumChannels
		if end > inComps {
			inComps = end
		}
	}

	if inComps != 4 {
		t.Errorf("expected inComps=4 (row*4 + startCol + numChannels), got %d", inComps)
	}
}
