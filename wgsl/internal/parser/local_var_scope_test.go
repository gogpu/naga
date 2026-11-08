package parser

import (
	"errors"
	"strings"
	"testing"
)

// TestLocalVarAddressSpace pins WGSL §var-decls: "Variables in the private,
// storage, uniform, workgroup, and handle address spaces must only be declared
// in module scope, while variables in the function address space must only be
// declared in function scope."
//
// Rust naga rejects a local declaration of any other space in the parser,
// before the name is read (InvalidLocalVariableAddressSpace,
// front/wgsl/parse/mod.rs:1213); the message and the first note below are
// Rust's, the spec line is ours.
func TestLocalVarAddressSpace(t *testing.T) {
	for _, space := range []string{"workgroup", "private", "storage", "uniform"} {
		t.Run(space, func(t *testing.T) {
			src := "fn f() { var<" + space + "> v: u32; }"
			_, err := tryParseSource(t, src)
			if err == nil {
				t.Fatalf("var<%s> in a function body parsed, want the invalid address space error", space)
			}
			want := "invalid address space for local variable: `" + space + "`"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}

			var pe ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %T, want ParseError", err)
			}
			if pe.Token.Lexeme != space {
				t.Errorf("error token = %q, want the address space word %q", pe.Token.Lexeme, space)
			}
			wantNotes := []string{
				"local variables can only use 'function' address space",
				"WGSL spec §var-decls: variables in the private, storage, uniform, workgroup and handle address spaces must only be declared in module scope",
			}
			if len(pe.Notes) != len(wantNotes) {
				t.Fatalf("notes = %q, want %q", pe.Notes, wantNotes)
			}
			for i, wantNote := range wantNotes {
				if pe.Notes[i] != wantNote {
					t.Errorf("notes[%d] = %q, want %q", i, pe.Notes[i], wantNote)
				}
			}
		})
	}
}

// TestLocalVarFunctionSpaceAccepted keeps the valid function-scope forms: an
// explicit var<function> (optional per WGSL §var-decls, "Specifying the
// function address space is optional") and a plain var.
func TestLocalVarFunctionSpaceAccepted(t *testing.T) {
	for _, src := range []string{
		"fn f() { var<function> v: u32; }",
		"fn f() { var v: u32; }",
	} {
		if _, err := tryParseSource(t, src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

// TestModuleScopeWorkgroupVarAccepted keeps the check local: var<workgroup> at
// module scope is the normal declaration and still parses.
func TestModuleScopeWorkgroupVarAccepted(t *testing.T) {
	module := parseSource(t, "var<workgroup> w: array<u32, 64>;")
	if len(module.GlobalVars) != 1 || module.GlobalVars[0].AddressSpace != "workgroup" {
		t.Fatalf("global vars = %+v, want one workgroup variable", module.GlobalVars)
	}
}
