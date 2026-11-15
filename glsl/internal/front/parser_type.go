package front

import "github.com/gogpu/naga/ir"

// This file parses types, array specifiers and the qualifier lists, Rust naga's
// parser/types.rs.

// punct returns a token predicate that matches the punctuation token written s.
func punct(s string) func(glslToken) bool {
	return func(t glslToken) bool { return t.isPunct(s) }
}

// parseExprInArena parses one expression into an arena of its own, which is
// what a constant expression outside a statement needs: array sizes, layout
// values, case labels and initializers each keep their expressions in their own
// node.
func (p *parser) parseExprInArena(parse func(*exprs) (hirHandle, *frontError)) (*exprs, *frontError) {
	e := &exprs{}
	root, err := parse(e)
	if err != nil {
		return nil, err
	}
	e.Root = root
	return e, nil
}

// parseArraySpecifier parses the array dimensions written after a type or a
// declarator's name, Rust naga's parse_array_specifier
// (parser/types.rs:19-73). An empty `[]` is an unsized dimension.
func (p *parser) parseArraySpecifier() ([]arraySpec, *frontError) {
	var specs []arraySpec
	for p.atPunct("[") {
		open, err := p.expectPunct("[")
		if err != nil {
			return nil, err
		}
		if len(specs) >= maxTypeDepth {
			// Every dimension is one level of the type's nesting.
			return nil, errMessage(open.span, errTypeDepth)
		}
		span := open.span
		var size *exprs
		if closeTok := p.bumpIf(punct("]")); closeTok != nil {
			span = subsumeSpan(span, closeTok.span)
		} else {
			size, err = p.parseExprInArena(p.parseConstantExpr)
			if err != nil {
				return nil, err
			}
			closeTok, err := p.expectPunct("]")
			if err != nil {
				return nil, err
			}
			span = subsumeSpan(span, closeTok.span)
		}
		specs = append(specs, arraySpec{Size: size, Span: span})
	}
	return specs, nil
}

// parseType parses a type as Rust naga's parse_type does
// (parser/types.rs:75-134). A nil spec with a span is the `void` type, which
// only a function's return type may be.
func (p *parser) parseType() (*typeSpec, ir.Span, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return nil, p.last, err
	}
	spec := &typeSpec{}
	switch {
	case tok.kind == tokVoid:
		return nil, tok.span, nil
	case tok.kind == tokTypeName:
		spec.Builtin = tok.typeName
	case tok.kind == tokStruct:
		def, err := p.parseStructDefinition(tok)
		if err != nil {
			return nil, tok.span, err
		}
		spec.Name = def.Name
		spec.StructDef = def
	case tok.kind == tokIdent:
		if !p.types[tok.text] {
			return nil, tok.span, errUnknownType(tok.text, tok.span)
		}
		spec.Name = tok.text
	default:
		return nil, tok.span, invalidToken(tok, wantKeyword(tokVoid), wantKeyword(tokStruct), wantTypeName)
	}

	arrays, err := p.parseArraySpecifier()
	if err != nil {
		return nil, tok.span, err
	}
	spec.Arrays = arrays
	spec.Span = tok.span
	if len(arrays) > 0 {
		// Rust returns a span that covers the type and its array specifiers.
		spec.Span = subsumeSpan(spec.Span, arrays[len(arrays)-1].Span)
	}
	if spec.Builtin != nil {
		p.checkDialectType(spec.Builtin, tok.text, tok.span)
		p.checkFloatPrecision(spec.Builtin, tok.span)
	}
	return spec, spec.Span, nil
}

