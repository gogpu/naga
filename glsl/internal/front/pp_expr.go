package front

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// ifParser evaluates a preprocessor integer expression over int and
// punctuation tokens.
type ifParser struct {
	tokens []ppToken
	pos    int
	state  *directiveState
	span   ir.Span // the whole expression's span, for an unexpected end
}

// evalCondition evaluates a #if/#elif expression: defined() is resolved first,
// macros expand, remaining identifiers become 0, and the integer expression is
// parsed with C's precedence. A malformed expression is an error and the
// condition is false.
func (d *directiveState) evalCondition(toks []ppToken, span ir.Span) (int64, bool) {
	value, rest, ok := d.evalIntExpr(toks, span)
	if !ok {
		return 0, false
	}
	if len(rest) > 0 {
		d.errorf(rest[0].span, "unexpected token in a preprocessor expression")
		return 0, false
	}
	return value, true
}

// evalIntExpr evaluates one integer constant expression from toks and returns
// the value and the tokens after the expression. #line uses the remainder for
// its optional source-file number.
func (d *directiveState) evalIntExpr(toks []ppToken, span ir.Span) (int64, []ppToken, bool) {
	resolved, ok := d.resolveDefined(toks)
	if !ok {
		return 0, nil, false
	}
	expanded := d.expandAll(resolved)

	filtered := make([]ppToken, 0, len(expanded))
	for _, t := range expanded {
		switch t.kind {
		case ppIdent:
			// An identifier that survived expansion is 0 (pp-rs parser_if).
			filtered = append(filtered, ppToken{
				kind: ppInt, intValue: 0, intSigned: true, intWidth: 32,
				span: t.span, line: t.line,
			})
		case ppFloat:
			d.errorf(t.span, "floating-point value in a preprocessor expression")
			return 0, nil, false
		case ppInt, ppPunct:
			filtered = append(filtered, t)
		default:
			d.errorf(t.span, "unexpected token in a preprocessor expression")
			return 0, nil, false
		}
	}

	p := &ifParser{tokens: filtered, state: d, span: span}
	if len(filtered) == 0 {
		d.errorf(span, "expected an expression")
		return 0, nil, false
	}
	value, ok := p.parseLogicalOr()
	if !ok {
		return 0, nil, false
	}
	return value, filtered[p.pos:], true
}

// unexpectedEOF reports an expression that ends too early.
func (p *ifParser) unexpectedEOF() (int64, bool) {
	p.state.errorf(p.span, "unexpected end of a preprocessor expression")
	return 0, false
}

// resolveDefined replaces `defined name` and `defined(name)` with 1 or 0. It
// runs before macro expansion, which is what keeps an operand from being
// expanded.
func (d *directiveState) resolveDefined(toks []ppToken) ([]ppToken, bool) {
	out := make([]ppToken, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind != ppIdent || t.text != "defined" {
			out = append(out, t)
			continue
		}
		var name string
		switch {
		case i+1 < len(toks) && toks[i+1].kind == ppIdent:
			name = toks[i+1].text
			i++
		case i+3 < len(toks) && toks[i+1].isPunct("(") && toks[i+2].kind == ppIdent && toks[i+3].isPunct(")"):
			name = toks[i+2].text
			i += 3
		default:
			d.errorf(t.span, "expected a macro name after defined")
			return nil, false
		}
		value := uint64(0)
		if d.isDefined(name) {
			value = 1
		}
		out = append(out, ppToken{
			kind: ppInt, intValue: value, intWidth: 32, span: t.span, line: t.line,
		})
	}
	return out, true
}

func (p *ifParser) peek() (ppToken, bool) {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos], true
	}
	return ppToken{}, false
}

func (p *ifParser) take() (ppToken, bool) {
	if p.pos >= len(p.tokens) {
		return ppToken{}, false
	}
	t := p.tokens[p.pos]
	p.pos++
	return t, true
}

func (p *ifParser) fail(t ppToken, message string) (int64, bool) {
	p.state.errorf(t.span, "%s", message)
	return 0, false
}

func (p *ifParser) parseLogicalOr() (int64, bool) {
	left, ok := p.parseLogicalAnd()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !t.isPunct("||") {
			return left, true
		}
		p.pos++
		right, ok := p.parseLogicalAnd()
		if !ok {
			return 0, false
		}
		left = boolInt(left != 0 || right != 0)
	}
}

func (p *ifParser) parseLogicalAnd() (int64, bool) {
	left, ok := p.parseBitOr()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !t.isPunct("&&") {
			return left, true
		}
		p.pos++
		right, ok := p.parseBitOr()
		if !ok {
			return 0, false
		}
		left = boolInt(left != 0 && right != 0)
	}
}

func (p *ifParser) parseBitOr() (int64, bool) {
	left, ok := p.parseBitXor()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !t.isPunct("|") {
			return left, true
		}
		p.pos++
		right, ok := p.parseBitXor()
		if !ok {
			return 0, false
		}
		left |= right
	}
}

func (p *ifParser) parseBitXor() (int64, bool) {
	left, ok := p.parseBitAnd()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !t.isPunct("^") {
			return left, true
		}
		p.pos++
		right, ok := p.parseBitAnd()
		if !ok {
			return 0, false
		}
		left ^= right
	}
}

