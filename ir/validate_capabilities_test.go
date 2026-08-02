package ir

import (
	"strings"
	"testing"
)

// =============================================================================
// Capability validation tests — tests that ValidateWithCapabilities correctly
// enforces capability restrictions on scalar types (f64, i64, u64, f16).
// =============================================================================

// newModuleWithType creates a minimal module containing a single type.
func newModuleWithType(inner TypeInner) *Module {
	return &Module{
		Types: []Type{
			{Name: "", Inner: inner},
		},
	}
}

// expectCapErrors validates a module with given caps and expects errors containing substrings.
func expectCapErrors(t *testing.T, module *Module, caps Capabilities, expectedSubstrings ...string) {
	t.Helper()
	errors, err := ValidateWithCapabilities(module, caps)
	if err != nil {
		t.Fatalf("ValidateWithCapabilities returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Fatalf("expected validation errors, got none")
	}
	for _, expected := range expectedSubstrings {
		found := false
		for _, ve := range errors {
			if strings.Contains(ve.Error(), expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected validation error containing %q, but not found.\nErrors: %v", expected, errors)
		}
	}
}

// expectNoCapErrors validates a module with given caps and expects no errors.
func expectNoCapErrors(t *testing.T, module *Module, caps Capabilities) {
	t.Helper()
	errors, err := ValidateWithCapabilities(module, caps)
	if err != nil {
		t.Fatalf("ValidateWithCapabilities returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("expected no validation errors, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  - %s", e.Error())
		}
	}
}

func TestValidateWithCapabilities_ScalarTypes(t *testing.T) {
	tests := []struct {
		name    string
		inner   TypeInner
		caps    Capabilities
		wantErr string // empty = no error expected
	}{
		// f64 (Float width=8)
		{"f64 rejected without cap", ScalarType{Kind: ScalarFloat, Width: 8}, 0, "FLOAT64"},
		{"f64 accepted with cap", ScalarType{Kind: ScalarFloat, Width: 8}, CapFloat64, ""},
		{"f64 accepted with CapAll", ScalarType{Kind: ScalarFloat, Width: 8}, CapAll, ""},
		// i64 (Sint width=8)
		{"i64 rejected without cap", ScalarType{Kind: ScalarSint, Width: 8}, 0, "SHADER_INT64"},
		{"i64 accepted with cap", ScalarType{Kind: ScalarSint, Width: 8}, CapShaderInt64, ""},
		// u64 (Uint width=8)
		{"u64 rejected without cap", ScalarType{Kind: ScalarUint, Width: 8}, 0, "SHADER_INT64"},
		{"u64 accepted with cap", ScalarType{Kind: ScalarUint, Width: 8}, CapShaderInt64, ""},
		// f16 (Float width=2)
		{"f16 rejected without cap", ScalarType{Kind: ScalarFloat, Width: 2}, 0, "SHADER_FLOAT16"},
		{"f16 accepted with cap", ScalarType{Kind: ScalarFloat, Width: 2}, CapShaderFloat16, ""},
		// Always-allowed types
		{"f32 always accepted", ScalarType{Kind: ScalarFloat, Width: 4}, 0, ""},
		{"i32 always accepted", ScalarType{Kind: ScalarSint, Width: 4}, 0, ""},
		{"u32 always accepted", ScalarType{Kind: ScalarUint, Width: 4}, 0, ""},
		{"bool always accepted", ScalarType{Kind: ScalarBool, Width: 1}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModuleWithType(tt.inner)
			if tt.wantErr != "" {
				expectCapErrors(t, m, tt.caps, tt.wantErr)
			} else {
				expectNoCapErrors(t, m, tt.caps)
			}
		})
	}
}

func TestValidateWithCapabilities_VectorTypes(t *testing.T) {
	tests := []struct {
		name    string
		inner   TypeInner
		caps    Capabilities
		wantErr string
	}{
		{"vec4<f64> rejected", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}, 0, "FLOAT64"},
		{"vec4<f64> accepted", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}, CapFloat64, ""},
		{"vec3<i64> rejected", VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarSint, Width: 8}}, 0, "SHADER_INT64"},
		{"vec3<i64> accepted", VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarSint, Width: 8}}, CapShaderInt64, ""},
		{"vec2<u64> rejected", VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarUint, Width: 8}}, 0, "SHADER_INT64"},
		{"vec2<u64> accepted", VectorType{Size: Vec2, Scalar: ScalarType{Kind: ScalarUint, Width: 8}}, CapShaderInt64, ""},
		{"vec4<f16> rejected", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}, 0, "SHADER_FLOAT16"},
		{"vec4<f16> accepted", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}, CapShaderFloat16, ""},
		{"vec4<f32> always accepted", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModuleWithType(tt.inner)
			if tt.wantErr != "" {
				expectCapErrors(t, m, tt.caps, tt.wantErr)
			} else {
				expectNoCapErrors(t, m, tt.caps)
			}
		})
	}
}