// checkDialectType refuses a type the dialect does not have (design §15). Each
// diagnostic keeps glslang's wording and names the version that introduces the
// type; it is reported once per feature, as glslang stops at the first. name is
// the spelling the source wrote, which the cube-array message uses.
func (p *parser) checkDialectType(tn *typeName, name string, span ir.Span) {
	scalar, hasScalar := typeScalar(tn.Inner)
	if hasScalar {
		switch {
		case scalar.Kind == ir.ScalarFloat && scalar.Width == 8 && !p.dialect.supportsDouble():
			p.gate("double", span, p.dialect.unsupported("double", " (double requires GLSL 4.00)"))
		case scalar.Kind == ir.ScalarFloat && scalar.Width == 2 && !p.float16Allowed():
			p.gate("float16_t", span, p.dialect.unsupported("float16_t",
				" (half-float arithmetic requires the GL_EXT_shader_explicit_arithmetic_types_float16 extension in GLSL 3.30 core and is not part of GLSL ES 3.00)"))
		}
	}
	if image, ok := tn.Inner.(ir.ImageType); ok && image.Class == ir.ImageClassStorage && !p.dialect.supportsStorageImages() {
		// glslang: a bare `uniform image2D img;` prints "'image2D' : Reserved
		// word"; with a layout format qualifier it prints "'image load store' :
		// not supported for this version or the enabled extensions"
		// (img330bare.frag, img330.frag).
		p.gate("image load store", span, p.dialect.unsupported("image load store",
			" (storage images require GLSL 4.20 or ES 3.10)"))
	}
	if tn.Combined && tn.Dim == ir.DimCube && tn.Arrayed && !p.dialect.supportsCubeArraySamplers() {
		// glslang: "'samplerCubeArray' : Reserved word" and the same for
		// samplerCubeArrayShadow (cubearr330.frag, cubearr300.frag,
		// cubearrshadow330.frag); cube array textures are GLSL 4.00 / ES 3.20.
		p.gate("cube array sampler", span, p.dialect.unsupported(name,
			" (cube array samplers require GLSL 4.00 or ES 3.20)"))
	}
}

// float16Allowed reports whether float16_t types are accepted: Vulkan GLSL
// always, GL 3.30 core when GL_EXT_shader_explicit_arithmetic_types_float16 is
// enabled by #extension (design §15's "with extension" cell; glslang accepts
// gl330/float16-extension.frag), and never in GLSL ES 3.00, whose cell is "no".
func (p *parser) float16Allowed() bool {
	if p.dialect.vulkan() {
		return true
	}
	return p.dialect.gl330() && p.f16Extension
}

// gate records the first occurrence of a dialect-gate diagnostic.
func (p *parser) gate(feature string, span ir.Span, message string) {
	if p.reportedGates == nil {
		p.reportedGates = map[string]bool{}
	}
	if p.reportedGates[feature] {
		return
	}
	p.reportedGates[feature] = true
	p.errors = append(p.errors, *errSemantic(span, "%s", message))
}

// typeScalar returns the scalar of a scalar, vector or matrix type, the shape
// a GLSL scalar type name denotes.
func typeScalar(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch inner := inner.(type) {
	case ir.ScalarType:
		return inner, true
	case ir.VectorType:
		return inner.Scalar, true
	case ir.MatrixType:
		return inner.Scalar, true
	default:
		return ir.ScalarType{}, false
	}
}

// checkFloatPrecision enforces the ES 3.00 fragment rule that every declared
// float type needs a default precision in scope (GLSL ES 3.00 §4.5.3: the
// fragment language has no default precision for float). glslang reports
// "'float' : type requires declaration of default precision qualifier" at the
// first float type of the shader, and accepts a `precision ... float;`
// statement only when it precedes that first use, so the rule is checked at
// the type token in source order and once (frag300_noprec.frag,
// prec_after.frag, prec_fn_before.frag). A declaration that names a precision
// itself is exempt (`out highp vec4 c;`), which explicitPrecision carries; the
// vertex language has default highp and the desktop dialects ignore precision.
func (p *parser) checkFloatPrecision(tn *typeName, span ir.Span) {
	if p.dialect.vulkan() || !p.dialect.es() || p.dialect.version != 300 || p.stage != ir.StageFragment {
		return
	}
	if p.defaultFloatPrecision || p.explicitPrecision || p.inPrecisionDecl || p.inTypeConstructor || p.precisionErrorReported {
		return
	}
	scalar, ok := typeScalar(tn.Inner)
	if !ok || scalar.Kind != ir.ScalarFloat || scalar.Width != 4 {
		return
	}
	p.precisionErrorReported = true
	p.errors = append(p.errors, *errSemantic(span,
		"'float' : type requires declaration of default precision qualifier"))
}

// parseConstructorType parses the type name of a type constructor, as in
// vec4(...) or float[3](...). Its type is an expression and not a declaration,
// so the ES 3.00 default float precision rule does not apply: glslang accepts
// `vec2(1.0)` with no precision statement (p_ctor.frag).
func (p *parser) parseConstructorType() (typeSpec, ir.Span, *frontError) {
	p.inTypeConstructor = true
	spec, span, err := p.parseTypeNonVoid()
	p.inTypeConstructor = false
	return spec, span, err
}

// parseTypeExemptingPrecision parses a type of a declaration that names a
// precision qualifier itself, which the ES 3.00 default float precision rule
// does not require.
func (p *parser) parseTypeExemptingPrecision(explicit bool) (*typeSpec, ir.Span, *frontError) {
	saved := p.explicitPrecision
	p.explicitPrecision = explicit
	spec, span, err := p.parseType()
	p.explicitPrecision = saved
	return spec, span, err
}

