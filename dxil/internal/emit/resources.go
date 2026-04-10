package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// Resource handling for DXIL emission.
//
// This file handles CBV (constant buffer views), SRV (shader resource views),
// sampler bindings, and UAV stubs via dx.op intrinsics.
//
// Reference: Mesa nir_to_dxil.c emit_resources(), emit_createhandle_call_pre_6_6()

// dx.op function name for resource handle creation.
const dxOpCreateHandleName = "dx.op.createHandle"

// DXIL resource classes (matches DXIL spec D3D12_SHADER_INPUT_BIND_DESC).
const (
	resourceClassSRV     uint8 = 0
	resourceClassUAV     uint8 = 1
	resourceClassCBV     uint8 = 2
	resourceClassSampler uint8 = 3
)

// resourceInfo describes a single resource binding discovered during analysis.
type resourceInfo struct {
	varHandle  ir.GlobalVariableHandle
	name       string
	class      uint8 // resourceClassSRV, resourceClassUAV, resourceClassCBV, resourceClassSampler
	rangeID    int
	group      uint32
	binding    uint32
	typeHandle ir.TypeHandle
	handleID   int // emitter value ID of the created handle (-1 if not yet created)
}

// analyzeResources scans the module's global variables and classifies them
// into resource categories. Must be called before emitting function bodies.
func (e *Emitter) analyzeResources() {
	e.resources = nil
	e.resourceHandles = make(map[ir.GlobalVariableHandle]int)

	// Track per-class range IDs.
	rangeCounters := [4]int{} // SRV, UAV, CBV, Sampler

	for i := range e.ir.GlobalVariables {
		gv := &e.ir.GlobalVariables[i]

		class, ok := e.classifyGlobalVariable(gv)
		if !ok {
			continue
		}

		// Push constants / immediate data may not have explicit bindings.
		// Create a synthetic binding for them so they can be accessed as CBV.
		if gv.Binding == nil {
			if gv.Space == ir.SpacePushConstant || gv.Space == ir.SpaceImmediate {
				// Assign to group=0, binding=nextCBV to avoid conflict.
				gv.Binding = &ir.ResourceBinding{
					Group:   0,
					Binding: uint32(rangeCounters[resourceClassCBV]), //nolint:gosec // range counter is small
				}
			} else {
				continue
			}
		}

		rangeID := rangeCounters[class]
		rangeCounters[class]++

		info := resourceInfo{
			varHandle:  ir.GlobalVariableHandle(uint32(i)),
			name:       gv.Name,
			class:      class,
			rangeID:    rangeID,
			group:      gv.Binding.Group,
			binding:    gv.Binding.Binding,
			typeHandle: gv.Type,
			handleID:   -1,
		}

		e.resourceHandles[info.varHandle] = len(e.resources)
		e.resources = append(e.resources, info)
	}
}

// classifyGlobalVariable determines the resource class of a global variable.
// Returns the class and true if it is a resource; false if it is not.
func (e *Emitter) classifyGlobalVariable(gv *ir.GlobalVariable) (uint8, bool) {
	switch gv.Space {
	case ir.SpaceUniform, ir.SpacePushConstant, ir.SpaceImmediate:
		// Uniform, push constant, and immediate data are all accessed as CBV in DXIL.
		// Push constants map to a constant buffer (b-register).
		return resourceClassCBV, true

	case ir.SpaceStorage:
		return resourceClassUAV, true

	case ir.SpaceHandle:
		// Handle space: classify by the pointed-to type.
		if int(gv.Type) < len(e.ir.Types) {
			inner := e.ir.Types[gv.Type].Inner
			switch inner.(type) {
			case ir.ImageType:
				return resourceClassSRV, true
			case ir.SamplerType:
				return resourceClassSampler, true
			}
		}
		return 0, false

	default:
		return 0, false
	}
}

// emitResourceHandles creates dx.op.createHandle calls for all analyzed resources.
// Must be called at function entry before any resource accesses.
//
// dx.op.createHandle(i32 57, i8 class, i32 rangeID, i32 index, i1 false)
func (e *Emitter) emitResourceHandles() {
	if len(e.resources) == 0 {
		return
	}

	handleTy := e.getDxHandleType()
	createFn := e.getDxOpCreateHandleFunc()

	for i := range e.resources {
		res := &e.resources[i]

		opcodeVal := e.getIntConstID(int64(OpCreateHandle))
		classVal := e.getI8ConstID(int64(res.class))
		rangeIDVal := e.getIntConstID(int64(res.rangeID))
		indexVal := e.getIntConstID(int64(res.binding))
		nonUniformVal := e.getI1ConstID(0) // false

		handleID := e.addCallInstr(createFn, handleTy,
			[]int{opcodeVal, classVal, rangeIDVal, indexVal, nonUniformVal})

		res.handleID = handleID
	}
}

// getResourceHandleID returns the emitter value ID of the handle for a
// given global variable, or -1 if not a resource.
func (e *Emitter) getResourceHandleID(varHandle ir.GlobalVariableHandle) (int, bool) {
	idx, ok := e.resourceHandles[varHandle]
	if !ok {
		return -1, false
	}
	return e.resources[idx].handleID, true
}

// --- Texture sampling ---

// emitImageSample emits a dx.op.sample call for texture sampling.
func (e *Emitter) emitImageSample(fn *ir.Function, sample ir.ExprImageSample) (int, error) {
	// Resolve image and sampler handles.
	imageHandleID, err := e.resolveResourceHandle(fn, sample.Image)
	if err != nil {
		return 0, fmt.Errorf("image handle: %w", err)
	}
	samplerHandleID, err := e.resolveResourceHandle(fn, sample.Sampler)
	if err != nil {
		return 0, fmt.Errorf("sampler handle: %w", err)
	}

	// Emit coordinate expression.
	if _, err := e.emitExpression(fn, sample.Coordinate); err != nil {
		return 0, fmt.Errorf("coordinate: %w", err)
	}

	// Determine coordinate dimensionality.
	coordType := e.resolveExprType(fn, sample.Coordinate)
	coordComps := componentCount(coordType)

	// Determine the overload from the image type.
	ol := overloadF32 // textures almost always return f32

	resRetTy := e.getDxResRetType(ol)
	sampleFn := e.getDxOpSampleFunc(ol)

	// Build operands: opcode, texHandle, samplerHandle, coord0..3, offset0..2, clamp, undef
	// Total 12 operands for sample.
	undefVal := e.getUndefConstID()
	zeroF := e.getFloatConstID(0.0)

	opcodeVal := e.getIntConstID(int64(OpSample))

	// Scalarize coordinates into coord0..coord3.
	coords := [4]int{zeroF, zeroF, zeroF, zeroF}
	for i := 0; i < 4 && i < coordComps; i++ {
		coords[i] = e.getComponentID(sample.Coordinate, i)
	}

	operands := []int{
		opcodeVal,                                  // i32 opcode
		imageHandleID,                              // %handle texture
		samplerHandleID,                            // %handle sampler
		coords[0], coords[1], coords[2], coords[3], // coord0..3
		zeroF, zeroF, zeroF, // offset0..2
		zeroF,    // clamp
		undefVal, // undef
	}

	retID := e.addCallInstr(sampleFn, resRetTy, operands)

	// Extract 4 components from the result and store as pending components.
	scalarTy := e.overloadReturnType(ol)
	comps := make([]int, 4)
	for i := 0; i < 4; i++ {
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, i},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}
	e.pendingComponents = comps

	return comps[0], nil
}

// resolveResourceHandle evaluates the expression and returns the resource
// handle value ID. The expression must be an ExprGlobalVariable that was
// classified as a resource.
func (e *Emitter) resolveResourceHandle(fn *ir.Function, exprHandle ir.ExpressionHandle) (int, error) {
	expr := fn.Expressions[exprHandle]
	if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
		if handleID, found := e.getResourceHandleID(gv.Variable); found {
			return handleID, nil
		}
		return 0, fmt.Errorf("global variable %d is not a resource", gv.Variable)
	}
	// Fallback: emit the expression and hope it resolves.
	return e.emitExpression(fn, exprHandle)
}

