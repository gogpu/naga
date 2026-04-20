// Package container — PSV0 (Pipeline State Validation) part encoding.
//
// PSV0 contains runtime information used by the D3D12 driver to validate
// pipeline state without running the shader. The format consists of:
//
//  1. PSV runtime info size (uint32) + runtime info struct
//  2. Resource count (uint32) + optional resource bindings
//  3. String table (uint32 size + data, 4-byte aligned)
//  4. Semantic index table (uint32 count + data)
//  5. Optional: PSV signature elements
//  6. Optional: I/O dependency tables
//
// For SM 6.0 (validator < 1.6), the runtime info is PSVRuntimeInfo1
// (the minimum required version that includes signature element counts).
//
// Reference: Mesa dxil_container.c dxil_container_add_state_validation(),
// Mesa dxil_signature.h struct dxil_psv_runtime_info_*.
package container

import (
	"encoding/binary"
)

// PSVShaderKind matches the Microsoft D3D12 DXIL runtime ABI for the
// PSV0 (Pipeline State Validation) ShaderStage byte. Values MUST be
// bit-exact with D3D11_SHADER_VERSION_TYPE — D3D12 validates the stage
// byte during CreateGraphicsPipelineState and rejects mismatched blobs
// with E_INVALIDARG.
//
// Source of truth (keep in sync):
//   - DXC:  reference/dxil/dxc/include/dxc/DXIL/DxilConstants.h:244
//     enum class ShaderKind { Pixel=0, Vertex=1, Geometry=2, ... }
//     "Must match D3D11_SHADER_VERSION_TYPE"
//   - Mesa: reference/dxil/mesa/src/microsoft/compiler/dxil_module.h:44
//     DXIL_PIXEL_SHADER=0, DXIL_VERTEX_SHADER=1, ...
//
// This enum is ABI-locked by TestPSVShaderKindMatchesMicrosoftABI in
// psv_test.go. Do not change values without updating both upstream
// references and the lock test in the same commit.
type PSVShaderKind uint8

const (
	PSVPixel         PSVShaderKind = 0
	PSVVertex        PSVShaderKind = 1
	PSVGeometry      PSVShaderKind = 2
	PSVHull          PSVShaderKind = 3
	PSVDomain        PSVShaderKind = 4
	PSVCompute       PSVShaderKind = 5
	PSVLibrary       PSVShaderKind = 6
	PSVRayGeneration PSVShaderKind = 7
	PSVIntersection  PSVShaderKind = 8
	PSVAnyHit        PSVShaderKind = 9
	PSVClosestHit    PSVShaderKind = 10
	PSVMiss          PSVShaderKind = 11
	PSVCallable      PSVShaderKind = 12
	PSVMesh          PSVShaderKind = 13
	PSVAmplification PSVShaderKind = 14
	PSVNode          PSVShaderKind = 15
	PSVInvalid       PSVShaderKind = 16
)

// PSVInfo holds the data needed to build a PSV0 part.
type PSVInfo struct {
	ShaderStage PSVShaderKind

	// Vertex shader specific.
	OutputPositionPresent bool

	// Pixel shader specific.
	DepthOutput     bool
	SampleFrequency bool

	// Mesh shader specific (PSVRuntimeInfo0 union + PSVRuntimeInfo1 union + PSVRuntimeInfo2).
	MaxOutputVertices   uint16
	MaxOutputPrimitives uint16
	MeshOutputTopology  uint8 // 0=undefined, 1=line, 2=triangle
	NumThreadsX         uint32
	NumThreadsY         uint32
	NumThreadsZ         uint32

	// PSVRuntimeInfo3 extension: entry function name offset in string table.
	EntryFunctionName uint32

	// Signature element counts (for PSV1).
	SigInputElements            uint8
	SigOutputElements           uint8
	SigPatchConstOrPrimElements uint8 // patch constant (hull) or primitive (mesh) element count

	// Number of packed signature vectors.
	SigInputVectors  uint8
	SigOutputVectors uint8

	// Wave lane counts (0 = unused, 0xFFFFFFFF = any).
	MinWaveLaneCount uint32
	MaxWaveLaneCount uint32

	// UsesViewID is true when the entry point reads SV_ViewID via
	// dx.op.viewID. Encoded as RTI1 byte 1 (offset 25 from RTI0 base).
	// The validator regenerates this flag from the bitcode's
	// dx.op.viewID instruction usage and memcmp's against PSV0.
	UsesViewID bool

	// Semantic string table entries (null-terminated, will be 4-byte aligned).
	StringTable []byte

	// Semantic index table entries.
	SemanticIndexTable []uint32

	// PSV signature elements (optional).
	PSVSigInputs  []PSVSignatureElement
	PSVSigOutputs []PSVSignatureElement

	// Resource bindings (BUG-DXIL-022). dxil.dll validator compares PSV0
	// resource count against the bitcode !dx.resources list; a stale 0
	// here fails "ResourceCount mismatch" for any shader touching buffers,
	// textures, or samplers.
	ResourceBindings []PSVResourceBinding

	// InputToOutputTable is the per-input-component dependency bitmap in
	// the layout PSVDependencyTable expects: 4 components per input
	// vector, MaskDwordsForComponents(SigOutputVectors) dwords per input
	// component. Total size: 4 * SigInputVectors * MaskDwords(SigOutputVectors).
	//
	// Each dword is a bitmap whose bit `c` is set when the given input
	// component contributes to output component `c`. Populated by
	// buildPSV from the dxil/internal/viewid analyzer output.
	//
	// Nil for shaders with no input or output signature vectors.
	InputToOutputTable []uint32
}

