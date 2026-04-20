// Package bitcheck implements a minimal LLVM 3.7 bitstream reader that
// walks a DXIL container far enough to verify the `!dx.entryPoints` named
// metadata is well-formed — specifically that every entry-point tuple has
// a non-null function reference in operand 0.
//
// Microsoft's IDxcValidator (dxil.dll) crashes with an access violation
// at dxil.dll+0xe9da (NULL+0x18) when it walks entry-point metadata and
// encounters a null function reference. This package runs as a defensive
// pre-check inside the dxcvalidator wrapper — ANY input (naga output,
// DXC output, third-party tool output, hand-crafted garbage) is scanned
// BEFORE being handed to dxil.dll. Malformed inputs return a clean Go
// error instead of triggering the AV.
//
// The reader is intentionally scoped — it only understands enough of the
// LLVM 3.7 bitstream format to:
//
//  1. Unwrap the DXBC container and find the DXIL part
//  2. Unwrap the DxilProgramHeader and find the bitcode bytes
//  3. Enumerate top-level blocks to find MODULE_BLOCK (id 8)
//  4. Inside MODULE_BLOCK, find METADATA_BLOCK (id 15)
//  5. Decode METADATA_NAME / METADATA_NAMED_NODE / METADATA_NODE /
//     METADATA_OLD_NODE / METADATA_VALUE records sufficient to identify
//     the "dx.entryPoints" named metadata and walk its operand tuples
//  6. Verify operand 0 of each tuple is a non-null METADATA_VALUE that
//     references a function by index (not encoded as null)
//
// Everything else — function bodies, constants, types, instruction streams,
// non-metadata blocks — is skipped via block-length fast-forward. This is
// NOT a general-purpose LLVM bitcode parser; it is a targeted hardening
// layer for one specific AV class.
//
// LLVM 3.7 bitstream reference:
// https://releases.llvm.org/3.7.1/docs/BitCodeFormat.html
//
// The symmetry with dxil/internal/bitcode/writer.go (our emitter's bit-
// level writer) is intentional — both implement the same primitives from
// opposite directions.
package bitcheck
