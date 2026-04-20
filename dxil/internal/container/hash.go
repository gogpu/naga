// Package container implements DXIL container hashing.
//
// This file implements the BYPASS hash sentinel and the retail hash
// algorithm from Microsoft's INF-0004 specification.

package container

import (
	"crypto/md5" //nolint:gosec // MD5 is the DXBC-specified shader-hash digest, not a security primitive
	"encoding/binary"
)

// BypassHash is the BYPASS sentinel hash value.
// When this 16-byte value is placed in the container header's digest field,
// the D3D12 runtime (AgilitySDK 1.615+) allows the shader to execute
// without validating the hash.
//
// Reference: https://github.com/microsoft/hlsl-specs/blob/main/proposals/infra/INF-0004-validator-hashing.md
var BypassHash = [16]byte{
	0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01,
	0x01, 0x01, 0x01, 0x01,
}

// PreviewBypassHash is the PREVIEW_BYPASS sentinel.
// Shaders with this hash only execute when developer mode and
// D3D12ExperimentalShaderModels are enabled.
var PreviewBypassHash = [16]byte{
	0x02, 0x02, 0x02, 0x02,
	0x02, 0x02, 0x02, 0x02,
	0x02, 0x02, 0x02, 0x02,
	0x02, 0x02, 0x02, 0x02,
}

// SetBypassHash writes the BYPASS sentinel hash into the container's
// digest field (bytes 4-19 of the DXBC header).
func SetBypassHash(containerData []byte) {
	if len(containerData) < 20 {
		return
	}
	copy(containerData[4:20], BypassHash[:])
}

// WriteShaderHashPart fills the HASH part body (flags=0, digest=MD5(bitcode))
// inside the given container. The "shader hash" stored in the HASH part is
// a standard MD5 (NOT the retail modified MD5) of the raw LLVM bitcode
// content — everything from the bitcode wrapper magic (0x42 0x43 0xC0 0xDE)
// through the end of the bitcode stream. The bitcode content is the payload
// of the DXIL part's inner bitcode wrapper (DxilProgramHeader followed by
// BitcodeHeader: BitcodeOffset and BitcodeSize fields point to it).
//
// Without this value set correctly, D3D12 rejects graphics pipelines with
// "Shader is corrupt" (HRESULT 0x80070057) even when the container hash
// and bitcode are valid.
//
// Reference: empirically confirmed that DXC's HASH part body equals
// md5(DXIL_bitcode_only) for its compiled shaders. Flags byte is retail (0).
func WriteShaderHashPart(containerData []byte) error {
	if len(containerData) < 32 {
		return nil
	}
	// Locate the DXIL part and HASH part.
	partCount := binary.LittleEndian.Uint32(containerData[28:32])
	partOffsets := make([]uint32, partCount)
	for i := range partOffsets {
		partOffsets[i] = binary.LittleEndian.Uint32(containerData[32+i*4 : 36+i*4])
	}
	var dxilPartBody []byte
	var hashPartBody []byte
	for _, off := range partOffsets {
		if off+8 > uint32(len(containerData)) { //nolint:gosec // DXIL containers are always < 4GB; len is bounded
			continue
		}
		fcc := containerData[off : off+4]
		partSize := binary.LittleEndian.Uint32(containerData[off+4 : off+8])
		body := containerData[off+8 : off+8+partSize]
		if string(fcc) == "DXIL" {
			dxilPartBody = body
		} else if string(fcc) == "HASH" {
			hashPartBody = body
		}
	}
	if dxilPartBody == nil || hashPartBody == nil || len(hashPartBody) < 20 {
		return nil
	}
	// DxilProgramHeader: ProgramVersion(4) + SizeInUint32(4) + DxilMagic(4)
	// + DxilVersion(4) + BitcodeOffset(4) + BitcodeSize(4). Bitcode starts
	// at DxilProgramHeader + 8 + BitcodeOffset (offset is relative to start
	// of bitcode header at +8).
	if len(dxilPartBody) < 24 {
		return nil
	}
	bcOffset := binary.LittleEndian.Uint32(dxilPartBody[16:20]) + 8
	bcSize := binary.LittleEndian.Uint32(dxilPartBody[20:24])
	if uint32(len(dxilPartBody)) < bcOffset+bcSize { //nolint:gosec // DXIL containers are always < 4GB; len is bounded
		return nil
	}
	bitcode := dxilPartBody[bcOffset : bcOffset+bcSize]
	//nolint:gosec // MD5 is the DXBC-specified shader-hash digest
	digest := md5.Sum(bitcode)
	// Flags = 0 (retail format).
	for i := 0; i < 4; i++ {
		hashPartBody[i] = 0
	}
	copy(hashPartBody[4:20], digest[:])
	return nil
}

