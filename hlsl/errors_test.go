// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"
)

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want string
	}{
		{ErrUnsupportedFeature, "UnsupportedFeature"},
		{ErrMissingBinding, "MissingBinding"},
		{ErrInvalidShaderModel, "InvalidShaderModel"},
		{ErrInternalError, "InternalError"},
		{ErrInvalidModule, "InvalidModule"},
		{ErrUnsupportedType, "UnsupportedType"},
		{ErrEntryPointNotFound, "EntryPointNotFound"},
		{ErrorKind(255), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("ErrorKind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	// Error without span
	err1 := &Error{
		Kind:    ErrMissingBinding,
		Message: "resource 'buffer0' has no binding",
	}
	got1 := err1.Error()
	if !strings.Contains(got1, "MissingBinding") {
		t.Errorf("Error() should contain kind, got %q", got1)
	}
	if !strings.Contains(got1, "resource 'buffer0'") {
		t.Errorf("Error() should contain message, got %q", got1)
	}

	// Error with span
	err2 := &Error{
		Kind:    ErrUnsupportedFeature,
		Message: "ray tracing requires SM 6.3+",
		Span:    &Span{Start: 100, End: 150},
	}
	got2 := err2.Error()
	if !strings.Contains(got2, "100") || !strings.Contains(got2, "150") {
		t.Errorf("Error() with span should contain location, got %q", got2)
	}
}

func TestNewError(t *testing.T) {
	err := NewError(ErrInternalError, "unexpected nil pointer")

	if err.Kind != ErrInternalError {
		t.Errorf("Kind = %v, want ErrInternalError", err.Kind)
	}
	if err.Message != "unexpected nil pointer" {
		t.Errorf("Message = %q, want \"unexpected nil pointer\"", err.Message)
	}
	if err.Span != nil {
		t.Error("Span should be nil")
	}
}

func TestNewErrorWithSpan(t *testing.T) {
	err := NewErrorWithSpan(ErrUnsupportedType, "unsupported atomic type", 50, 75)

	if err.Kind != ErrUnsupportedType {
		t.Errorf("Kind = %v, want ErrUnsupportedType", err.Kind)
	}
	if err.Message != "unsupported atomic type" {
		t.Errorf("Message = %q, want \"unsupported atomic type\"", err.Message)
	}
	if err.Span == nil {
		t.Fatal("Span should not be nil")
	}
	if err.Span.Start != 50 {
		t.Errorf("Span.Start = %d, want 50", err.Span.Start)
	}
	if err.Span.End != 75 {
		t.Errorf("Span.End = %d, want 75", err.Span.End)
	}
}

func TestError_Predicates(t *testing.T) {
	tests := []struct {
		name                 string
		err                  *Error
		isUnsupportedFeature bool
		isMissingBinding     bool
		isInternalError      bool
	}{
		{
			name:                 "unsupported feature",
			err:                  &Error{Kind: ErrUnsupportedFeature},
			isUnsupportedFeature: true,
		},
		{
			name:             "missing binding",
			err:              &Error{Kind: ErrMissingBinding},
			isMissingBinding: true,
		},
		{
			name:            "internal error",
			err:             &Error{Kind: ErrInternalError},
			isInternalError: true,
		},
		{
			name: "other error",
			err:  &Error{Kind: ErrInvalidModule},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsUnsupportedFeature(); got != tt.isUnsupportedFeature {
				t.Errorf("IsUnsupportedFeature() = %v, want %v", got, tt.isUnsupportedFeature)
			}
			if got := tt.err.IsMissingBinding(); got != tt.isMissingBinding {
				t.Errorf("IsMissingBinding() = %v, want %v", got, tt.isMissingBinding)
			}
			if got := tt.err.IsInternalError(); got != tt.isInternalError {
				t.Errorf("IsInternalError() = %v, want %v", got, tt.isInternalError)
			}
		})
	}
}

func TestSpan(t *testing.T) {
	span := &Span{Start: 10, End: 20}

	if span.Start != 10 {
		t.Errorf("Start = %d, want 10", span.Start)
	}
	if span.End != 20 {
		t.Errorf("End = %d, want 20", span.End)
	}
}
