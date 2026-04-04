// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package-level nolint for storage functions prepared for future integration.
// These functions implement HLSL buffer and atomic operations and will be used
// when the full statement codegen calls storage operations.
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
	intrinsic, hlslErr := atomicFunctionToHLSL(fun)
	if hlslErr != nil {
		return hlslErr
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

// =============================================================================
// ByteAddressBuffer Access Chain (matches Rust naga storage.rs)
// =============================================================================

// alignmentFromVectorSize returns the alignment factor for a vector size.
// Bi=2, Tri=4, Quad=4 (matching Rust naga's Alignment::from(VectorSize)).
func alignmentFromVectorSize(size ir.VectorSize) uint32 {
	switch size {
	case 2:
		return 2
	case 3, 4:
		return 4
	default:
		return 1
	}
}

// fillAccessChain populates w.tempAccessChain with byte offset steps
// for accessing a storage buffer component. Returns the global variable handle.
// Matches Rust naga's Writer::fill_access_chain.
func (w *Writer) fillAccessChain(curExpr ir.ExpressionHandle) (ir.GlobalVariableHandle, error) {
	w.tempAccessChain = w.tempAccessChain[:0]

	for {
		if int(curExpr) >= len(w.currentFunction.Expressions) {
			return 0, fmt.Errorf("fillAccessChain: invalid expression handle %d", curExpr)
		}
		expr := w.currentFunction.Expressions[curExpr]

		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			// Reached the root global variable.
			// If this global has a dynamic_storage_buffer_offsets_index, add a buffer offset entry.
			gv := w.module.GlobalVariables[e.Variable]
			if gv.Binding != nil {
				rb := ResourceBinding{Group: gv.Binding.Group, Binding: gv.Binding.Binding}
				if bt, ok := w.options.BindingMap[rb]; ok {
					if bt.DynamicStorageBufferOffsetsIndex != nil {
						w.tempAccessChain = append(w.tempAccessChain, subAccess{
							kind:         subAccessBufferOffset,
							bufferGroup:  gv.Binding.Group,
							bufferOffset: *bt.DynamicStorageBufferOffsetsIndex,
						})
					}
				}
			}
			// Chain was built leaf-to-root (same as Rust), no reversal needed.
			return e.Variable, nil

		case ir.ExprAccess:
			// Runtime-indexed access: need to compute parent type for stride
			sub, err := w.computeSubAccess(e.Base, true, e.Index, 0)
			if err != nil {
				return 0, err
			}
			w.tempAccessChain = append(w.tempAccessChain, sub)
			curExpr = e.Base

		case ir.ExprAccessIndex:
			// Constant-indexed access
			sub, err := w.computeSubAccess(e.Base, false, 0, e.Index)
			if err != nil {
				return 0, err
			}
			w.tempAccessChain = append(w.tempAccessChain, sub)
			curExpr = e.Base

		default:
			return 0, fmt.Errorf("fillAccessChain: unsupported expression %T", expr.Kind)
		}
	}
}

// computeSubAccess computes a single SubAccess step given a base expression and index.
// isRuntime=true means the index is a runtime expression handle; false means constant index.
func (w *Writer) computeSubAccess(base ir.ExpressionHandle, isRuntime bool, runtimeIndex ir.ExpressionHandle, constIndex uint32) (subAccess, error) {
	// Resolve the base expression's type to determine the parent container
	baseInner := w.resolveStorageBaseType(base)
	if baseInner == nil {
		return subAccess{}, fmt.Errorf("computeSubAccess: cannot resolve type for expression %d", base)
	}

	// Dereference pointer if needed
	if ptr, ok := baseInner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			baseInner = w.module.Types[ptr.Base].Inner
		}
	}

	switch inner := baseInner.(type) {
	case ir.StructType:
		// Struct member access: always constant offset
		if int(constIndex) < len(inner.Members) {
			return subAccess{kind: subAccessOffset, offset: inner.Members[constIndex].Offset}, nil
		}
		return subAccess{}, fmt.Errorf("struct member index %d out of range", constIndex)

	case ir.ArrayType:
		stride := inner.Stride
		if isRuntime {
			return subAccess{kind: subAccessIndex, value: runtimeIndex, stride: stride}, nil
		}
		return subAccess{kind: subAccessOffset, offset: stride * constIndex}, nil

	case ir.VectorType:
		scalarWidth := uint32(inner.Scalar.Width)
		if isRuntime {
			return subAccess{kind: subAccessIndex, value: runtimeIndex, stride: scalarWidth}, nil
		}
		return subAccess{kind: subAccessOffset, offset: scalarWidth * constIndex}, nil

	case ir.MatrixType:
		// Column stride: alignment(rows) * scalar_width
		rowStride := alignmentFromVectorSize(inner.Rows) * uint32(inner.Scalar.Width)
		if isRuntime {
			return subAccess{kind: subAccessIndex, value: runtimeIndex, stride: rowStride}, nil
		}
		return subAccess{kind: subAccessOffset, offset: rowStride * constIndex}, nil

	case ir.ValuePointerType:
		// ValuePointer is pointer to scalar/vector, stride = scalar width.
		// Matches Rust: TypeInner::ValuePointer { scalar, .. } => Parent::Array { stride: scalar.width }
		scalarWidth := uint32(inner.Scalar.Width)
		if isRuntime {
			return subAccess{kind: subAccessIndex, value: runtimeIndex, stride: scalarWidth}, nil
		}
		return subAccess{kind: subAccessOffset, offset: scalarWidth * constIndex}, nil

	default:
		return subAccess{}, fmt.Errorf("computeSubAccess: unsupported base type %T", baseInner)
	}
}