// --- dx.op function declarations and DXIL types ---

// getDxHandleType returns the %dx.types.Handle opaque struct type.
func (e *Emitter) getDxHandleType() *module.Type {
	if e.dxHandleType != nil {
		return e.dxHandleType
	}
	// %dx.types.Handle = type { i8* }
	i8PtrTy := e.mod.GetPointerType(e.mod.GetIntType(8))
	e.dxHandleType = e.mod.GetStructType("dx.types.Handle", []*module.Type{i8PtrTy})
	return e.dxHandleType
}

// getDxResRetType returns the %dx.types.ResRet.XX struct type.
func (e *Emitter) getDxResRetType(ol overloadType) *module.Type {
	scalarTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)
	name := "dx.types.ResRet" + overloadSuffix(ol)
	// ResRet has 4 scalar components + 1 i32 status.
	return e.mod.GetStructType(name, []*module.Type{scalarTy, scalarTy, scalarTy, scalarTy, i32Ty})
}

// getDxOpCreateHandleFunc creates the dx.op.createHandle function declaration.
// Signature: %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1)
func (e *Emitter) getDxOpCreateHandleFunc() *module.Function {
	key := dxOpKey{name: dxOpCreateHandleName, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	i1Ty := e.mod.GetIntType(1)

	params := []*module.Type{i32Ty, i8Ty, i32Ty, i32Ty, i1Ty}
	funcTy := e.mod.GetFunctionType(handleTy, params)
	fn := e.mod.AddFunction(dxOpCreateHandleName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpSampleFunc creates the dx.op.sample function declaration.
// Signature: %dx.types.ResRet.XX @dx.op.sample.XX(
//
//	i32 opcode, %handle tex, %handle sampler,
//	float coord0..3, float offset0..2, float clamp, i32 undef)
func (e *Emitter) getDxOpSampleFunc(ol overloadType) *module.Function {
	name := "dx.op.sample"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	resRetTy := e.getDxResRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{
		i32Ty,                      // opcode
		handleTy,                   // texture handle
		handleTy,                   // sampler handle
		f32Ty, f32Ty, f32Ty, f32Ty, // coord0..3
		f32Ty, f32Ty, f32Ty, // offset0..2
		f32Ty, // clamp
		i32Ty, // undef
	}
	funcTy := e.mod.GetFunctionType(resRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getI1ConstID returns the emitter value ID for a cached i1 constant.
func (e *Emitter) getI1ConstID(v int64) int {
	key := v + (1 << 41) // offset to distinguish from i32 and i8
	if id, ok := e.intConsts[key]; ok {
		return id
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(1), v)
	id := e.allocValue()
	e.intConsts[key] = id
	e.constMap[id] = c
	return id
}

// --- dx.resources metadata ---

// emitResourceMetadata emits the !dx.resources named metadata node.
// Called from emitMetadata when resources are present.
//
// Format: !dx.resources = !{!srvs, !uavs, !cbvs, !samplers}
// Each class has different field counts matching the DXIL spec.
//
// Reference: Mesa nir_to_dxil.c emit_srv_metadata/emit_uav_metadata/emit_cbv_metadata/emit_sampler_metadata
func (e *Emitter) emitResourceMetadata() *module.MetadataNode {
	if len(e.resources) == 0 {
		return nil
	}

	// Group resources by class.
	var srvs, uavs, cbvs, samplers []*module.MetadataNode

	for i := range e.resources {
		res := &e.resources[i]

		switch res.class {
		case resourceClassSRV:
			srvs = append(srvs, e.buildSRVMetadata(res))
		case resourceClassUAV:
			uavs = append(uavs, e.buildUAVMetadata(res))
		case resourceClassCBV:
			cbvs = append(cbvs, e.buildCBVMetadata(res))
		case resourceClassSampler:
			samplers = append(samplers, e.buildSamplerMetadata(res))
		}
	}

	// Build the 4-element tuple: !{!srvs, !uavs, !cbvs, !samplers}
	// Each element is either a tuple of entries or null.
	elements := [4][]*module.MetadataNode{srvs, uavs, cbvs, samplers}
	mdParts := make([]*module.MetadataNode, 4)
	for i, list := range elements {
		if len(list) > 0 {
			mdParts[i] = e.mod.AddMetadataTuple(list)
		}
		// nil represents null in metadata
	}

	mdResources := e.mod.AddMetadataTuple(mdParts)
	e.mod.AddNamedMetadata("dx.resources", []*module.MetadataNode{mdResources})

	return mdResources
}

// fillResourceMetadataCommon builds the first 6 fields common to all resource classes.
// Returns [6]*MetadataNode: {rangeID, undefPtr, name, space, lowerBound, rangeSize}.
//
// Reference: Mesa nir_to_dxil.c fill_resource_metadata() line ~453
func (e *Emitter) fillResourceMetadataCommon(res *resourceInfo, structType *module.Type) [6]*module.MetadataNode {
	i32Ty := e.mod.GetIntType(32)

	// fields[1] = metadata value wrapping an undef pointer to the resource struct type.
	// DXC validator requires this non-null reference.
	// Reference: Mesa fill_resource_metadata() line ~457-458
	pointerType := e.mod.GetPointerType(structType)
	pointerUndef := e.mod.AddUndefConst(pointerType)

	return [6]*module.MetadataNode{
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.rangeID))), // fields[0]: resource ID
		e.mod.AddMetadataValue(pointerType, pointerUndef),                // fields[1]: global constant symbol (undef ptr)
		e.mod.AddMetadataString(res.name),                                // fields[2]: name
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.group))),   // fields[3]: space ID
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.binding))), // fields[4]: lower bound
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(1)),                  // fields[5]: range size (1 for non-array)
	}
}

// getResourceStructType returns an LLVM struct type appropriate for this resource.
// For CBV: named struct wrapping a float array sized to the buffer.
// For SRV/UAV: the dx.types.Handle-like struct or image type struct.
// For Sampler: struct.SamplerState { i32 }.
//
// Reference: Mesa nir_to_dxil.c emit_cbv() line ~1549-1552, emit_sampler_metadata() line ~1597-1598
func (e *Emitter) getResourceStructType(res *resourceInfo) *module.Type {
	switch res.class {
	case resourceClassCBV:
		// CBV struct: named struct containing a float array.
		// Mesa: buffer_type = struct { float[size] } where size = num vec4 regs.
		numVec4 := e.computeCBVVec4Count(res)
		f32Ty := e.mod.GetFloatType(32)
		arrayTy := e.mod.GetArrayType(f32Ty, uint(numVec4)) //nolint:gosec // numVec4 always positive
		name := res.name
		if name == "" {
			name = "cb"
		}
		return e.mod.GetStructType(name, []*module.Type{arrayTy})

	case resourceClassSampler:
		// Sampler: struct.SamplerState { i32 }
		// Reference: Mesa nir_to_dxil.c line ~1597-1598
		i32Ty := e.mod.GetIntType(32)
		return e.mod.GetStructType("struct.SamplerState", []*module.Type{i32Ty})

	case resourceClassSRV:
		// SRV: use the image component type to build a typed resource struct.
		compType := e.getResourceComponentType(res)
		scalarTy := e.componentTypeToLLVMType(compType)
		return e.mod.GetStructType("struct.SRVType", []*module.Type{scalarTy})

	case resourceClassUAV:
		// UAV: similar to SRV struct for metadata purposes.
		compType := e.getResourceComponentType(res)
		scalarTy := e.componentTypeToLLVMType(compType)
		return e.mod.GetStructType("struct.UAVType", []*module.Type{scalarTy})

	default:
		// Fallback: i8 pointer struct.
		i8Ty := e.mod.GetIntType(8)
		return e.mod.GetStructType("struct.Resource", []*module.Type{i8Ty})
	}
}

