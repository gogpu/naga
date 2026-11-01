// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"

	"github.com/gogpu/naga/backend"
)

// ErrorKind categorizes HLSL compilation errors.
type ErrorKind uint8

const (
	// ErrUnsupportedFeature indicates a shader feature not supported by the target.
	ErrUnsupportedFeature ErrorKind = iota

	// ErrMissingBinding indicates a resource binding was not found in BindingMap.
	ErrMissingBinding

	// ErrInvalidShaderModel indicates an invalid or unsupported shader model.
	ErrInvalidShaderModel

	// ErrInternalError indicates an internal compiler error.
	ErrInternalError

	// ErrInvalidModule indicates the IR module is malformed.
	ErrInvalidModule

	// ErrUnsupportedType indicates a type that cannot be represented in HLSL.
	ErrUnsupportedType
)

// String returns a human-readable error kind name.
func (k ErrorKind) String() string {
	switch k {
	case ErrUnsupportedFeature:
		return "UnsupportedFeature"
	case ErrMissingBinding:
		return "MissingBinding"
	case ErrInvalidShaderModel:
		return "InvalidShaderModel"
	case ErrInternalError:
		return "InternalError"
	case ErrInvalidModule:
		return "InvalidModule"
	case ErrUnsupportedType:
		return "UnsupportedType"
	default:
		return "Unknown"
	}
}

// Span represents a source location for error reporting.
type Span struct {
	// Start is the byte offset of the span start.
	Start uint32

	// End is the byte offset of the span end.
	End uint32
}

// Error represents an HLSL compilation error.
type Error struct {
	// Kind categorizes the error.
	Kind ErrorKind

	// Message provides details about the error.
	Message string

	// Span optionally identifies the source location.
	Span *Span
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Span != nil {
		return fmt.Sprintf("hlsl %s at [%d:%d]: %s", e.Kind, e.Span.Start, e.Span.End, e.Message)
	}
	return fmt.Sprintf("hlsl %s: %s", e.Kind, e.Message)
}

// Unwrap returns the shared category of the error, so that callers can
// match it with errors.Is: backend.ErrMissingBinding for a missing binding,
// backend.ErrUnsupported for what HLSL or the shader model cannot express.
func (e *Error) Unwrap() error {
	switch e.Kind {
	case ErrMissingBinding:
		return backend.ErrMissingBinding
	case ErrUnsupportedFeature, ErrUnsupportedType, ErrInvalidShaderModel:
		return backend.ErrUnsupported
	}
	return nil
}

// NewError creates a new HLSL error without span information.
func NewError(kind ErrorKind, message string) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
		Span:    nil,
	}
}

// NewErrorWithSpan creates a new HLSL error with span information.
func NewErrorWithSpan(kind ErrorKind, message string, start, end uint32) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
		Span:    &Span{Start: start, End: end},
	}
}

// IsUnsupportedFeature returns true if the error is ErrUnsupportedFeature.
func (e *Error) IsUnsupportedFeature() bool {
	return e.Kind == ErrUnsupportedFeature
}

// IsMissingBinding returns true if the error is ErrMissingBinding.
func (e *Error) IsMissingBinding() bool {
	return e.Kind == ErrMissingBinding
}

// IsInternalError returns true if the error is ErrInternalError.
func (e *Error) IsInternalError() bool {
	return e.Kind == ErrInternalError
}
