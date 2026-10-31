// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// lowerWGSLNoFatal is lowerWGSL returning the frontend's error.
func lowerWGSLNoFatal(source string) (*ir.Module, error) {
	tokens, err := wgsl.NewLexer(source).Tokenize()
	if err != nil {
		return nil, err
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		return nil, err
	}
	return wgsl.LowerWithSource(ast, source)
}

// TestValidateControlFlowAbilities follows Rust naga's ControlFlowAbility
// (valid/function.rs): a switch case may break, a loop body may break and
// continue, a continuing block may do neither and may not return, and a
// loop inside a continuing block may break and continue again. The WGSL
// spec allows a break in a switch outside any loop; we rejected it with
// "break outside of loop".
func TestValidateControlFlowAbilities(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string // "" = valid
	}{
		{"break in switch outside loop", `switch a { case 1u: { if a > 0u { break; } } default: {} }`, ""},
		{"break in switch in loop", `loop { switch a { case 1u: { break; } default: { break; } } }`, ""},
		{"continue in switch in loop", `for (var i = 0u; i < 2u; i++) { switch a { case 1u: { continue; } default: {} } }`, ""},
		{"loop inside continuing", `var i = 0u; loop { if i > 2u { break; } continuing { loop { break; } i++; } }`, ""},
		{"continue in switch outside loop", `switch a { case 1u: { continue; } default: {} }`, "`continue` is used outside of a `loop`"},
		// Rust checks only the stage of a discard; we rejected it in a continuing block.
		{"discard in continuing", `var i = 0u; loop { if i > a { break; } continuing { if i == 3u { discard; } i++; } }`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) a: u32) {\n" + tt.body + "\n}"
			if strings.Contains(tt.body, "discard") {
				src = "@fragment fn main(@location(0) @interpolate(flat) a: u32) {\n" + tt.body + "\n}"
			}
			mod, err := lowerWGSLNoFatal(src)
			if err != nil {
				if tt.wantErr != "" {
					return // the frontend rejected it first, which is fine
				}
				t.Fatalf("lower: %v", err)
			}
			errs, err := ir.Validate(mod)
			if err != nil {
				t.Fatal(err)
			}
			var msgs []string
			for _, e := range errs {
				msgs = append(msgs, e.Error())
			}
			all := strings.Join(msgs, "\n")
			switch {
			case tt.wantErr == "" && len(errs) > 0:
				t.Errorf("valid shader rejected:\n%s", all)
			case tt.wantErr != "" && !strings.Contains(all, tt.wantErr):
				t.Errorf("errors %q, want one containing %q", all, tt.wantErr)
			}
		})
	}
}