// buildSRVMetadata builds SRV metadata entry with 9 fields.
//
// Reference: Mesa nir_to_dxil.c emit_srv_metadata() line ~468-492
func (e *Emitter) buildSRVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	resKind := e.resourceKind(res)
	compType := e.getResourceComponentType(res)

	// fields[6] = resource shape (kind)
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(resKind)))

	// fields[7] = sample count (i1 false)
	// Reference: Mesa emit_srv_metadata() line ~480
	mdSampleCount := e.addMetadataI1(false)

	// fields[8] = element type tag metadata, or null for raw/structured buffers.
	// Reference: Mesa emit_srv_metadata() line ~481-489
	var mdTag *module.MetadataNode
	if resKind != 11 && resKind != 12 { // not RawBuffer(11) or StructuredBuffer(12)
		tagNodes := []*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)), // DXIL_TYPED_BUFFER_ELEMENT_TYPE_TAG = 0
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(compType))),
		}
		mdTag = e.mod.AddMetadataTuple(tagNodes)
	}
	// nil mdTag for raw buffer

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind,        // fields[6]: resource shape
		mdSampleCount, // fields[7]: sample count
		mdTag,         // fields[8]: element type tag (or null)
	}

	return e.mod.AddMetadataTuple(fields)
}

// buildUAVMetadata builds UAV metadata entry with 11 fields.
//
// Reference: Mesa nir_to_dxil.c emit_uav_metadata() line ~494-521
func (e *Emitter) buildUAVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	resKind := e.resourceKind(res)
	compType := e.getResourceComponentType(res)

	// fields[6] = resource shape
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(resKind)))

	// fields[7] = globally coherent (i1 false)
	// Reference: Mesa emit_uav_metadata() line ~507
	mdCoherent := e.addMetadataI1(false)

	// fields[8] = has counter (i1 false)
	mdCounter := e.addMetadataI1(false)

	// fields[9] = is ROV (i1 false)
	mdROV := e.addMetadataI1(false)

	// fields[10] = element type tag (or null for raw/structured buffer)
	// Reference: Mesa emit_uav_metadata() line ~510-518
	var mdTag *module.MetadataNode
	if resKind != 11 && resKind != 12 { // not RawBuffer or StructuredBuffer
		tagNodes := []*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)), // DXIL_TYPED_BUFFER_ELEMENT_TYPE_TAG = 0
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(compType))),
		}
		mdTag = e.mod.AddMetadataTuple(tagNodes)
	}

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind,     // fields[6]: resource shape
		mdCoherent, // fields[7]: globally coherent
		mdCounter,  // fields[8]: has counter
		mdROV,      // fields[9]: is ROV
		mdTag,      // fields[10]: element type tag (or null)
	}

	return e.mod.AddMetadataTuple(fields)
}

// buildCBVMetadata builds CBV metadata entry with 8 fields.
// fields[6] = constant buffer size in bytes (NOT resource kind).
//
// Reference: Mesa nir_to_dxil.c emit_cbv_metadata() line ~523-535
func (e *Emitter) buildCBVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	// fields[6] = constant buffer size in bytes.
	// Mesa: 4 * size (where size = number of vec4 registers).
	// Reference: Mesa emit_cbv() line ~1557: emit_cbv_metadata(..., 4 * size)
	cbvSizeBytes := e.computeCBVVec4Count(res) * 4 // each vec4 register = 4 floats = 16 bytes... wait
	// Actually Mesa passes 4 * size where size = num float elements in the array.
	// The array is float[size], so total = 4 * size bytes.
	// But our computeCBVVec4Count returns the array size (num floats).
	// So bytes = 4 * numFloats = 4 * computeCBVVec4Count.
	mdSize := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(cbvSizeBytes)))

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdSize, // fields[6]: constant buffer size in bytes
		nil,    // fields[7]: null (metadata)
	}

	return e.mod.AddMetadataTuple(fields)
}

// buildSamplerMetadata builds Sampler metadata entry with 8 fields.
//
// Reference: Mesa nir_to_dxil.c emit_sampler_metadata() line ~537-551
func (e *Emitter) buildSamplerMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	// fields[6] = sampler kind (0=default, 1=comparison).
	// Reference: Mesa emit_sampler_metadata() line ~545-547
	samplerKind := 0 // DXIL_SAMPLER_KIND_DEFAULT
	// TODO: detect comparison samplers from IR SamplerType if available.
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(samplerKind)))

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind, // fields[6]: sampler kind
		nil,    // fields[7]: null (metadata)
	}

	return e.mod.AddMetadataTuple(fields)
}

// addMetadataI1 creates a metadata value node wrapping an i1 boolean constant.
//
// Reference: Mesa dxil_module.c dxil_get_metadata_int1() line ~2897
func (e *Emitter) addMetadataI1(value bool) *module.MetadataNode { //nolint:unparam // value can be true for coherent UAVs
	i1Ty := e.mod.GetIntType(1)
	var v int64
	if value {
		v = 1
	}
	c := e.mod.AddIntConst(i1Ty, v)
	return e.mod.AddMetadataValue(i1Ty, c)
}

// computeCBVVec4Count returns the number of float elements in the CBV array
// representation. For the struct backing this CBV, we compute the total size
// in bytes and then convert to float count: numFloats = ceil(structSize / 4).
//
// Reference: Mesa nir_to_dxil.c emit_cbv() line ~1549-1550:
//
//	array_type = dxil_module_get_array_type(m, float32, size)
//	where size = number of float elements
func (e *Emitter) computeCBVVec4Count(res *resourceInfo) int {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return 4 // default: 1 vec4 register
	}
	sizeBytes := e.computeIRTypeSizeBytes(e.ir.Types[res.typeHandle].Inner)
	if sizeBytes == 0 {
		return 4 // default
	}
	// Round up to 16-byte boundary (cbuffer register size), then convert to float count.
	aligned := (sizeBytes + 15) &^ 15
	return int(aligned / 4)
}

// computeIRTypeSizeBytes computes the size in bytes of an IR type.
// Used for CBV buffer size calculation.
func (e *Emitter) computeIRTypeSizeBytes(inner ir.TypeInner) uint32 {
	switch t := inner.(type) {
	case ir.ScalarType:
		return uint32(t.Width)
	case ir.VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	case ir.MatrixType:
		// Each column is a vector of t.Rows components.
		colSize := uint32(t.Rows) * uint32(t.Scalar.Width)
		// Columns are aligned to 16 bytes in cbuffers.
		colAligned := (colSize + 15) &^ 15
		return uint32(t.Columns) * colAligned
	case ir.ArrayType:
		if int(t.Base) < len(e.ir.Types) {
			elemSize := e.computeIRTypeSizeBytes(e.ir.Types[t.Base].Inner)
			arrayLen := uint32(0)
			if t.Size.Constant != nil {
				arrayLen = *t.Size.Constant
			}
			// Use stride if available, otherwise compute from element size.
			if t.Stride > 0 {
				return arrayLen * t.Stride
			}
			return arrayLen * elemSize
		}
		return 0
	case ir.StructType:
		if len(t.Members) == 0 {
			return 0
		}
		// Use the last member's offset + size.
		lastMember := &t.Members[len(t.Members)-1]
		lastSize := uint32(0)
		if int(lastMember.Type) < len(e.ir.Types) {
			lastSize = e.computeIRTypeSizeBytes(e.ir.Types[lastMember.Type].Inner)
		}
		return lastMember.Offset + lastSize
	default:
		return 4 // fallback: assume 4 bytes
	}
}