// parseTypeNonVoidExemptingPrecision is parseTypeExemptingPrecision for a type
// that cannot be void.
func (p *parser) parseTypeNonVoidExemptingPrecision(explicit bool) (typeSpec, ir.Span, *frontError) {
	saved := p.explicitPrecision
	p.explicitPrecision = explicit
	spec, span, err := p.parseTypeNonVoid()
	p.explicitPrecision = saved
	return spec, span, err
}

// parseTypeNonVoid parses a type that cannot be void.
func (p *parser) parseTypeNonVoid() (typeSpec, ir.Span, *frontError) {
	spec, span, err := p.parseType()
	if err != nil {
		return typeSpec{}, span, err
	}
	if spec == nil {
		return typeSpec{}, span, errSemantic(span, "Type can't be void")
	}
	return *spec, span, nil
}

// parseStructDefinition parses `struct Name { members }` after its `struct`
// keyword and declares the type name, Rust naga's inline struct branch of
// parse_type (parser/types.rs:84-106). A definition without a declarator,
// `struct S { ... };`, reaches the same code through the type of a declaration.
func (p *parser) parseStructDefinition(structToken glslToken) (*structDef, *frontError) {
	// Only a struct definition inside a struct definition grows the type depth;
	// a flat member type is not nested any deeper than the struct that holds it.
	if err := p.enterType(structToken.span); err != nil {
		return nil, err
	}
	defer p.leaveType()

	nameTok, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expectPunct("{"); err != nil {
		return nil, err
	}
	members, err := p.parseStructMembers()
	if err != nil {
		return nil, err
	}
	end, err := p.expectPunct("}")
	if err != nil {
		return nil, err
	}
	def := &structDef{
		Name:    nameTok.text,
		Members: members,
		Span:    subsumeSpan(structToken.span, end.span),
	}
	// The name is a type from here on; Rust inserts it into lookup_type at the
	// same point (parser/types.rs:104).
	p.types[def.Name] = true
	return def, nil
}

// parseStructMembers parses the member list of a struct definition, Rust naga's
// parse_struct_declaration_list (parser/declarations.rs:625-683). Member offsets
// are a layout concern of lowering, so only the types, the names and the
// dimensions are parsed here.
func (p *parser) parseStructMembers() ([]structMember, *frontError) {
	var members []structMember
	for {
		// A member may carry a precision qualifier of its own (GLSL ES 3.00
		// §4.5.4; the desktop dialects accept and ignore it), which satisfies
		// the default float precision rule for that member.
		explicit := p.acceptPrecisionQualifier()
		base, baseSpan, err := p.parseTypeNonVoidExemptingPrecision(explicit)
		if err != nil {
			return nil, err
		}
		for {
			nameTok, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			arrays, err := p.parseArraySpecifier()
			if err != nil {
				return nil, err
			}
			members = append(members, structMember{
				Typ:    base,
				Name:   nameTok.text,
				Arrays: arrays,
				Span:   subsumeSpan(baseSpan, nameTok.span),
			})
			if p.bumpIf(punct(",")) == nil {
				break
			}
		}
		if _, err := p.expectPunct(";"); err != nil {
			return nil, err
		}
		next, err := p.expectPeek()
		if err != nil {
			return nil, err
		}
		if next.isPunct("}") {
			return members, nil
		}
	}
}