// ComputeRetailHash computes the retail (modified MD5) hash of the
// container data and writes it into the digest field (bytes 4..20).
//
// This implements the "Retail Hash" algorithm from INF-0004, which is
// a modified MD5 that puts the byte count at x[0] instead of x[14],
// and uses a different x[15] encoding.
//
// The hash input spans bytes 20..end (starting at the Version field
// in DxilContainerHeader). DXC's DxcContainerBuilder.cpp:189-192:
//
//	HashStartOffset = offsetof(DxilContainerHeader, Version) = 20
//	DataToHash = ContainerHeader + HashStartOffset
//	AmountToHash = ContainerSizeInBytes - HashStartOffset
//
// NOT the whole buffer with the digest zeroed — that produced hashes
// that D3D12 runtime rejected as mismatched and returned id=67/93
// "shader is corrupt" even for otherwise valid DXIL. Using the DXC
// start offset yields byte-for-byte matching hashes against DXC-
// compiled golden blobs.
func ComputeRetailHash(containerData []byte) {
	if len(containerData) < 20 {
		return
	}
	hash := retailMD5(containerData[20:])
	copy(containerData[4:20], hash[:])
}

// retailMD5 computes the modified MD5 hash per INF-0004.
//
//nolint:gosec // intentional uint32<->byte conversions throughout (MD5 algorithm)
func retailMD5(data []byte) [16]byte {
	byteCount := uint32(len(data))
	leftOver := byteCount & 0x3f
	var padAmount uint32
	twoRowsPadding := false
	if leftOver < 56 {
		padAmount = 56 - leftOver
	} else {
		padAmount = 120 - leftOver
		twoRowsPadding = true
	}

	state := [4]uint32{0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476}
	n := (byteCount + padAmount + 8) >> 6

	offset := uint32(0)
	nextEndState := n - 1
	if twoRowsPadding {
		nextEndState = n - 2
	}

	for i := uint32(0); i < n; i++ {
		pX := retailMD5Block(data, byteCount, offset, i, n, &nextEndState,
			twoRowsPadding, padAmount)
		md5Transform(&state, pX)
		offset += 64
	}

	var result [16]byte
	binary.LittleEndian.PutUint32(result[0:], state[0])
	binary.LittleEndian.PutUint32(result[4:], state[1])
	binary.LittleEndian.PutUint32(result[8:], state[2])
	binary.LittleEndian.PutUint32(result[12:], state[3])
	return result
}

// retailMD5Block prepares the 16-word input block for iteration i.
func retailMD5Block(data []byte, byteCount, offset, i, n uint32,
	nextEndState *uint32, twoRowsPadding bool, padAmount uint32,
) []uint32 {
	if i != *nextEndState {
		return bytesToUint32s(data[offset : offset+64])
	}

	var x [16]uint32
	if !twoRowsPadding && i == n-1 {
		remainder := byteCount - offset
		x[0] = byteCount << 3
		copyBytesToUint32s(x[:], 4, data[offset:offset+remainder])
		padIntoUint32s(x[:], 4+remainder, padAmount)
		x[15] = 1 | (byteCount << 1)
	} else if twoRowsPadding {
		if i == n-2 {
			remainder := byteCount - offset
			copyBytesToUint32s(x[:], 0, data[offset:offset+remainder])
			padIntoUint32s(x[:], remainder, padAmount-56)
			*nextEndState = n - 1
		} else if i == n-1 {
			x[0] = byteCount << 3
			padIntoUint32sFromOffset(x[:], 4, padAmount-56)
			x[15] = 1 | (byteCount << 1)
		}
	}
	return x[:]
}

