package front

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// ppBlock is one open conditional block.
type ppBlock struct {
	span         ir.Span
	hadValid     bool
	hadElse      bool
	outerSkipped bool
}

// directiveState turns the token stream into the preprocessed one: directives
// become ppResult metadata, conditional skipping follows the #if family, and
// macros expand.
type directiveState struct {
	macros map[string]*ppMacro

	version      ppVersion
	hasVersion   bool
	extensions   []ppExtension
	pragmas      []ppPragma
	lineOffset   int64
	hadDirective bool
	hadToken     bool
	blocks       []ppBlock
	skipping     bool

	errors    []ppError
	budget    int
	budgetHit bool
}

// errorf records a diagnostic.
func (d *directiveState) errorf(span ir.Span, format string, args ...any) {
	d.errors = append(d.errors, ppError{Message: fmt.Sprintf(format, args...), Span: span})
}

// process runs the directives over toks and returns the expanded stream. Every
// directive is consumed through the end of its line; conditional blocks decide
// whether the tokens between directives are kept.
func (d *directiveState) process(toks []ppToken) []ppToken {
	var out []ppToken
	for i := 0; i < len(toks); {
		t := toks[i]
		if t.kind == ppHash && t.startOfLine {
			i++
			name := ""
			if i < len(toks) && toks[i].kind == ppIdent && toks[i].line == t.line {
				name = toks[i].text
				i++
			} else if !d.skipping {
				d.errorf(t.span, "expected a directive name")
			}
			start := i
			for i < len(toks) && toks[i].kind != ppNewline {
				i++
			}
			line := toks[start:i]
			if i < len(toks) {
				i++ // the newline that ends the directive
			}
			d.directive(name, line, t)
			continue
		}
		i++
		if t.kind == ppNewline || d.skipping {
			continue
		}
		if t.kind == ppHash {
			d.errorf(t.span, "unexpected '#'")
			continue
		}
		d.hadToken = true
		t.lineOffset = d.lineOffset
		out = append(out, t)
	}
	if len(d.blocks) > 0 {
		d.errorf(d.blocks[0].span, "unterminated conditional directive")
	}
	return d.expandAll(out)
}

// directive handles one directive line.
func (d *directiveState) directive(name string, line []ppToken, hash ppToken) {
	switch name {
	case "define":
		if !d.skipping {
			d.defineDirective(line, hash.span)
		}
	case "undef":
		if !d.skipping {
			d.undefDirective(line, hash.span)
		}
	case "if":
		d.ifDirective(line, hash.span)
	case "ifdef":
		d.ifDefDirective(line, hash.span, true)
	case "ifndef":
		d.ifDefDirective(line, hash.span, false)
	case "elif":
		d.elifDirective(line, hash.span)
	case "else":
		d.elseDirective(line, hash.span)
	case "endif":
		d.endifDirective(line, hash.span)
	case "version":
		if !d.skipping {
			d.versionDirective(line, hash.span)
		}
	case "extension":
		if !d.skipping {
			d.extensionDirective(line, hash.span)
		}
	case "pragma":
		if !d.skipping {
			d.pragmaDirective(line, hash.span)
		}
	case "line":
		if !d.skipping {
			d.lineDirective(line, hash)
		}
	case "error":
		if !d.skipping {
			d.errorf(hash.span, "error directive")
		}
	default:
		if !d.skipping && name != "" {
			d.errorf(hash.span, "unknown directive: #%s", name)
		}
	}
	d.hadDirective = true
}

// identArg takes the leading identifier of a directive line and returns the
// rest.
func identArg(line []ppToken) (string, []ppToken) {
	if len(line) > 0 && line[0].kind == ppIdent {
		return line[0].text, line[1:]
	}
	return "", line
}