// parseTypeQualifiers parses the qualifier list written before a declaration's
// type, Rust naga's parse_type_qualifiers (parser/types.rs:169-301). Repeated
// qualifiers are semantic errors that do not stop the parse.
func (p *parser) parseTypeQualifiers() (typeQualifiers, *frontError) {
	var q typeQualifiers
	for p.peekTypeQualifier() {
		tok, err := p.bump()
		if err != nil {
			return q, err
		}
		if tok.kind == tokLayout {
			if err := p.parseLayoutQualifierList(&q); err != nil {
				return q, err
			}
			continue
		}
		q.Span = subsumeSpan(q.Span, tok.span)
		switch tok.kind {
		case tokInvariant:
			if q.Invariant {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"Cannot use more than one invariant qualifier per declaration"))
			}
			q.Invariant, q.InvariantSpan = true, tok.span
		case tokFlat, tokSmooth, tokNoPerspective:
			if q.Interpolation != interpolationNone {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"Cannot use more than one interpolation qualifier per declaration"))
			}
			q.Interpolation, q.InterpolationSpan = interpolationOf(tok.kind), tok.span
		case tokConst, tokIn, tokOut, tokUniform, tokShared, tokBuffer:
			if q.Storage != storageNone {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"Cannot use more than one storage qualifier per declaration"))
			}
			q.Storage, q.StorageSpan = storageOf(tok.kind), tok.span
			if tok.kind == tokBuffer && !p.dialect.supportsStorageBuffers() {
				// glslang rejects the declaration as a syntax error at the
				// `buffer` token: storage blocks are GLSL 4.30 / ES 3.10.
				p.gate("buffer", tok.span, p.dialect.unsupported("buffer",
					" (storage buffers require GLSL 4.30 or ES 3.10)"))
			}
		case tokCentroid, tokSample:
			if q.Sampling != samplingNone {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"Cannot use more than one sampling qualifier per declaration"))
			}
			q.Sampling, q.SamplingSpan = samplingOf(tok.kind), tok.span
		case tokHighP, tokMediumP, tokLowP:
			if q.Precision != precisionNone {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"Cannot use more than one precision qualifier per declaration"))
			}
			q.Precision, q.PrecisionSpan = precisionOf(tok.kind), tok.span
		case tokReadOnly, tokWriteOnly:
			access := ir.StorageAccessLoad
			if tok.kind == tokWriteOnly {
				access = ir.StorageAccessStore
			}
			const loadStore = ir.StorageAccessLoad | ir.StorageAccessStore
			if !q.HasStorageAccess {
				q.StorageAccess, q.HasStorageAccess = loadStore, true
			}
			// A qualifier may only narrow the access: seeing a bit a previous
			// qualifier already removed means the same qualifier was written
			// twice (parser/types.rs:277-294).
			if missing := loadStore &^ access; q.StorageAccess&missing != missing {
				p.errors = append(p.errors, *errSemantic(tok.span,
					"The same memory qualifier can only be used once"))
			}
			q.StorageAccess &= access
			q.StorageAccessSpan = subsumeSpan(q.StorageAccessSpan, tok.span)
		case tokRestrict:
			// The qualifier affects nothing in the IR, as in Rust.
		default:
			// peekTypeQualifier only admits the kinds handled above; reaching
			// this line is a bug in the frontend, not in the shader.
			return q, errInternal("unhandled type qualifier", tok.span)
		}
	}
	return q, nil
}

// interpolationOf maps an interpolation keyword kind to its qualifier.
func interpolationOf(k tokenKind) interpolationQualifier {
	switch k {
	case tokFlat:
		return interpolationFlat
	case tokSmooth:
		return interpolationSmooth
	default:
		return interpolationNoPerspective
	}
}

// storageOf maps a storage keyword kind to its qualifier.
func storageOf(k tokenKind) storageQualifier {
	switch k {
	case tokConst:
		return storageConst
	case tokIn:
		return storageInput
	case tokOut:
		return storageOutput
	case tokUniform:
		return storageUniform
	case tokShared:
		return storageShared
	default:
		return storageBuffer
	}
}

// samplingOf maps a sampling keyword kind to its qualifier.
func samplingOf(k tokenKind) samplingQualifier {
	if k == tokCentroid {
		return samplingCentroid
	}
	return samplingSample
}

// precisionOf maps a precision keyword kind to its qualifier.
func precisionOf(k tokenKind) precisionQualifier {
	switch k {
	case tokLowP:
		return precisionLow
	case tokMediumP:
		return precisionMedium
	default:
		return precisionHigh
	}
}

// parseLayoutQualifierList parses a layout(...) qualifier list, Rust naga's
// parse_layout_qualifier_id_list (parser/types.rs:303-323).
func (p *parser) parseLayoutQualifierList(q *typeQualifiers) *frontError {
	open, err := p.expectPunct("(")
	if err != nil {
		return err
	}
	q.Span = subsumeSpan(q.Span, open.span)
	for {
		if err := p.parseLayoutQualifier(q); err != nil {
			return err
		}
		if p.bumpIf(punct(",")) == nil {
			break
		}
	}
	closeTok, err := p.expectPunct(")")
	if err != nil {
		return err
	}
	q.Span = subsumeSpan(q.Span, closeTok.span)
	return nil
}

