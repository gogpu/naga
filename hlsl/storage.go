// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package-level nolint for storage functions prepared for future integration.
// These functions implement HLSL buffer and atomic operations and will be used
// when the full statement codegen calls storage operations.
//
//nolint:unused // Functions prepared for integration in statements phase
package hlsl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Buffer Type Constants
// =============================================================================

// HLSL buffer type constants.
const (
	// Byte address buffer types (raw buffer access)
	hlslByteAddressBuffer   = "ByteAddressBuffer"
	hlslRWByteAddressBuffer = "RWByteAddressBuffer"

	// Structured buffer types (typed buffer access)
	hlslStructuredBuffer   = "StructuredBuffer"
	hlslRWStructuredBuffer = "RWStructuredBuffer"

	// Constant buffer type
	hlslCBuffer = "cbuffer"

	// Append/Consume buffer types
	hlslAppendStructuredBuffer  = "AppendStructuredBuffer"
	hlslConsumeStructuredBuffer = "ConsumeStructuredBuffer"
)

// HLSL atomic intrinsic names.
const (
	hlslInterlockedAdd             = "InterlockedAdd"
	hlslInterlockedAnd             = "InterlockedAnd"
	hlslInterlockedOr              = "InterlockedOr"
	hlslInterlockedXor             = "InterlockedXor"
	hlslInterlockedMin             = "InterlockedMin"
	hlslInterlockedMax             = "InterlockedMax"
	hlslInterlockedExchange        = "InterlockedExchange"
	hlslInterlockedCompareExchange = "InterlockedCompareExchange"
	hlslInterlockedCompareStore    = "InterlockedCompareStore"
)

// =============================================================================
// Buffer Type Generation
// =============================================================================

// writeByteAddressBuffer writes a ByteAddressBuffer or RWByteAddressBuffer declaration.
// ByteAddressBuffer provides raw byte-level access to buffer data.
//
// HLSL syntax:
//
//	ByteAddressBuffer buf : register(t0);         // Read-only
//	RWByteAddressBuffer buf : register(u0);       // Read-write
func (w *Writer) writeByteAddressBuffer(name string, binding *BindTarget, readOnly bool) {
	bufType := hlslRWByteAddressBuffer
	regType := "u"
	if readOnly {
		bufType = hlslByteAddressBuffer
		regType = "t"
	}

	if binding != nil {
		w.writeLine("%s %s : register(%s%d, space%d);", bufType, name, regType, binding.Register, binding.Space)
	} else {
		w.writeLine("%s %s;", bufType, name)
	}
}

// writeStructuredBuffer writes a StructuredBuffer or RWStructuredBuffer declaration.
// StructuredBuffer provides typed access to buffer data.
//
// HLSL syntax:
//
//	StructuredBuffer<T> buf : register(t0);       // Read-only
//	RWStructuredBuffer<T> buf : register(u0);     // Read-write
func (w *Writer) writeStructuredBuffer(name, elementType string, binding *BindTarget, readOnly bool) {
	bufType := hlslRWStructuredBuffer
	regType := "u"
	if readOnly {
		bufType = hlslStructuredBuffer
		regType = "t"
	}

	if binding != nil {
		w.writeLine("%s<%s> %s : register(%s%d, space%d);", bufType, elementType, name, regType, binding.Register, binding.Space)
	} else {
		w.writeLine("%s<%s> %s;", bufType, elementType, name)
	}
}

// writeConstantBuffer writes a cbuffer declaration.
// Constant buffers are optimized for read-only access patterns.
//
// HLSL syntax:
//
//	cbuffer Name : register(b0, space0) {
//	    float4x4 mvp;
//	    float4 color;
//	};
func (w *Writer) writeConstantBuffer(name string, members []cbufferMember, binding *BindTarget) {
	if binding != nil {
		w.writeLine("%s %s : register(b%d, space%d) {", hlslCBuffer, name, binding.Register, binding.Space)
	} else {
		w.writeLine("%s %s {", hlslCBuffer, name)
	}
	w.pushIndent()

	for i := range members {
		member := &members[i]
		w.writeLine("%s %s;", member.typeName, member.name)
	}

	w.popIndent()
	w.writeLine("};")
}

// cbufferMember represents a member in a constant buffer.
type cbufferMember struct {
	name     string
	typeName string
}