// ifDirective handles #if. The expression is evaluated only when the enclosing
// block is active; a skipped block pushes its own skipped block so nesting stays
// balanced.
func (d *directiveState) ifDirective(line []ppToken, span ir.Span) {
	block := ppBlock{span: span, outerSkipped: d.skipping}
	if !d.skipping {
		if len(d.blocks) >= maxPPNesting {
			d.errorf(span, "conditional directives nest deeper than %d", maxPPNesting)
		}
		cond, ok := d.evalCondition(line, span)
		d.skipping = !ok || cond == 0
		block.hadValid = !d.skipping
	}
	d.blocks = append(d.blocks, block)
}

// ifDefDirective handles #ifdef and #ifndef.
func (d *directiveState) ifDefDirective(line []ppToken, span ir.Span, want bool) {
	block := ppBlock{span: span, outerSkipped: d.skipping}
	if !d.skipping {
		if len(d.blocks) >= maxPPNesting {
			d.errorf(span, "conditional directives nest deeper than %d", maxPPNesting)
		}
		name, rest := identArg(line)
		if name == "" || len(rest) > 0 {
			d.errorf(span, "expected an identifier")
			d.skipping = true
		} else {
			defined := d.isDefined(name)
			if !want {
				defined = !defined
			}
			d.skipping = !defined
			block.hadValid = defined
		}
	}
	d.blocks = append(d.blocks, block)
}

// elifDirective handles #elif. A block that already had a true branch skips the
// condition entirely, as pp-rs does.
func (d *directiveState) elifDirective(line []ppToken, span ir.Span) {
	if len(d.blocks) == 0 {
		d.errorf(span, "#elif without #if")
		return
	}
	block := &d.blocks[len(d.blocks)-1]
	if block.hadElse {
		d.errorf(span, "#elif after #else")
		return
	}
	if block.outerSkipped || block.hadValid {
		d.skipping = true
		return
	}
	cond, ok := d.evalCondition(line, span)
	if ok && cond != 0 {
		d.skipping = false
		block.hadValid = true
	} else {
		d.skipping = true
	}
}

// elseDirective handles #else.
func (d *directiveState) elseDirective(line []ppToken, span ir.Span) {
	if len(line) > 0 && !d.skipping {
		d.errorf(line[0].span, "unexpected token after #else")
	}
	if len(d.blocks) == 0 {
		d.errorf(span, "#else without #if")
		return
	}
	block := &d.blocks[len(d.blocks)-1]
	if block.hadElse {
		d.errorf(span, "#else after #else")
		return
	}
	block.hadElse = true
	d.skipping = block.outerSkipped || block.hadValid
}

// endifDirective handles #endif.
func (d *directiveState) endifDirective(line []ppToken, span ir.Span) {
	if len(d.blocks) == 0 {
		d.errorf(span, "#endif without #if")
		return
	}
	if len(line) > 0 && !d.skipping {
		d.errorf(line[0].span, "unexpected token after #endif")
	}
	block := d.blocks[len(d.blocks)-1]
	d.blocks = d.blocks[:len(d.blocks)-1]
	d.skipping = block.outerSkipped
}

// isDefined reports whether name is defined, predefined macros included.
func (d *directiveState) isDefined(name string) bool {
	switch name {
	case "__LINE__":
		return true
	case "__VERSION__":
		return d.hasVersion
	case "GL_ES":
		return d.hasVersion && d.version.Profile == ppProfileES
	}
	_, ok := d.macros[name]
	return ok
}