// PSVResourceType mirrors DXC DxilPipelineStateValidation.h enum
// class PSVResourceType. Values are ABI-locked — see dxc include file.
type PSVResourceType uint32

const (
	PSVResTypeInvalid                  PSVResourceType = 0
	PSVResTypeSampler                  PSVResourceType = 1
	PSVResTypeCBV                      PSVResourceType = 2
	PSVResTypeSRVTyped                 PSVResourceType = 3
	PSVResTypeSRVRaw                   PSVResourceType = 4
	PSVResTypeSRVStructured            PSVResourceType = 5
	PSVResTypeUAVTyped                 PSVResourceType = 6
	PSVResTypeUAVRaw                   PSVResourceType = 7
	PSVResTypeUAVStructured            PSVResourceType = 8
	PSVResTypeUAVStructuredWithCounter PSVResourceType = 9
)

// PSVResourceKind mirrors DXC DxilPipelineStateValidation.h enum
// class PSVResourceKind.
type PSVResourceKind uint32

const (
	PSVResKindInvalid                 PSVResourceKind = 0
	PSVResKindTexture1D               PSVResourceKind = 1
	PSVResKindTexture2D               PSVResourceKind = 2
	PSVResKindTexture2DMS             PSVResourceKind = 3
	PSVResKindTexture3D               PSVResourceKind = 4
	PSVResKindTextureCube             PSVResourceKind = 5
	PSVResKindTexture1DArray          PSVResourceKind = 6
	PSVResKindTexture2DArray          PSVResourceKind = 7
	PSVResKindTexture2DMSArray        PSVResourceKind = 8
	PSVResKindTextureCubeArray        PSVResourceKind = 9
	PSVResKindTypedBuffer             PSVResourceKind = 10
	PSVResKindRawBuffer               PSVResourceKind = 11
	PSVResKindStructuredBuffer        PSVResourceKind = 12
	PSVResKindCBuffer                 PSVResourceKind = 13
	PSVResKindSampler                 PSVResourceKind = 14
	PSVResKindTBuffer                 PSVResourceKind = 15
	PSVResKindRTAccelerationStructure PSVResourceKind = 16
)

// PSVResourceBinding matches DXC PSVResourceBindInfo1 (24 bytes on-wire).
type PSVResourceBinding struct {
	ResType    PSVResourceType
	Space      uint32
	LowerBound uint32
	UpperBound uint32
	ResKind    PSVResourceKind
	ResFlags   uint32
}

// psvResourceBindInfo1Size is the wire size of one PSVResourceBindInfo1
// record. DXC emits this size as the element-size prefix before the
// binding array when resourceCount > 0.
const psvResourceBindInfo1Size = 24

// PSVSignatureElement matches Mesa's dxil_psv_signature_element (8 bytes).
type PSVSignatureElement struct {
	SemanticNameOffset    uint32
	SemanticIndexesOffset uint32
	Rows                  uint8
	StartRow              uint8
	ColsAndStart          uint8 // 0:4=Cols, 4:6=StartCol, 6:7=Allocated
	SemanticKind          uint8
	ComponentType         uint8
	InterpolationMode     uint8
	DynamicMaskAndStream  uint8
	Reserved              uint8
}

