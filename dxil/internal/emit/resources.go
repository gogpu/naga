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
		if gv.Binding == nil {
			continue
		}

		class, ok := e.classifyGlobalVariable(gv)
		if !ok {
			continue
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
	case ir.SpaceUniform:
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
// Each list contains entries: !{i32 rangeID, null, !"name", i32 space, i32 lower, i32 upper, i32 kind, null}
func (e *Emitter) emitResourceMetadata() *module.MetadataNode {
	if len(e.resources) == 0 {
		return nil
	}

	i32Ty := e.mod.GetIntType(32)

	// Group resources by class.
	var srvs, uavs, cbvs, samplers []*module.MetadataNode

	for i := range e.resources {
		res := &e.resources[i]
		entry := e.buildResourceMetadataEntry(res, i32Ty)

		switch res.class {
		case resourceClassSRV:
			srvs = append(srvs, entry)
		case resourceClassUAV:
			uavs = append(uavs, entry)
		case resourceClassCBV:
			cbvs = append(cbvs, entry)
		case resourceClassSampler:
			samplers = append(samplers, entry)
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

// buildResourceMetadataEntry builds a single resource metadata entry.
// Format: !{i32 rangeID, null, !"name", i32 space, i32 lowerBound, i32 upperBound, i32 kind, null}
func (e *Emitter) buildResourceMetadataEntry(res *resourceInfo, i32Ty *module.Type) *module.MetadataNode {
	mdRangeID := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.rangeID)))
	mdName := e.mod.AddMetadataString(res.name)
	mdSpace := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.group)))
	mdLower := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.binding)))
	mdUpper := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.binding)+1))
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(e.resourceKind(res))))

	return e.mod.AddMetadataTuple([]*module.MetadataNode{
		mdRangeID, // i32 rangeID
		nil,       // null (global variable ref, simplified)
		mdName,    // !"name"
		mdSpace,   // i32 space
		mdLower,   // i32 lowerBound
		mdUpper,   // i32 upperBound
		mdKind,    // i32 kind
		nil,       // null (additional properties)
	})
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
		return 4 // default Texture2D for UAV
	default:
		return 0
	}
}