func (p *ifParser) parseBitAnd() (int64, bool) {
	left, ok := p.parseEquality()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !t.isPunct("&") {
			return left, true
		}
		p.pos++
		right, ok := p.parseEquality()
		if !ok {
			return 0, false
		}
		left &= right
	}
}

func (p *ifParser) parseEquality() (int64, bool) {
	left, ok := p.parseRelational()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !(t.isPunct("==") || t.isPunct("!=")) {
			return left, true
		}
		p.pos++
		right, ok := p.parseRelational()
		if !ok {
			return 0, false
		}
		if t.isPunct("==") {
			left = boolInt(left == right)
		} else {
			left = boolInt(left != right)
		}
	}
}

func (p *ifParser) parseRelational() (int64, bool) {
	left, ok := p.parseShift()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !(t.isPunct("<") || t.isPunct(">") || t.isPunct("<=") || t.isPunct(">=")) {
			return left, true
		}
		p.pos++
		right, ok := p.parseShift()
		if !ok {
			return 0, false
		}
		switch t.text {
		case "<":
			left = boolInt(left < right)
		case ">":
			left = boolInt(left > right)
		case "<=":
			left = boolInt(left <= right)
		default:
			left = boolInt(left >= right)
		}
	}
}

func (p *ifParser) parseShift() (int64, bool) {
	left, ok := p.parseAdditive()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !(t.isPunct("<<") || t.isPunct(">>")) {
			return left, true
		}
		p.pos++
		right, ok := p.parseAdditive()
		if !ok {
			return 0, false
		}
		if right < 0 || right > 63 {
			return p.fail(t, "shift count out of range in a preprocessor expression")
		}
		if t.isPunct("<<") {
			left <<= uint(right)
		} else {
			left >>= uint(right)
		}
	}
}

func (p *ifParser) parseAdditive() (int64, bool) {
	left, ok := p.parseMultiplicative()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !(t.isPunct("+") || t.isPunct("-")) {
			return left, true
		}
		p.pos++
		right, ok := p.parseMultiplicative()
		if !ok {
			return 0, false
		}
		var overflow bool
		if t.isPunct("+") {
			left, overflow = addChecked(left, right)
		} else {
			left, overflow = addChecked(left, -right)
			if right == math.MinInt64 {
				overflow = true
			}
		}
		if overflow {
			return p.fail(t, "integer overflow in a preprocessor expression")
		}
	}
}

func (p *ifParser) parseMultiplicative() (int64, bool) {
	left, ok := p.parseUnary()
	if !ok {
		return 0, false
	}
	for {
		t, ok := p.peek()
		if !ok || !(t.isPunct("*") || t.isPunct("/") || t.isPunct("%")) {
			return left, true
		}
		p.pos++
		right, ok := p.parseUnary()
		if !ok {
			return 0, false
		}
		switch t.text {
		case "*":
			value, overflow := mulChecked(left, right)
			if overflow {
				return p.fail(t, "integer overflow in a preprocessor expression")
			}
			left = value
		case "/":
			if right == 0 {
				return p.fail(t, "division by zero in a preprocessor expression")
			}
			if left == math.MinInt64 && right == -1 {
				return p.fail(t, "integer overflow in a preprocessor expression")
			}
			left /= right
		default:
			if right == 0 {
				return p.fail(t, "division by zero in a preprocessor expression")
			}
			if left == math.MinInt64 && right == -1 {
				return p.fail(t, "integer overflow in a preprocessor expression")
			}
			left %= right
		}
	}
}

func (p *ifParser) parseUnary() (int64, bool) {
	t, ok := p.peek()
	if !ok {
		return p.unexpectedEOF()
	}
	if t.kind == ppPunct {
		switch t.text {
		case "+", "-", "!", "~":
			p.pos++
			value, ok := p.parseUnary()
			if !ok {
				return 0, false
			}
			switch t.text {
			case "+":
				return value, true
			case "-":
				if value == math.MinInt64 {
					return p.fail(t, "integer overflow in a preprocessor expression")
				}
				return -value, true
			case "!":
				return boolInt(value == 0), true
			default:
				return ^value, true
			}
		}
	}
	return p.parseAtom()
}

func (p *ifParser) parseAtom() (int64, bool) {
	t, ok := p.take()
	if !ok {
		return p.unexpectedEOF()
	}
	switch t.kind {
	case ppInt:
		return int64(t.intValue), true
	case ppPunct:
		if !t.isPunct("(") {
			return p.fail(t, "unexpected token in a preprocessor expression")
		}
		value, ok := p.parseLogicalOr()
		if !ok {
			return 0, false
		}
		end, ok := p.take()
		if !ok || !end.isPunct(")") {
			return p.fail(t, "expected ')' in a preprocessor expression")
		}
		return value, true
	default:
		return p.fail(t, "unexpected token in a preprocessor expression")
	}
}

// boolInt is the integer value of a preprocessor condition.
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// addChecked adds with overflow detection.
func addChecked(a, b int64) (int64, bool) {
	sum := a + b
	if (b > 0 && sum < a) || (b < 0 && sum > a) {
		return 0, true
	}
	return sum, false
}

// mulChecked multiplies with overflow detection.
func mulChecked(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	if a == math.MinInt64 && b == -1 || b == math.MinInt64 && a == -1 {
		return 0, true
	}
	product := a * b
	return product, product/b != a
}
