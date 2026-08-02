package ir

// Capabilities represents hardware/API features available to the shader.
// Mirrors Rust naga valid::Capabilities bitflags.
//
// When compiling WGSL, types like f64, i64, u64, and f16 are only valid
// if the caller enables the corresponding capability. Without it, the
// lowerer rejects the shader with a descriptive error.
//
// Default value (0) means no extended capabilities are enabled,
// which matches Rust naga's default behavior where only f32/i32/u32/bool
// are allowed without explicit capability opt-in.
type Capabilities uint32

const (
	// CapFloat64 enables f64 scalar type (Float width=8).
	// Maps to Rust naga valid::Capabilities::FLOAT64 (1 << 1).
	CapFloat64 Capabilities = 1 << 1

	// CapShaderInt64 enables i64 and u64 scalar types (Sint/Uint width=8).
	// Maps to Rust naga valid::Capabilities::SHADER_INT64 (1 << 16).
	CapShaderInt64 Capabilities = 1 << 16

	// CapShaderFloat16 enables f16 scalar type (Float width=2).
	// Maps to Rust naga valid::Capabilities::SHADER_FLOAT16 (1 << 26).
	CapShaderFloat16 Capabilities = 1 << 26

	// CapAll enables all capabilities. Used by test infrastructure
	// and tools that need to accept any valid WGSL without restriction.
	CapAll Capabilities = CapFloat64 | CapShaderInt64 | CapShaderFloat16
)

// Contains reports whether c includes all the flags in flag.
func (c Capabilities) Contains(flag Capabilities) bool {
	return c&flag == flag
}
