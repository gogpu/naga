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

// PSVShaderKind matches DXIL's PSVShaderKind enum (used in PSV1.ShaderStage).
type PSVShaderKind uint8

const (
	PSVVertex        PSVShaderKind = 0
	PSVPixel         PSVShaderKind = 1
	PSVCompute       PSVShaderKind = 5
	PSVMesh          PSVShaderKind = 13
	PSVAmplification PSVShaderKind = 14
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

	// Semantic string table entries (null-terminated, will be 4-byte aligned).
	StringTable []byte

	// Semantic index table entries.
	SemanticIndexTable []uint32

	// PSV signature elements (optional).
	PSVSigInputs  []PSVSignatureElement
	PSVSigOutputs []PSVSignatureElement
}

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

// runtimeInfo1Size is the wire size of PSVRuntimeInfo1 as defined by Mesa.
// PSVRuntimeInfo0 = 24 bytes (union 14 bytes padded to 16 + 4 + 4)
// PSVRuntimeInfo1 = PSVRuntimeInfo0(24) + 1(stage) + 1(uses_view_id) +
//
//	2(union) + 3(sig elements) + 1(sig_input_vectors) + 4(sig_output_vectors[4]) = 36
const runtimeInfo1Size = 36

// runtimeInfo2Size is the wire size of PSVRuntimeInfo2.
// PSVRuntimeInfo2 = PSVRuntimeInfo1(36) + 3*4(numthreads) = 48
const runtimeInfo2Size = 48

// runtimeInfo3Size is the wire size of PSVRuntimeInfo3.
// PSVRuntimeInfo3 = PSVRuntimeInfo2(48) + 4(EntryFunctionName) = 52
// Required for validator 1.8+ (which checks entry point name in PSV).
const runtimeInfo3Size = 52

// EncodePSV0 serializes PSVInfo into the PSV0 part binary format.
//
//nolint:gocyclo,cyclop,funlen // PSV encoding requires many stage-specific branches
func EncodePSV0(info PSVInfo) []byte {
	// Use PSVRuntimeInfo3 for stages with numthreads + entry name (mesh/amplification/compute),
	// PSVRuntimeInfo1 for others.
	psvSize := uint32(runtimeInfo1Size)
	if info.ShaderStage == PSVCompute || info.ShaderStage == PSVMesh || info.ShaderStage == PSVAmplification {
		if info.EntryFunctionName > 0 || len(info.StringTable) > 0 {
			psvSize = uint32(runtimeInfo3Size) // includes EntryFunctionName
		} else {
			psvSize = uint32(runtimeInfo2Size)
		}
	}

	// Calculate total part size.
	resourceCount := uint32(0) // no resource bindings for now

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

	// Add PSV signature elements if present.
	numSigInputs := len(info.PSVSigInputs)
	numSigOutputs := len(info.PSVSigOutputs)
	if numSigInputs > 0 || numSigOutputs > 0 {
		totalSize += 4                                                       // element size field
		totalSize += uint32(numSigInputs) * uint32(psvSignatureElementSize)  //nolint:gosec // bounded
		totalSize += uint32(numSigOutputs) * uint32(psvSignatureElementSize) //nolint:gosec // bounded
	}

	out := make([]byte, totalSize)
	pos := 0

	// 1. PSV runtime info size.
	binary.LittleEndian.PutUint32(out[pos:], psvSize)
	pos += 4

	// 2. PSV runtime info (PSVRuntimeInfo1).
	// PSVRuntimeInfo0 (24 bytes):
	//   Bytes 0-15: stage-specific union (max 16 bytes padded)
	//   Bytes 16-19: min_expected_wave_lane_count
	//   Bytes 20-23: max_expected_wave_lane_count
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
	// uses_view_id = 0
	if info.ShaderStage == PSVMesh {
		out[pos+27] = info.MeshOutputTopology // mesh output topology in union byte 3
	}
	out[pos+28] = info.SigInputElements
	out[pos+29] = info.SigOutputElements
	out[pos+30] = info.SigPatchConstOrPrimElements
	out[pos+31] = info.SigInputVectors
	out[pos+32] = info.SigOutputVectors

	// PSVRuntimeInfo2 extension (12 bytes at offset 36) — for compute/mesh/amplification.
	if psvSize >= runtimeInfo2Size {
		binary.LittleEndian.PutUint32(out[pos+36:], info.NumThreadsX)
		binary.LittleEndian.PutUint32(out[pos+40:], info.NumThreadsY)
		binary.LittleEndian.PutUint32(out[pos+44:], info.NumThreadsZ)
	}

	// PSVRuntimeInfo3 extension (4 bytes at offset 48) — EntryFunctionName.
	if psvSize >= runtimeInfo3Size {
		binary.LittleEndian.PutUint32(out[pos+48:], info.EntryFunctionName)
	}
	pos += int(psvSize)

	// 3. Resource count.
	binary.LittleEndian.PutUint32(out[pos:], resourceCount)
	pos += 4

	// No resource bindings (resourceCount == 0).

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

	return out
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