// resolveStorageBaseType resolves the TypeInner for an expression, used for storage access chains.
func (w *Writer) resolveStorageBaseType(handle ir.ExpressionHandle) ir.TypeInner {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return nil
	}
	resolution := &w.currentFunction.ExpressionTypes[handle]
	if resolution.Handle != nil {
		h := *resolution.Handle
		if int(h) < len(w.module.Types) {
			return w.module.Types[h].Inner
		}
	}
	return resolution.Value
}

// writeStorageAddress emits an HLSL expression for the byte offset described by
// the given access chain. Matches Rust naga's Writer::write_storage_address.
func (w *Writer) writeStorageAddress(chain []subAccess) error {
	if len(chain) == 0 {
		w.out.WriteString("0")
		return nil
	}
	for i, access := range chain {
		if i > 0 {
			w.out.WriteByte('+')
		}
		switch access.kind {
		case subAccessOffset:
			fmt.Fprintf(&w.out, "%d", access.offset)
		case subAccessIndex:
			if err := w.writeExpression(access.value); err != nil {
				return err
			}
			fmt.Fprintf(&w.out, "*%d", access.stride)
		case subAccessBufferOffset:
			fmt.Fprintf(&w.out, "__dynamic_buffer_offsets%d._%d", access.bufferGroup, access.bufferOffset)
		}
	}
	return nil
}

// scalarHLSLCast returns the asfloat/asint/asuint cast for a scalar kind.
func scalarHLSLCast(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarFloat:
		return "asfloat"
	case ir.ScalarSint:
		return "asint"
	case ir.ScalarUint:
		return "asuint"
	default:
		return "asuint"
	}
}

