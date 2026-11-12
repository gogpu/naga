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
)

// checkModuleSize reports whether a module of n bytes is within
// maxModuleBytes.
func checkModuleSize(n int64) error {
	if n > maxModuleBytes {
		return errModuleTooLarge
	}
	return nil
}