// psvSignatureElementSize is the wire size of one PSVSignatureElement.
const psvSignatureElementSize = 4 + 4 + 8 // two uint32 + 8 bytes = 16 bytes

// Wire sizes of the PSVRuntimeInfo* struct variants, per Microsoft DXC
// DxilPipelineStateValidation.h:110-180.
//
//	PSVRuntimeInfo0 = 24 B (union 14 B padded to 16 + 4 MinWave + 4 MaxWave)
//	PSVRuntimeInfo1 = 36 B = Info0 + 1 ShaderStage + 1 UsesViewID + 2 union
//	                         + 3 sig-elem counts + 1 SigInputVectors
//	                         + 4 SigOutputVectors[PSV_GS_MAX_STREAMS]
//	PSVRuntimeInfo2 = 48 B = Info1 + 3*4 NumThreadsX/Y/Z
//	PSVRuntimeInfo3 = 52 B = Info2 + 4 EntryFunctionName
//	PSVRuntimeInfo4 = 60 B = Info3 + 8 NumBytesGroupSharedMemory
//
// Deprecated: runtimeInfo1Size / runtimeInfo2Size are kept as documentation of
// older layout versions. The emit path uses runtimeInfo3Size unconditionally
// (BUG-DXIL-009): modern dxil.dll (validator 1.6+) interprets the psv_size
// header as "this is at least this version" and reads PSVRuntimeInfo3 fields
// (NumThreads/EntryFunctionName) regardless of shader stage. Writing 36 B
// caused the validator to read past the end of our payload into adjacent
// memory and AV at dxil.dll+0xe9da (NULL+0x18). Fix: always emit 52 B with
// NumThreads/EntryFunctionName zero-padded for non-compute stages. See
// docs/dev/research/BUG-DXIL-VALIDATOR-REAL-PHASE0-FINDINGS.md § "The AV
// mechanism, now fully understood".
const (
	runtimeInfo1Size = 36 // Deprecated — kept for layout documentation only.
	runtimeInfo2Size = 48 // Deprecated — kept for layout documentation only.
	runtimeInfo3Size = 52 // Active emit size for all shader stages.
)

