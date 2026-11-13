package front

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// lookupType is one entry of the SPIR-V type id table: the IR type and, where
// the SPIR-V type wraps another (a vector's scalar, a pointer's base, ...), the
// id it wraps, which later instructions need (Rust naga's LookupType).
type lookupType struct {
	handle  ir.TypeHandle
	baseID  uint32
	hasBase bool
}

// lookupMember is one struct member of a type: its SPIR-V type id and whether
// it is row-major (Rust naga's LookupMember).
type lookupMember struct {
	typeID   uint32
	rowMajor bool
}

// lookupMemberKey identifies a struct member of a type.
type lookupMemberKey struct {
	handle ir.TypeHandle
	member uint32
}

// lookupFunctionType is one OpTypeFunction: its parameter and return type ids.
type lookupFunctionType struct {
	parameterTypeIDs []uint32
	returnTypeID     uint32
}

// variableClass is Rust naga's ExtendedClass: what a variable is for the IR.
type variableClass uint8

const (
	classGlobal variableClass = iota
	classInput
	classOutput
)

// extendedClass is where a SPIR-V storage class puts a variable.
type extendedClass struct {
	class variableClass
	// space is the address space of a classGlobal variable.
	space ir.AddressSpace
}

// insertType interns a type and keeps the module's type arena in sync with the
// registry, which deduplicates by the full definition as Rust naga's
// UniqueArena does.
func (p *parser) insertType(name string, inner ir.TypeInner) ir.TypeHandle {
	depth := p.typeDepth(inner)
	handle := p.types.GetOrCreate(name, inner)
	p.module.Types = p.types.GetTypes()
	for len(p.typeDepths) < len(p.module.Types) {
		p.typeDepths = append(p.typeDepths, depth)
	}
	return handle
}

// typeDepth returns how many levels a type with this definition nests: one
// for a type without component types, one more than the deepest component
// otherwise. The components are in the arena already, so this is no walk.
func (p *parser) typeDepth(inner ir.TypeInner) int {
	component := 0
	switch t := inner.(type) {
	case ir.ArrayType:
		component = p.handleDepth(t.Base)
	case ir.BindingArrayType:
		component = p.handleDepth(t.Base)
	case ir.PointerType:
		component = p.handleDepth(t.Base)
	case ir.StructType:
		for _, member := range t.Members {
			component = max(component, p.handleDepth(member.Type))
		}
	}
	return component + 1
}

// handleDepth returns the depth recorded for a type of the arena.
func (p *parser) handleDepth(handle ir.TypeHandle) int {
	if int(handle) < len(p.typeDepths) {
		return p.typeDepths[handle]
	}
	return 0
}

// checkTypeDepth refuses a type definition nested deeper than maxTypeDepth,
// before it enters the arena.
func (p *parser) checkTypeDepth(inner ir.TypeInner) error {
	if p.typeDepth(inner) > maxTypeDepth {
		return errTypeTooDeep
	}
	return nil
}

// typeLookup returns the table entry of a type id.
func (p *parser) typeLookup(id uint32) (lookupType, error) {
	lookup, ok := p.lookupType[id]
	if !ok {
		return lookupType{}, errInvalidID(id)
	}
	return lookup, nil
}

