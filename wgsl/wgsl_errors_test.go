package wgsl

import (
	"strings"
	"testing"
)

// tryParse attempts to parse WGSL source and returns the first error, if any.
func tryParse(source string) error {
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return err
	}
	parser := NewParser(tokens)
	_, err = parser.Parse()
	return err
}

// tryLower attempts to parse and lower WGSL source and returns the first error, if any.
func tryLower(source string) error {
	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return err
	}
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return err
	}
	_, err = LowerWithSource(ast, source)
	return err
}

// TestWGSLErrors_ParseErrors tests that invalid WGSL is correctly rejected at parse time.
func TestWGSLErrors_ParseErrors(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		errContains string
	}{
		// --- Unexpected token at top level ---
		{
			name:        "unexpected_token_at_top_level",
			source:      `42;`,
			errContains: "unexpected token",
		},
		{
			name:        "unexpected_keyword_at_top_level",
			source:      `return 0;`,
			errContains: "unexpected token",
		},

		// --- Missing function name ---
		{
			name:        "missing_function_name",
			source:      `fn () {}`,
			errContains: "expected function name",
		},

		// --- Missing opening paren in function ---
		{
			name:        "missing_function_open_paren",
			source:      `fn foo {}`,
			errContains: "expected (",
		},

		// --- Missing closing paren in function (parser tries to parse param) ---
		{
			name:        "missing_function_close_paren",
			source:      `fn foo( {}`,
			errContains: "expected parameter name",
		},

		// --- Missing function body brace ---
		{
			name:        "missing_function_body_open_brace",
			source:      `fn foo() return 0;`,
			errContains: "expected {",
		},

		// --- Missing struct name ---
		{
			name:        "missing_struct_name",
			source:      `struct { x: f32 }`,
			errContains: "expected struct name",
		},

		// --- Missing struct opening brace ---
		{
			name:        "missing_struct_open_brace",
			source:      `struct Foo x: f32`,
			errContains: "expected {",
		},

		// --- Missing member name in struct ---
		{
			name:        "missing_struct_member_name",
			source:      `struct Foo { : f32 }`,
			errContains: "expected member name",
		},

		// --- Missing colon in struct member ---
		{
			name:        "missing_struct_member_colon",
			source:      `struct Foo { x f32 }`,
			errContains: "expected :",
		},

		// --- Expected type in struct member ---
		{
			name:        "missing_struct_member_type",
			source:      `struct Foo { x: }`,
			errContains: "expected type",
		},

		// --- Missing variable name ---
		{
			name:        "missing_var_name",
			source:      `var : f32;`,
			errContains: "expected variable name",
		},

		// --- Missing constant name ---
		{
			name:        "missing_const_name",
			source:      `const = 1;`,
			errContains: "expected constant name",
		},

		// --- Missing const initializer (=) ---
		{
			name:        "missing_const_equal_sign",
			source:      `const x 1;`,
			errContains: "expected =",
		},

		// --- Missing let name ---
		{
			name:        "missing_let_name_top_level",
			source:      `let = 1;`,
			errContains: "expected variable name",
		},

		// --- Missing alias name ---
		{
			name:        "missing_alias_name",
			source:      `alias = f32;`,
			errContains: "expected alias name",
		},

		// --- Missing alias equal sign ---
		{
			name:        "missing_alias_equal",
			source:      `alias MyFloat f32;`,
			errContains: "expected =",
		},

		// --- Missing parameter name ---
		{
			name:        "missing_param_name",
			source:      `fn foo(: f32) {}`,
			errContains: "expected parameter name",
		},

		// --- Missing parameter colon ---
		{
			name:        "missing_param_colon",
			source:      `fn foo(x f32) {}`,
			errContains: "expected :",
		},

		// --- Missing parameter type ---
		{
			name:        "missing_param_type",
			source:      `fn foo(x: ) {}`,
			errContains: "expected type",
		},

		// --- Missing return type after arrow ---
		{
			name:        "missing_return_type",
			source:      `fn foo() -> {}`,
			errContains: "expected type",
		},

		// --- Unexpected token in expression ---
		{
			name:        "unexpected_token_in_expression",
			source:      `fn foo() { let x = ; }`,
			errContains: "unexpected token",
		},

		// --- Missing closing brace in function body ---
		{
			name:        "missing_function_close_brace",
			source:      `fn foo() { let x = 1;`,
			errContains: "expected }",
		},

		// --- Missing for loop open paren ---
		{
			name:        "missing_for_open_paren",
			source:      `fn foo() { for {} }`,
			errContains: "expected (",
		},

		// --- Missing member name after dot ---
		{
			name:        "missing_member_name_after_dot",
			source:      `fn foo() { let x = a.; }`,
			errContains: "expected member name",
		},

		// --- Missing closing paren in expression ---
		{
			name:        "missing_close_paren_in_expr",
			source:      `fn foo() { let x = (1 + 2; }`,
			errContains: "expected )",
		},

		// --- Missing let equal sign in function ---
		{
			name:        "missing_let_equal_in_function",
			source:      `fn foo() { let x 1; }`,
			errContains: "expected =",
		},

		// --- Expected case or default in switch ---
		{
			name:        "expected_case_or_default",
			source:      `fn foo() { switch x { 1 {} } }`,
			errContains: "expected 'case' or 'default'",
		},

		// --- Missing switch body brace ---
		{
			name:        "missing_switch_open_brace",
			source:      `fn foo() { switch x case 1 {} }`,
			errContains: "expected {",
		},

		// --- Ptr type missing address space ---
		{
			name:        "ptr_missing_address_space",
			source:      `fn foo(x: ptr<, f32>) {}`,
			errContains: "expected address space",
		},

		// --- Ptr type missing comma ---
		{
			name:        "ptr_missing_comma",
			source:      `fn foo(x: ptr<function f32>) {}`,
			errContains: "expected ,",
		},

		// --- Array missing closing angle bracket ---
		{
			name:        "array_missing_close_angle",
			source:      `fn foo(x: array<f32) {}`,
			errContains: "expected >",
		},

		// --- Bitcast missing angle bracket ---
		{
			name:        "bitcast_missing_less",
			source:      `fn foo() { let x = bitcast(1u); }`,
			errContains: "expected <",
		},

		// --- Loop missing opening brace ---
		{
			name:        "loop_missing_open_brace",
			source:      `fn foo() { loop return 0; }`,
			errContains: "expected {",
		},

		// --- Missing closing brace in struct ---
		{
			name:        "missing_struct_close_brace",
			source:      `struct Foo { x: f32`,
			errContains: "expected }",
		},

		// --- Const missing initializer value ---
		{
			name:        "const_missing_value",
			source:      `const X = ;`,
			errContains: "unexpected token",
		},

		// --- Multiple parsing errors (synchronization) ---
		{
			name:        "multiple_errors_synchronize",
			source:      `42; fn foo() {}`,
			errContains: "unexpected token",
		},

		// --- binding_array missing < ---
		{
			name:        "binding_array_missing_less",
			source:      `fn foo(x: binding_array) {}`,
			errContains: "expected <",
		},

		// --- Loop missing closing brace ---
		{
			name:        "loop_missing_close_brace",
			source:      `fn foo() { loop { break;`,
			errContains: "expected }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tryParse(tt.source)
			if err == nil {
				t.Fatal("expected parse error, got nil")
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

// TestWGSLErrors_LowerErrors tests that semantically invalid WGSL is rejected during lowering.
func TestWGSLErrors_LowerErrors(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		errContains string
	}{
		// --- Unknown type ---
		{
			name:        "unknown_type_in_var",
			source:      `fn foo() { var x: unknown_type; }`,
			errContains: "unknown type: unknown_type",
		},
		{
			name:        "unknown_type_in_function_param",
			source:      `fn foo(x: nonexistent_type) {}`,
			errContains: "unknown type: nonexistent_type",
		},
		{
			name:        "unknown_type_in_return",
			source:      `fn foo() -> bogus_type { return bogus_type(); }`,
			errContains: "unknown type: bogus_type",
		},

		// --- Unresolved identifier ---
		{
			name:        "unresolved_identifier",
			source:      `fn foo() { let x = unknown_var; }`,
			errContains: "unresolved identifier: unknown_var",
		},
		{
			name:        "unresolved_identifier_in_expression",
			source:      `fn foo() { let x = 1 + undefined_name; }`,
			errContains: "unresolved identifier: undefined_name",
		},

		// --- Unknown function ---
		{
			name:        "unknown_function",
			source:      `fn foo() { let x = nonexistent_func(1); }`,
			errContains: "unknown function: nonexistent_func",
		},

		// --- Global var missing type and initializer ---
		{
			name:        "global_var_no_type_no_init",
			source:      `var<private> x;`,
			errContains: "type annotation required without initializer",
		},

		// --- Global var unknown type ---
		{
			name:        "global_var_unknown_type",
			source:      `var<private> x: fake_type;`,
			errContains: "unknown type: fake_type",
		},

		// --- Local var no type no init ---
		{
			name:        "local_var_no_type_no_init",
			source:      `fn foo() { var x; }`,
			errContains: "type required without initializer",
		},

		// --- Module constant must have initializer ---
		{
			name:        "const_no_initializer",
			source:      `const X: f32;`,
			errContains: "expected =",
		},

		// --- Struct member unknown type ---
		{
			name:        "struct_member_unknown_type",
			source:      `struct Foo { x: some_undefined_type }`,
			errContains: "unknown type: some_undefined_type",
		},

		// --- Unknown type in global var with explicit type ---
		{
			name:        "unknown_parameterized_type",
			source:      `fn foo(x: custom_thing<f32>) {}`,
			errContains: "unsupported parameterized type: custom_thing",
		},

		// --- Atomic without type params resolves as unknown simple type ---
		{
			name:        "atomic_no_type_params",
			source:      `fn foo(x: atomic<>) {}`,
			errContains: "unknown type: atomic",
		},

		// --- Module constant with unsupported initializer ---
		{
			name:        "const_unsupported_binary_init",
			source:      `const X = 10 / 0;`,
			errContains: "unsupported initializer",
		},

		// --- Module constant with unsupported unary expression ---
		{
			name:        "const_unsupported_unary_init",
			source:      `const X = !true;`,
			errContains: "unsupported unary expression",
		},

		// --- Texture function wrong arg count ---
		{
			name: "texture_sample_bias_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
fn foo() {
	let x = textureSampleBias(t, s, vec2<f32>(0.0, 0.0));
}`,
			errContains: "textureSampleBias requires 4 arguments",
		},
		{
			name: "texture_sample_level_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
fn foo() {
	let x = textureSampleLevel(t, s, vec2<f32>(0.0, 0.0));
}`,
			errContains: "textureSampleLevel requires 4 arguments",
		},
		{
			name: "texture_sample_grad_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
fn foo() {
	let x = textureSampleGrad(t, s, vec2<f32>(0.0, 0.0));
}`,
			errContains: "textureSampleGrad requires 5 arguments",
		},

		// --- Select wrong arg count ---
		{
			name:        "select_wrong_arg_count",
			source:      `fn foo() { let x = select(1.0, 2.0); }`,
			errContains: "select() requires exactly 3 arguments",
		},

		// --- Math function missing args ---
		{
			name:        "math_function_no_args",
			source:      `fn foo() { let x = abs(); }`,
			errContains: "requires at least one argument",
		},

		// --- arrayLength wrong arg count ---
		{
			name:        "array_length_wrong_args",
			source:      `fn foo() { let x = arrayLength(); }`,
			errContains: "arrayLength requires exactly 1 argument",
		},

		// --- Derivative function wrong arg count ---
		{
			name:        "derivative_wrong_arg_count",
			source:      `fn foo() { let x = dpdx(1.0, 2.0); }`,
			errContains: "derivative function requires exactly 1 argument",
		},

		// --- textureStore wrong arg count ---
		{
			name: "texture_store_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;
fn foo() {
	textureStore(t, vec2<i32>(0, 0));
}`,
			errContains: "textureStore requires 3 arguments",
		},

		// --- Atomic function wrong arg count ---
		{
			name: "atomic_store_wrong_args",
			source: `
@group(0) @binding(0) var<storage, read_write> buf: atomic<u32>;
fn foo() {
	atomicStore(buf);
}`,
			errContains: "atomicStore requires 2 arguments",
		},
		// --- atomicCompareExchangeWeak wrong arg count ---
		{
			name: "atomic_compare_exchange_wrong_args",
			source: `
@group(0) @binding(0) var<storage, read_write> buf: atomic<u32>;
fn foo() {
	let x = atomicCompareExchangeWeak(buf, 1u);
}`,
			errContains: "atomicCompareExchangeWeak requires 3 arguments",
		},

		// --- Relational function wrong arg count ---
		{
			name:        "relational_function_wrong_args",
			source:      `fn foo() { let x = all(true, false); }`,
			errContains: "relational function requires exactly 1 argument",
		},

		// --- Unsupported parameterized type ---
		{
			name:        "unsupported_parameterized_type_weird",
			source:      `fn foo(x: something<i32>) {}`,
			errContains: "unsupported parameterized type: something",
		},

		// --- Unknown type in struct member used in function ---
		{
			name: "struct_member_bad_type_in_function",
			source: `
struct Foo { x: nonexistent }
fn foo() { var f: Foo; }`,
			errContains: "unknown type: nonexistent",
		},

		// --- Global var with unsupported call type ---
		{
			name:        "global_var_unsupported_call_type",
			source:      `var<private> x = unknownFunc(1);`,
			errContains: "unsupported call type: unknownFunc",
		},

		// --- Module constant must have initializer ---
		{
			name:        "const_must_have_init",
			source:      `const X: f32;`,
			errContains: "expected =",
		},

		// --- Local const must have initializer (parse level) ---
		{
			name:        "local_const_must_have_init",
			source:      `fn foo() { const x: f32; }`,
			errContains: "expected =",
		},

		// --- Unknown function in function body ---
		{
			name:        "unknown_function_in_body",
			source:      `fn foo() { nonexistent(); }`,
			errContains: "unknown function: nonexistent",
		},

		// --- Function param with unknown type (wrapped) ---
		{
			name:        "function_param_unknown_type_wrapped",
			source:      `fn foo(a: i32, b: fake_t) {}`,
			errContains: "unknown type: fake_t",
		},

		// --- textureSampleCompare wrong arg count ---
		{
			name: "texture_sample_compare_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
fn foo() {
	let x = textureSampleCompare(t, s, vec2<f32>(0.0, 0.0));
}`,
			errContains: "textureSampleCompare requires at least 4 arguments",
		},

		// --- textureSampleBaseClampToEdge wrong arg count ---
		{
			name: "texture_sample_base_clamp_wrong_args",
			source: `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
fn foo() {
	let x = textureSampleBaseClampToEdge(t, s);
}`,
			errContains: "textureSampleBaseClampToEdge requires at least 3 arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tryLower(tt.source)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
			}
		})
	}
}