// EncodePSV0 serializes PSVInfo into the PSV0 part binary format.
//
// The runtime-info struct is always emitted at PSVRuntimeInfo3 size (52 B)
// regardless of shader stage. NumThreadsX/Y/Z and EntryFunctionName fields
// for non-compute/non-mesh/non-amplification stages are zero-padded. See
// BUG-DXIL-009 for rationale.
//
//nolint:funlen,gocyclo,cyclop // PSV encoding requires many stage-specific branches
func EncodePSV0(info PSVInfo) []byte {
	// BUG-DXIL-009: always emit PSVRuntimeInfo3 (52 B). Modern dxil.dll
	// validators (1.6+) require this layout regardless of stage. Unused
	// fields are zero-padded.
	psvSize := uint32(runtimeInfo3Size)

	// Calculate total part size.
	resourceCount := uint32(len(info.ResourceBindings)) //nolint:gosec // bounded by IR global variable count

	// String table: 4-byte aligned.
	stringTableData := info.StringTable
	stringTableSize := uint32(len(stringTableData)) //nolint:gosec // bounded
	alignedStringTableSize := (stringTableSize + 3) & ^uint32(3)

	// Semantic index table.
	semIndexCount := uint32(len(info.SemanticIndexTable)) //nolint:gosec // bounded

	totalSize := 4 + psvSize + // psv_size field + runtime info
		4 + // resource_count
		4 + alignedStringTableSize + // string_table_size + data
		4 + semIndexCount*4 // sem_index_count + data

	// Resource bindings: when non-empty, DXC prefixes the array with a
	// 4-byte element-size field set to sizeof(PSVResourceBindInfo1).
	if resourceCount > 0 {
		totalSize += 4 + resourceCount*psvResourceBindInfo1Size
	}

	// Add PSV signature elements if present.
	numSigInputs := len(info.PSVSigInputs)
	numSigOutputs := len(info.PSVSigOutputs)
	if numSigInputs > 0 || numSigOutputs > 0 {
		totalSize += 4                                                       // element size field
		totalSize += uint32(numSigInputs) * uint32(psvSignatureElementSize)  //nolint:gosec // bounded
		totalSize += uint32(numSigOutputs) * uint32(psvSignatureElementSize) //nolint:gosec // bounded
	}

	// PSV dependency tables (BUG-DXIL-022 follow-up). dxil.dll's
	// SimplePSV well-formedness walker (DxilContainerValidation.cpp:609)
	// requires the PSV0 part end exactly at the computed offset after
	// these tables. Without them the validator emits "PSV0 part is not
	// well-formed". The content is computed via instruction analysis in
	// dxc; we emit the correct SIZE filled with zeros, which satisfies
	// the structural check and lets the deeper PSVContentVerifier run.
	depBytes := psvDependencyTableSize(info)
	totalSize += depBytes

	out := make([]byte, totalSize)
	pos := 0

	// 1. PSV runtime info size.
	binary.LittleEndian.PutUint32(out[pos:], psvSize)
	pos += 4

	// 2. PSV runtime info (PSVRuntimeInfo3, 52 bytes).
	// PSVRuntimeInfo0 (24 bytes):
	//   Bytes 0-15: stage-specific union (max 16 bytes padded)
	//   Bytes 16-19: min_expected_wave_lane_count
	//   Bytes 20-23: max_expected_wave_lane_count
	switch info.ShaderStage {
	case PSVVertex:
		if info.OutputPositionPresent {
			out[pos] = 1
		}
	case PSVPixel:
		if info.DepthOutput {
			out[pos] = 1
		}
		if info.SampleFrequency {
			out[pos+1] = 1
		}
	case PSVMesh:
		// Mesh shader union at offset 12-15:
		//   uint16 MaxOutputVertices (offset 12)
		//   uint16 MaxOutputPrimitives (offset 14)
		binary.LittleEndian.PutUint16(out[pos+12:], info.MaxOutputVertices)
		binary.LittleEndian.PutUint16(out[pos+14:], info.MaxOutputPrimitives)
	}
	binary.LittleEndian.PutUint32(out[pos+16:], info.MinWaveLaneCount)
	binary.LittleEndian.PutUint32(out[pos+20:], info.MaxWaveLaneCount)

	// PSVRuntimeInfo1 extension (12 bytes at offset 24):
	//   Byte 0: shader_stage
	//   Byte 1: uses_view_id
	//   Bytes 2-3: union (for mesh: byte 3 = mesh_output_topology)
	//   Byte 4: sig_input_elements
	//   Byte 5: sig_output_elements
	//   Byte 6: sig_patch_const_or_prim_elements
	//   Byte 7: sig_input_vectors
	//   Bytes 8-11: sig_output_vectors[4]
	out[pos+24] = uint8(info.ShaderStage)
	if info.UsesViewID {
		out[pos+25] = 1
	}
	if info.ShaderStage == PSVMesh {
		out[pos+27] = info.MeshOutputTopology // mesh output topology in union byte 3
	}

	// BUG-DXIL-009 self-consistency clamp + BUG-DXIL-019 architectural note:
	// the validator uses SigInputElements/SigOutputElements from RTI1 to
	// iterate PSVSignatureElement entries that follow the string and
	// semantic-index tables in PSV0. If we declare non-zero counts without
	// matching entries, the validator's payload walker fails with the
	// opaque "PSV0 part is not well-formed" error.
	//
	// Until BUG-DXIL-019 (populate PSVSigInputs/PSVSigOutputs in buildPSV
	// from the same data BUG-DXIL-012 puts in `!dx.entryPoints[0][2]`),
	// we clamp the counts to zero. Cost: PSV0 counts disagree with the
	// signature metadata in the bitcode, so the validator returns
	// HRESULT 0x80aa0013 with text:
	//   "DXIL container mismatch for 'SigOutputElements' between
	//    'PSV0' part:('0') and DXIL module:('1')"
	// — which is a clean, actionable error pointing at exactly the right
	// next step. Better than the opaque "not well-formed" we get if we
	// remove the clamp without populating entries.
	sigInputElements := info.SigInputElements
	sigOutputElements := info.SigOutputElements
	sigPatchElements := info.SigPatchConstOrPrimElements
	sigInputVectors := info.SigInputVectors
	sigOutputVectors := info.SigOutputVectors
	if len(info.PSVSigInputs) == 0 && len(info.PSVSigOutputs) == 0 {
		sigInputElements = 0
		sigOutputElements = 0
		sigPatchElements = 0
		sigInputVectors = 0
		sigOutputVectors = 0
	}
	out[pos+28] = sigInputElements
	out[pos+29] = sigOutputElements
	out[pos+30] = sigPatchElements
	out[pos+31] = sigInputVectors
	out[pos+32] = sigOutputVectors

	// PSVRuntimeInfo2 extension (12 bytes at offset 36).
	// NumThreadsX/Y/Z are zero for non-compute/mesh/amplification stages.
	binary.LittleEndian.PutUint32(out[pos+36:], info.NumThreadsX)
	binary.LittleEndian.PutUint32(out[pos+40:], info.NumThreadsY)
	binary.LittleEndian.PutUint32(out[pos+44:], info.NumThreadsZ)

	// PSVRuntimeInfo3 extension (4 bytes at offset 48) — EntryFunctionName.
	// Offset into the string table below; zero if no entry name is provided.
	binary.LittleEndian.PutUint32(out[pos+48:], info.EntryFunctionName)

	pos += int(psvSize)

	// 3. Resource count (+ per-record size prefix + bindings when non-empty).
	binary.LittleEndian.PutUint32(out[pos:], resourceCount)
	pos += 4
	if resourceCount > 0 {
		binary.LittleEndian.PutUint32(out[pos:], uint32(psvResourceBindInfo1Size))
		pos += 4
		for i := range info.ResourceBindings {
			rb := &info.ResourceBindings[i]
			binary.LittleEndian.PutUint32(out[pos:], uint32(rb.ResType))
			binary.LittleEndian.PutUint32(out[pos+4:], rb.Space)
			binary.LittleEndian.PutUint32(out[pos+8:], rb.LowerBound)
			binary.LittleEndian.PutUint32(out[pos+12:], rb.UpperBound)
			binary.LittleEndian.PutUint32(out[pos+16:], uint32(rb.ResKind))
			binary.LittleEndian.PutUint32(out[pos+20:], rb.ResFlags)
			pos += int(psvResourceBindInfo1Size)
		}
	}

	// 4. String table.
	binary.LittleEndian.PutUint32(out[pos:], alignedStringTableSize)
	pos += 4
	copy(out[pos:], stringTableData)
	pos += int(alignedStringTableSize)

	// 5. Semantic index table.
	binary.LittleEndian.PutUint32(out[pos:], semIndexCount)
	pos += 4
	for _, idx := range info.SemanticIndexTable {
		binary.LittleEndian.PutUint32(out[pos:], idx)
		pos += 4
	}

	// 6. PSV signature elements.
	if numSigInputs > 0 || numSigOutputs > 0 {
		binary.LittleEndian.PutUint32(out[pos:], uint32(psvSignatureElementSize))
		pos += 4

		for i := range info.PSVSigInputs {
			pos = writePSVSigElement(out, pos, &info.PSVSigInputs[i])
		}
		for i := range info.PSVSigOutputs {
			pos = writePSVSigElement(out, pos, &info.PSVSigOutputs[i])
		}
	}

	// Dependency tables (BUG-DXIL-011 Phase 3 / BUG-DXIL-018 follow-up).
	// Content comes from info.InputToOutputTable, populated by buildPSV
	// via the dxil/internal/viewid dataflow analyzer. Layout mirrors
	// PSVDependencyTable exactly:
	//
	//   pData[inputComp * MaskDwordsForComponents(outVectors)] = bitmap
	//
	// where inputComp = inputVectorRow * 4 + col.
	writePSVDepTable(out, pos, depBytes, sigInputVectors, sigOutputVectors, info.InputToOutputTable)
	_ = depBytes // bytes accounted for in totalSize allocation above

	return out
}