// DXIL component type constants.
// Reference: Mesa dxil_enums.h enum dxil_component_type line ~170
const (
	dxilCompTypeF32 = 9  // DXIL_COMP_TYPE_F32
	dxilCompTypeI32 = 4  // DXIL_COMP_TYPE_I32
	dxilCompTypeU32 = 5  // DXIL_COMP_TYPE_U32
	dxilCompTypeF16 = 8  // DXIL_COMP_TYPE_F16
	dxilCompTypeF64 = 10 // DXIL_COMP_TYPE_F64
)

// getResourceComponentType returns the DXIL component type for a resource.
func (e *Emitter) getResourceComponentType(res *resourceInfo) int {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return dxilCompTypeF32 // default
	}
	inner := e.ir.Types[res.typeHandle].Inner
	if img, ok := inner.(ir.ImageType); ok {
		return e.sampledKindToDxilCompType(img.SampledKind)
	}
	return dxilCompTypeF32
}

// sampledKindToDxilCompType converts an IR ScalarKind to DXIL component type.
// Used for image/texture types where we only have the kind, not full scalar type.
func (e *Emitter) sampledKindToDxilCompType(kind ir.ScalarKind) int {
	switch kind {
	case ir.ScalarFloat:
		return dxilCompTypeF32
	case ir.ScalarSint:
		return dxilCompTypeI32
	case ir.ScalarUint:
		return dxilCompTypeU32
	default:
		return dxilCompTypeF32
	}
}

// componentTypeToLLVMType returns the LLVM type for a DXIL component type.
func (e *Emitter) componentTypeToLLVMType(compType int) *module.Type {
	switch compType {
	case dxilCompTypeF16:
		return e.mod.GetFloatType(16)
	case dxilCompTypeF64:
		return e.mod.GetFloatType(64)
	case dxilCompTypeI32:
		return e.mod.GetIntType(32)
	case dxilCompTypeU32:
		return e.mod.GetIntType(32)
	default:
		return e.mod.GetFloatType(32)
	}
}

// --- CBV (constant buffer) loads ---

// cbvPointerChain describes a resolved pointer chain leading to a CBV field.
type cbvPointerChain struct {
	varHandle  ir.GlobalVariableHandle
	byteOffset uint32        // accumulated byte offset into the struct (static part)
	fieldType  ir.TypeInner  // the IR type of the final accessed field
	scalar     ir.ScalarType // scalar element type for overload selection
	// dynIndexExpr and dynStride are set when the chain contains a dynamic Access.
	// The total byte offset = dynIndex * dynStride + byteOffset.
	dynIndexExpr ir.ExpressionHandle // dynamic array index expression (0 = no dynamic)
	dynStride    uint32              // array element stride in bytes (0 = no dynamic)
	hasDynIndex  bool                // true if this chain has a dynamic index component
}

// resolveCBVPointerChain walks an expression chain (AccessIndex → GlobalVariable)
// and determines whether it leads to a CBV global variable. If so, it returns
// the CBV pointer chain info including the accumulated byte offset.
//
// Supported patterns:
//   - ExprGlobalVariable (load entire CBV struct — offset 0)
//   - ExprAccessIndex → ExprGlobalVariable (load struct field)
//   - ExprAccessIndex → ExprAccessIndex → ExprGlobalVariable (nested struct/vector)
//
// Reference: Mesa nir_to_dxil.c emit_load_ubo_vec4() — offset is passed as
// register index (byte_offset / 16).
func (e *Emitter) resolveCBVPointerChain(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*cbvPointerChain, bool) {
	// Walk the expression chain to find the root global variable and accumulate offsets.
	var indices []uint32
	handle := ptrHandle
	var dynIndexExpr ir.ExpressionHandle
	var dynStride uint32
	hasDynIndex := false

	for {
		if int(handle) >= len(fn.Expressions) {
			return nil, false
		}
		expr := fn.Expressions[handle]

		switch ek := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			// Found root — check if it's a CBV.
			if _, found := e.resourceHandles[ek.Variable]; !found {
				return nil, false
			}
			idx := e.resourceHandles[ek.Variable]
			res := &e.resources[idx]
			if res.class != resourceClassCBV {
				return nil, false
			}

			// Now walk indices forward to compute byte offset.
			// When hasDynIndex is true, the dynamic Access already handles the
			// array dimension, so we start from the array's element type.
			byteOffset, fieldType, scalar, ok := e.computeCBVFieldOffset(ek.Variable, indices, hasDynIndex)
			if !ok {
				return nil, false
			}

			return &cbvPointerChain{
				varHandle:    ek.Variable,
				byteOffset:   byteOffset,
				fieldType:    fieldType,
				scalar:       scalar,
				dynIndexExpr: dynIndexExpr,
				dynStride:    dynStride,
				hasDynIndex:  hasDynIndex,
			}, true

		case ir.ExprAccessIndex:
			indices = append([]uint32{ek.Index}, indices...)
			handle = ek.Base

		case ir.ExprAccess:
			// Dynamic array access: we need the array stride to compute the offset.
			// The stride comes from the type of the base expression's inner array.
			if !hasDynIndex {
				// Only support one level of dynamic access.
				stride := e.resolveArrayStride(fn, ek.Base)
				if stride == 0 {
					return nil, false
				}
				dynIndexExpr = ek.Index
				dynStride = stride
				hasDynIndex = true
				handle = ek.Base
			} else {
				// Multiple dynamic accesses — not supported.
				return nil, false
			}

		default:
			return nil, false
		}
	}
}

// resolveArrayStride determines the array stride for a given expression's type.
// Used when computing dynamic CBV offsets for array[i] access patterns.
func (e *Emitter) resolveArrayStride(fn *ir.Function, exprHandle ir.ExpressionHandle) uint32 {
	if int(exprHandle) >= len(fn.Expressions) {
		return 0
	}
	expr := fn.Expressions[exprHandle]
	gv, ok := expr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return 0
	}
	if int(gv.Variable) >= len(e.ir.GlobalVariables) {
		return 0
	}
	gvDef := &e.ir.GlobalVariables[gv.Variable]
	if int(gvDef.Type) >= len(e.ir.Types) {
		return 0
	}
	inner := e.ir.Types[gvDef.Type].Inner
	if arr, ok := inner.(ir.ArrayType); ok {
		return arr.Stride
	}
	return 0
}

// resolveCBVRegIndex computes the CBV register index and component offset for
// a cbufferLoadLegacy call. Handles both static and dynamic (stride-based) indices.
// Returns (regIndexValueID, componentOffset, error).
func (e *Emitter) resolveCBVRegIndex(fn *ir.Function, chain *cbvPointerChain, scalarWidth uint32) (int, uint32, error) {
	if !chain.hasDynIndex {
		regIndex := chain.byteOffset / 16
		compOffset := (chain.byteOffset % 16) / scalarWidth
		return e.getIntConstID(int64(regIndex)), compOffset, nil
	}

	dynID, err := e.emitExpression(fn, chain.dynIndexExpr)
	if err != nil {
		return 0, 0, fmt.Errorf("CBV dynamic index: %w", err)
	}

	compOffset := (chain.byteOffset % 16) / scalarWidth
	i32Ty := e.mod.GetIntType(32)

	if chain.dynStride%16 == 0 {
		// Stride is register-aligned — simple multiply + constant offset.
		regsPerElem := chain.dynStride / 16
		regIdx := dynID
		if regsPerElem > 1 {
			mulID := e.getIntConstID(int64(regsPerElem))
			regIdx = e.addBinOpInstr(i32Ty, BinOpMul, dynID, mulID)
		}
		if staticRegOff := chain.byteOffset / 16; staticRegOff > 0 {
			offID := e.getIntConstID(int64(staticRegOff))
			regIdx = e.addBinOpInstr(i32Ty, BinOpAdd, regIdx, offID)
		}
		return regIdx, compOffset, nil
	}

	// Non-aligned stride: totalByteOff = dynIndex * stride + byteOffset, regIndex = totalByteOff >> 4.
	strideID := e.getIntConstID(int64(chain.dynStride))
	totalOff := e.addBinOpInstr(i32Ty, BinOpMul, dynID, strideID)
	if chain.byteOffset > 0 {
		offID := e.getIntConstID(int64(chain.byteOffset))
		totalOff = e.addBinOpInstr(i32Ty, BinOpAdd, totalOff, offID)
	}
	shiftID := e.getIntConstID(4) // >> 4 = / 16
	regIdx := e.addBinOpInstr(i32Ty, BinOpLShr, totalOff, shiftID)
	return regIdx, compOffset, nil
}