// updateLayouter lays out the types added since the last call, which array and
// struct parsing read strides and alignments from.
func (p *parser) updateLayouter() error {
	if err := p.layouter.Update(p.module); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

// parseTypeVoid records the void type id; the IR has no void type.
func (p *parser) parseTypeVoid(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(2); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	p.voidTypeID, p.hasVoidType = id, true
	return nil
}

// parseTypeBool parses OpTypeBool into a bool scalar.
func (p *parser) parseTypeBool(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(2); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	handle := p.insertType(dec.name, ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	p.lookupType[id] = lookupType{handle: handle}
	return nil
}

// parseTypeInt parses OpTypeInt into a signed or unsigned scalar.
func (p *parser) parseTypeInt(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	widthWord, err := p.next()
	if err != nil {
		return err
	}
	sign, err := p.next()
	if err != nil {
		return err
	}
	var kind ir.ScalarKind
	switch sign {
	case 0:
		kind = ir.ScalarUint
	case 1:
		kind = ir.ScalarSint
	default:
		return errInvalidSign(sign)
	}
	width, err := mapWidth(widthWord)
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	handle := p.insertType(dec.name, ir.ScalarType{Kind: kind, Width: width})
	p.lookupType[id] = lookupType{handle: handle}
	return nil
}

// parseTypeFloat parses OpTypeFloat into a float scalar.
func (p *parser) parseTypeFloat(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	widthWord, err := p.next()
	if err != nil {
		return err
	}
	width, err := mapWidth(widthWord)
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	handle := p.insertType(dec.name, ir.ScalarType{Kind: ir.ScalarFloat, Width: width})
	p.lookupType[id] = lookupType{handle: handle}
	return nil
}

// parseTypeVector parses OpTypeVector into a vector of a scalar type.
func (p *parser) parseTypeVector(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	base, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	scalar, ok := p.module.Types[base.handle].Inner.(ir.ScalarType)
	if !ok {
		return errInvalidInnerType(typeID)
	}
	componentCount, err := p.next()
	if err != nil {
		return err
	}
	size, err := mapVectorSize(componentCount)
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	handle := p.insertType(dec.name, ir.VectorType{Size: size, Scalar: scalar})
	p.lookupType[id] = lookupType{handle: handle, baseID: typeID, hasBase: true}
	return nil
}

// parseTypeMatrix parses OpTypeMatrix into a matrix of a vector's rows.
func (p *parser) parseTypeMatrix(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	vectorTypeID, err := p.next()
	if err != nil {
		return err
	}
	numColumns, err := p.next()
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	vectorLookup, err := p.typeLookup(vectorTypeID)
	if err != nil {
		return err
	}
	vector, ok := p.module.Types[vectorLookup.handle].Inner.(ir.VectorType)
	if !ok {
		return errInvalidInnerType(vectorTypeID)
	}
	columns, err := mapVectorSize(numColumns)
	if err != nil {
		return err
	}
	handle := p.insertType(dec.name, ir.MatrixType{Columns: columns, Rows: vector.Size, Scalar: vector.Scalar})
	p.lookupType[id] = lookupType{handle: handle, baseID: vectorTypeID, hasBase: true}
	return nil
}

// parseTypeFunction records an OpTypeFunction; functions have no IR type.
func (p *parser) parseTypeFunction(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	returnTypeID, err := p.next()
	if err != nil {
		return err
	}
	parameters := make([]uint32, 0, inst.wc-3)
	for range inst.wc - 3 {
		parameterID, err := p.next()
		if err != nil {
			return err
		}
		parameters = append(parameters, parameterID)
	}
	p.lookupFunctionType[id] = lookupFunctionType{parameterTypeIDs: parameters, returnTypeID: returnTypeID}
	return nil
}

// parseTypePointer parses OpTypePointer. A pointer in the Handle space is
// dropped: the handle type stands for it, as in Rust naga.
func (p *parser) parseTypePointer(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	storageClass, err := p.next()
	if err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	base, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	baseInner := p.module.Types[base.handle].Inner

	space, spaceKnown := pointerSpace(baseInner)
	if !spaceKnown {
		// A struct decorated BufferBlock is the legacy spelling of a storage
		// buffer; its access was collected while the struct was parsed.
		if _, isStorageBuffer := p.lookupStorageBufferTypes[base.handle]; isStorageBuffer {
			space, spaceKnown = ir.SpaceStorage, true
		}
	}
	if !spaceKnown {
		extClass, err := mapStorageClass(storageClass)
		if err != nil {
			return err
		}
		switch extClass.class {
		case classGlobal:
			space = extClass.space
		default:
			space = ir.SpacePrivate
		}
	}

	// Runtime-sized arrays live only in Storage: naga IR has no other space
	// for them (Rust's UnsupportedRuntimeArrayStorageClass).
	if array, ok := baseInner.(ir.ArrayType); ok && array.Size.Constant == nil && array.Size.Pending == nil {
		if space != ir.SpaceStorage {
			return errUnsupportedRuntimeArrayStorageClass
		}
	}

	lookup := base
	if space != ir.SpaceHandle {
		// A pointer is one level deeper than its base, so a chain of pointers
		// is bounded like a chain of arrays.
		pointer := ir.PointerType{Base: base.handle, Space: space}
		if err := p.checkTypeDepth(pointer); err != nil {
			return err
		}
		lookup = lookupType{
			handle:  p.insertType(dec.name, pointer),
			baseID:  typeID,
			hasBase: true,
		}
	}
	p.lookupType[id] = lookup
	return nil
}

// parseTypeArray parses OpTypeArray. An array of an image or a sampler is made
// a binding array: SPIR-V tells the two apart by usage, not by type (Rust naga
// makes the same assumption).
func (p *parser) parseTypeArray(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(4); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	lengthID, err := p.next()
	if err != nil {
		return err
	}
	length, err := p.constantArraySize(lengthID)
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	base, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	if err := p.updateLayouter(); err != nil {
		return err
	}

	baseInner := p.module.Types[base.handle].Inner
	var inner ir.TypeInner
	if imageOrSampler(baseInner) {
		inner = ir.BindingArrayType{Base: base.handle, Size: &length}
	} else {
		stride := dec.arrayStride
		if !dec.hasArrayStride {
			stride = p.strideOf(base.handle)
		}
		inner = ir.ArrayType{Base: base.handle, Size: ir.ArraySize{Constant: &length}, Stride: stride}
	}
	if err := p.checkTypeDepth(inner); err != nil {
		return err
	}
	handle := p.insertType(dec.name, inner)
	p.lookupType[id] = lookupType{handle: handle, baseID: typeID, hasBase: true}
	return nil
}

// parseTypeRuntimeArray parses OpTypeRuntimeArray into a runtime-sized array,
// or a binding array of images or samplers.
func (p *parser) parseTypeRuntimeArray(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	base, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	if err := p.updateLayouter(); err != nil {
		return err
	}

	baseInner := p.module.Types[base.handle].Inner
	var inner ir.TypeInner
	if imageOrSampler(baseInner) {
		inner = ir.BindingArrayType{Base: base.handle}
	} else {
		stride := dec.arrayStride
		if !dec.hasArrayStride {
			stride = p.strideOf(base.handle)
		}
		inner = ir.ArrayType{Base: base.handle, Stride: stride}
	}
	if err := p.checkTypeDepth(inner); err != nil {
		return err
	}
	handle := p.insertType(dec.name, inner)
	p.lookupType[id] = lookupType{handle: handle, baseID: typeID, hasBase: true}
	return nil
}

// parseTypeStruct parses OpTypeStruct: its members' offsets come from Offset
// decorations, or from the layout when a member has none.
func (p *parser) parseTypeStruct(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(2); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	parentDecor := p.takeDecoration(id)
	isStorageBuffer := parentDecor.storageBuffer
	if err := p.updateLayouter(); err != nil {
		return err
	}

	memberCount := uint32(inst.wc) - 2
	members := make([]ir.StructMember, 0, memberCount)
	memberLookups := make([]lookupMember, 0, memberCount)
	var storageAccess ir.StorageAccess
	var span, alignment uint32
	for i := range memberCount {
		typeID, err := p.next()
		if err != nil {
			return err
		}
		ty, err := p.typeLookup(typeID)
		if err != nil {
			return err
		}
		dec := p.takeMemberDecoration(id, i)

		storageAccess |= dec.storageAccess()
		memberLookups = append(memberLookups, lookupMember{
			typeID:   typeID,
			rowMajor: dec.rowMajor != nil && *dec.rowMajor,
		})

		layout := p.layouter.Layout(ty.handle)
		span = roundUp(span, layout.Alignment)
		alignment = max(alignment, layout.Alignment)

		binding, _ := dec.ioBinding()
		if dec.hasOffset {
			span = dec.offset
		}
		offset := span
		span += layout.Size

		if matrix, ok := p.module.Types[ty.handle].Inner.(ir.MatrixType); ok && dec.hasMatrixStride {
			// Rust naga's Alignment::from(rows): a vec3 row is aligned like a
			// vec4, so a 3-row matrix has a stride of 4 columns.
			expected := rowAlignment(matrix.Rows) * uint32(matrix.Scalar.Width)
			if dec.matrixStride != expected {
				return errUnsupportedMatrixStride(dec.matrixStride, uint8(matrix.Columns), uint8(matrix.Rows), matrix.Scalar.Width)
			}
		}

		member := ir.StructMember{Name: dec.name, Type: ty.handle, Offset: offset}
		if binding != nil {
			member.Binding = &binding
		}
		members = append(members, member)
	}
	span = roundUp(span, alignment)

	inner := ir.StructType{Members: members, Span: span}
	if err := p.checkTypeDepth(inner); err != nil {
		return err
	}
	handle := p.insertType(parentDecor.name, inner)
	if isStorageBuffer {
		p.lookupStorageBufferTypes[handle] = storageAccess
	}
	for i, member := range memberLookups {
		p.lookupMember[lookupMemberKey{handle: handle, member: uint32(i)}] = member
	}
	p.lookupType[id] = lookupType{handle: handle}
	return nil
}

// parseTypeImage parses OpTypeImage into an image type.
func (p *parser) parseTypeImage(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(9); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	sampleTypeID, err := p.next()
	if err != nil {
		return err
	}
	dimWord, err := p.next()
	if err != nil {
		return err
	}
	isDepth, err := p.next()
	if err != nil {
		return err
	}
	isArray, err := p.next()
	if err != nil {
		return err
	}
	isMSAA, err := p.next()
	if err != nil {
		return err
	}
	isSampled, err := p.next()
	if err != nil {
		return err
	}
	format, err := p.next()
	if err != nil {
		return err
	}

	dim, err := mapImageDim(dimWord)
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)

	// Ensure there is a type for texture coordinates without extra
	// components, as Rust naga does.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	switch dim {
	case ir.Dim1D:
		p.insertType("", f32)
	case ir.Dim2D:
		p.insertType("", ir.VectorType{Size: ir.Vec2, Scalar: f32})
	case ir.Dim3D, ir.DimCube:
		p.insertType("", ir.VectorType{Size: ir.Vec3, Scalar: f32})
	}

	base, err := p.typeLookup(sampleTypeID)
	if err != nil {
		return err
	}
	kind, ok := scalarKind(p.module.Types[base.handle].Inner)
	if !ok {
		return errInvalidImageBaseType(base.handle)
	}

	image := ir.ImageType{
		Dim:     dim,
		Arrayed: isArray != 0,
	}
	switch {
	case isDepth == 1:
		if isSampled == 2 {
			return errInvalidImageDepthStorage
		}
		image.Class = ir.ImageClassDepth
		image.Multisampled = isMSAA != 0
	case isSampled == 2 && format == 0:
		return errInvalidStorageImageWithoutFormat
	case format != 0 && (isSampled == 0 || isSampled == 2):
		image.Class = ir.ImageClassStorage
		// A SPIR-V image type carries no access; the variable's NonReadable /
		// NonWritable decorations do, and parseGlobalVariable copies the type
		// with them applied. The declared type is never used as it stands, and
		// the IR's validator rejects an unset access (ADR-010 E2), so it says
		// what SPIR-V means: either.
		image.StorageAccess = ir.StorageAccessLoad | ir.StorageAccessStore
		if image.StorageFormat, err = mapImageFormat(format); err != nil {
			return err
		}
	default:
		image.Class = ir.ImageClassSampled
		image.SampledKind = kind
		image.Multisampled = isMSAA != 0
	}
	handle := p.insertType(dec.name, image)
	p.lookupType[id] = lookupType{handle: handle, baseID: sampleTypeID, hasBase: true}
	return nil
}