// writePSVDepTable writes the per-input-component dependency bitmap at
// `pos`, bounded by `depBytes`. If `table` is nil or empty the region is
// left zero-filled (make() default), matching our old probe behavior.
//
// Layout per PSVDependencyTable (DxilPipelineStateValidation.h:295):
//   - Table size in dwords = MaskDwordsForComponents(sigOut) * 4 * sigIn
//   - Indexed as table[inputComp * maskDwords .. (inputComp+1)*maskDwords)
//   - Each bit `c` set means this input component influences output
//     component `c`.
//
// BUG-DXIL-018 Phase 3: replaces the session-11 probe that hard-coded
// 0x03 at the first dword for 1-in/1-out shaders. The IR-level
// viewid.Analyze pass-through analyzer now produces the same bits (and
// more) for real dataflow, so the probe is no longer needed.
func writePSVDepTable(out []byte, pos int, depBytes uint32, sigIn, sigOut uint8, table []uint32) {
	if depBytes < 4 || sigIn == 0 || sigOut == 0 {
		return
	}
	for i, v := range table {
		off := pos + i*4
		if uint32(off-pos) >= depBytes { //nolint:gosec // off-pos = i*4, bounded by table length * 4
			break
		}
		if off+4 > len(out) {
			break
		}
		binary.LittleEndian.PutUint32(out[off:], v)
	}
}

