package front

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// decoration collects the values of OpDecorate, OpMemberDecorate, OpName and
// OpMemberName for one id until its definition consumes them: SPIR-V allows
// either order, and Rust naga merges them the same way.
type decoration struct {
	name string

	builtIn   *uint32
	location  *uint32
	index     *uint32
	descSet   *uint32
	descIndex *uint32
	specID    *uint32
	// storageBuffer marks a BufferBlock-decorated struct: the legacy Vulkan
	// 1.0 spelling of a storage buffer, upgraded when the variable is parsed.
	storageBuffer bool

	offset    uint32
	hasOffset bool

	arrayStride    uint32
	hasArrayStride bool

	matrixStride    uint32
	hasMatrixStride bool

	// rowMajor is nil without ColMajor or RowMajor (Rust naga's
	// matrix_major: Option<Majority>).
	rowMajor *bool

	invariant     bool
	interpolation *ir.Interpolation
	sampling      *ir.Sampling

	flags decorationFlags
}

// decorationFlags is Rust naga's DecorationFlags: the memory decorations that
// are not part of a binding.
type decorationFlags uint32

const (
	decorationNonReadable decorationFlags = 1 << 0
	decorationNonWritable decorationFlags = 1 << 1
	decorationCoherent    decorationFlags = 1 << 2
	decorationVolatile    decorationFlags = 1 << 3
)

// takeDecoration returns the decoration record of id and removes it: its
// definition consumes it, as Rust naga's future_decor.remove. An id without
// one gets an empty record.
func (p *parser) takeDecoration(id uint32) *decoration {
	dec, ok := p.decorations[id]
	if !ok {
		return &decoration{}
	}
	delete(p.decorations, id)
	return dec
}

// takeMemberDecoration returns the decoration record of a struct member and
// removes it.
func (p *parser) takeMemberDecoration(id, member uint32) *decoration {
	key := memberKey{typeID: id, member: member}
	dec, ok := p.memberDecorations[key]
	if !ok {
		return &decoration{}
	}
	delete(p.memberDecorations, key)
	return dec
}

// storageAccess is the StorageAccess the flags describe: both, without
// NonReadable or NonWritable.
func (d *decoration) storageAccess() ir.StorageAccess {
	access := ir.StorageAccessLoad | ir.StorageAccessStore
	if d.flags&decorationNonReadable != 0 {
		access &^= ir.StorageAccessLoad
	}
	if d.flags&decorationNonWritable != 0 {
		access &^= ir.StorageAccessStore
	}
	return access
}

// resourceBinding returns the DescriptorSet and Binding decorations, if both
// are present.
func (d *decoration) resourceBinding() *ir.ResourceBinding {
	if d.descSet == nil || d.descIndex == nil {
		return nil
	}
	return &ir.ResourceBinding{Group: *d.descSet, Binding: *d.descIndex}
}

// ioBinding returns the binding of an input or output variable, as Rust
// naga's Decoration::io_binding.
func (d *decoration) ioBinding() (ir.Binding, error) {
	switch {
	case d.builtIn != nil && d.location == nil:
		builtin, err := mapBuiltin(*d.builtIn, d.invariant)
		if err != nil {
			return nil, err
		}
		return ir.BuiltinBinding{Builtin: builtin, Invariant: d.invariant}, nil
	case d.builtIn == nil && d.location != nil && d.index != nil:
		// The dual-source blending index pins the location's interpolation
		// and sampling: Rust naga leaves both unset here.
		blendSrc := *d.index
		return ir.LocationBinding{Location: *d.location, BlendSrc: &blendSrc}, nil
	case d.builtIn == nil && d.location != nil:
		return ir.LocationBinding{
			Location:      *d.location,
			Interpolation: d.interpolation,
			Sampling:      d.sampling,
		}, nil
	}
	return nil, errMissingDecoration(spvenums.DecorationLocation)
}

// mapBuiltin maps a SPIR-V BuiltIn decoration, as Rust naga's map_builtin.
func mapBuiltin(word uint32, invariant bool) (ir.BuiltinValue, error) {
	switch spvenums.BuiltIn(word) {
	case spvenums.BuiltInPosition, spvenums.BuiltInFragCoord:
		return ir.BuiltinPosition, nil
	case spvenums.BuiltInViewIndex:
		return ir.BuiltinViewIndex, nil
	case spvenums.BuiltInBaseInstance:
		return ir.BuiltinBaseInstance, nil
	case spvenums.BuiltInBaseVertex:
		return ir.BuiltinBaseVertex, nil
	case spvenums.BuiltInClipDistance:
		return ir.BuiltinClipDistances, nil
	case spvenums.BuiltInCullDistance:
		return ir.BuiltinCullDistance, nil
	case spvenums.BuiltInInstanceIndex:
		return ir.BuiltinInstanceIndex, nil
	case spvenums.BuiltInPointSize:
		return ir.BuiltinPointSize, nil
	case spvenums.BuiltInVertexIndex:
		return ir.BuiltinVertexIndex, nil
	case spvenums.BuiltInDrawIndex:
		return ir.BuiltinDrawIndex, nil
	case spvenums.BuiltInFragDepth:
		return ir.BuiltinFragDepth, nil
	case spvenums.BuiltInPointCoord:
		return ir.BuiltinPointCoord, nil
	case spvenums.BuiltInFrontFacing:
		return ir.BuiltinFrontFacing, nil
	case spvenums.BuiltInPrimitiveId:
		return ir.BuiltinPrimitiveIndex, nil
	case spvenums.BuiltInBaryCoordKHR:
		return ir.BuiltinBarycentric, nil
	case spvenums.BuiltInBaryCoordNoPerspKHR:
		return ir.BuiltinBarycentricNoPerspective, nil
	case spvenums.BuiltInSampleId:
		return ir.BuiltinSampleIndex, nil
	case spvenums.BuiltInSampleMask:
		return ir.BuiltinSampleMask, nil
	case spvenums.BuiltInGlobalInvocationId:
		return ir.BuiltinGlobalInvocationID, nil
	case spvenums.BuiltInLocalInvocationId:
		return ir.BuiltinLocalInvocationID, nil
	case spvenums.BuiltInLocalInvocationIndex:
		return ir.BuiltinLocalInvocationIndex, nil
	case spvenums.BuiltInWorkgroupId:
		return ir.BuiltinWorkGroupID, nil
	case spvenums.BuiltInWorkgroupSize:
		return ir.BuiltinWorkGroupSize, nil
	case spvenums.BuiltInNumWorkgroups:
		return ir.BuiltinNumWorkGroups, nil
	case spvenums.BuiltInNumSubgroups:
		return ir.BuiltinNumSubgroups, nil
	case spvenums.BuiltInSubgroupId:
		return ir.BuiltinSubgroupID, nil
	case spvenums.BuiltInSubgroupSize:
		return ir.BuiltinSubgroupSize, nil
	case spvenums.BuiltInSubgroupLocalInvocationId:
		return ir.BuiltinSubgroupInvocationID, nil
	}
	return 0, errUnsupportedBuiltIn(word)
}