// =============================================================================
// Buffer Load Operations
// =============================================================================

// writeBufferLoad writes a buffer load operation.
// ByteAddressBuffer uses Load, Load2, Load3, Load4 methods.
//
// HLSL syntax:
//
//	uint val = buf.Load(offset);
//	uint2 vals = buf.Load2(offset);
//	uint3 vals = buf.Load3(offset);
//	uint4 vals = buf.Load4(offset);
func (w *Writer) writeBufferLoad(bufferExpr string, offset string, components int) {
	switch components {
	case 1:
		fmt.Fprintf(&w.out, "%s.Load(%s)", bufferExpr, offset)
	case 2:
		fmt.Fprintf(&w.out, "%s.Load2(%s)", bufferExpr, offset)
	case 3:
		fmt.Fprintf(&w.out, "%s.Load3(%s)", bufferExpr, offset)
	default:
		fmt.Fprintf(&w.out, "%s.Load4(%s)", bufferExpr, offset)
	}
}

// writeBufferLoadT writes a template buffer load operation (SM 5.1+).
// Template loads allow loading arbitrary types from byte address buffers.
//
// HLSL syntax:
//
//	T val = buf.Load<T>(offset);
func (w *Writer) writeBufferLoadT(bufferExpr, typeName, offset string) {
	fmt.Fprintf(&w.out, "%s.Load<%s>(%s)", bufferExpr, typeName, offset)
}

// =============================================================================
// Buffer Store Operations
// =============================================================================

// writeBufferStore writes a buffer store operation.
// RWByteAddressBuffer uses Store, Store2, Store3, Store4 methods.
//
// HLSL syntax:
//
//	buf.Store(offset, value);
//	buf.Store2(offset, values);
//	buf.Store3(offset, values);
//	buf.Store4(offset, values);
func (w *Writer) writeBufferStore(bufferExpr, offset, value string, components int) {
	switch components {
	case 1:
		w.writeLine("%s.Store(%s, %s);", bufferExpr, offset, value)
	case 2:
		w.writeLine("%s.Store2(%s, %s);", bufferExpr, offset, value)
	case 3:
		w.writeLine("%s.Store3(%s, %s);", bufferExpr, offset, value)
	default:
		w.writeLine("%s.Store4(%s, %s);", bufferExpr, offset, value)
	}
}

// =============================================================================
// Atomic Operations
// =============================================================================

// writeAtomicOp writes an atomic operation intrinsic call.
// Atomic operations provide thread-safe access to shared data.
//
// HLSL syntax:
//
//	InterlockedAdd(dest, value, originalValue);
//	InterlockedAnd(dest, value, originalValue);
//	InterlockedOr(dest, value, originalValue);
//	InterlockedXor(dest, value, originalValue);
//	InterlockedMin(dest, value, originalValue);
//	InterlockedMax(dest, value, originalValue);
func (w *Writer) writeAtomicOp(fun ir.AtomicFunction, dest, value string, result *string) error {
	intrinsic, err := atomicFunctionToHLSL(fun)
	if err != nil {
		return err
	}

	if result != nil {
		w.writeLine("%s(%s, %s, %s);", intrinsic, dest, value, *result)
	} else {
		w.writeLine("%s(%s, %s);", intrinsic, dest, value)
	}
	return nil
}

// writeAtomicCompareExchange writes an atomic compare-exchange operation.
// Compares dest with compare, and if equal, replaces dest with value.
//
// HLSL syntax:
//
//	InterlockedCompareExchange(dest, compare, value, originalValue);
func (w *Writer) writeAtomicCompareExchange(dest, compare, value, result string) {
	w.writeLine("%s(%s, %s, %s, %s);", hlslInterlockedCompareExchange, dest, compare, value, result)
}

// writeAtomicCompareStore writes an atomic compare-store operation.
// Like compare-exchange but does not return the original value.
//
// HLSL syntax:
//
//	InterlockedCompareStore(dest, compare, value);
func (w *Writer) writeAtomicCompareStore(dest, compare, value string) {
	w.writeLine("%s(%s, %s, %s);", hlslInterlockedCompareStore, dest, compare, value)
}

// writeAtomicExchange writes an atomic exchange operation.
// Atomically replaces dest with value and returns the original value.
//
// HLSL syntax:
//
//	InterlockedExchange(dest, value, originalValue);
func (w *Writer) writeAtomicExchange(dest, value, result string) {
	w.writeLine("%s(%s, %s, %s);", hlslInterlockedExchange, dest, value, result)
}

