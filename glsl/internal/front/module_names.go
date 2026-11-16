package front

// This file holds the module-scope name table (P2-GLSLIN-026) and the
// reserved-identifier refusal (P2-GLSLIN-025) every declaration of a
// module-scope name runs first.
//
// The table is one entry per name a declaration puts in the module's
// namespace: variables and constants of every storage class, plain uniforms
// collected into the default block, opaque resources, combined image samplers,
// block instances with a name, and the members of an instance-less interface
// block — those are the names GLSL resolves at global scope. The members of a
// block declared with an instance name are reached through that instance and
// are not module-scope names (GLSL 4.60 §4.3.9; glslang accepts two such blocks
// with a member of the same name, and a plain uniform equal to one of their
// members). struct type names and struct members are a namespace of their own:
// glslang accepts `struct gl_S { float gl_y; };` in both its GL and Vulkan
// modes, so the reserved-prefix refusal does not reach them either.
//
// A repeated name used to lower silently — the later declaration won, the
// earlier one's references still resolved to it — where glslang refuses with
// "'a' : redefinition" (P2-GLSLIN-026). A name beginning with the
// implementation's gl_ prefix was accepted where glslang refuses every user
// declaration of one with "'gl_x' : identifiers starting with "gl_" are
// reserved" (probed 2026-11-16, glslangValidator 11:16.2.0).
//
// Rust naga's GLSL frontend has neither check: SymbolTable::add replaces a name
// silently and add_global discards the replaced symbol (context.rs:232), and
// nothing there examines the gl_ prefix. Both refusals are therefore a
// deliberate difference from Rust in the direction the specification decides —
// the frontend's better-where-the-spec-decides principle — and each is pinned
// by the tests named above.

import (
	"strings"

	"github.com/gogpu/naga/ir"
)

// reservedIdentifierPrefix is the prefix GLSL reserves for the implementation
// (GLSL 4.60 §3.7: identifiers starting with "gl_" are reserved for use by
// OpenGL and may not be declared in a shader as a variable or a function).
const reservedIdentifierPrefix = "gl_"

// declareModuleName records one module-scope name and refuses a reserved or
// repeated one: the checks every declaration of a module-scope name runs
// before it builds anything. The span is the declaration's own, so a refusal
// points at the shader's text.
func (f *Frontend) declareModuleName(name string, span ir.Span) *frontError {
	if err := refuseReservedIdentifier(name, span); err != nil {
		return err
	}
	if f.moduleNames[name] {
		return errRedefinition(name, span)
	}
	f.moduleNames[name] = true
	return nil
}

// refuseReservedIdentifier refuses a user declaration whose name begins with
// the reserved gl_ prefix, with glslang's wording. The builtin variables and
// functions (`gl_Position` and the rest) are references, not declarations, and
// never pass through here; a shader that redeclares one is refused like any
// other declaration of a reserved name, as glslang refuses it.
func refuseReservedIdentifier(name string, span ir.Span) *frontError {
	if !strings.HasPrefix(name, reservedIdentifierPrefix) {
		return nil
	}
	return errSemantic(span, "'%s' : identifiers starting with \"gl_\" are reserved", name)
}
