package snapshot_test

import (
	"fmt"
	"strconv"
	"unicode"
)

// parseRONModule parses a Rust naga RON IR snapshot into ronModule.
func parseRONModule(data string) (*ronModule, error) {
	p := newRONParser(data)
	return p.parseModule()
}

func (p *ronParser) parseModule() (*ronModule, error) {
	m := &ronModule{}

	if err := p.expect('('); err != nil {
		return nil, fmt.Errorf("module open: %w", err)
	}

	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}

		key, err := p.parseIdent()
		if err != nil {
			return nil, fmt.Errorf("module field key: %w", err)
		}
		if err := p.expect(':'); err != nil {
			return nil, fmt.Errorf("module field colon after %q: %w", key, err)
		}

		switch key {
		case "types":
			m.Types, err = p.parseTypeArray()
		case "special_types":
			m.SpecialTypes, err = p.parseSpecialTypes()
		case "constants":
			m.Constants, err = p.parseConstantArray()
		case "overrides":
			m.Overrides, err = p.parseOverrideArray()
		case "global_variables":
			m.GlobalVariables, err = p.parseGlobalVariableArray()
		case "global_expressions":
			m.GlobalExpressions, err = p.parseExpressionArray()
		case "functions":
			m.Functions, err = p.parseFunctionArray()
		case "entry_points":
			m.EntryPoints, err = p.parseEntryPointArray()
		case "diagnostic_filters":
			m.DiagnosticFilters, err = p.parseDiagnosticFilterArray()
		case "diagnostic_filter_leaf":
			m.DiagFilterLeaf, err = p.parseSomeUint()
		case "doc_comments":
			err = p.skipValue()
		default:
			return nil, fmt.Errorf("unknown module field %q at pos %d", key, p.pos)
		}
		if err != nil {
			return nil, fmt.Errorf("module.%s: %w", key, err)
		}
		p.parseFieldSep()
	}

	return m, nil
}

// parseTypeArray parses [...] of types.
func (p *ronParser) parseTypeArray() ([]ronType, error) {
	return parseArray(p, p.parseType)
}

func (p *ronParser) parseType() (ronType, error) {
	var t ronType
	if err := p.expect('('); err != nil {
		return t, err
	}

	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return t, err
		}
		if err := p.expect(':'); err != nil {
			return t, err
		}

		switch key {
		case "name":
			t.Name, err = p.parseSomeString()
		case "inner":
			t.Inner, err = p.parseTypeInner()
		default:
			return t, fmt.Errorf("unknown type field %q", key)
		}
		if err != nil {
			return t, fmt.Errorf("type.%s: %w", key, err)
		}
		p.parseFieldSep()
	}

	return t, nil
}

func (p *ronParser) parseTypeInner() (ronTypeInner, error) {
	tag, err := p.parseIdent()
	if err != nil {
		return ronTypeInner{}, err
	}

	fields := make(map[string]interface{})
	ti := ronTypeInner{Tag: tag, Fields: fields}

	switch tag {
	case "Scalar":
		// Scalar((kind: Uint, width: 4))
		if err := p.expect('('); err != nil {
			return ti, err
		}
		s, err := p.parseScalar()
		if err != nil {
			return ti, err
		}
		fields["kind"] = s[0]
		fields["width"] = s[1]
		if err := p.expect(')'); err != nil {
			return ti, err
		}
		return ti, nil

	case "Vector":
		// Vector(size: Tri, scalar: (kind: Uint, width: 4))
		return p.parseStructFields(tag)

	case "Matrix":
		// Matrix(columns: Quad, rows: Tri, scalar: (...))
		return p.parseStructFields(tag)

	case "Array":
		// Array(base: 0, size: Dynamic|Constant(N), stride: 4)
		return p.parseStructFields(tag)

	case "Struct":
		// Struct(members: [...], span: N)
		return p.parseStructFields(tag)

	case "Pointer":
		// Pointer(base: N, space: Function)
		return p.parseStructFields(tag)

	case "Atomic":
		// Atomic((kind: Sint, width: 4))
		if err := p.expect('('); err != nil {
			return ti, err
		}
		s, err := p.parseScalar()
		if err != nil {
			return ti, err
		}
		fields["kind"] = s[0]
		fields["width"] = s[1]
		if err := p.expect(')'); err != nil {
			return ti, err
		}
		return ti, nil

	case "Image":
		return p.parseStructFields(tag)

	case "Sampler":
		return p.parseStructFields(tag)

	case "AccelerationStructure":
		return p.parseStructFields(tag)

	case "RayQuery":
		return p.parseStructFields(tag)

	case "BindingArray":
		return p.parseStructFields(tag)

	default:
		return ti, fmt.Errorf("unknown type inner %q", tag)
	}
}