// atomicFunctionToHLSL maps IR atomic functions to HLSL intrinsic names.
func atomicFunctionToHLSL(fun ir.AtomicFunction) (string, error) {
	switch fun.(type) {
	case ir.AtomicAdd:
		return hlslInterlockedAdd, nil
	case ir.AtomicSubtract:
		// HLSL doesn't have InterlockedSubtract; use InterlockedAdd with negated value
		return hlslInterlockedAdd, nil
	case ir.AtomicAnd:
		return hlslInterlockedAnd, nil
	case ir.AtomicExclusiveOr:
		return hlslInterlockedXor, nil
	case ir.AtomicInclusiveOr:
		return hlslInterlockedOr, nil
	case ir.AtomicMin:
		return hlslInterlockedMin, nil
	case ir.AtomicMax:
		return hlslInterlockedMax, nil
	case ir.AtomicExchange:
		return hlslInterlockedExchange, nil
	default:
		return "", fmt.Errorf("unsupported atomic function: %T", fun)
	}
}

// isAtomicSubtract checks if the atomic function is a subtract operation.
// Used to determine if value needs negation for HLSL InterlockedAdd.
func isAtomicSubtract(fun ir.AtomicFunction) bool {
	_, ok := fun.(ir.AtomicSubtract)
	return ok
}

// =============================================================================
// Register Binding Management
// =============================================================================

// getRegisterBinding returns the HLSL register binding string for a resource.
// Supports all register types: b (cbuffer), t (texture/SRV), s (sampler), u (UAV).
//
// HLSL syntax examples:
//
//	: register(b0)
//	: register(t0, space1)
//	: register(u0, space0)
func getRegisterBinding(regType RegisterType, binding *BindTarget) string {
	if binding == nil {
		return ""
	}
	return fmt.Sprintf(": register(%s%d, space%d)", regType.String(), binding.Register, binding.Space)
}

// getSpaceBinding returns a binding with explicit space specification.
// Used for resources that need non-zero descriptor spaces.
//
// HLSL syntax:
//
//	: register(t0, space1)
func getSpaceBinding(regType RegisterType, register, space uint32) string {
	return fmt.Sprintf(": register(%s%d, space%d)", regType.String(), register, space)
}

// formatBinding formats a complete binding string including array size if present.
func formatBinding(regType RegisterType, binding BindTarget) string {
	base := fmt.Sprintf("register(%s%d, space%d)", regType.String(), binding.Register, binding.Space)
	if binding.BindingArraySize != nil {
		return fmt.Sprintf(": %s /* array[%d] */", base, *binding.BindingArraySize)
	}
	return ": " + base
}

// getRegisterTypeForAddressSpace returns the appropriate register type for an address space.
func getRegisterTypeForAddressSpace(space ir.AddressSpace, readOnly bool) RegisterType {
	switch space {
	case ir.SpaceUniform:
		return RegisterTypeB
	case ir.SpaceStorage:
		if readOnly {
			return RegisterTypeT
		}
		return RegisterTypeU
	case ir.SpaceHandle:
		// Handle space is for samplers and textures
		return RegisterTypeT
	default:
		return RegisterTypeT
	}
}

// =============================================================================
// Address Calculation Helpers
// =============================================================================

// calculateBufferOffset calculates the byte offset for accessing a struct member.
// Used for packed struct access in byte address buffers.
//
// Parameters:
//   - baseOffset: The byte offset of the struct in the buffer
//   - memberOffset: The byte offset of the member within the struct
//
// Returns: Total byte offset (baseOffset + memberOffset)
func calculateBufferOffset(baseOffset, memberOffset uint32) uint32 {
	return baseOffset + memberOffset
}

// alignedOffset returns an offset aligned to the specified alignment.
// HLSL constant buffers require 16-byte alignment for most types.
//
// Parameters:
//   - offset: Current byte offset
//   - alignment: Required alignment (must be power of 2)
//
// Returns: Aligned offset >= input offset
func alignedOffset(offset, alignment uint32) uint32 {
	if alignment == 0 {
		return offset
	}
	return (offset + alignment - 1) &^ (alignment - 1)
}