// versionDirective handles #version: 440/450/460 with an optional core profile,
// 330 core and 300 es. Anything else, and a #version that is not the first
// directive, is an error (the frontend requires the directive).
func (d *directiveState) versionDirective(line []ppToken, span ir.Span) {
	if d.hadDirective || d.hadToken {
		d.errorf(span, "#version must occur first in shader")
	}
	if len(line) == 0 || line[0].kind != ppInt {
		d.errorf(span, "expected a version number")
		return
	}
	if line[0].intValue > math.MaxUint16 {
		d.errorf(line[0].span, "Invalid version: %d", line[0].intValue)
		return
	}
	value := uint16(line[0].intValue)
	profileName := ""
	if len(line) > 1 {
		if line[1].kind != ppIdent {
			d.errorf(line[1].span, "unexpected token after the version number")
			return
		}
		profileName = line[1].text
		if len(line) > 2 {
			d.errorf(line[2].span, "unexpected token after #version")
			return
		}
	}
	var profile ppProfile
	switch value {
	case 440, 450, 460:
		switch profileName {
		case "", "core":
			profile = ppProfileCore
		case "es", "compatibility":
			d.errorf(span, "Invalid profile: %s", profileName)
			return
		default:
			d.errorf(span, "Invalid profile: %s", profileName)
			return
		}
	case 330:
		if profileName != "core" {
			d.errorf(span, "Invalid profile: %s (330 requires core)", profileName)
			return
		}
		profile = ppProfileCore
	case 300:
		if profileName != "es" {
			d.errorf(span, "Invalid profile: %s (300 requires es)", profileName)
			return
		}
		profile = ppProfileES
	default:
		d.errorf(line[0].span, "Invalid version: %d", value)
		return
	}
	d.version = ppVersion{Value: value, Profile: profile, Span: span}
	d.hasVersion = true
}

// extensionDirective handles #extension name : behavior.
func (d *directiveState) extensionDirective(line []ppToken, span ir.Span) {
	if len(line) != 3 || line[0].kind != ppIdent || !line[1].isPunct(":") || line[2].kind != ppIdent {
		d.errorf(span, "expected #extension name : behavior")
		return
	}
	var behavior ppExtensionBehavior
	switch line[2].text {
	case "require":
		behavior = ppExtensionRequire
	case "enable":
		behavior = ppExtensionEnable
	case "warn":
		behavior = ppExtensionWarn
	case "disable":
		behavior = ppExtensionDisable
	default:
		d.errorf(line[2].span, "invalid extension behavior: %s", line[2].text)
		return
	}
	d.extensions = append(d.extensions, ppExtension{Name: line[0].text, Behavior: behavior, Span: span})
}

// pragmaDirective handles #pragma, recording its name and arguments. The
// frontend accepts #pragma shader_stage(stage) and warns for the rest.
func (d *directiveState) pragmaDirective(line []ppToken, span ir.Span) {
	if len(line) == 0 || line[0].kind != ppIdent {
		d.errorf(span, "expected a pragma name")
		return
	}
	d.pragmas = append(d.pragmas, ppPragma{
		Name: line[0].text,
		Args: append([]ppToken(nil), line[1:]...),
		Span: span,
	})
}

// lineDirective handles #line: it shifts the line number __LINE__ reports. The
// optional source-file number is evaluated and ignored, as pp-rs ignores it.
func (d *directiveState) lineDirective(line []ppToken, hash ppToken) {
	value, rest, ok := d.evalIntExpr(line, hash.span)
	if !ok {
		return
	}
	if value < 0 || value > math.MaxInt32 {
		d.errorf(hash.span, "line number out of range: %d", value)
		return
	}
	if len(rest) > 0 {
		_, rest, ok = d.evalIntExpr(rest, hash.span)
		if !ok {
			return
		}
		if len(rest) > 0 {
			d.errorf(rest[0].span, "unexpected token after #line")
			return
		}
	}
	d.lineOffset = value - int64(hash.line) - 1
}

