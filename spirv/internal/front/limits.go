package front

// Limits on the untrusted input this package parses (design §16 of the
// SPIR-V frontend design). Each limit is named here, checked before the
// memory or the stack use it bounds, and covered by a test that asserts the
// error rather than by a comment.
const (
	// maxModuleBytes is the largest module this package accepts: the byte
	// offsets of the public spirv.ParseError are 32-bit, so a larger module
	// cannot name its instructions.
	maxModuleBytes int64 = 1<<32 - 1

	// maxArrayElements caps an array's element count: it is read from a
	// constant before the IR is built, so a huge size never drives an
	// allocation in the frontend or in a backend that sizes storage from it.
	maxArrayElements = 1 << 24

	// maxStorageExpressions caps the expressions the functions of a module
	// get for its globals and constants together: (globals + constants) ×
	// functions. Every function's arena starts with one expression per global
	// and per constant, as Rust naga's make_expression_storage, so a small
	// module of many constants and many functions would otherwise build an IR
	// quadratic in its size (4,000 constants and 4,000 functions, 208 KB, made
	// 16M expressions). The running total is checked before each function's
	// arena is filled.
	maxStorageExpressions = 1 << 20

	// maxNesting caps how deeply structured control flow (selections, loops,
	// switches) nests. Lowering the body tree recurses once per level, and
	// every out-of-scope check walks the body's parent chain, so the depth is
	// refused when a body is created, before either grows. The WGSL frontend
	// caps brace nesting at 127, so every module it produces fits.
	maxNesting = 128

	// maxTypeDepth caps how deeply types nest: a scalar, vector, matrix,
	// image or sampler is one level, an array, binding array, struct or
	// pointer one more than its deepest component. SPIR-V lets a chain of
	// OpTypeArray, OpTypeStruct or OpTypePointer grow with the module, and
	// every walk over the IR's types (the layouter, the validator, the
	// backends' type writers) recurses once per level, so the depth is refused
	// when the type is declared. The one type built without that check is an
	// entry point wrapper's result struct, one level above the result types it
	// gathers, so no IR type the frontend builds is deeper than
	// maxTypeDepth+1. The WGSL spec's own limit is 15 levels.
	maxTypeDepth = 64
)

// checkModuleSize reports whether a module of n bytes is within
// maxModuleBytes.
func checkModuleSize(n int64) error {
	if n > maxModuleBytes {
		return errModuleTooLarge
	}
	return nil
}
