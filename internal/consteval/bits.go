package consteval

// Two's-complement reinterpretations. WGSL defines conversion between integer
// types of one width as reinterpreting the bits and concrete integer
// arithmetic as wrapping modulo 2^width, so these conversions are meant to
// wrap; every such conversion in the package goes through one of them.

func u64FromI64(v int64) uint64 { return uint64(v) } //nolint:gosec // two's-complement reinterpretation is the defined result

func i64FromU64(v uint64) int64 { return int64(v) } //nolint:gosec // two's-complement reinterpretation is the defined result

func i32Low(v uint64) int32 { return int32(v) } //nolint:gosec // keeps the low 32 bits: the defined result

func u32Low(v uint64) uint32 { return uint32(v) } //nolint:gosec // keeps the low 32 bits: the defined result
