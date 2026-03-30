package spirv

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Tests for Phase 3 fixes: force_loop_bounding, dot4 polyfill, SSA entry args
// =============================================================================

// TestForceLoopBounding verifies that loops get a force-bounding counter
// (vec2<u32> variable that decrements each iteration) when ForceLoopBounding=true.
func TestForceLoopBounding(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, // [0] i32
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, // [1] bool
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(0)}},     // [0]
						{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [1]
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtLoop{
							Body: ir.Block{
								{Kind: ir.StmtBreak{}},
							},
						}},
					},
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	t.Run("enabled", func(t *testing.T) {
		opts := DefaultOptions()
		opts.ForceLoopBounding = true
		backend := NewBackend(opts)
		spvBytes, err := backend.Compile(module)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		// Force loop bounding creates vec2<u32> type + ISub + IEqual + All
		if !divmodHasOpcode(spvBytes, OpAll) {
			t.Error("ForceLoopBounding should emit OpAll for vec2<bool> reduction")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		opts := DefaultOptions()
		opts.ForceLoopBounding = false
		backend := NewBackend(opts)
		spvBytes, err := backend.Compile(module)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		// Without force bounding, no OpAll needed for loop counter
		if divmodHasOpcode(spvBytes, OpAll) {
			t.Error("ForceLoopBounding=false should not emit OpAll for loop counter")
		}
	})
}

// TestDot4PolyfillWhenCapNotAvailable verifies that dot4I8Packed/dot4U8Packed
// fall back to a polyfill (BitFieldExtract + IMul + IAdd) when DotProduct
// capability is not in CapabilitiesAvailable.
func TestDot4PolyfillWhenCapNotAvailable(t *testing.T) {
	source := `@compute @workgroup_size(1)
fn main() {
    let a: u32 = 0x01020304u;
    let b: u32 = 0x05060708u;
    let result: u32 = dot4U8Packed(a, b);
    _ = result;
}`
	opts := DefaultOptions()
	// Restrict capabilities to just Matrix — no DotProduct
	opts.CapabilitiesAvailable = map[Capability]struct{}{
		CapabilityShader: {},
		CapabilityMatrix: {},
	}

	spvBytes := compileWGSLForCapabilityTestWithOpts(t, source, opts)
	caps := extractCapabilities(spvBytes)

	// Should NOT have DotProduct capabilities (polyfill used instead)
	assertNoCapability(t, caps, CapabilityDotProduct)
	assertNoCapability(t, caps, CapabilityDotProductInput4x8BitPacked)

	// Should have OpBitFieldUExtract (part of polyfill)
	if !divmodHasOpcode(spvBytes, OpBitFieldUExtract) {
		t.Error("dot4 polyfill should use OpBitFieldUExtract")
	}
}

// TestDot4NativeWhenCapAvailable verifies dot4 uses native instructions
// when DotProduct capability is available.
func TestDot4NativeWhenCapAvailable(t *testing.T) {
	source := `@compute @workgroup_size(1)
fn main() {
    let a: u32 = 0x01020304u;
    let b: u32 = 0x05060708u;
    let result: u32 = dot4U8Packed(a, b);
    _ = result;
}`
	opts := DefaultOptions()
	// All capabilities available (default)
	spvBytes := compileWGSLForCapabilityTestWithOpts(t, source, opts)
	caps := extractCapabilities(spvBytes)

	// Should have DotProduct capabilities (native path)
	assertCapability(t, caps, CapabilityDotProduct)
	assertCapability(t, caps, CapabilityDotProductInput4x8BitPacked)
}