// computeCBVFieldOffset walks the type hierarchy using the given indices
// to compute the byte offset of the accessed field within the CBV struct.
// When skipArrayLevel is true, the top-level array type is skipped (the array
// dimension is handled by a dynamic index elsewhere).
// Returns (byteOffset, fieldType, scalarType, ok).
func (e *Emitter) computeCBVFieldOffset(varHandle ir.GlobalVariableHandle, indices []uint32, skipArrayLevel bool) (uint32, ir.TypeInner, ir.ScalarType, bool) {
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return 0, nil, ir.ScalarType{}, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return 0, nil, ir.ScalarType{}, false
	}

	currentType := e.ir.Types[gv.Type].Inner
	var byteOffset uint32

	// When a dynamic Access handles the array dimension, skip past the top-level array
	// so the static indices only traverse the element type (struct).
	if skipArrayLevel {
		if arr, ok := currentType.(ir.ArrayType); ok {
			if int(arr.Base) < len(e.ir.Types) {
				currentType = e.ir.Types[arr.Base].Inner
			}
		}
	}

	for _, idx := range indices {
		switch ct := currentType.(type) {
		case ir.StructType:
			if int(idx) >= len(ct.Members) {
				return 0, nil, ir.ScalarType{}, false
			}
			member := &ct.Members[idx]
			byteOffset += member.Offset
			if int(member.Type) >= len(e.ir.Types) {
				return 0, nil, ir.ScalarType{}, false
			}
			currentType = e.ir.Types[member.Type].Inner

		case ir.ArrayType:
			// Array element access: offset = idx * stride.
			byteOffset += idx * ct.Stride
			if int(ct.Base) >= len(e.ir.Types) {
				return 0, nil, ir.ScalarType{}, false
			}
			currentType = e.ir.Types[ct.Base].Inner

		case ir.VectorType:
			// Vector component access: offset = idx * scalar_width.
			byteOffset += idx * uint32(ct.Scalar.Width)
			currentType = ct.Scalar

		case ir.MatrixType:
			// Matrix column access: offset = idx * column_size (rows * scalar_width).
			colSize := uint32(ct.Rows) * uint32(ct.Scalar.Width)
			byteOffset += idx * colSize
			currentType = ir.VectorType{Size: ct.Rows, Scalar: ct.Scalar}

		default:
			return 0, nil, ir.ScalarType{}, false
		}
	}

	// Determine the scalar type of the final field.
	scalar, ok := scalarOfType(currentType)
	if !ok {
		// Default to f32 for struct types that we'll flatten.
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}

	return byteOffset, currentType, scalar, true
}

// emitCBVLoad emits a dx.op.cbufferLoadLegacy call and extracts the needed
// components based on the field type and byte offset.
//
// dx.op.cbufferLoadLegacy signature:
//
//	%dx.types.CBufRet.XX @dx.op.cbufferLoadLegacy.XX(i32 59, %dx.types.Handle handle, i32 regIndex)
//
// regIndex = byteOffset / 16 (each CBV register is 16 bytes = 4 floats).
// The result is a struct with 4 components (for f32/i32). Individual fields
// are extracted with extractvalue at index = (byteOffset % 16) / scalarWidth.
//
// Reference: Mesa nir_to_dxil.c load_ubo() line ~3061, emit_load_ubo_vec4() line ~3527
func (e *Emitter) emitCBVLoad(fn *ir.Function, chain *cbvPointerChain) (int, error) {
	// Get the resource handle for this CBV.
	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return 0, fmt.Errorf("CBV handle not found for global variable %d", chain.varHandle)
	}

	// Matrix types require special handling: each column is in a separate
	// 16-byte CBV register. We emit one cbufferLoadLegacy per column.
	if mt, isMat := chain.fieldType.(ir.MatrixType); isMat {
		return e.emitCBVMatrixLoad(handleID, chain, mt)
	}

	ol := overloadForScalar(chain.scalar)
	scalarWidth := uint32(chain.scalar.Width)

	// Get or create the CBufRet type and function declaration.
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)

	// Determine how many scalar components to extract based on the field type.
	// For structs, we recursively count all scalar members across register boundaries.
	numComps := cbvComponentCount(e.ir, chain.fieldType)

	// If the field spans multiple registers (e.g., struct with >4 scalars),
	// we need multiple cbufferLoadLegacy calls.
	if numComps > 4 {
		return e.emitCBVMultiRegLoad(handleID, chain, ol, scalarWidth, numComps)
	}

	// Single register path: scalar / vector / small struct.
	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	regIndexVal, compOffset, dynErr := e.resolveCBVRegIndex(fn, chain, scalarWidth)
	if dynErr != nil {
		return 0, dynErr
	}

	retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

	comps := make([]int, numComps)
	for i := 0; i < numComps; i++ {
		extractIdx := int(compOffset) + i
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, extractIdx},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}

	if numComps > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// emitCBVMultiRegLoad loads multiple CBV registers to fill all scalar components.