func TestValidateWithCapabilities_MatrixTypes(t *testing.T) {
	tests := []struct {
		name    string
		inner   TypeInner
		caps    Capabilities
		wantErr string
	}{
		{"mat4x4<f64> rejected", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}, 0, "FLOAT64"},
		{"mat4x4<f64> accepted", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 8}}, CapFloat64, ""},
		{"mat2x2<f16> rejected", MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}, 0, "SHADER_FLOAT16"},
		{"mat2x2<f16> accepted", MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarFloat, Width: 2}}, CapShaderFloat16, ""},
		{"mat4x4<f32> always accepted", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModuleWithType(tt.inner)
			if tt.wantErr != "" {
				expectCapErrors(t, m, tt.caps, tt.wantErr)
			} else {
				expectNoCapErrors(t, m, tt.caps)
			}
		})
	}
}

func TestValidateWithCapabilities_AtomicTypes(t *testing.T) {
	tests := []struct {
		name    string
		inner   TypeInner
		caps    Capabilities
		wantErr string
	}{
		{"atomic<i64> rejected", AtomicType{Scalar: ScalarType{Kind: ScalarSint, Width: 8}}, 0, "SHADER_INT64"},
		{"atomic<i64> accepted", AtomicType{Scalar: ScalarType{Kind: ScalarSint, Width: 8}}, CapShaderInt64, ""},
		{"atomic<u64> rejected", AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 8}}, 0, "SHADER_INT64"},
		{"atomic<u64> accepted", AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 8}}, CapShaderInt64, ""},
		{"atomic<i32> always accepted", AtomicType{Scalar: ScalarType{Kind: ScalarSint, Width: 4}}, 0, ""},
		{"atomic<u32> always accepted", AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModuleWithType(tt.inner)
			if tt.wantErr != "" {
				expectCapErrors(t, m, tt.caps, tt.wantErr)
			} else {
				expectNoCapErrors(t, m, tt.caps)
			}
		})
	}
}

func TestValidateWithCapabilities_BackwardCompatibility(t *testing.T) {
	// Validate() (without capabilities) uses CapAll — should accept everything.
	m := &Module{
		Types: []Type{
			{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}},
			{Name: "i64", Inner: ScalarType{Kind: ScalarSint, Width: 8}},
			{Name: "u64", Inner: ScalarType{Kind: ScalarUint, Width: 8}},
			{Name: "f16", Inner: ScalarType{Kind: ScalarFloat, Width: 2}},
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}

	errors, err := Validate(m)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(errors) > 0 {
		t.Errorf("Validate() with CapAll should accept all types, got %d errors:", len(errors))
		for _, e := range errors {
			t.Errorf("  - %s", e.Error())
		}
	}
}