// nextDecoration reads the decoration of inst and applies it to dec.
// baseWords is the number of words before the decoration: the instruction
// word and the id, plus the member for OpMemberDecorate. An unknown
// decoration is a warning whose operands are skipped, as Rust naga logs it.
func (p *parser) nextDecoration(inst instruction, baseWords uint16, dec *decoration) error {
	raw, err := p.next()
	if err != nil {
		return err
	}
	decoration := spvenums.Decoration(raw)
	if !decoration.Known() {
		return errInvalidDecoration(raw)
	}
	// readUint reads a decoration's single operand.
	readUint := func(target **uint32) error {
		if err := inst.expect(baseWords + 2); err != nil {
			return err
		}
		value, err := p.next()
		if err != nil {
			return err
		}
		*target = &value
		return nil
	}
	// noOperand checks that a decoration has no operand.
	noOperand := func() error {
		return inst.expect(baseWords + 1)
	}
	switch decoration {
	case spvenums.DecorationBuiltIn:
		return readUint(&dec.builtIn)
	case spvenums.DecorationLocation:
		return readUint(&dec.location)
	case spvenums.DecorationIndex:
		return readUint(&dec.index)
	case spvenums.DecorationDescriptorSet:
		return readUint(&dec.descSet)
	case spvenums.DecorationBinding:
		return readUint(&dec.descIndex)
	case spvenums.DecorationSpecId:
		return readUint(&dec.specID)
	case spvenums.DecorationOffset:
		if err := inst.expect(baseWords + 2); err != nil {
			return err
		}
		value, err := p.next()
		if err != nil {
			return err
		}
		dec.offset, dec.hasOffset = value, true
		return nil
	case spvenums.DecorationArrayStride:
		if err := inst.expect(baseWords + 2); err != nil {
			return err
		}
		value, err := p.next()
		if err != nil {
			return err
		}
		// A stride of zero is no stride (Rust naga's NonZeroU32).
		dec.arrayStride, dec.hasArrayStride = value, value != 0
		return nil
	case spvenums.DecorationMatrixStride:
		if err := inst.expect(baseWords + 2); err != nil {
			return err
		}
		value, err := p.next()
		if err != nil {
			return err
		}
		dec.matrixStride, dec.hasMatrixStride = value, value != 0
		return nil
	case spvenums.DecorationBufferBlock:
		dec.storageBuffer = true
		return noOperand()
	case spvenums.DecorationBlock:
		// Ignored: it appears on every buffer and changes nothing in the IR.
		// Rust naga demotes it to a debug log for the same reason.
		return noOperand()
	case spvenums.DecorationInvariant:
		dec.invariant = true
		return noOperand()
	case spvenums.DecorationNoPerspective:
		interpolation := ir.InterpolationLinear
		dec.interpolation = &interpolation
		return noOperand()
	case spvenums.DecorationFlat:
		interpolation := ir.InterpolationFlat
		dec.interpolation = &interpolation
		return noOperand()
	case spvenums.DecorationPerVertexKHR:
		interpolation := ir.InterpolationPerVertex
		dec.interpolation = &interpolation
		return noOperand()
	case spvenums.DecorationCentroid:
		sampling := ir.SamplingCentroid
		dec.sampling = &sampling
		return noOperand()
	case spvenums.DecorationSample:
		sampling := ir.SamplingSample
		dec.sampling = &sampling
		return noOperand()
	case spvenums.DecorationNonReadable:
		dec.flags |= decorationNonReadable
		return noOperand()
	case spvenums.DecorationNonWritable:
		dec.flags |= decorationNonWritable
		return noOperand()
	case spvenums.DecorationCoherent:
		dec.flags |= decorationCoherent
		return noOperand()
	case spvenums.DecorationVolatile:
		dec.flags |= decorationVolatile
		return noOperand()
	case spvenums.DecorationColMajor:
		column := false
		dec.rowMajor = &column
		return noOperand()
	case spvenums.DecorationRowMajor:
		row := true
		dec.rowMajor = &row
		return noOperand()
	}
	// An unknown decoration: warn and skip its operands. Block decorations
	// are demoted to a debug level in Rust naga because they appear
	// everywhere; the warning is kept here for both.
	p.warnf("Unknown decoration %s", bareName(decoration.String(), "Decoration"))
	for range inst.wc - (baseWords + 1) {
		if _, err := p.next(); err != nil {
			return err
		}
	}
	return nil
}