// writeStorageLoad emits HLSL to load a value from a ByteAddressBuffer.
// The current tempAccessChain describes the byte offset within the buffer.
// tyHandle is an optional type handle for struct/array constructor name lookup.
// Matches Rust naga's Writer::write_storage_load.
func (w *Writer) writeStorageLoad(varHandle ir.GlobalVariableHandle, resultTy ir.TypeInner, tyHandle *ir.TypeHandle) error {
	varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(varHandle)}]

	switch inner := resultTy.(type) {
	case ir.ScalarType:
		if inner.Width == 4 {
			cast := scalarHLSLCast(inner.Kind)
			fmt.Fprintf(&w.out, "%s(%s.Load(", cast, varName)
			// Save and restore chain (matching Rust borrow checker pattern)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString("))")
			w.tempAccessChain = chain
		} else {
			typeName := scalarToHLSLStr(inner)
			fmt.Fprintf(&w.out, "%s.Load<%s>(", varName, typeName)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(")")
			w.tempAccessChain = chain
		}

	case ir.VectorType:
		size := inner.Size
		if inner.Scalar.Width == 4 {
			cast := scalarHLSLCast(inner.Scalar.Kind)
			fmt.Fprintf(&w.out, "%s(%s.Load%d(", cast, varName, size)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString("))")
			w.tempAccessChain = chain
		} else {
			typeName := scalarToHLSLStr(inner.Scalar)
			fmt.Fprintf(&w.out, "%s.Load<%s%d>(", varName, typeName, size)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(")")
			w.tempAccessChain = chain
		}

	case ir.MatrixType:
		typeName := scalarToHLSLStr(inner.Scalar)
		fmt.Fprintf(&w.out, "%s%dx%d(", typeName, inner.Columns, inner.Rows)
		rowStride := alignmentFromVectorSize(inner.Rows) * uint32(inner.Scalar.Width)
		for i := ir.VectorSize(0); i < inner.Columns; i++ {
			if i > 0 {
				w.out.WriteString(", ")
			}
			w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: uint32(i) * rowStride})
			colType := ir.VectorType{Size: inner.Rows, Scalar: inner.Scalar}
			if err := w.writeStorageLoad(varHandle, colType, nil); err != nil {
				return err
			}
			w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
		}
		w.out.WriteString(")")

	case ir.StructType:
		// Load struct member by member using wrapped constructor
		resolvedHandle := tyHandle
		if resolvedHandle == nil {
			resolvedHandle = w.getTypeHandleForInner(resultTy)
		}
		if resolvedHandle != nil {
			constructorName := fmt.Sprintf("Construct%s", w.typeNames[*resolvedHandle])
			w.out.WriteString(constructorName)
			w.out.WriteByte('(')
			for i, member := range inner.Members {
				if i > 0 {
					w.out.WriteString(", ")
				}
				w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: member.Offset})
				memberTy := w.getTypeInner(member.Type)
				memberHandle := &member.Type
				if err := w.writeStorageLoad(varHandle, memberTy, memberHandle); err != nil {
					return err
				}
				w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
			}
			w.out.WriteByte(')')
		}

	case ir.ArrayType:
		if inner.Size.Constant != nil {
			resolvedHandle := tyHandle
			if resolvedHandle == nil {
				resolvedHandle = w.getTypeHandleForInner(resultTy)
			}
			if resolvedHandle != nil {
				// Use hlslTypeId for array constructor names (e.g., "array2_float_")
				// rather than typeNames which may use generic names like "type_1_".
				constructorName := fmt.Sprintf("Construct%s", w.hlslTypeId(*resolvedHandle))
				w.out.WriteString(constructorName)
				w.out.WriteByte('(')
				for i := uint32(0); i < *inner.Size.Constant; i++ {
					if i > 0 {
						w.out.WriteString(", ")
					}
					w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: i * inner.Stride})
					baseTy := w.getTypeInner(inner.Base)
					baseHandle := &inner.Base
					if err := w.writeStorageLoad(varHandle, baseTy, baseHandle); err != nil {
						return err
					}
					w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
				}
				w.out.WriteByte(')')
			}
		}
	}

	return nil
}