// parseTypeSampledImage maps the sampled image type to its image type: the
// pair is produced by OpSampledImage at use sites.
func (p *parser) parseTypeSampledImage(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	image, err := p.typeLookup(imageID)
	if err != nil {
		return err
	}
	p.lookupType[id] = lookupType{handle: image.handle, baseID: imageID, hasBase: true}
	return nil
}

// parseTypeSampler parses OpTypeSampler. The comparison property is applied
// later, from how the sampler is used.
func (p *parser) parseTypeSampler(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(2); err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	dec := p.takeDecoration(id)
	handle := p.insertType(dec.name, ir.SamplerType{Comparison: false})
	p.lookupType[id] = lookupType{handle: handle}
	return nil
}

// pointerSpace returns the address space of a pointer type, as Rust naga's
// TypeInner::pointer_space.
func pointerSpace(inner ir.TypeInner) (ir.AddressSpace, bool) {
	switch t := inner.(type) {
	case ir.PointerType:
		return t.Space, true
	case ir.ValuePointerType:
		return t.Space, true
	}
	return 0, false
}

// imageOrSampler reports whether inner is an image or a sampler type, the
// element kinds SPIR-V arrays turn into binding arrays.
func imageOrSampler(inner ir.TypeInner) bool {
	switch inner.(type) {
	case ir.ImageType, ir.SamplerType:
		return true
	}
	return false
}

