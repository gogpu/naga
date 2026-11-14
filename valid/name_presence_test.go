package valid_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// namePresenceModule is a small module that exercises every entity the IR
// records name presence for.
func namePresenceModule(t *testing.T) *ir.Module {
	t.Helper()
	return parse(t, "@group(0) @binding(0) var<storage, read_write> data: array<u32>;\n"+
		"struct S { x: u32 }\n"+
		"var<private> s: S;\n"+
		"const c: i32 = 1;\n"+
		"override o: f32 = 1.0;\n"+
		"fn helper(a: i32) -> i32 { var l: i32 = a; return l + c; }\n"+
		"@compute @workgroup_size(1) fn main() { s.x = data[0]; data[0] = u32(helper(1)) + u32(o); }\n")
}

// TestNamePresenceInvariant: a set NamePresent with a non-empty name is
// rejected — the pair would otherwise have two encodings of "present" — for
// every entity kind the IR records presence for.
func TestNamePresenceInvariant(t *testing.T) {
	baseline := namePresenceModule(t)
	if _, err := valid.Validate(baseline, valid.Options{}); err != nil {
		t.Fatalf("baseline: %v", err)
	}

	tests := []struct {
		name string
		set  func(*ir.Module) bool
	}{
		{
			name: "global variable",
			set: func(m *ir.Module) bool {
				for i := range m.GlobalVariables {
					if m.GlobalVariables[i].Name != "" {
						m.GlobalVariables[i].NamePresent = true
						return true
					}
				}
				return false
			},
		},
		{
			name: "constant",
			set: func(m *ir.Module) bool {
				for i := range m.Constants {
					if m.Constants[i].Name != "" {
						m.Constants[i].NamePresent = true
						return true
					}
				}
				return false
			},
		},
		{
			name: "override",
			set: func(m *ir.Module) bool {
				for i := range m.Overrides {
					if m.Overrides[i].Name != "" {
						m.Overrides[i].NamePresent = true
						return true
					}
				}
				return false
			},
		},
		{
			name: "type",
			set: func(m *ir.Module) bool {
				for i := range m.Types {
					if m.Types[i].Name != "" {
						m.Types[i].NamePresent = true
						return true
					}
				}
				return false
			},
		},
		{
			name: "struct member",
			set: func(m *ir.Module) bool {
				for i := range m.Types {
					if st, ok := m.Types[i].Inner.(ir.StructType); ok {
						for j := range st.Members {
							if st.Members[j].Name != "" {
								st.Members[j].NamePresent = true
								m.Types[i].Inner = st
								return true
							}
						}
					}
				}
				return false
			},
		},
		{
			name: "function",
			set: func(m *ir.Module) bool {
				for i := range m.Functions {
					if m.Functions[i].Name != "" {
						m.Functions[i].NamePresent = true
						return true
					}
				}
				return false
			},
		},
		{
			name: "function argument",
			set: func(m *ir.Module) bool {
				for i := range m.Functions {
					for j := range m.Functions[i].Arguments {
						if m.Functions[i].Arguments[j].Name != "" {
							m.Functions[i].Arguments[j].NamePresent = true
							return true
						}
					}
				}
				return false
			},
		},
		{
			name: "local variable",
			set: func(m *ir.Module) bool {
				for i := range m.Functions {
					for j := range m.Functions[i].LocalVars {
						if m.Functions[i].LocalVars[j].Name != "" {
							m.Functions[i].LocalVars[j].NamePresent = true
							return true
						}
					}
				}
				return false
			},
		},
		{
			name: "entry point function",
			set: func(m *ir.Module) bool {
				for i := range m.EntryPoints {
					if m.EntryPoints[i].Function.Name != "" {
						m.EntryPoints[i].Function.NamePresent = true
						return true
					}
				}
				return false
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := namePresenceModule(t)
			if !tt.set(m) {
				t.Fatalf("the module has no named %s", tt.name)
			}
			expectRejected(t, m, valid.Options{}, "NamePresent is set on a non-empty name")
		})
	}
}

// TestNamePresenceEmptyNameIsValid: a present but empty name passes the
// validator — it is the SPIR-V Some("") case the namers render "unnamed".
func TestNamePresenceEmptyNameIsValid(t *testing.T) {
	m := namePresenceModule(t)
	changed := false
	for i := range m.GlobalVariables {
		if m.GlobalVariables[i].Name != "" {
			m.GlobalVariables[i].Name = ""
			m.GlobalVariables[i].NamePresent = true
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("the module has no named global")
	}
	if _, err := valid.Validate(m, valid.Options{}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