// getScalarTypeSize returns the size in bytes for a scalar type.
func getScalarTypeSize(scalar ir.ScalarType) uint32 {
	return uint32(scalar.Width)
}

// getTypeAlignment returns the alignment requirement in bytes for a type.
func getTypeAlignment(module *ir.Module, handle ir.TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 4 // Default alignment
	}

	typ := &module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return uint32(inner.Width)
	case ir.VectorType:
		// Vectors align to their component count * scalar size, capped at 16 bytes
		size := uint32(inner.Size) * uint32(inner.Scalar.Width)
		if size > 16 {
			return 16
		}
		return size
	case ir.MatrixType:
		// Matrices align to 16 bytes (column-major, each column is a vec4)
		return 16
	case ir.ArrayType:
		// Arrays align to element alignment, rounded up to 16 bytes
		elemAlign := getTypeAlignment(module, inner.Base)
		if elemAlign < 16 {
			return 16
		}
		return elemAlign
	case ir.StructType:
		// Structs align to their largest member's alignment
		maxAlign := uint32(4)
		for i := range inner.Members {
			memberAlign := getTypeAlignment(module, inner.Members[i].Type)
			if memberAlign > maxAlign {
				maxAlign = memberAlign
			}
		}
		return maxAlign
	default:
		return 4
	}
}

// getTypeSize returns the size in bytes for a type.
func getTypeSize(module *ir.Module, handle ir.TypeHandle) uint32 {
	if int(handle) >= len(module.Types) {
		return 4 // Default size
	}

	typ := &module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return uint32(inner.Width)
	case ir.VectorType:
		return uint32(inner.Size) * uint32(inner.Scalar.Width)
	case ir.MatrixType:
		// Column-major: columns * rows * scalar_size
		return uint32(inner.Columns) * uint32(inner.Rows) * uint32(inner.Scalar.Width)
	case ir.ArrayType:
		if inner.Size.Constant != nil {
			elemSize := getTypeSize(module, inner.Base)
			stride := inner.Stride
			if stride == 0 {
				stride = alignedOffset(elemSize, getTypeAlignment(module, inner.Base))
			}
			return stride * (*inner.Size.Constant)
		}
		return 0 // Runtime-sized array
	case ir.StructType:
		return inner.Span
	default:
		return 4
	}
}

// =============================================================================
// Storage Buffer Helpers
// =============================================================================

// isStorageBufferReadOnly determines if a storage buffer should be read-only.
// This is determined by the global variable's usage patterns in the shader.
func isStorageBufferReadOnly(global *ir.GlobalVariable) bool {
	// For now, assume storage buffers are read-write
	// Full implementation would analyze usage to determine read-only status
	_ = global
	return false
}

// getBufferElementType returns the element type for a buffer variable.
// For arrays, returns the array element type; otherwise returns the type itself.
func (w *Writer) getBufferElementType(typeHandle ir.TypeHandle) (string, bool) {
	if int(typeHandle) >= len(w.module.Types) {
		return "", false
	}

	typ := &w.module.Types[typeHandle]

	// Check for runtime-sized array (common for storage buffers)
	if arr, ok := typ.Inner.(ir.ArrayType); ok {
		elemType := w.getTypeName(arr.Base)
		isRuntime := arr.Size.Constant == nil
		return elemType, isRuntime
	}

	// Not an array, return the type itself
	return w.typeToHLSL(typ), false
}

// writeStorageBufferDeclaration writes a complete storage buffer declaration.
// Handles both structured and byte address buffer types.
func (w *Writer) writeStorageBufferDeclaration(name string, typeHandle ir.TypeHandle, binding *BindTarget, readOnly bool) {
	// Get the actual type (unwrap pointer if needed)
	actualType := typeHandle
	if int(typeHandle) < len(w.module.Types) {
		if ptr, ok := w.module.Types[typeHandle].Inner.(ir.PointerType); ok {
			actualType = ptr.Base
		}
	}

	elemType, isRuntime := w.getBufferElementType(actualType)

	if isRuntime {
		// Runtime-sized array uses structured buffer
		w.writeStructuredBuffer(name, elemType, binding, readOnly)
	} else {
		// Fixed-size type uses structured buffer with the full type
		typeName := w.getTypeName(actualType)
		w.writeStructuredBuffer(name, typeName, binding, readOnly)
	}
}