// scalarKind returns the scalar kind of a scalar, vector or matrix type; an
// image's sampled type must be one of them.
func scalarKind(inner ir.TypeInner) (ir.ScalarKind, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind, true
	case ir.VectorType:
		return t.Scalar.Kind, true
	case ir.MatrixType:
		return t.Scalar.Kind, true
	}
	return 0, false
}

// scalarOf returns the scalar type of a scalar, vector or matrix type.
func scalarOf(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	}
	return ir.ScalarType{}, false
}

// strideOf returns the array stride the layouter gives the element type: its
// size rounded up to its alignment.
func (p *parser) strideOf(base ir.TypeHandle) uint32 {
	layout := p.layouter.Layout(base)
	return roundUp(layout.Size, layout.Alignment)
}

// rowAlignment returns the alignment in components of a matrix's rows, as
// Rust naga's Alignment::from(VectorSize): a vec2 row aligns to 2, a vec3 or
// vec4 row to 4.
func rowAlignment(rows ir.VectorSize) uint32 {
	switch rows {
	case ir.Vec2:
		return 2
	case ir.Vec3, ir.Vec4:
		return 4
	}
	return 1
}

// roundUp rounds value up to the next multiple of alignment, which is a power
// of two here as every IR alignment is.
func roundUp(value, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return (value + alignment - 1) / alignment * alignment
}