// parseScalar parses (kind: X, width: N) returning [kind, width].
func (p *ronParser) parseScalar() ([2]string, error) {
	var result [2]string
	if err := p.expect('('); err != nil {
		return result, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return result, err
		}
		if err := p.expect(':'); err != nil {
			return result, err
		}
		val, err := p.parseIdent()
		if err != nil {
			// might be a number
			p2 := p.pos
			_ = p2
			numStr, err2 := p.parseNumber()
			if err2 != nil {
				return result, fmt.Errorf("scalar field %q value: %w", key, err)
			}
			val = numStr
		}
		switch key {
		case "kind":
			result[0] = val
		case "width":
			result[1] = val
		}
		p.parseFieldSep()
	}
	return result, nil
}

// parseStructFields parses (key: value, ...) into a tagged union with a fields map.
func (p *ronParser) parseStructFields(tag string) (ronTypeInner, error) {
	fields := make(map[string]interface{})
	ti := ronTypeInner{Tag: tag, Fields: fields}

	if err := p.expect('('); err != nil {
		return ti, err
	}

	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return ti, err
		}
		if err := p.expect(':'); err != nil {
			return ti, err
		}

		val, err := p.parseGenericValue()
		if err != nil {
			return ti, fmt.Errorf("field %q: %w", key, err)
		}
		fields[key] = val
		p.parseFieldSep()
	}

	return ti, nil
}

// parseGenericValue parses any RON value: number, string, ident, array, struct, Some/None, etc.
func (p *ronParser) parseGenericValue() (interface{}, error) {
	p.skipWhitespaceAndComments()
	if p.atEnd() {
		return nil, fmt.Errorf("unexpected EOF")
	}

	ch := p.peek()

	// String
	if ch == '"' {
		s, err := p.parseString()
		return s, err
	}

	// Array
	if ch == '[' {
		return p.parseGenericArray()
	}

	// Tuple/struct with parens
	if ch == '(' {
		return p.parseGenericTuple()
	}

	// Map
	if ch == '{' {
		return p.parseGenericMap()
	}

	// Number (starts with digit or - or +)
	if unicode.IsDigit(ch) || ch == '-' || ch == '+' {
		// Check if this is a negative sign followed by digit
		if (ch == '-' || ch == '+') && p.pos+1 < len(p.input) && unicode.IsDigit(p.input[p.pos+1]) {
			s, err := p.parseNumber()
			return s, err
		}
		if unicode.IsDigit(ch) {
			s, err := p.parseNumber()
			return s, err
		}
	}

	// Identifier or enum variant
	ident, err := p.parseIdent()
	if err != nil {
		return nil, err
	}

	// Check for function-call style: Ident(...)
	p.skipWhitespaceAndComments()
	if !p.atEnd() && p.peek() == '(' {
		// Enum variant with payload
		inner, err := p.parseGenericTuple()
		if err != nil {
			return nil, fmt.Errorf("enum %s: %w", ident, err)
		}
		return map[string]interface{}{
			"_tag":   ident,
			"_inner": inner,
		}, nil
	}

	// Plain identifier (enum variant without payload, or boolean, etc.)
	return ident, nil
}

func (p *ronParser) parseGenericArray() (interface{}, error) {
	if err := p.expect('['); err != nil {
		return nil, err
	}
	var items []interface{}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(']') {
			break
		}
		val, err := p.parseGenericValue()
		if err != nil {
			return nil, err
		}
		items = append(items, val)
		p.parseFieldSep()
	}
	return items, nil
}