// Used when a struct or large vector spans multiple 16-byte registers.
func (e *Emitter) emitCBVMultiRegLoad(
	handleID int,
	chain *cbvPointerChain,
	ol overloadType,
	scalarWidth uint32,
	totalComps int,
) (int, error) {
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	compsPerReg := 16 / int(scalarWidth)
	baseRegIndex := chain.byteOffset / 16
	baseCompOffset := int((chain.byteOffset % 16) / scalarWidth)

	comps := make([]int, 0, totalComps)

	currentReg := int(baseRegIndex)
	currentComp := baseCompOffset
	remaining := totalComps

	for remaining > 0 {
		regIndexVal := e.getIntConstID(int64(currentReg))
		retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

		// Extract components from this register.
		available := compsPerReg - currentComp
		toExtract := remaining
		if toExtract > available {
			toExtract = available
		}

		for i := 0; i < toExtract; i++ {
			extractIdx := currentComp + i
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, extractIdx},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}

		remaining -= toExtract
		currentReg++
		currentComp = 0 // subsequent registers start at component 0
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// emitCBVMatrixLoad loads a matrix from a CBV by issuing one cbufferLoadLegacy
// per column. Each column occupies one 16-byte register.
//
// For mat4x4<f32>: 4 registers, 4 components each = 16 total components.
// For mat3x3<f32>: 3 registers, 3 components each = 9 total components
// (but each register still returns 4 components; we only use the first 3).
//
// Reference: Mesa nir_to_dxil.c — matrix loads decompose to per-column loads.
func (e *Emitter) emitCBVMatrixLoad(
	handleID int,
	chain *cbvPointerChain,
	mt ir.MatrixType,
) (int, error) {
	ol := overloadForScalar(chain.scalar)
	scalarWidth := uint32(chain.scalar.Width)
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)

	cols := int(mt.Columns)
	rows := int(mt.Rows)
	totalComps := cols * rows

	// Each column is aligned to 16 bytes in a cbuffer.
	colAligned := (uint32(rows)*scalarWidth + 15) &^ 15 //nolint:gosec // rows is small (2-4)
	baseRegIndex := chain.byteOffset / 16

	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	comps := make([]int, 0, totalComps)
	for col := 0; col < cols; col++ {
		regIdx := baseRegIndex + uint32(col)*(colAligned/16)
		regIndexVal := e.getIntConstID(int64(regIdx))

		retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

		for row := 0; row < rows; row++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, row},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// getDxCBufRetType returns the %dx.types.CBufRet.XX struct type.
// For f32/i32: 4 fields. For f64/i64: 2 fields. For f16/i16: 8 fields.
//
// Reference: Mesa dxil_module.c dxil_module_get_cbuf_ret_type() line ~706
func (e *Emitter) getDxCBufRetType(ol overloadType) *module.Type {
	scalarTy := e.overloadReturnType(ol)
	name := "dx.types.CBufRet" + overloadSuffix(ol)

	var numFields int
	switch ol {
	case overloadF64, overloadI64:
		numFields = 2
	case overloadF16, overloadI16:
		numFields = 8
		name += ".8"
	default: // f32, i32
		numFields = 4
	}

	fields := make([]*module.Type, numFields)
	for i := range fields {
		fields[i] = scalarTy
	}

	return e.mod.GetStructType(name, fields)
}

// getDxOpCBufLoadFunc creates the dx.op.cbufferLoadLegacy.XX function declaration.
// Signature: %dx.types.CBufRet.XX @dx.op.cbufferLoadLegacy.XX(i32, %dx.types.Handle, i32)
//
// Reference: Mesa nir_to_dxil.c load_ubo() — dxil_get_function("dx.op.cbufferLoadLegacy", overload)
func (e *Emitter) getDxOpCBufLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.cbufferLoadLegacy"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	cbufRetTy := e.getDxCBufRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty}
	funcTy := e.mod.GetFunctionType(cbufRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// --- UAV (storage buffer) operations ---

// uavPointerChain describes a resolved pointer chain leading to a UAV element.
type uavPointerChain struct {
	varHandle  ir.GlobalVariableHandle
	indexExpr  ir.ExpressionHandle // array index expression (dynamic) — used when isConstIndex is false
	constIndex uint32              // constant array index — used when isConstIndex is true
	isConstIdx bool                // true if this is a constant-index access
	isWhole    bool                // true if this is a whole-buffer load (direct GlobalVariable reference)
	elemType   ir.TypeInner        // element type (what's being loaded/stored)
	scalar     ir.ScalarType       // scalar element type for overload selection
	// stride and fieldByteOffset are used for struct-in-array access patterns.
	// The final raw buffer index = (arrayIndex * stride + fieldByteOffset) / scalarWidth.
	stride          uint32 // array element stride in bytes (0 = no stride adjustment)
	fieldByteOffset uint32 // byte offset of the accessed field within the struct element
}

// resolveUAVPointerChain walks an expression chain and determines whether it
// leads to a UAV (storage buffer) global variable. If so, returns chain info.
//
// Supported patterns:
//   - ExprAccess → ExprGlobalVariable (dynamic array index)
//   - ExprAccessIndex → ExprGlobalVariable (constant array index)
//   - ExprGlobalVariable (entire buffer — not typical)
//
// Reference: Mesa nir_to_dxil.c emit_store_ssbo() for storage buffer write patterns
func (e *Emitter) resolveUAVPointerChain(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*uavPointerChain, bool) {
	if int(ptrHandle) >= len(fn.Expressions) {
		return nil, false
	}
	expr := fn.Expressions[ptrHandle]

	switch ek := expr.Kind.(type) {
	case ir.ExprAccess:
		// Dynamic index: data[index] where index is a runtime expression.
		return e.resolveUAVAccessChain(fn, ek.Base, ek.Index, true)

	case ir.ExprAccessIndex:
		// Constant index: data[N] where N is compile-time constant.
		return e.resolveUAVAccessIndexChain(fn, ek.Base, ek.Index)

	case ir.ExprGlobalVariable:
		// Direct reference to the UAV global variable (whole-buffer load/store).
		// This handles patterns like: let x = storage_var; or output = value;
		return e.resolveUAVDirectGlobal(ek.Variable)

	default:
		return nil, false
	}
}

// resolveUAVDirectGlobal handles direct reference to a UAV global variable
// (loading/storing the entire buffer content, not a specific element).
// The element type and scalar are derived from the variable's type.
func (e *Emitter) resolveUAVDirectGlobal(varHandle ir.GlobalVariableHandle) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if res.class != resourceClassUAV {
		return nil, false
	}

	// For a direct GlobalVariable reference, we load the entire type at index 0.
	// We use the full variable type (not element type) since we're loading everything.
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	fullType := e.ir.Types[gv.Type].Inner
	scalar, ok := scalarOfType(fullType)
	if !ok {
		// For struct/array types, default to f32 scalar for the overload.
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		// Try to find the actual scalar from nested types.
		if s, found := deepScalarOfType(e.ir, fullType); found {
			scalar = s
		}
	}

	return &uavPointerChain{
		varHandle:  varHandle,
		constIndex: 0,
		isConstIdx: true,
		elemType:   fullType,
		scalar:     scalar,
		isWhole:    true,
	}, true
}

// deepScalarOfType finds the scalar type buried inside structs, arrays, and vectors.
func deepScalarOfType(irMod *ir.Module, inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.ArrayType:
		if int(t.Base) < len(irMod.Types) {
			return deepScalarOfType(irMod, irMod.Types[t.Base].Inner)
		}
	case ir.StructType:
		if len(t.Members) > 0 && int(t.Members[0].Type) < len(irMod.Types) {
			return deepScalarOfType(irMod, irMod.Types[t.Members[0].Type].Inner)
		}
	}
	return ir.ScalarType{}, false
}

// resolveUAVIndex computes the buffer index value ID for a UAV access.
// Handles simple constant/dynamic indices, as well as stride-scaled struct field access.
func (e *Emitter) resolveUAVIndex(fn *ir.Function, chain *uavPointerChain) (int, error) {
	scalarW := uint32(chain.scalar.Width)
	if scalarW == 0 {
		scalarW = 4
	}

	if chain.stride > 0 {
		return e.resolveUAVStridedIndex(fn, chain, scalarW)
	}
	if chain.isConstIdx {
		return e.getIntConstID(int64(chain.constIndex)), nil
	}
	indexID, err := e.emitExpression(fn, chain.indexExpr)
	if err != nil {
		return 0, fmt.Errorf("UAV index: %w", err)
	}
	return indexID, nil
}

// resolveUAVStridedIndex handles stride-scaled index for struct-in-array UAV access.
// index = arrayIndex * (stride / scalarWidth) + (fieldByteOffset / scalarWidth).
func (e *Emitter) resolveUAVStridedIndex(fn *ir.Function, chain *uavPointerChain, scalarW uint32) (int, error) {
	strideWords := chain.stride / scalarW
	fieldOffWords := chain.fieldByteOffset / scalarW

	if chain.isConstIdx {
		finalIdx := int64(chain.constIndex)*int64(strideWords) + int64(fieldOffWords)
		return e.getIntConstID(finalIdx), nil
	}

	dynID, err := e.emitExpression(fn, chain.indexExpr)
	if err != nil {
		return 0, fmt.Errorf("UAV strided index: %w", err)
	}
	i32Ty := e.mod.GetIntType(32)
	indexID := dynID
	if strideWords > 1 {
		strideID := e.getIntConstID(int64(strideWords))
		indexID = e.addBinOpInstr(i32Ty, BinOpMul, dynID, strideID)
	}
	if fieldOffWords > 0 {
		offID := e.getIntConstID(int64(fieldOffWords))
		indexID = e.addBinOpInstr(i32Ty, BinOpAdd, indexID, offID)
	}
	return indexID, nil
}