// mapWidth converts a SPIR-V bit width to bytes, as Rust naga's map_width.
func mapWidth(word uint32) (uint8, error) {
	width := word >> 3
	if width > 0xff {
		return 0, errInvalidTypeWidth(word)
	}
	return uint8(width), nil
}

// mapVectorSize converts a SPIR-V component count to a vector size.
func mapVectorSize(word uint32) (ir.VectorSize, error) {
	switch word {
	case 2:
		return ir.Vec2, nil
	case 3:
		return ir.Vec3, nil
	case 4:
		return ir.Vec4, nil
	}
	return 0, errInvalidVectorSize(word)
}

// mapImageDim converts a SPIR-V Dim to an image dimension.
func mapImageDim(word uint32) (ir.ImageDimension, error) {
	switch spvenums.Dim(word) {
	case spvenums.Dim1D:
		return ir.Dim1D, nil
	case spvenums.Dim2D:
		return ir.Dim2D, nil
	case spvenums.Dim3D:
		return ir.Dim3D, nil
	case spvenums.DimCube:
		return ir.DimCube, nil
	}
	return 0, errUnsupportedImageDim(word)
}

// mapImageFormat converts a SPIR-V ImageFormat to a storage format, as Rust
// naga's map_image_format.
func mapImageFormat(word uint32) (ir.StorageFormat, error) {
	switch spvenums.ImageFormat(word) {
	case spvenums.ImageFormatR8:
		return ir.StorageFormatR8Unorm, nil
	case spvenums.ImageFormatR8Snorm:
		return ir.StorageFormatR8Snorm, nil
	case spvenums.ImageFormatR8ui:
		return ir.StorageFormatR8Uint, nil
	case spvenums.ImageFormatR8i:
		return ir.StorageFormatR8Sint, nil
	case spvenums.ImageFormatR16:
		return ir.StorageFormatR16Unorm, nil
	case spvenums.ImageFormatR16Snorm:
		return ir.StorageFormatR16Snorm, nil
	case spvenums.ImageFormatR16ui:
		return ir.StorageFormatR16Uint, nil
	case spvenums.ImageFormatR16i:
		return ir.StorageFormatR16Sint, nil
	case spvenums.ImageFormatR16f:
		return ir.StorageFormatR16Float, nil
	case spvenums.ImageFormatRg8:
		return ir.StorageFormatRg8Unorm, nil
	case spvenums.ImageFormatRg8Snorm:
		return ir.StorageFormatRg8Snorm, nil
	case spvenums.ImageFormatRg8ui:
		return ir.StorageFormatRg8Uint, nil
	case spvenums.ImageFormatRg8i:
		return ir.StorageFormatRg8Sint, nil
	case spvenums.ImageFormatR32ui:
		return ir.StorageFormatR32Uint, nil
	case spvenums.ImageFormatR32i:
		return ir.StorageFormatR32Sint, nil
	case spvenums.ImageFormatR32f:
		return ir.StorageFormatR32Float, nil
	case spvenums.ImageFormatRg16:
		return ir.StorageFormatRg16Unorm, nil
	case spvenums.ImageFormatRg16Snorm:
		return ir.StorageFormatRg16Snorm, nil
	case spvenums.ImageFormatRg16ui:
		return ir.StorageFormatRg16Uint, nil
	case spvenums.ImageFormatRg16i:
		return ir.StorageFormatRg16Sint, nil
	case spvenums.ImageFormatRg16f:
		return ir.StorageFormatRg16Float, nil
	case spvenums.ImageFormatRgba8:
		return ir.StorageFormatRgba8Unorm, nil
	case spvenums.ImageFormatRgba8Snorm:
		return ir.StorageFormatRgba8Snorm, nil
	case spvenums.ImageFormatRgba8ui:
		return ir.StorageFormatRgba8Uint, nil
	case spvenums.ImageFormatRgba8i:
		return ir.StorageFormatRgba8Sint, nil
	case spvenums.ImageFormatRgb10a2ui:
		return ir.StorageFormatRgb10a2Uint, nil
	case spvenums.ImageFormatRgb10A2:
		return ir.StorageFormatRgb10a2Unorm, nil
	case spvenums.ImageFormatR11fG11fB10f:
		return ir.StorageFormatRg11b10Ufloat, nil
	case spvenums.ImageFormatR64ui:
		return ir.StorageFormatR64Uint, nil
	case spvenums.ImageFormatRg32ui:
		return ir.StorageFormatRg32Uint, nil
	case spvenums.ImageFormatRg32i:
		return ir.StorageFormatRg32Sint, nil
	case spvenums.ImageFormatRg32f:
		return ir.StorageFormatRg32Float, nil
	case spvenums.ImageFormatRgba16:
		return ir.StorageFormatRgba16Unorm, nil
	case spvenums.ImageFormatRgba16Snorm:
		return ir.StorageFormatRgba16Snorm, nil
	case spvenums.ImageFormatRgba16ui:
		return ir.StorageFormatRgba16Uint, nil
	case spvenums.ImageFormatRgba16i:
		return ir.StorageFormatRgba16Sint, nil
	case spvenums.ImageFormatRgba16f:
		return ir.StorageFormatRgba16Float, nil
	case spvenums.ImageFormatRgba32ui:
		return ir.StorageFormatRgba32Uint, nil
	case spvenums.ImageFormatRgba32i:
		return ir.StorageFormatRgba32Sint, nil
	case spvenums.ImageFormatRgba32f:
		return ir.StorageFormatRgba32Float, nil
	}
	return 0, errUnsupportedImageFormat(word)
}