// defineDirective handles #define, object-like and function-like.
func (d *directiveState) defineDirective(line []ppToken, span ir.Span) {
	if len(line) == 0 || line[0].kind != ppIdent {
		d.errorf(span, "expected a macro name")
		return
	}
	m := &ppMacro{name: line[0].text}
	rest := line[1:]
	if len(rest) > 0 && rest[0].isPunct("(") && !rest[0].leadingSpace {
		m.functionLike = true
		i := 1
		if i < len(rest) && rest[i].isPunct(")") {
			i++
		} else {
			for {
				if i >= len(rest) || rest[i].kind != ppIdent {
					d.errorf(span, "expected a macro parameter name")
					return
				}
				name := rest[i].text
				for _, p := range m.params {
					if p == name {
						d.errorf(rest[i].span, "duplicate macro parameter: %s", name)
						return
					}
				}
				m.params = append(m.params, name)
				i++
				if i < len(rest) && rest[i].isPunct(",") {
					i++
					continue
				}
				if i < len(rest) && rest[i].isPunct(")") {
					i++
					break
				}
				d.errorf(span, "expected ',' or ')' in the macro parameter list")
				return
			}
		}
		m.body = append([]ppToken(nil), rest[i:]...)
	} else {
		m.body = append([]ppToken(nil), rest...)
	}

	if old, ok := d.macros[m.name]; ok {
		if !sameMacro(old, m) {
			d.errorf(line[0].span, "macro redefined: %s", m.name)
		}
		return
	}
	d.macros[m.name] = m
}

// sameMacro reports whether two macros are identical, the condition under which
// redefinition is legal.
func sameMacro(a, b *ppMacro) bool {
	if a.functionLike != b.functionLike || len(a.params) != len(b.params) || len(a.body) != len(b.body) {
		return false
	}
	for i := range a.params {
		if a.params[i] != b.params[i] {
			return false
		}
	}
	for i := range a.body {
		x, y := a.body[i], b.body[i]
		if x.kind != y.kind || x.text != y.text || x.intValue != y.intValue || x.floatValue != y.floatValue {
			return false
		}
	}
	return true
}

// undefDirective handles #undef.
func (d *directiveState) undefDirective(line []ppToken, span ir.Span) {
	name, rest := identArg(line)
	if name == "" || len(rest) > 0 {
		d.errorf(span, "expected a macro name")
		return
	}
	delete(d.macros, name)
}

// expandAll expands macros over toks, bounded by the token budget.
func (d *directiveState) expandAll(toks []ppToken) []ppToken {
	d.budget = ppTokenBudgetFactor*len(toks) + ppTokenBudgetBase
	if d.budget > maxPPTokens {
		d.budget = maxPPTokens
	}
	out := make([]ppToken, 0, len(toks))
	d.expandSeq(toks, nil, ppToken{}, 0, &out)
	return out
}

// emit appends t unless the budget is spent, which is reported once.
func (d *directiveState) emit(out *[]ppToken, t ppToken) {
	if len(*out) >= d.budget {
		if !d.budgetHit {
			d.budgetHit = true
			d.errorf(t.span, "preprocessed token limit exceeded")
		}
		return
	}
	*out = append(*out, t)
}

// withActive returns a copy of active with name added: a macro being expanded
// is not expanded again inside its own replacement.
func withActive(active map[string]bool, name string) map[string]bool {
	next := make(map[string]bool, len(active)+1)
	for k := range active {
		next[k] = true
	}
	next[name] = true
	return next
}