func (p *ronParser) parseGenericTuple() (interface{}, error) {
	if err := p.expect('('); err != nil {
		return nil, err
	}

	// Check if it's a named-field struct: (key: value, ...)
	// or a positional tuple: (1, 2, 3)
	// Save position to backtrack
	saved := p.pos
	p.skipWhitespaceAndComments()

	if p.tryConsume(')') {
		// Empty tuple
		return map[string]interface{}{}, nil
	}

	// Try to detect if first item is a key: value pair
	p.skipWhitespaceAndComments()
	if !p.atEnd() && p.peek() == '"' {
		// Quoted string as first element — this is a positional tuple
		p.pos = saved
		return p.parseTuplePositional()
	}

	// Try reading an ident
	identStart := p.pos
	_, err := p.parseIdent()
	if err != nil {
		// Not an ident, parse as positional
		p.pos = saved
		return p.parseTuplePositional()
	}

	p.skipWhitespaceAndComments()
	if !p.atEnd() && p.peek() == ':' {
		// Named fields
		p.pos = identStart // back up to re-read the key
		return p.parseTupleNamed()
	}

	// Could be a positional tuple starting with an ident like "Compute"
	// or an enum variant like SomeVariant(...)
	p.pos = saved
	return p.parseTuplePositional()
}

func (p *ronParser) parseTupleNamed() (interface{}, error) {
	// Already inside parens (consumed '(')
	fields := make(map[string]interface{})
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		val, err := p.parseGenericValue()
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		fields[key] = val
		p.parseFieldSep()
	}
	return fields, nil
}

func (p *ronParser) parseTuplePositional() (interface{}, error) {
	// Already consumed '('
	var items []interface{}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		val, err := p.parseGenericValue()
		if err != nil {
			return nil, err
		}
		items = append(items, val)
		p.parseFieldSep()
	}
	return items, nil
}

func (p *ronParser) parseGenericMap() (interface{}, error) {
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume('}') {
			break
		}
		// Key can be a complex value (e.g., enum variant)
		key, err := p.parseGenericValue()
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		if err := p.expect(':'); err != nil {
			return nil, fmt.Errorf("map colon: %w", err)
		}
		val, err := p.parseGenericValue()
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		result[fmt.Sprintf("%v", key)] = val
		p.parseFieldSep()
	}
	return result, nil
}

// parseConstantArray parses the constants array.
func (p *ronParser) parseConstantArray() ([]ronConstant, error) {
	return parseArray(p, p.parseConstant)
}