// mapStorageClass maps a SPIR-V storage class to where the variable goes.
func mapStorageClass(word uint32) (extendedClass, error) {
	switch spvenums.StorageClass(word) {
	case spvenums.StorageClassFunction:
		return extendedClass{class: classGlobal, space: ir.SpaceFunction}, nil
	case spvenums.StorageClassInput:
		return extendedClass{class: classInput}, nil
	case spvenums.StorageClassOutput:
		return extendedClass{class: classOutput}, nil
	case spvenums.StorageClassPrivate:
		return extendedClass{class: classGlobal, space: ir.SpacePrivate}, nil
	case spvenums.StorageClassUniformConstant:
		return extendedClass{class: classGlobal, space: ir.SpaceHandle}, nil
	case spvenums.StorageClassStorageBuffer:
		return extendedClass{class: classGlobal, space: ir.SpaceStorage}, nil
	case spvenums.StorageClassUniform:
		return extendedClass{class: classGlobal, space: ir.SpaceUniform}, nil
	case spvenums.StorageClassWorkgroup:
		return extendedClass{class: classGlobal, space: ir.SpaceWorkGroup}, nil
	case spvenums.StorageClassPushConstant:
		return extendedClass{class: classGlobal, space: ir.SpaceImmediate}, nil
	}
	return extendedClass{}, errUnsupportedStorageClass(word)
}