// md5Transform performs one MD5 block transformation on state using
// the 16-word input block pX. This is a direct port of the MD5 algorithm
// from RFC 1321 / INF-0004 and cannot be meaningfully decomposed.
//
//nolint:funlen // MD5 algorithm: 64 fixed operations per RFC 1321
func md5Transform(state *[4]uint32, pX []uint32) {
	a, b, c, d := state[0], state[1], state[2], state[3]

	// Round 1
	a = ff(a, b, c, d, pX[0], 7, 0xd76aa478)
	d = ff(d, a, b, c, pX[1], 12, 0xe8c7b756)
	c = ff(c, d, a, b, pX[2], 17, 0x242070db)
	b = ff(b, c, d, a, pX[3], 22, 0xc1bdceee)
	a = ff(a, b, c, d, pX[4], 7, 0xf57c0faf)
	d = ff(d, a, b, c, pX[5], 12, 0x4787c62a)
	c = ff(c, d, a, b, pX[6], 17, 0xa8304613)
	b = ff(b, c, d, a, pX[7], 22, 0xfd469501)
	a = ff(a, b, c, d, pX[8], 7, 0x698098d8)
	d = ff(d, a, b, c, pX[9], 12, 0x8b44f7af)
	c = ff(c, d, a, b, pX[10], 17, 0xffff5bb1)
	b = ff(b, c, d, a, pX[11], 22, 0x895cd7be)
	a = ff(a, b, c, d, pX[12], 7, 0x6b901122)
	d = ff(d, a, b, c, pX[13], 12, 0xfd987193)
	c = ff(c, d, a, b, pX[14], 17, 0xa679438e)
	b = ff(b, c, d, a, pX[15], 22, 0x49b40821)

	// Round 2
	a = gg(a, b, c, d, pX[1], 5, 0xf61e2562)
	d = gg(d, a, b, c, pX[6], 9, 0xc040b340)
	c = gg(c, d, a, b, pX[11], 14, 0x265e5a51)
	b = gg(b, c, d, a, pX[0], 20, 0xe9b6c7aa)
	a = gg(a, b, c, d, pX[5], 5, 0xd62f105d)
	d = gg(d, a, b, c, pX[10], 9, 0x02441453)
	c = gg(c, d, a, b, pX[15], 14, 0xd8a1e681)
	b = gg(b, c, d, a, pX[4], 20, 0xe7d3fbc8)
	a = gg(a, b, c, d, pX[9], 5, 0x21e1cde6)
	d = gg(d, a, b, c, pX[14], 9, 0xc33707d6)
	c = gg(c, d, a, b, pX[3], 14, 0xf4d50d87)
	b = gg(b, c, d, a, pX[8], 20, 0x455a14ed)
	a = gg(a, b, c, d, pX[13], 5, 0xa9e3e905)
	d = gg(d, a, b, c, pX[2], 9, 0xfcefa3f8)
	c = gg(c, d, a, b, pX[7], 14, 0x676f02d9)
	b = gg(b, c, d, a, pX[12], 20, 0x8d2a4c8a)

	// Round 3
	a = hh(a, b, c, d, pX[5], 4, 0xfffa3942)
	d = hh(d, a, b, c, pX[8], 11, 0x8771f681)
	c = hh(c, d, a, b, pX[11], 16, 0x6d9d6122)
	b = hh(b, c, d, a, pX[14], 23, 0xfde5380c)
	a = hh(a, b, c, d, pX[1], 4, 0xa4beea44)
	d = hh(d, a, b, c, pX[4], 11, 0x4bdecfa9)
	c = hh(c, d, a, b, pX[7], 16, 0xf6bb4b60)
	b = hh(b, c, d, a, pX[10], 23, 0xbebfbc70)
	a = hh(a, b, c, d, pX[13], 4, 0x289b7ec6)
	d = hh(d, a, b, c, pX[0], 11, 0xeaa127fa)
	c = hh(c, d, a, b, pX[3], 16, 0xd4ef3085)
	b = hh(b, c, d, a, pX[6], 23, 0x04881d05)
	a = hh(a, b, c, d, pX[9], 4, 0xd9d4d039)
	d = hh(d, a, b, c, pX[12], 11, 0xe6db99e5)
	c = hh(c, d, a, b, pX[15], 16, 0x1fa27cf8)
	b = hh(b, c, d, a, pX[2], 23, 0xc4ac5665)

	// Round 4
	a = ii(a, b, c, d, pX[0], 6, 0xf4292244)
	d = ii(d, a, b, c, pX[7], 10, 0x432aff97)
	c = ii(c, d, a, b, pX[14], 15, 0xab9423a7)
	b = ii(b, c, d, a, pX[5], 21, 0xfc93a039)
	a = ii(a, b, c, d, pX[12], 6, 0x655b59c3)
	d = ii(d, a, b, c, pX[3], 10, 0x8f0ccc92)
	c = ii(c, d, a, b, pX[10], 15, 0xffeff47d)
	b = ii(b, c, d, a, pX[1], 21, 0x85845dd1)
	a = ii(a, b, c, d, pX[8], 6, 0x6fa87e4f)
	d = ii(d, a, b, c, pX[15], 10, 0xfe2ce6e0)
	c = ii(c, d, a, b, pX[6], 15, 0xa3014314)
	b = ii(b, c, d, a, pX[13], 21, 0x4e0811a1)
	a = ii(a, b, c, d, pX[4], 6, 0xf7537e82)
	d = ii(d, a, b, c, pX[11], 10, 0xbd3af235)
	c = ii(c, d, a, b, pX[2], 15, 0x2ad7d2bb)
	b = ii(b, c, d, a, pX[9], 21, 0xeb86d391)

	state[0] += a
	state[1] += b
	state[2] += c
	state[3] += d
}