// parseLayoutQualifier parses one entry of a layout(...) list, Rust naga's
// parse_layout_qualifier_id (parser/types.rs:325-389). An entry that is not an
// identifier is a semantic error: Rust records it and keeps the list going.
func (p *parser) parseLayoutQualifier(q *typeQualifiers) *frontError {
	tok, err := p.bump()
	if err != nil {
		return err
	}
	if tok.kind != tokIdent {
		p.errors = append(p.errors, *invalidToken(tok, wantIdentifier))
		return nil
	}
	entry := layoutQualifier{Name: tok.text, Span: tok.span}
	switch tok.text {
	case "std140":
		entry.Kind, entry.Name, entry.Struct = layoutStructure, "", structLayoutStd140
	case "std430":
		entry.Kind, entry.Name, entry.Struct = layoutStructure, "", structLayoutStd430
	case "index":
		entry.Kind = layoutIndex
		entry.HasValue = true
		if _, err := p.expectPunct("="); err != nil {
			return err
		}
		entry.Value, err = p.parseExprInArena(p.parseConstantExpr)
		if err != nil {
			return err
		}
	default:
		if format, ok := imageFormat(tok.text); ok {
			entry.Kind, entry.Format = layoutFormat, format
			break
		}
		entry.Kind = layoutNamed
		if assign := p.bumpIf(punct("=")); assign != nil {
			entry.HasValue = true
			entry.Span = subsumeSpan(entry.Span, assign.span)
			var err *frontError
			entry.Value, err = p.parseExprInArena(p.parseConstantExpr)
			if err != nil {
				return err
			}
		}
	}
	q.Layout = append(q.Layout, entry)
	return nil
}

// imageFormat maps a layout qualifier to a storage image format, Rust naga's
// map_image_format (parser/types.rs:401-454). Rust's table lists rgb10_a2ui
// twice; the uint entry is the one that is reachable, so that is what this
// table keeps.
func imageFormat(word string) (ir.StorageFormat, bool) {
	format, ok := imageFormats[word]
	return format, ok
}

// imageFormats are the GLSL storage image format qualifiers.
var imageFormats = map[string]ir.StorageFormat{
	// float-image-format-qualifier
	"rgba32f":        ir.StorageFormatRgba32Float,
	"rgba16f":        ir.StorageFormatRgba16Float,
	"rg32f":          ir.StorageFormatRg32Float,
	"rg16f":          ir.StorageFormatRg16Float,
	"r11f_g11f_b10f": ir.StorageFormatRg11b10Ufloat,
	"r32f":           ir.StorageFormatR32Float,
	"r16f":           ir.StorageFormatR16Float,
	"rgba16":         ir.StorageFormatRgba16Unorm,
	"rgb10_a2ui":     ir.StorageFormatRgb10a2Uint,
	"rgb10_a2":       ir.StorageFormatRgb10a2Unorm,
	"rgba8":          ir.StorageFormatRgba8Unorm,
	"rg16":           ir.StorageFormatRg16Unorm,
	"rg8":            ir.StorageFormatRg8Unorm,
	"r16":            ir.StorageFormatR16Unorm,
	"r8":             ir.StorageFormatR8Unorm,
	"rgba16_snorm":   ir.StorageFormatRgba16Snorm,
	"rgba8_snorm":    ir.StorageFormatRgba8Snorm,
	"rg16_snorm":     ir.StorageFormatRg16Snorm,
	"rg8_snorm":      ir.StorageFormatRg8Snorm,
	"r16_snorm":      ir.StorageFormatR16Snorm,
	"r8_snorm":       ir.StorageFormatR8Snorm,

	// int-image-format-qualifier
	"rgba32i": ir.StorageFormatRgba32Sint,
	"rgba16i": ir.StorageFormatRgba16Sint,
	"rgba8i":  ir.StorageFormatRgba8Sint,
	"rg32i":   ir.StorageFormatRg32Sint,
	"rg16i":   ir.StorageFormatRg16Sint,
	"rg8i":    ir.StorageFormatRg8Sint,
	"r32i":    ir.StorageFormatR32Sint,
	"r16i":    ir.StorageFormatR16Sint,
	"r8i":     ir.StorageFormatR8Sint,

	// uint-image-format-qualifier
	"rgba32ui": ir.StorageFormatRgba32Uint,
	"rgba16ui": ir.StorageFormatRgba16Uint,
	"rgba8ui":  ir.StorageFormatRgba8Uint,
	"r64ui":    ir.StorageFormatR64Uint,
	"rg32ui":   ir.StorageFormatRg32Uint,
	"rg16ui":   ir.StorageFormatRg16Uint,
	"rg8ui":    ir.StorageFormatRg8Uint,
	"r32ui":    ir.StorageFormatR32Uint,
	"r16ui":    ir.StorageFormatR16Uint,
	"r8ui":     ir.StorageFormatR8Uint,
}
