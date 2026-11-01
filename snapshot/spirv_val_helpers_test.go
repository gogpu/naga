package snapshot_test

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// parseWGSL parses WGSL source and lowers it to an IR module, returning an
// error on failure.
func parseWGSL(source string) (*ir.Module, error) {
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return module, nil
}

// spirvBinary is spirv.Compile without its TranslationInfo.
func spirvBinary(module *ir.Module, opts spirv.Options) ([]byte, error) {
	binary, _, err := spirv.Compile(module, opts)
	return binary, err
}

// generateSPIRVBinary compiles an IR module to SPIR-V binary bytes with the
// default options, so spirv-val checks the default bounds-check policy
// (Restrict) over the whole corpus. The binding map comes from the shader's
// Rust parameters: an unbounded binding array takes its size from it, as
// wgpu passes it, and Restrict needs that size.
func generateSPIRVBinary(name string, module *ir.Module) ([]byte, error) {
	opts := spirv.DefaultOptions()
	rust := readRustSPVConfig(name)
	opts.BindingMap, opts.FakeMissingBindings = rust.BindingMap, rust.FakeMissingBindings
	spvBytes, err := spirvBinary(module, opts)
	// A shader that needs a newer version than the default (subgroup
	// operations need 1.3), which the backend refuses to raise, takes the
	// one its Rust parameters give.
	if err != nil && strings.Contains(err.Error(), "need SPIR-V") {
		opts.Version = rust.Version
		spvBytes, err = spirvBinary(module, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("SPIR-V generation error: %w", err)
	}
	return spvBytes, nil
}