// expandSeq expands macros in toks and appends the result to out. active holds
// the macros whose replacement is being expanded; invocation is the token of
// the outermost invocation, which is what __LINE__ reports inside a macro body.
func (d *directiveState) expandSeq(toks []ppToken, active map[string]bool, invocation ppToken, depth int, out *[]ppToken) {
	if depth > maxExpansionDepth {
		d.errorf(invocation.span, "macro expansion nests deeper than %d", maxExpansionDepth)
		return
	}
	for i := 0; i < len(toks); i++ {
		if d.budgetHit {
			return // a spent budget ends the walk instead of expanding more
		}
		t := toks[i]
		if t.kind != ppIdent {
			d.emit(out, t)
			continue
		}
		switch t.text {
		case "__LINE__":
			line := t.line
			offset := t.lineOffset
			if active != nil {
				line = invocation.line
				offset = invocation.lineOffset
			}
			value := int64(line) + offset
			if value < 0 || value > math.MaxUint32 {
				d.errorf(t.span, "line number out of range: %d", value)
				value = 0
			}
			d.emit(out, ppToken{
				kind: ppInt, intValue: uint64(value), intWidth: 32,
				span: t.span, line: line,
			})
			continue
		case "__VERSION__":
			if d.hasVersion {
				d.emit(out, ppToken{kind: ppInt, intValue: uint64(d.version.Value), intWidth: 32, span: t.span, line: t.line})
				continue
			}
		case "GL_ES":
			if d.hasVersion && d.version.Profile == ppProfileES {
				d.emit(out, ppToken{kind: ppInt, intValue: 1, intWidth: 32, span: t.span, line: t.line})
				continue
			}
		}

		m, ok := d.macros[t.text]
		if !ok || active[t.text] {
			d.emit(out, t)
			continue
		}
		inv := invocation
		if active == nil {
			inv = t
		}
		if !m.functionLike {
			body := d.substitute(m, nil, t)
			d.expandSeq(body, withActive(active, t.text), inv, depth+1, out)
			continue
		}
		if i+1 >= len(toks) || !toks[i+1].isPunct("(") {
			d.emit(out, t)
			continue
		}
		args, next, errMsg := parseMacroArgs(toks, i+1)
		if errMsg != "" {
			d.errorf(t.span, "%s in a call to %s", errMsg, m.name)
			d.emit(out, t)
			continue
		}
		if len(m.params) == 0 {
			if len(args) == 1 && len(args[0]) == 0 {
				args = nil
			} else {
				d.errorf(t.span, "macro %s takes no arguments", m.name)
				d.emit(out, t)
				i = next - 1
				continue
			}
		}
		if len(args) != len(m.params) {
			d.errorf(t.span, "macro %s expects %d argument(s), found %d", m.name, len(m.params), len(args))
			d.emit(out, t)
			i = next - 1
			continue
		}
		expanded := make([][]ppToken, len(args))
		for k := range args {
			expanded[k] = d.expandArg(args[k], active)
		}
		body := d.substitute(m, expanded, t)
		d.expandSeq(body, withActive(active, t.text), inv, depth+1, out)
		i = next - 1
	}
}

// expandArg expands one macro argument before substitution.
func (d *directiveState) expandArg(toks []ppToken, active map[string]bool) []ppToken {
	var out []ppToken
	d.expandSeq(toks, active, ppToken{}, 0, &out)
	return out
}

// substitute returns the macro's replacement with parameters replaced by their
// expanded arguments. A body token without a source span (an option define)
// takes the invocation's span.
func (d *directiveState) substitute(m *ppMacro, args [][]ppToken, call ppToken) []ppToken {
	body := make([]ppToken, 0, len(m.body))
	for _, b := range m.body {
		if b.kind == ppIdent && args != nil {
			if k := indexOfString(m.params, b.text); k >= 0 {
				body = append(body, args[k]...)
				continue
			}
		}
		if !b.span.IsDefined() {
			b.span, b.line = call.span, call.line
		}
		body = append(body, b)
	}
	return body
}

// indexOfString returns the index of s in list, or -1.
func indexOfString(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// parseMacroArgs splits the arguments of a function-like macro call. open is
// the index of its '('. It returns the raw argument token slices and the index
// just past the closing ')'.
func parseMacroArgs(toks []ppToken, open int) (args [][]ppToken, next int, errMsg string) {
	depth := 0
	args = [][]ppToken{{}}
	for i := open + 1; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.isPunct("("):
			depth++
		case t.isPunct(")"):
			if depth == 0 {
				return args, i + 1, ""
			}
			depth--
		case t.isPunct(",") && depth == 0:
			args = append(args, nil)
			continue
		}
		args[len(args)-1] = append(args[len(args)-1], t)
	}
	return nil, len(toks), "unterminated macro arguments"
}
