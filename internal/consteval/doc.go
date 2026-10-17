// Package consteval evaluates WGSL const-expressions: the shader-creation-time
// evaluation that the WGSL specification requires for const declarations and
// that naga also applies to constant operands in function bodies.
//
// It is the only constant evaluator in naga, used by every lowering context,
// like Rust naga's proc::ConstantEvaluator (ADR-007). It has two layers:
//
//   - The numeric kernel works on scalars and vectors of ir.LiteralValue:
//     Math, Binary, Unary, Cast, Relational and Select. It has no arena.
//   - The Evaluator applies the kernel to an expression arena, appending the
//     evaluated result, as Rust's try_eval_and_append does.
//
// A result is always finite and representable: an operation whose result
// would overflow, leave its function's domain, divide by zero or not fit the
// target type returns an *Error instead, as the specification requires
// ("A shader-creation error results if any const-expression of floating-point
// type overflows or evaluates to NaN or infinity"). Arithmetic on concrete
// integer types wraps modulo 2^bitwidth, also as specified.
package consteval