// writeStorageStore emits HLSL to store a value to an RWByteAddressBuffer.
// Matches Rust naga's Writer::write_storage_store.
func (w *Writer) writeStorageStore(varHandle ir.GlobalVariableHandle, sv storeValue, level int) error {
	if level > 20 {
		return fmt.Errorf("writeStorageStore: recursion depth limit exceeded (level=%d)", level)
	}
	// Resolve the stored value's type
	var resultTy ir.TypeInner
	switch sv.kind {
	case storeValueExpression:
		resultTy = w.getExpressionTypeInner(sv.expr)
	case storeValueTempIndex:
		// We stored to a temp variable: type comes from the container's element type
		resultTy = w.getTypeInner(sv.base)
	case storeValueTempAccess:
		if int(sv.base) < len(w.module.Types) {
			if st, ok := w.module.Types[sv.base].Inner.(ir.StructType); ok {
				if int(sv.memberIdx) < len(st.Members) {
					resultTy = w.getTypeInner(st.Members[sv.memberIdx].Type)
				}
			}
		}
	}
	if resultTy == nil {
		return fmt.Errorf("writeStorageStore: cannot resolve type")
	}

	varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(varHandle)}]
	indent := w.indentStr(level)

	switch inner := resultTy.(type) {
	case ir.ScalarType:
		if inner.Width == 4 {
			fmt.Fprintf(&w.out, "%s%s.Store(", indent, varName)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(", asuint(")
			if err := w.writeStoreValue(sv); err != nil {
				return err
			}
			w.out.WriteString("));\n")
			w.tempAccessChain = chain
		} else {
			fmt.Fprintf(&w.out, "%s%s.Store(", indent, varName)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(", ")
			if err := w.writeStoreValue(sv); err != nil {
				return err
			}
			w.out.WriteString(");\n")
			w.tempAccessChain = chain
		}

	case ir.VectorType:
		size := inner.Size
		if inner.Scalar.Width == 4 {
			fmt.Fprintf(&w.out, "%s%s.Store%d(", indent, varName, size)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(", asuint(")
			if err := w.writeStoreValue(sv); err != nil {
				return err
			}
			w.out.WriteString("));\n")
			w.tempAccessChain = chain
		} else {
			fmt.Fprintf(&w.out, "%s%s.Store(", indent, varName)
			chain := w.tempAccessChain
			w.tempAccessChain = nil
			if err := w.writeStorageAddress(chain); err != nil {
				return err
			}
			w.out.WriteString(", ")
			if err := w.writeStoreValue(sv); err != nil {
				return err
			}
			w.out.WriteString(");\n")
			w.tempAccessChain = chain
		}

	case ir.StructType:
		// Decompose into member stores using a temporary
		fmt.Fprintf(&w.out, "%s{\n", indent)
		depth := level + 1
		innerIndent := w.indentStr(depth)
		structTy := w.getTypeHandleForInner(resultTy)
		if structTy == nil {
			return fmt.Errorf("cannot find type handle for struct")
		}
		structName := w.typeNames[*structTy]
		fmt.Fprintf(&w.out, "%s%s _value%d = ", innerIndent, structName, depth)
		if err := w.writeStoreValue(sv); err != nil {
			return err
		}
		w.out.WriteString(";\n")
		for i, member := range inner.Members {
			w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: member.Offset})
			memberSv := storeValue{
				kind:      storeValueTempAccess,
				depth:     depth,
				base:      *structTy,
				memberIdx: uint32(i),
			}
			if err := w.writeStorageStore(varHandle, memberSv, depth); err != nil {
				return err
			}
			w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
		}
		fmt.Fprintf(&w.out, "%s}\n", indent)

	case ir.MatrixType:
		rowStride := alignmentFromVectorSize(inner.Rows) * uint32(inner.Scalar.Width)
		fmt.Fprintf(&w.out, "%s{\n", indent)
		depth := level + 1
		innerIndent := w.indentStr(depth)
		typeName := scalarToHLSLStr(inner.Scalar)
		fmt.Fprintf(&w.out, "%s%s%dx%d _value%d = ", innerIndent, typeName, uint8(inner.Columns), uint8(inner.Rows), depth)
		if err := w.writeStoreValue(sv); err != nil {
			return err
		}
		w.out.WriteString(";\n")
		vecSize := uint8(inner.Rows)
		for i := ir.VectorSize(0); i < inner.Columns; i++ {
			w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: uint32(i) * rowStride})
			// Directly emit vector store for each column (Store2/Store3/Store4)
			if inner.Scalar.Width == 4 && vecSize >= 2 {
				fmt.Fprintf(&w.out, "%s%s.Store%d(", innerIndent, varName, vecSize)
				chain := w.tempAccessChain
				w.tempAccessChain = nil
				if err := w.writeStorageAddress(chain); err != nil {
					return err
				}
				fmt.Fprintf(&w.out, ", asuint(_value%d[%d]));\n", depth, uint8(i))
				w.tempAccessChain = chain
			} else {
				fmt.Fprintf(&w.out, "%s%s.Store(", innerIndent, varName)
				chain := w.tempAccessChain
				w.tempAccessChain = nil
				if err := w.writeStorageAddress(chain); err != nil {
					return err
				}
				fmt.Fprintf(&w.out, ", asuint(_value%d[%d]));\n", depth, uint8(i))
				w.tempAccessChain = chain
			}
			w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
		}
		fmt.Fprintf(&w.out, "%s}\n", indent)

	case ir.ArrayType:
		if inner.Size.Constant != nil {
			fmt.Fprintf(&w.out, "%s{\n", indent)
			depth := level + 1
			innerIndent := w.indentStr(depth)
			// Write temp var declaration
			fmt.Fprintf(&w.out, "%s", innerIndent)
			w.writeTypeByHandle(inner.Base)
			fmt.Fprintf(&w.out, " _value%d", depth)
			if inner.Size.Constant != nil {
				fmt.Fprintf(&w.out, "[%d]", *inner.Size.Constant)
			}
			w.out.WriteString(" = ")
			if err := w.writeStoreValue(sv); err != nil {
				return err
			}
			w.out.WriteString(";\n")
			for i := uint32(0); i < *inner.Size.Constant; i++ {
				w.tempAccessChain = append(w.tempAccessChain, subAccess{kind: subAccessOffset, offset: i * inner.Stride})
				elemSv := storeValue{
					kind:  storeValueTempIndex,
					depth: depth,
					index: i,
					base:  inner.Base,
				}
				if err := w.writeStorageStore(varHandle, elemSv, depth); err != nil {
					return err
				}
				w.tempAccessChain = w.tempAccessChain[:len(w.tempAccessChain)-1]
			}
			fmt.Fprintf(&w.out, "%s}\n", indent)
		}
	}

	return nil
}