// MD5 round helper functions.

func ff(a, b, c, d, x uint32, s uint8, ac uint32) uint32 {
	a += ((b & c) | (^b & d)) + x + ac
	a = (a << s) | (a >> (32 - s))
	return a + b
}

func gg(a, b, c, d, x uint32, s uint8, ac uint32) uint32 {
	a += ((b & d) | (c & ^d)) + x + ac
	a = (a << s) | (a >> (32 - s))
	return a + b
}

func hh(a, b, c, d, x uint32, s uint8, ac uint32) uint32 {
	a += (b ^ c ^ d) + x + ac
	a = (a << s) | (a >> (32 - s))
	return a + b
}

func ii(a, b, c, d, x uint32, s uint8, ac uint32) uint32 {
	a += (c ^ (b | ^d)) + x + ac
	a = (a << s) | (a >> (32 - s))
	return a + b
}

// MD5 padding constant.
var md5Padding = [64]byte{0x80}

// copyBytesToUint32s copies src bytes into dst uint32 array starting
// at byte offset byteOff.
func copyBytesToUint32s(dst []uint32, byteOff uint32, src []byte) {
	buf := uint32sToBytes(dst)
	copy(buf[byteOff:], src)
	for i := range dst {
		dst[i] = bytesToUint32LE(buf[i*4:])
	}
}

// padIntoUint32s writes MD5 padding into dst starting at byte offset.
func padIntoUint32s(dst []uint32, byteOff, padLen uint32) {
	buf := uint32sToBytes(dst)
	for i := uint32(0); i < padLen && i < uint32(len(md5Padding)); i++ {
		if int(byteOff+i) < len(buf) {
			buf[byteOff+i] = md5Padding[i]
		}
	}
	for i := range dst {
		dst[i] = bytesToUint32LE(buf[i*4:])
	}
}

// padIntoUint32sFromOffset writes MD5 padding from a given padding offset.
func padIntoUint32sFromOffset(dst []uint32, byteOff uint32, padOffset uint32) {
	buf := uint32sToBytes(dst)
	remaining := uint32(len(md5Padding)) - padOffset
	for i := uint32(0); i < remaining; i++ {
		idx := int(byteOff + i)
		if idx < len(buf) {
			buf[idx] = md5Padding[padOffset+i]
		}
	}
	for i := range dst {
		dst[i] = bytesToUint32LE(buf[i*4:])
	}
}

func uint32sToBytes(u []uint32) []byte {
	buf := make([]byte, len(u)*4)
	for i, v := range u {
		binary.LittleEndian.PutUint32(buf[i*4:], v)
	}
	return buf
}

func bytesToUint32s(data []byte) []uint32 {
	n := len(data) / 4
	result := make([]uint32, n)
	for i := 0; i < n; i++ {
		result[i] = bytesToUint32LE(data[i*4:])
	}
	return result
}

func bytesToUint32LE(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