func TestValidateWithCapabilities_MultipleCaps(t *testing.T) {
	// Combined capabilities allow multiple type widths.
	m := &Module{
		Types: []Type{
			{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}},
			{Name: "i64", Inner: ScalarType{Kind: ScalarSint, Width: 8}},
		},
	}

	// Only CapFloat64 — i64 should be rejected
	errors, err := ValidateWithCapabilities(m, CapFloat64)
	if err != nil {
		t.Fatalf("ValidateWithCapabilities returned error: %v", err)
	}
	if len(errors) == 0 {
		t.Fatal("expected errors for i64 without SHADER_INT64")
	}
	foundInt64Err := false
	for _, e := range errors {
		if strings.Contains(e.Error(), "SHADER_INT64") {
			foundInt64Err = true
		}
		// Should NOT have FLOAT64 error
		if strings.Contains(e.Error(), "FLOAT64") {
			t.Error("unexpected FLOAT64 error — f64 should be accepted with CapFloat64")
		}
	}
	if !foundInt64Err {
		t.Error("expected SHADER_INT64 error for i64")
	}

	// Both caps — all should pass
	expectNoCapErrors(t, m, CapFloat64|CapShaderInt64)
}

func TestValidateWithCapabilities_NoCaps(t *testing.T) {
	// With zero capabilities, ALL extended types should be rejected.
	m := &Module{
		Types: []Type{
			{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}},
			{Name: "i64", Inner: ScalarType{Kind: ScalarSint, Width: 8}},
			{Name: "u64", Inner: ScalarType{Kind: ScalarUint, Width: 8}},
			{Name: "f16", Inner: ScalarType{Kind: ScalarFloat, Width: 2}},
		},
	}

	errors, err := ValidateWithCapabilities(m, 0)
	if err != nil {
		t.Fatalf("ValidateWithCapabilities returned error: %v", err)
	}
	// Expect 4 errors — one for each type
	if len(errors) != 4 {
		t.Errorf("expected 4 validation errors, got %d:", len(errors))
		for _, e := range errors {
			t.Errorf("  - %s", e.Error())
		}
	}
}

func TestValidateWithCapabilities_StructWithF64Member(t *testing.T) {
	// Struct members reference types by handle — the type itself is validated
	// in the validateTypes loop, so a struct containing an f64 member type
	// will produce the error on the f64 type, not on the struct.
	m := &Module{
		Types: []Type{
			{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}}, // type[0]
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // type[1]
			{Name: "MyStruct", Inner: StructType{Members: []StructMember{ // type[2]
				{Name: "a", Type: TypeHandle(0)}, // f64
				{Name: "b", Type: TypeHandle(1)}, // f32
			}}},
		},
	}

	// Without CapFloat64, the f64 type itself is rejected
	expectCapErrors(t, m, 0, "FLOAT64")

	// With CapFloat64, everything passes
	expectNoCapErrors(t, m, CapFloat64)
}

func TestValidateWithCapabilities_Contains(t *testing.T) {
	tests := []struct {
		name string
		c    Capabilities
		flag Capabilities
		want bool
	}{
		{"zero contains nothing", 0, CapFloat64, false},
		{"CapFloat64 contains CapFloat64", CapFloat64, CapFloat64, true},
		{"CapFloat64 does not contain CapShaderInt64", CapFloat64, CapShaderInt64, false},
		{"CapAll contains CapFloat64", CapAll, CapFloat64, true},
		{"CapAll contains CapShaderInt64", CapAll, CapShaderInt64, true},
		{"CapAll contains CapShaderFloat16", CapAll, CapShaderFloat16, true},
		{"combined caps", CapFloat64 | CapShaderInt64, CapFloat64, true},
		{"combined caps missing", CapFloat64 | CapShaderInt64, CapShaderFloat16, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.Contains(tt.flag)
			if got != tt.want {
				t.Errorf("Capabilities(%d).Contains(%d) = %v, want %v", tt.c, tt.flag, got, tt.want)
			}
		})
	}
}
