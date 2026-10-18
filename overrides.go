// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package naga

import (
	"errors"

	"github.com/gogpu/naga/internal/overrides"
	"github.com/gogpu/naga/ir"
)

// PipelineConstants are the values of pipeline-overridable constants
// (WebGPU GPUProgrammableStage.constants). A key is an override's @id as a
// decimal string, or its name if it has no @id; keys are compared exactly, as
// WGSL identifiers.
type PipelineConstants map[string]float64

// EntryPointSelector selects one entry point by stage and name. It is not
// ir.EntryPoint.
type EntryPointSelector struct {
	Stage ir.ShaderStage
	Name  string
}

// OverrideOptions configure ProcessOverrides. The zero value processes every
// entry point with no pipeline constants: every override takes its default.
type OverrideOptions struct {
	// Constants are the pipeline-constant values.
	Constants PipelineConstants
	// EntryPoint, if not nil, keeps only that entry point; only overrides it
	// uses need a value.
	EntryPoint *EntryPointSelector
}

// ProcessOverrides returns module with every override expression replaced by
// a constant holding its pipeline value or default, and every expression that
// becomes constant evaluated — Rust naga's
// back::pipeline_constants::process_overrides. Call it before a backend.
//
// module is never modified, and ProcessOverrides may be called concurrently
// on the same module. The result may be module itself when there is nothing
// to do (no overrides, and no EntryPoint or at most one entry point); treat it
// as read-only. Handles in the result are not stable relative to module.
//
// A module produced by the WGSL frontend with no overrides (len(Overrides) ==
// 0) never needs processing.
//
// The result keeps the overrides arena, with each initializer evaluated, as
// in Rust. With EntryPoint the selected entry point is EntryPoints[0]. Processing
// a processed module again with other constants is not supported: process the
// module from the frontend for every pipeline.
//
// Errors: a *PipelineConstantError for a constant (errors.Is with
// ErrOverrideNotFound, ErrOverrideMissingValue, ErrOverrideValueNotFinite,
// ErrOverrideValueOutOfRange); ErrEntryPointNotFound;
// ErrNegativeMeshOutputMax; an evaluation error of
// an expression that becomes constant (for example a division by zero after
// substitution); or an invalid processed module. Keys are checked in sorted
// order, so the same input reports the same key.
func ProcessOverrides(module *ir.Module, opts OverrideOptions) (*ir.Module, error) {
	var sel *overrides.Selector
	if opts.EntryPoint != nil {
		sel = &overrides.Selector{Stage: opts.EntryPoint.Stage, Name: opts.EntryPoint.Name}
	}
	m, err := overrides.Process(module, opts.Constants, sel)
	if err != nil {
		var ke *overrides.KeyError
		if errors.As(err, &ke) {
			return nil, &PipelineConstantError{Key: ke.Key, Err: ke.Err}
		}
		return nil, err
	}
	return m, nil
}

// PipelineConstantError is an error about one pipeline constant or override
// (Rust naga's PipelineConstantError). Err is one of the ErrOverride*
// sentinels; Error is Rust's message.
type PipelineConstantError struct {
	Key string // the pipeline-constant key, or the override's identifier string
	Err error
}

func (e *PipelineConstantError) Error() string { return overrides.Message(e.Key, e.Err) }

// Unwrap returns Err, for errors.Is.
func (e *PipelineConstantError) Unwrap() error { return e.Err }

// Pipeline-constant errors.
var (
	// ErrOverrideNotFound: a key names no override of the module.
	ErrOverrideNotFound = overrides.ErrNotFound
	// ErrOverrideMissingValue: an override an entry point uses has no value
	// and no default.
	ErrOverrideMissingValue = overrides.ErrMissingValue
	// ErrOverrideValueNotFinite: NaN or an infinity for a numeric override.
	ErrOverrideValueNotFinite = overrides.ErrValueNotFinite
	// ErrOverrideValueOutOfRange: the value does not fit the override's type.
	ErrOverrideValueOutOfRange = overrides.ErrValueOutOfRange
	// ErrNegativeWorkgroupSize: a workgroup size given by an override is not
	// positive.
	ErrNegativeWorkgroupSize = overrides.ErrNegativeWorkgroupSize
	// ErrNegativeMeshOutputMax: a mesh output limit given by an override is
	// negative.
	ErrNegativeMeshOutputMax = overrides.ErrNegativeMeshOutputMax
	// ErrEntryPointNotFound: OverrideOptions.EntryPoint selects no entry point.
	ErrEntryPointNotFound = overrides.ErrEntryPointNotFound
)