// writeStoreValue emits the value expression for a storage store.
func (w *Writer) writeStoreValue(sv storeValue) error {
	switch sv.kind {
	case storeValueExpression:
		return w.writeExpression(sv.expr)
	case storeValueTempIndex:
		fmt.Fprintf(&w.out, "_value%d[%d]", sv.depth, sv.index)
	case storeValueTempAccess:
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(sv.base), handle2: sv.memberIdx}]
		if memberName == "" {
			memberName = fmt.Sprintf("member_%d", sv.memberIdx)
		}
		fmt.Fprintf(&w.out, "_value%d.%s", sv.depth, memberName)
	}
	return nil
}

// indentStr returns whitespace for the given indent level (4 spaces per level).
func (w *Writer) indentStr(level int) string {
	const spaces = "                                " // 32 spaces
	n := level * 4
	if n > len(spaces) {
		n = len(spaces)
	}
	return spaces[:n]
}

// scalarToHLSLStr returns the HLSL type name for a scalar.
func scalarToHLSLStr(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarSint:
		if s.Width == 8 {
			return "int64_t"
		}
		return "int"
	case ir.ScalarUint:
		if s.Width == 8 {
			return "uint64_t"
		}
		return "uint"
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return "half"
		case 8:
			return "double"
		}
		return "float"
	case ir.ScalarBool:
		return "bool"
	default:
		return "uint"
	}
}

// getTypeInner returns the TypeInner for a type handle.
func (w *Writer) getTypeInner(handle ir.TypeHandle) ir.TypeInner {
	if int(handle) < len(w.module.Types) {
		return w.module.Types[handle].Inner
	}
	return nil
}

// getTypeHandleForInner finds the TypeHandle matching a given TypeInner.
// This is needed when we have a Value resolution but need the Handle for constructor names.
func (w *Writer) getTypeHandleForInner(inner ir.TypeInner) *ir.TypeHandle {
	// This is a simple lookup -- for now search through types
	for i := range w.module.Types {
		if typesMatch(w.module.Types[i].Inner, inner) {
			h := ir.TypeHandle(i)
			return &h
		}
	}
	return nil
}

// typesMatch checks if two TypeInner values are the same.
func typesMatch(a, b ir.TypeInner) bool {
	// Simple type identity check
	switch at := a.(type) {
	case ir.ScalarType:
		if bt, ok := b.(ir.ScalarType); ok {
			return at.Kind == bt.Kind && at.Width == bt.Width
		}
	case ir.VectorType:
		if bt, ok := b.(ir.VectorType); ok {
			return at.Size == bt.Size && at.Scalar.Kind == bt.Scalar.Kind && at.Scalar.Width == bt.Scalar.Width
		}
	}
	// For struct/array/matrix, pointer equality of the type
	return false
}

// writeTypeByHandle writes the HLSL type name for a type handle.
func (w *Writer) writeTypeByHandle(handle ir.TypeHandle) {
	name := w.getTypeName(handle)
	w.out.WriteString(name)
}

// isStoragePointer checks if an expression refers to a storage buffer component.
// It walks the Access/AccessIndex chain back to the root to check if it's a
// GlobalVariable in the Storage address space.
func (w *Writer) isStoragePointer(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}

	// First, try the fast path: check ExpressionTypes for a pointer type
	if int(handle) < len(w.currentFunction.ExpressionTypes) {
		resolution := &w.currentFunction.ExpressionTypes[handle]
		var inner ir.TypeInner
		if resolution.Handle != nil {
			h := *resolution.Handle
			if int(h) < len(w.module.Types) {
				inner = w.module.Types[h].Inner
			}
		} else {
			inner = resolution.Value
		}
		if ptr, ok := inner.(ir.PointerType); ok {
			return ptr.Space == ir.SpaceStorage
		}
		if vp, ok := inner.(ir.ValuePointerType); ok {
			return vp.Space == ir.SpaceStorage
		}
	}

	// Slow path: walk the expression chain back to the root global variable
	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return false
		}
		expr := w.currentFunction.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				return w.module.GlobalVariables[e.Variable].Space == ir.SpaceStorage
			}
			return false
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		default:
			return false
		}
	}
}