// psvComputeMaskDwordsFromVectors mirrors the DXC helper of the same
// name: returns ceil(Vectors/8) — one dword per up-to-8 vectors.
func psvComputeMaskDwordsFromVectors(vectors uint32) uint32 {
	return (vectors + 7) >> 3
}

// psvComputeInputOutputTableDwords mirrors the DXC helper: each input
// component (4 per vector) gets one bitmask spanning all output
// components for that stream, packed at MaskDwords(outVectors) per row.
func psvComputeInputOutputTableDwords(in, out uint32) uint32 {
	return psvComputeMaskDwordsFromVectors(out) * in * 4
}

// psvDependencyTableSize returns the total byte size of the dependency
// tables that must follow the signature elements in PSV0. Mirrors the
// SimplePSV walker in DxilContainerValidation.cpp:609. We zero-fill
// them — the structural well-formedness check only depends on size.
func psvDependencyTableSize(info PSVInfo) uint32 {
	if len(info.PSVSigInputs) == 0 && len(info.PSVSigOutputs) == 0 {
		return 0
	}
	const numStreams = 4
	// SigOutputVectors is currently a single uint8 covering stream 0;
	// streams 1-3 are zero (no GS support yet). Mirror the layout the
	// validator expects regardless.
	sigOutputVectors := [numStreams]uint32{uint32(info.SigOutputVectors), 0, 0, 0}
	sigInputVectors := uint32(info.SigInputVectors)

	var size uint32
	// 1. Optional ViewID dependency tables — only when UsesViewID is set.
	//    We never set UsesViewID, so this block is currently dead, but
	//    kept for future-proofing.
	//
	// 2. Always: per-stream Input→Output dependency tables.
	for _, ov := range sigOutputVectors {
		if ov == 0 {
			continue
		}
		size += 4 * psvComputeInputOutputTableDwords(sigInputVectors, ov)
	}
	return size
}

func writePSVSigElement(out []byte, pos int, elem *PSVSignatureElement) int {
	binary.LittleEndian.PutUint32(out[pos:], elem.SemanticNameOffset)
	binary.LittleEndian.PutUint32(out[pos+4:], elem.SemanticIndexesOffset)
	out[pos+8] = elem.Rows
	out[pos+9] = elem.StartRow
	out[pos+10] = elem.ColsAndStart
	out[pos+11] = elem.SemanticKind
	out[pos+12] = elem.ComponentType
	out[pos+13] = elem.InterpolationMode
	out[pos+14] = elem.DynamicMaskAndStream
	out[pos+15] = elem.Reserved
	return pos + psvSignatureElementSize
}

// AddPSV0 adds a PSV0 part to the container.
func (c *Container) AddPSV0(info PSVInfo) {
	data := EncodePSV0(info)
	c.parts = append(c.parts, part{fourCC: FourCCPSV0, data: data})
}

// BuildPSVStringTable constructs a minimal PSV0 string table containing the
// entry function name and returns the byte slice plus the offset of the entry
// name within the table.
//
// Layout mirrors the convention used by DXC for non-mesh shaders:
//
//	offset 0: '\0'             — empty string for signature elements without
//	                              explicit semantic names
//	offset 1: entryName + '\0' — the shader entry point name
//	offset n: zero padding up to 4-byte alignment
//
// The returned offset is always 1 (the position immediately after the leading
// '\0'). Caller assigns it to PSVInfo.EntryFunctionName and the slice to
// PSVInfo.StringTable.
//
// If entryName is empty the table contains just a single '\0' aligned to 4
// bytes and the returned offset is 0.
func BuildPSVStringTable(entryName string) (table []byte, entryOffset uint32) {
	if entryName == "" {
		return []byte{0, 0, 0, 0}, 0
	}
	// Leading '\0' + entry name + null terminator.
	table = make([]byte, 0, 1+len(entryName)+1+3)
	table = append(table, 0)
	entryOffset = uint32(len(table)) //nolint:gosec // bounded by entryName length
	table = append(table, entryName...)
	table = append(table, 0)
	// 4-byte align.
	for len(table)%4 != 0 {
		table = append(table, 0)
	}
	return table, entryOffset
}