// resolveUAVAccessChain resolves a dynamic-index access to a UAV global variable.
// Handles patterns:
//   - Access(dynIdx, GlobalVariable) — direct array element access
//   - Access(dynIdx, AccessIndex(structFieldIdx, GlobalVariable)) — struct-wrapped array access
func (e *Emitter) resolveUAVAccessChain(fn *ir.Function, baseHandle, indexHandle ir.ExpressionHandle, _ bool) (*uavPointerChain, bool) {
	if int(baseHandle) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[baseHandle]

	var varHandle ir.GlobalVariableHandle

	switch bk := baseExpr.Kind.(type) {
	case ir.ExprGlobalVariable:
		varHandle = bk.Variable

	case ir.ExprAccessIndex:
		// Access(dynIdx, AccessIndex(structFieldIdx, GlobalVariable))
		// The AccessIndex unwraps the struct wrapper (e.g., .particles field).
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		varHandle = gv

	default:
		return nil, false
	}

	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if res.class != resourceClassUAV {
		return nil, false
	}

	// Determine element type from the array base type.
	elemType, scalar, ok := e.resolveUAVElementType(varHandle)
	if !ok {
		return nil, false
	}

	return &uavPointerChain{
		varHandle: varHandle,
		indexExpr: indexHandle,
		elemType:  elemType,
		scalar:    scalar,
	}, true
}

// resolveToGlobalVariable walks through an expression to find the underlying GlobalVariable handle.
// Handles chains like AccessIndex(idx, Access(dynIdx, AccessIndex(idx, GlobalVariable)))
// by peeling off intermediate AccessIndex and Access expressions.
func (e *Emitter) resolveToGlobalVariable(fn *ir.Function, handle ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, false
	}
	expr := fn.Expressions[handle]
	switch ek := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		return ek.Variable, true
	case ir.ExprAccessIndex:
		return e.resolveToGlobalVariable(fn, ek.Base)
	case ir.ExprAccess:
		return e.resolveToGlobalVariable(fn, ek.Base)
	default:
		return 0, false
	}
}

// resolveUAVAccessIndexChain resolves a constant-index access to a UAV.
// Handles patterns:
//   - AccessIndex(idx, GlobalVariable) — direct field/element access
//   - AccessIndex(fieldIdx, Access(dynIdx, ...)) — array[i].field
//   - AccessIndex(fieldIdx, AccessIndex(arrIdx, ...)) — array[N].field OR struct.data[N]
func (e *Emitter) resolveUAVAccessIndexChain(fn *ir.Function, baseHandle ir.ExpressionHandle, constIdx uint32) (*uavPointerChain, bool) {
	if int(baseHandle) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[baseHandle]

	switch bk := baseExpr.Kind.(type) {
	case ir.ExprGlobalVariable:
		// Direct: AccessIndex(idx, GlobalVariable)
		return e.resolveUAVFromGlobal(bk.Variable, constIdx, true)

	case ir.ExprAccess:
		// Nested: AccessIndex(fieldIdx, Access(dynIdx, ...))
		// The dynamic Access indexes into the array; the AccessIndex selects a struct field.
		// The Access base can be GlobalVariable or AccessIndex(structFieldIdx, GlobalVariable).
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		chain, ok := e.resolveUAVStructFieldChain(gv, constIdx)
		if !ok {
			return nil, false
		}
		// Use the dynamic index from the Access expression.
		chain.indexExpr = bk.Index
		chain.isConstIdx = false
		return chain, true

	case ir.ExprAccessIndex:
		// Two sub-cases:
		// (a) AccessIndex(fieldIdx, AccessIndex(arrIdx, AccessIndex(structWrap, GV)))
		//     — struct element field access with constant array index (arr[N].field)
		// (b) AccessIndex(arrIdx, AccessIndex(structWrap, GV))
		//     — simple array element access (struct.data[N])
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		// First try as struct-in-array field access: constIdx is fieldIdx, bk.Index is arrIdx.
		if chain, ok := e.resolveUAVStructFieldChain(gv, constIdx); ok {
			chain.constIndex = bk.Index
			chain.isConstIdx = true
			return chain, true
		}
		// Fall back to simple array element access.
		return e.resolveUAVFromGlobal(gv, constIdx, true)
	}

	return nil, false
}

// resolveUAVStructFieldChain resolves an array[dynIdx].field pattern on a UAV.
// varHandle is the UAV global variable, fieldIdx is the struct member index.
// Returns a chain with stride and fieldByteOffset set for index computation.
func (e *Emitter) resolveUAVStructFieldChain(varHandle ir.GlobalVariableHandle, fieldIdx uint32) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if res.class != resourceClassUAV {
		return nil, false
	}

	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	// Get the outer type: should be array<Struct> or struct{array<Struct>}.
	inner := e.ir.Types[gv.Type].Inner
	if st, ok := inner.(ir.StructType); ok && len(st.Members) > 0 {
		inner = e.ir.Types[st.Members[0].Type].Inner
	}

	arr, ok := inner.(ir.ArrayType)
	if !ok {
		return nil, false
	}

	// Get the struct element type from the array.
	elemInner := e.ir.Types[arr.Base].Inner
	st, ok := elemInner.(ir.StructType)
	if !ok || int(fieldIdx) >= len(st.Members) {
		return nil, false
	}

	// Get the field type and its byte offset within the struct.
	member := &st.Members[fieldIdx]
	fieldInner := e.ir.Types[member.Type].Inner
	scalar, ok := scalarOfType(fieldInner)
	if !ok {
		return nil, false
	}

	return &uavPointerChain{
		varHandle:       varHandle,
		elemType:        fieldInner,
		scalar:          scalar,
		stride:          arr.Stride,
		fieldByteOffset: member.Offset,
	}, true
}

// resolveUAVFromGlobal checks if a global variable is a UAV and builds a chain.
func (e *Emitter) resolveUAVFromGlobal(varHandle ir.GlobalVariableHandle, constIdx uint32, isConst bool) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if res.class != resourceClassUAV {
		return nil, false
	}

	elemType, scalar, ok := e.resolveUAVElementType(varHandle)
	if !ok {
		return nil, false
	}

	return &uavPointerChain{
		varHandle:  varHandle,
		constIndex: constIdx,
		isConstIdx: isConst,
		elemType:   elemType,
		scalar:     scalar,
	}, true
}

// resolveUAVElementType determines the element type of a UAV's array contents.
// Storage buffers in naga IR are typed as array<T> or struct { array<T> }.
func (e *Emitter) resolveUAVElementType(varHandle ir.GlobalVariableHandle) (ir.TypeInner, ir.ScalarType, bool) {
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, ir.ScalarType{}, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, ir.ScalarType{}, false
	}

	inner := e.ir.Types[gv.Type].Inner

	// Unwrap struct wrapper if present (e.g., struct { data: array<u32> }).
	if st, ok := inner.(ir.StructType); ok {
		if len(st.Members) > 0 {
			memberType := e.ir.Types[st.Members[0].Type]
			inner = memberType.Inner
		}
	}

	// Now inner should be an ArrayType — get the element type.
	if arr, ok := inner.(ir.ArrayType); ok {
		elemInner := e.ir.Types[arr.Base].Inner
		scalar, found := scalarOfType(elemInner)
		if found {
			return elemInner, scalar, true
		}
	}

	// Direct scalar or vector type (rare for storage buffers).
	scalar, found := scalarOfType(inner)
	return inner, scalar, found
}