func (p *ronParser) parseConstant() (ronConstant, error) {
	var c ronConstant
	if err := p.expect('('); err != nil {
		return c, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return c, err
		}
		if err := p.expect(':'); err != nil {
			return c, err
		}
		switch key {
		case "name":
			c.Name, err = p.parseSomeString()
		case "ty":
			c.Ty, err = p.parseUint()
		case "init":
			c.Init, err = p.parseUint()
		default:
			return c, fmt.Errorf("unknown constant field %q", key)
		}
		if err != nil {
			return c, fmt.Errorf("constant.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return c, nil
}

// parseOverrideArray parses the overrides array.
func (p *ronParser) parseOverrideArray() ([]ronOverride, error) {
	return parseArray(p, p.parseOverride)
}

func (p *ronParser) parseOverride() (ronOverride, error) {
	var o ronOverride
	if err := p.expect('('); err != nil {
		return o, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return o, err
		}
		if err := p.expect(':'); err != nil {
			return o, err
		}
		switch key {
		case "name":
			o.Name, err = p.parseSomeString()
		case "id":
			o.ID, err = p.parseSomeUint()
		case "ty":
			o.Ty, err = p.parseUint()
		case "init":
			o.Init, err = p.parseSomeUint()
		default:
			return o, fmt.Errorf("unknown override field %q", key)
		}
		if err != nil {
			return o, fmt.Errorf("override.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return o, nil
}

// parseGlobalVariableArray parses global_variables.
func (p *ronParser) parseGlobalVariableArray() ([]ronGlobalVariable, error) {
	return parseArray(p, p.parseGlobalVariable)
}

func (p *ronParser) parseGlobalVariable() (ronGlobalVariable, error) {
	var g ronGlobalVariable
	if err := p.expect('('); err != nil {
		return g, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return g, err
		}
		if err := p.expect(':'); err != nil {
			return g, err
		}
		switch key {
		case "name":
			g.Name, err = p.parseSomeString()
		case "space":
			g.Space, g.Access, err = p.parseAddressSpace()
		case "binding":
			g.Binding, err = p.parseOptionalResourceBinding()
		case "ty":
			g.Ty, err = p.parseUint()
		case "init":
			g.Init, err = p.parseSomeUint()
		default:
			return g, fmt.Errorf("unknown global_variable field %q", key)
		}
		if err != nil {
			return g, fmt.Errorf("global_variable.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return g, nil
}

func (p *ronParser) parseAddressSpace() (string, string, error) {
	ident, err := p.parseIdent()
	if err != nil {
		return "", "", err
	}
	// Storage might have access: Storage(access: ("LOAD | STORE"))
	if ident == "Storage" {
		p.skipWhitespaceAndComments()
		if !p.atEnd() && p.peek() == '(' {
			p.advance() // (
			access := ""
			for {
				p.skipWhitespaceAndComments()
				if p.tryConsume(')') {
					break
				}
				key, err := p.parseIdent()
				if err != nil {
					return "", "", err
				}
				if err := p.expect(':'); err != nil {
					return "", "", err
				}
				if key == "access" {
					// ("LOAD | STORE")
					if err := p.expect('('); err != nil {
						return "", "", err
					}
					access, err = p.parseString()
					if err != nil {
						return "", "", err
					}
					if err := p.expect(')'); err != nil {
						return "", "", err
					}
				}
				p.parseFieldSep()
			}
			return "Storage", access, nil
		}
	}
	return ident, "", nil
}

func (p *ronParser) parseOptionalResourceBinding() (*ronResourceBinding, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	rb := &ronResourceBinding{}
	// ((group: N, binding: M))
	if err := p.expect('('); err != nil {
		return nil, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		switch key {
		case "group":
			rb.Group, err = p.parseUint()
		case "binding":
			rb.Binding, err = p.parseUint()
		}
		if err != nil {
			return nil, err
		}
		p.parseFieldSep()
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return rb, nil
}

// parseExpressionArray parses [...] of expressions.
func (p *ronParser) parseExpressionArray() ([]ronExpression, error) {
	return parseArray(p, p.parseExpression)
}

func (p *ronParser) parseExpression() (ronExpression, error) {
	// Expressions can be: Variant(fields) or Variant
	tag, err := p.parseIdent()
	if err != nil {
		return ronExpression{}, fmt.Errorf("expression tag: %w", err)
	}

	p.skipWhitespaceAndComments()
	fields := make(map[string]interface{})

	if !p.atEnd() && p.peek() == '(' {
		// Parse the inner value generically
		inner, err := p.parseGenericTuple()
		if err != nil {
			return ronExpression{}, fmt.Errorf("expression %s inner: %w", tag, err)
		}
		// inner can be a map (named fields) or a slice (positional)
		switch v := inner.(type) {
		case map[string]interface{}:
			fields = v
		case []interface{}:
			// Positional args, store as "_args"
			fields["_args"] = v
		}
	}

	return ronExpression{Tag: tag, Fields: fields}, nil
}

// parseStatementArray parses [...] of statements.
func (p *ronParser) parseStatementArray() ([]ronStatement, error) {
	return parseArray(p, p.parseStatement)
}

func (p *ronParser) parseStatement() (ronStatement, error) {
	tag, err := p.parseIdent()
	if err != nil {
		return ronStatement{}, fmt.Errorf("statement tag: %w", err)
	}

	p.skipWhitespaceAndComments()
	fields := make(map[string]interface{})

	if !p.atEnd() && p.peek() == '(' {
		inner, err := p.parseGenericTuple()
		if err != nil {
			return ronStatement{}, fmt.Errorf("statement %s inner: %w", tag, err)
		}
		switch v := inner.(type) {
		case map[string]interface{}:
			fields = v
		case []interface{}:
			fields["_args"] = v
		}
	}

	return ronStatement{Tag: tag, Fields: fields}, nil
}

// parseFunctionArray parses [...] of functions.
func (p *ronParser) parseFunctionArray() ([]ronFunction, error) {
	return parseArray(p, p.parseFunctionDef)
}

func (p *ronParser) parseFunctionDef() (ronFunction, error) {
	var f ronFunction
	if err := p.expect('('); err != nil {
		return f, err
	}
	return p.parseFunctionFields()
}

func (p *ronParser) parseFunctionFields() (ronFunction, error) {
	var f ronFunction
	f.NamedExpressions = make(map[uint32]string)

	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return f, err
		}
		if err := p.expect(':'); err != nil {
			return f, err
		}

		switch key {
		case "name":
			namePtr, err := p.parseSomeString()
			if err != nil {
				return f, fmt.Errorf("function.name: %w", err)
			}
			if namePtr != nil {
				f.Name = *namePtr
			}
		case "arguments":
			f.Arguments, err = p.parseFunctionArgumentArray()
		case "result":
			f.Result, err = p.parseOptionalFunctionResult()
		case "local_variables":
			f.LocalVariables, err = p.parseLocalVariableArray()
		case "expressions":
			f.Expressions, err = p.parseExpressionArray()
		case "named_expressions":
			f.NamedExpressions, err = p.parseNamedExpressions()
		case "body":
			f.Body, err = p.parseStatementArray()
		case "diagnostic_filter_leaf":
			f.DiagFilterLeaf, err = p.parseSomeUint()
		default:
			return f, fmt.Errorf("unknown function field %q", key)
		}
		if err != nil {
			return f, fmt.Errorf("function.%s: %w", key, err)
		}
		p.parseFieldSep()
	}

	return f, nil
}

func (p *ronParser) parseFunctionArgumentArray() ([]ronFunctionArgument, error) {
	return parseArray(p, p.parseFunctionArgument)
}

func (p *ronParser) parseFunctionArgument() (ronFunctionArgument, error) {
	var a ronFunctionArgument
	if err := p.expect('('); err != nil {
		return a, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return a, err
		}
		if err := p.expect(':'); err != nil {
			return a, err
		}
		switch key {
		case "name":
			a.Name, err = p.parseSomeString()
		case "ty":
			a.Ty, err = p.parseUint()
		case "binding":
			a.Binding, err = p.parseOptionalBinding()
		default:
			return a, fmt.Errorf("unknown argument field %q", key)
		}
		if err != nil {
			return a, fmt.Errorf("argument.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return a, nil
}

func (p *ronParser) parseOptionalBinding() (*ronBinding, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	return p.parseBinding()
}

func (p *ronParser) parseBinding() (*ronBinding, error) {
	tag, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	b := &ronBinding{Tag: tag, Fields: make(map[string]interface{})}

	p.skipWhitespaceAndComments()
	if !p.atEnd() && p.peek() == '(' {
		inner, err := p.parseGenericTuple()
		if err != nil {
			return nil, fmt.Errorf("binding %s inner: %w", tag, err)
		}
		switch v := inner.(type) {
		case map[string]interface{}:
			b.Fields = v
		case []interface{}:
			b.Fields["_args"] = v
		}
	}

	if err := p.expect(')'); err != nil {
		return nil, fmt.Errorf("binding close: %w", err)
	}
	return b, nil
}

func (p *ronParser) parseOptionalFunctionResult() (*ronFunctionResult, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	r := &ronFunctionResult{}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		switch key {
		case "ty":
			r.Ty, err = p.parseUint()
		case "binding":
			r.Binding, err = p.parseOptionalBinding()
		default:
			return nil, fmt.Errorf("unknown result field %q", key)
		}
		if err != nil {
			return nil, err
		}
		p.parseFieldSep()
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *ronParser) parseLocalVariableArray() ([]ronLocalVariable, error) {
	return parseArray(p, p.parseLocalVariable)
}

func (p *ronParser) parseLocalVariable() (ronLocalVariable, error) {
	var lv ronLocalVariable
	if err := p.expect('('); err != nil {
		return lv, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return lv, err
		}
		if err := p.expect(':'); err != nil {
			return lv, err
		}
		switch key {
		case "name":
			lv.Name, err = p.parseSomeString()
		case "ty":
			lv.Ty, err = p.parseUint()
		case "init":
			lv.Init, err = p.parseSomeUint()
		default:
			return lv, fmt.Errorf("unknown local_variable field %q", key)
		}
		if err != nil {
			return lv, fmt.Errorf("local_variable.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return lv, nil
}

func (p *ronParser) parseNamedExpressions() (map[uint32]string, error) {
	result := make(map[uint32]string)
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume('}') {
			break
		}
		keyNum, err := p.parseUint()
		if err != nil {
			return nil, err
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		val, err := p.parseString()
		if err != nil {
			return nil, err
		}
		result[keyNum] = val
		p.parseFieldSep()
	}
	return result, nil
}

func (p *ronParser) parseSpecialTypes() (ronSpecialTypes, error) {
	var st ronSpecialTypes
	st.PredeclaredTypes = make(map[string]uint32)

	if err := p.expect('('); err != nil {
		return st, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return st, err
		}
		if err := p.expect(':'); err != nil {
			return st, err
		}
		switch key {
		case "ray_desc":
			st.RayDesc, err = p.parseSomeUint()
		case "ray_intersection":
			st.RayIntersection, err = p.parseSomeUint()
		case "ray_vertex_return":
			st.RayVertexReturn, err = p.parseSomeUint()
		case "external_texture_params":
			st.ExternalTextureParams, err = p.parseSomeUint()
		case "external_texture_transfer_function":
			st.ExternalTextureTransferFn, err = p.parseSomeUint()
		case "predeclared_types":
			st.PredeclaredTypes, err = p.parsePredeclaredTypes()
		default:
			return st, fmt.Errorf("unknown special_types field %q", key)
		}
		if err != nil {
			return st, fmt.Errorf("special_types.%s: %w", key, err)
		}
		p.parseFieldSep()
	}
	return st, nil
}

func (p *ronParser) parsePredeclaredTypes() (map[string]uint32, error) {
	result := make(map[string]uint32)
	if err := p.expect('{'); err != nil {
		return nil, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume('}') {
			break
		}
		// Key is a complex enum like AtomicCompareExchangeWeakResult((kind: Uint, width: 4))
		// We'll read it as a generic value and stringify it
		keyVal, err := p.parseGenericValue()
		if err != nil {
			return nil, fmt.Errorf("predeclared_types key: %w", err)
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		val, err := p.parseUint()
		if err != nil {
			return nil, err
		}
		result[fmt.Sprintf("%v", keyVal)] = val
		p.parseFieldSep()
	}
	return result, nil
}

// parseEntryPointArray parses [...] of entry points.
func (p *ronParser) parseEntryPointArray() ([]ronEntryPoint, error) {
	return parseArray(p, p.parseEntryPoint)
}

func (p *ronParser) parseEntryPoint() (ronEntryPoint, error) {
	var ep ronEntryPoint
	if err := p.expect('('); err != nil {
		return ep, err
	}

	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return ep, err
		}
		if err := p.expect(':'); err != nil {
			return ep, err
		}

		switch key {
		case "name":
			ep.Name, err = p.parseString()
		case "stage":
			ep.Stage, err = p.parseIdent()
		case "early_depth_test":
			// None or Some(...)
			_, err = p.parseGenericValue()
		case "workgroup_size":
			ep.WorkgroupSize, err = p.parseWorkgroupSize()
		case "workgroup_size_overrides":
			_, err = p.parseGenericValue() // None or Some(...)
		case "function":
			if err := p.expect('('); err != nil {
				return ep, err
			}
			ep.Function, err = p.parseFunctionFields()
		case "mesh_info":
			ep.MeshInfo, err = p.parseOptionalMeshInfo()
		case "task_payload":
			ep.TaskPayload, err = p.parseSomeUint()
		default:
			return ep, fmt.Errorf("unknown entry_point field %q", key)
		}
		if err != nil {
			return ep, fmt.Errorf("entry_point.%s: %w", key, err)
		}
		p.parseFieldSep()
	}

	return ep, nil
}

func (p *ronParser) parseWorkgroupSize() ([3]uint32, error) {
	var ws [3]uint32
	if err := p.expect('('); err != nil {
		return ws, err
	}
	for i := 0; i < 3; i++ {
		v, err := p.parseUint()
		if err != nil {
			return ws, err
		}
		ws[i] = v
		p.parseFieldSep()
	}
	if err := p.expect(')'); err != nil {
		return ws, err
	}
	return ws, nil
}

func (p *ronParser) parseOptionalMeshInfo() (*ronMeshInfo, error) {
	isSome, err := p.parseSomeNone()
	if err != nil {
		return nil, err
	}
	if !isSome {
		return nil, nil
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	mi := &ronMeshInfo{}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		switch key {
		case "topology":
			mi.Topology, err = p.parseIdent()
		case "max_vertices":
			mi.MaxVertices, err = p.parseUint()
		case "max_vertices_override":
			mi.MaxVerticesOverride, err = p.parseSomeUint()
		case "max_primitives":
			mi.MaxPrimitives, err = p.parseUint()
		case "max_primitives_override":
			mi.MaxPrimitivesOverride, err = p.parseSomeUint()
		case "vertex_output_type":
			mi.VertexOutputType, err = p.parseUint()
		case "primitive_output_type":
			mi.PrimitiveOutputType, err = p.parseUint()
		case "output_variable":
			mi.OutputVariable, err = p.parseUint()
		default:
			return nil, fmt.Errorf("unknown mesh_info field %q", key)
		}
		if err != nil {
			return nil, err
		}
		p.parseFieldSep()
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return mi, nil
}

func (p *ronParser) parseDiagnosticFilterArray() ([]ronDiagnosticFilter, error) {
	return parseArray(p, p.parseDiagnosticFilter)
}

func (p *ronParser) parseDiagnosticFilter() (ronDiagnosticFilter, error) {
	var df ronDiagnosticFilter
	if err := p.expect('('); err != nil {
		return df, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return df, err
		}
		if err := p.expect(':'); err != nil {
			return df, err
		}
		switch key {
		case "inner":
			df.Inner, err = p.parseDiagnosticFilterInner()
		case "parent":
			df.Parent, err = p.parseSomeUint()
		default:
			return df, fmt.Errorf("unknown diagnostic_filter field %q", key)
		}
		if err != nil {
			return df, err
		}
		p.parseFieldSep()
	}
	return df, nil
}

func (p *ronParser) parseDiagnosticFilterInner() (ronDiagnosticFilterInner, error) {
	var dfi ronDiagnosticFilterInner
	if err := p.expect('('); err != nil {
		return dfi, err
	}
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(')') {
			break
		}
		key, err := p.parseIdent()
		if err != nil {
			return dfi, err
		}
		if err := p.expect(':'); err != nil {
			return dfi, err
		}
		switch key {
		case "new_severity":
			dfi.NewSeverity, err = p.parseIdent()
		case "triggering_rule":
			// Standard(DerivativeUniformity) — parse as generic value and stringify
			val, err2 := p.parseGenericValue()
			if err2 != nil {
				return dfi, err2
			}
			dfi.TriggeringRule = fmt.Sprintf("%v", val)
		default:
			return dfi, fmt.Errorf("unknown diagnostic_filter_inner field %q", key)
		}
		if err != nil {
			return dfi, err
		}
		p.parseFieldSep()
	}
	return dfi, nil
}

// parseArray is a generic array parser.
func parseArray[T any](p *ronParser, itemParser func() (T, error)) ([]T, error) {
	if err := p.expect('['); err != nil {
		return nil, err
	}
	var items []T
	for {
		p.skipWhitespaceAndComments()
		if p.tryConsume(']') {
			break
		}
		item, err := itemParser()
		if err != nil {
			return nil, fmt.Errorf("item[%d]: %w", len(items), err)
		}
		items = append(items, item)
		p.parseFieldSep()
	}
	return items, nil
}

// --- Helpers to extract typed values from generic parsed values ---

func ronGetString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func ronGetUint32(v interface{}) uint32 {
	switch val := v.(type) {
	case string:
		n, _ := strconv.ParseUint(val, 10, 32)
		return uint32(n)
	case uint32:
		return val
	case int:
		return uint32(val)
	case float64:
		return uint32(val)
	}
	return 0
}

func ronGetInt32(v interface{}) int32 {
	switch val := v.(type) {
	case string:
		n, _ := strconv.ParseInt(val, 10, 32)
		return int32(n)
	case int32:
		return val
	case int:
		return int32(val)
	case float64:
		return int32(val)
	}
	return 0
}

func ronGetFloat32(v interface{}) float32 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 32)
		return float32(f)
	case float32:
		return val
	case float64:
		return float32(val)
	}
	return 0
}

func ronGetFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	case float32:
		return float64(val)
	}
	return 0
}

func ronGetBool(v interface{}) bool {
	switch val := v.(type) {
	case string:
		return val == "true"
	case bool:
		return val
	}
	return false
}

func ronGetArray(v interface{}) []interface{} {
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

func ronGetMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}