// emitUAVLoad emits a dx.op.bufferLoad call for reading from a UAV (storage buffer).
//
// dx.op.bufferLoad signature:
//
//	%dx.types.ResRet.XX @dx.op.bufferLoad.XX(i32 68, %handle, i32 index, i32 offset)
//
// Returns the loaded value ID (first component for vectors).
//
// Reference: Mesa nir_to_dxil.c emit_bufferload_call() ~833
func (e *Emitter) emitUAVLoad(fn *ir.Function, chain *uavPointerChain) (int, error) {
	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return 0, fmt.Errorf("UAV handle not found for global variable %d", chain.varHandle)
	}

	indexID, err := e.resolveUAVIndex(fn, chain)
	if err != nil {
		return 0, err
	}

	ol := overloadForScalar(chain.scalar)
	resRetTy := e.getDxResRetType(ol)
	bufLoadFn := e.getDxOpBufferLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpBufferLoad))
	undefVal := e.getUndefConstID()

	// For whole-buffer loads (struct/array), count all scalar components and
	// emit multiple bufferLoad calls if needed (each returns up to 4 components).
	numComps := componentCount(chain.elemType)
	if chain.isWhole {
		numComps = cbvComponentCount(e.ir, chain.elemType)
	}

	if numComps <= 4 {
		// Single bufferLoad call.
		retID := e.addCallInstr(bufLoadFn, resRetTy, []int{opcodeVal, handleID, indexID, undefVal})
		comps := make([]int, numComps)
		for i := 0; i < numComps; i++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, i},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps[i] = extractID
		}
		if numComps > 1 {
			e.pendingComponents = comps
		}
		return comps[0], nil
	}

	// Multiple bufferLoad calls for types with >4 scalar components.
	comps := make([]int, 0, numComps)
	baseIndex := chain.constIndex
	remaining := numComps
	batchIdx := 0

	for remaining > 0 {
		count := 4
		if remaining < 4 {
			count = remaining
		}
		batchIndexID := e.getIntConstID(int64(baseIndex) + int64(batchIdx))
		retID := e.addCallInstr(bufLoadFn, resRetTy, []int{opcodeVal, handleID, batchIndexID, undefVal})
		for i := 0; i < count; i++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, i},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}
		remaining -= count
		batchIdx++
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}
	return comps[0], nil
}

// emitUAVStore emits a dx.op.bufferStore call for writing to a UAV (storage buffer).
//
// dx.op.bufferStore signature:
//
//	void @dx.op.bufferStore.XX(i32 69, %handle, i32 index, i32 offset,
//	                           XX val0, XX val1, XX val2, XX val3, i8 mask)
//
// Reference: Mesa nir_to_dxil.c emit_bufferstore_call() ~877
func (e *Emitter) emitUAVStore(fn *ir.Function, chain *uavPointerChain, valueHandle ir.ExpressionHandle) error {
	// Reject whole-buffer stores of large aggregates (e.g., output = w_mem.arr for array<u32, 512>).
	// These require element-by-element decomposition that we don't yet support.
	if chain.isWhole {
		numComps := cbvComponentCount(e.ir, chain.elemType)
		if numComps > 4 {
			return fmt.Errorf("whole-buffer UAV store of %d components not yet supported (use element-by-element copy)", numComps)
		}
	}

	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return fmt.Errorf("UAV handle not found for global variable %d", chain.varHandle)
	}

	indexID, err := e.resolveUAVIndex(fn, chain)
	if err != nil {
		return err
	}

	// Emit the value expression.
	valueID, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("UAV store value: %w", err)
	}

	ol := overloadForScalar(chain.scalar)
	bufStoreFn := e.getDxOpBufferStoreFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpBufferStore))
	undefI32 := e.getUndefConstID()
	// Value channel undefs must match the overload type (e.g., undef f32 for float stores).
	// Using i32 undef for float channels causes DXC "Invalid record" / "Cast of incompatible type".
	valUndefID := e.getTypedUndefConstID(e.overloadReturnType(ol))

	// Determine total number of scalar components.
	numComps := componentCount(chain.elemType)
	if chain.isWhole {
		numComps = cbvComponentCount(e.ir, chain.elemType)
	}

	if numComps <= 4 {
		// Single bufferStore call.
		vals := [4]int{valUndefID, valUndefID, valUndefID, valUndefID}
		for i := 0; i < numComps && i < 4; i++ {
			if numComps == 1 {
				vals[i] = valueID
			} else {
				vals[i] = e.getComponentID(valueHandle, i)
			}
		}
		writeMask := (1 << numComps) - 1
		maskVal := e.getI8ConstID(int64(writeMask))
		e.addCallInstr(bufStoreFn, e.mod.GetVoidType(), []int{
			opcodeVal, handleID, indexID, undefI32,
			vals[0], vals[1], vals[2], vals[3], maskVal,
		})
		return nil
	}

	// Multiple bufferStore calls for types with >4 scalar components.
	compIdx := 0
	batchIdx := 0
	remaining := numComps

	for remaining > 0 {
		count := 4
		if remaining < 4 {
			count = remaining
		}
		batchIndexID := e.getIntConstID(int64(chain.constIndex) + int64(batchIdx))
		vals := [4]int{valUndefID, valUndefID, valUndefID, valUndefID}
		for i := 0; i < count; i++ {
			vals[i] = e.getComponentID(valueHandle, compIdx+i)
		}
		writeMask := (1 << count) - 1
		maskVal := e.getI8ConstID(int64(writeMask))
		e.addCallInstr(bufStoreFn, e.mod.GetVoidType(), []int{
			opcodeVal, handleID, batchIndexID, undefI32,
			vals[0], vals[1], vals[2], vals[3], maskVal,
		})
		compIdx += count
		remaining -= count
		batchIdx++
	}

	return nil
}

// getDxOpBufferLoadFunc creates the dx.op.bufferLoad.XX function declaration.
// Signature: %dx.types.ResRet.XX @dx.op.bufferLoad.XX(i32, %handle, i32, i32)
//
// Reference: Mesa nir_to_dxil.c emit_bufferload_call() ~833
func (e *Emitter) getDxOpBufferLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.bufferLoad"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	resRetTy := e.getDxResRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(resRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpBufferStoreFunc creates the dx.op.bufferStore.XX function declaration.
// Signature: void @dx.op.bufferStore.XX(i32, %handle, i32, i32, XX, XX, XX, XX, i8)
//
// Reference: Mesa nir_to_dxil.c emit_bufferstore_call() ~877
func (e *Emitter) getDxOpBufferStoreFunc(ol overloadType) *module.Function {
	name := "dx.op.bufferStore"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	voidTy := e.mod.GetVoidType()
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valTy := e.overloadReturnType(ol)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, valTy, valTy, valTy, valTy, i8Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// resourceKind returns the DXIL resource kind integer for metadata.
// Reference: D3D12_SRV_DIMENSION / DXIL resource kinds.
func (e *Emitter) resourceKind(res *resourceInfo) int {
	switch res.class {
	case resourceClassCBV:
		return 13 // CBuffer
	case resourceClassSampler:
		return 0 // Sampler (no dimension)
	case resourceClassSRV:
		// Determine from the image type.
		if int(res.typeHandle) < len(e.ir.Types) {
			inner := e.ir.Types[res.typeHandle].Inner
			if img, ok := inner.(ir.ImageType); ok {
				switch img.Dim {
				case ir.Dim1D:
					return 2 // Texture1D
				case ir.Dim2D:
					return 4 // Texture2D
				case ir.Dim3D:
					return 7 // Texture3D
				case ir.DimCube:
					return 9 // TextureCube
				}
			}
		}
		return 4 // default Texture2D
	case resourceClassUAV:
		// Determine from type: typed buffer vs structured buffer vs raw buffer.
		if int(res.typeHandle) < len(e.ir.Types) {
			inner := e.ir.Types[res.typeHandle].Inner
			// Unwrap struct wrapper (common for storage buffers).
			if st, ok := inner.(ir.StructType); ok && len(st.Members) > 0 {
				inner = e.ir.Types[st.Members[0].Type].Inner
			}
			switch inner.(type) {
			case ir.ArrayType:
				return 12 // RawBuffer (for typed array storage buffers)
			}
		}
		return 12 // default RawBuffer for UAV storage buffers
	default:
		return 0
	}
}
