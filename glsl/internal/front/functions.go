package front

// This file is Rust naga's front/glsl/functions.rs: constructors, user calls,
// overload resolution, out and inout proxy writes, and the lowering of a
// function declaration or prototype.

import (
	"reflect"

	"github.com/gogpu/naga/ir"
)

// loweredArg is one already-lowered call argument with the span it was
// written at, Rust naga's (Handle<Expression>, Span) pair.
type loweredArg struct {
	expr ir.ExpressionHandle
	span ir.Span
}

// functionOrConstructorCall lowers a call expression, Rust naga's
// Frontend::function_or_constructor_call (functions.rs:34-58). ok is false
// when the call yields no value (a void function).
func (f *Frontend) functionOrConstructorCall(c *Context, e *hirArena, callee callee, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	args := make([]loweredArg, 0, len(rawArgs))
	for _, raw := range rawArgs {
		expr, exprSpan, err := c.lowerExpect(e, raw, rhsPos)
		if err != nil {
			return 0, false, err
		}
		args = append(args, loweredArg{expr: expr, span: exprSpan})
	}

	// A combined sampler constructor such as sampler2D(t, s) is a call of the
	// sampler builtin: the parser meets the name as a type, the builtin
	// catalog has the overloads (Rust's MacroCall::Sampler, builtins.rs).
	if callee.Typ != nil && callee.Typ.Builtin != nil && callee.Typ.Builtin.Combined {
		return f.functionCall(c, e, callee.Name, args, rawArgs, span)
	}

	if callee.Kind == calleeFunction {
		return f.functionCall(c, e, callee.Name, args, rawArgs, span)
	}

	ty, err := f.resolveTypeSpec(callee.Typ, structLayoutStd140)
	if err != nil {
		return 0, false, err
	}
	// An unsized array constructor takes its size from the argument list:
	// float[](a, b, c) builds an array of three floats. The source's
	// ArraySize::Dynamic names no size and no backend can construct it, so
	// the constructor's type is replaced by the sized one; the empty form has
	// no size to take and is refused (Rust expressions.rs:136-152).
	if array, ok := c.mod.Types[ty].Inner.(ir.ArrayType); ok && array.Size.Constant == nil && array.Size.Pending == nil {
		if len(args) == 0 {
			return 0, false, errMessage(span, "There must be at least one argument")
		}
		size := uint32(len(args))
		array.Size.Constant = &size
		ty = f.registerType(array)
	}
	if len(args) == 1 {
		h, cerr := f.constructorSingle(c, ty, args[0], span)
		return h, true, cerr
	}
	h, cerr := f.constructorMany(c, ty, args, span)
	return h, true, cerr
}

// constructorSingle lowers a constructor of one argument, Rust naga's
// constructor_single (functions.rs:61-208).
func (f *Frontend) constructorSingle(c *Context, ty ir.TypeHandle, value loweredArg, span ir.Span) (ir.ExpressionHandle, *frontError) {
	expr, exprSpan := value.expr, value.span
	exprTy, err := c.resolveType(expr, exprSpan)
	if err != nil {
		return 0, err
	}
	var vectorSize *ir.VectorSize
	if v, ok := exprTy.(ir.VectorType); ok {
		size := v.Size
		vectorSize = &size
	}
	exprScalar, hasScalar := scalarComponents(exprTy)
	exprIsBool := hasScalar && exprScalar.Kind == ir.ScalarBool

	// Casting from a bool uses a select between zero and one, which the IR's
	// As cannot express.
	targetScalar, hasTargetScalar := scalarComponents(c.mod.Types[ty].Inner)
	if hasTargetScalar && exprIsBool && targetScalar.Kind != ir.ScalarBool {
		scalar := targetScalar
		scalar.Width = 4
		zero, okZero := zeroLiteral(scalar)
		one, okOne := oneLiteral(scalar)
		if !okZero || !okOne {
			return 0, errMessage(span, "Bad type constructor")
		}
		reject, err := c.addExpression(ir.Literal{Value: zero}, exprSpan)
		if err != nil {
			return 0, err
		}
		accept, err := c.addExpression(ir.Literal{Value: one}, exprSpan)
		if err != nil {
			return 0, err
		}
		reject, err = c.implicitSplat(reject, span, vectorSize)
		if err != nil {
			return 0, err
		}
		accept, err = c.implicitSplat(accept, span, vectorSize)
		if err != nil {
			return 0, err
		}
		return c.addExpression(ir.ExprSelect{Condition: expr, Accept: accept, Reject: reject}, exprSpan)
	}

	switch inner := c.mod.Types[ty].Inner.(type) {
	case ir.VectorType:
		if vectorSize == nil {
			converted, err := c.forcedConversion(expr, exprSpan, inner.Scalar)
			if err != nil {
				return 0, err
			}
			// Re-resolve after the conversion: the appended As expression is
			// not typed yet (Rust calls resolve_type here, functions.rs:112).
			convertedTy, err := c.resolveType(converted, exprSpan)
			if err != nil {
				return 0, err
			}
			if _, ok := convertedTy.(ir.ScalarType); ok {
				return c.addExpression(ir.ExprSplat{Size: inner.Size, Value: converted}, span)
			}
			return f.vectorConstructor(c, ty, inner.Size, inner.Scalar, []loweredArg{{expr: converted, span: exprSpan}}, span)
		}
		if *vectorSize != inner.Size {
			expr, err = c.vectorResize(inner.Size, expr, exprSpan)
			if err != nil {
				return 0, err
			}
		}
		width := inner.Scalar.Width
		return c.addExpression(ir.ExprAs{Expr: expr, Kind: inner.Scalar.Kind, Convert: &width}, span)

	case ir.ScalarType:
		// A vector argument contributes its first component; a matrix its
		// first column and then that column's first component, two extracts
		// (functions.rs:116-130).
		consume := expr
		kind, rerr := c.resolveType(expr, exprSpan)
		if rerr != nil {
			return 0, rerr
		}
		switch kind.(type) {
		case ir.VectorType, ir.MatrixType:
			consume, err = c.addExpression(ir.ExprAccessIndex{Base: consume, Index: 0}, span)
			if err != nil {
				return 0, err
			}
		}
		if kind, rerr := c.resolveType(expr, exprSpan); rerr != nil {
			return 0, rerr
		} else if _, ok := kind.(ir.MatrixType); ok {
			consume, err = c.addExpression(ir.ExprAccessIndex{Base: consume, Index: 0}, span)
			if err != nil {
				return 0, err
			}
		}
		width := inner.Width
		return c.addExpression(ir.ExprAs{Expr: consume, Kind: inner.Kind, Convert: &width}, span)

	case ir.MatrixType:
		return f.matrixOneArg(c, ty, inner, value, span)

	case ir.StructType:
		if len(inner.Members) > 0 {
			if scalar, ok := scalarComponents(c.mod.Types[inner.Members[0].Type].Inner); ok {
				expr, err = c.implicitConversion(expr, exprSpan, scalar)
				if err != nil {
					return 0, err
				}
			}
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: []ir.ExpressionHandle{expr}}, span)

	case ir.ArrayType:
		if scalar, ok := scalarComponents(c.mod.Types[inner.Base].Inner); ok {
			expr, err = c.implicitConversion(expr, exprSpan, scalar)
			if err != nil {
				return 0, err
			}
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: []ir.ExpressionHandle{expr}}, span)

	default:
		f.addError(errMessage(span, "Bad type constructor"))
		return expr, nil
	}
}

// matrixOneArg lowers a matrix constructor of one argument, Rust naga's
// matrix_one_arg (functions.rs:211-356): a scalar fills the diagonal, a matrix
// is copied into the top-left corner and the rest is the identity.
func (f *Frontend) matrixOneArg(c *Context, ty ir.TypeHandle, m ir.MatrixType, value loweredArg, span ir.Span) (ir.ExpressionHandle, *frontError) {
	expr, exprSpan := value.expr, value.span
	expr, err := c.forcedConversion(expr, exprSpan, m.Scalar)
	if err != nil {
		return 0, err
	}
	components := make([]ir.ExpressionHandle, 0, m.Columns)
	vectorTy := f.registerType(ir.VectorType{Size: m.Rows, Scalar: m.Scalar})

	sourceTy, err := c.resolveType(expr, exprSpan)
	if err != nil {
		return 0, err
	}
	switch source := sourceTy.(type) {
	case ir.ScalarType:
		zero, ok := zeroLiteral(m.Scalar)
		if !ok {
			return 0, errMessage(span, "Bad type constructor")
		}
		zeroExpr, err := c.addExpression(ir.Literal{Value: zero}, span)
		if err != nil {
			return 0, err
		}
		for i := uint32(0); i < uint32(m.Columns); i++ {
			column := make([]ir.ExpressionHandle, 0, m.Rows)
			for r := uint32(0); r < uint32(m.Rows); r++ {
				if r == i {
					column = append(column, expr)
				} else {
					column = append(column, zeroExpr)
				}
			}
			h, err := c.addExpression(ir.ExprCompose{Type: vectorTy, Components: column}, span)
			if err != nil {
				return 0, err
			}
			components = append(components, h)
		}

	case ir.MatrixType:
		zero, okZero := zeroLiteral(m.Scalar)
		one, okOne := oneLiteral(m.Scalar)
		if !okZero || !okOne {
			return 0, errMessage(span, "Bad type constructor")
		}
		zeroExpr, err := c.addExpression(ir.Literal{Value: zero}, span)
		if err != nil {
			return 0, err
		}
		oneExpr, err := c.addExpression(ir.Literal{Value: one}, span)
		if err != nil {
			return 0, err
		}
		for i := uint32(0); i < uint32(m.Columns); i++ {
			if i >= uint32(source.Columns) {
				column := make([]ir.ExpressionHandle, 0, m.Rows)
				for r := uint32(0); r < uint32(m.Rows); r++ {
					if r == i {
						column = append(column, oneExpr)
					} else {
						column = append(column, zeroExpr)
					}
				}
				h, err := c.addExpression(ir.ExprCompose{Type: vectorTy, Components: column}, span)
				if err != nil {
					return 0, err
				}
				components = append(components, h)
				continue
			}
			vector, err := c.addExpression(ir.ExprAccessIndex{Base: expr, Index: i}, span)
			if err != nil {
				return 0, err
			}
			switch {
			case source.Rows < m.Rows:
				column := make([]ir.ExpressionHandle, 0, m.Rows)
				for r := uint32(0); r < uint32(m.Rows); r++ {
					switch {
					case r < uint32(source.Rows):
						component, err := c.addExpression(ir.ExprAccessIndex{Base: vector, Index: r}, span)
						if err != nil {
							return 0, err
						}
						column = append(column, component)
					case r == i:
						column = append(column, oneExpr)
					default:
						column = append(column, zeroExpr)
					}
				}
				h, err := c.addExpression(ir.ExprCompose{Type: vectorTy, Components: column}, span)
				if err != nil {
					return 0, err
				}
				components = append(components, h)
			case source.Rows == m.Rows:
				components = append(components, vector)
			default:
				trimmed, err := c.vectorResize(m.Rows, vector, span)
				if err != nil {
					return 0, err
				}
				components = append(components, trimmed)
			}
		}

	default:
		for i := uint32(0); i < uint32(m.Columns); i++ {
			components = append(components, expr)
		}
	}

	return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
}

// vectorConstructor lowers a vector constructor of several components, Rust
// naga's vector_constructor (functions.rs:358-412): components flatten into
// the vector's lanes and anything past them is dropped.
func (f *Frontend) vectorConstructor(c *Context, ty ir.TypeHandle, size ir.VectorSize, scalar ir.ScalarType, args []loweredArg, span ir.Span) (ir.ExpressionHandle, *frontError) {
	components := make([]ir.ExpressionHandle, 0, size)
	for _, arg := range args {
		expr, err := c.forcedConversion(arg.expr, arg.span, scalar)
		if err != nil {
			return 0, err
		}
		if len(components) >= vectorSizeCount(size) {
			break
		}
		argTy, aerr := c.resolveType(expr, arg.span)
		if aerr != nil {
			return 0, aerr
		}
		switch inner := argTy.(type) {
		case ir.ScalarType:
			components = append(components, expr)
		case ir.MatrixType:
			for col := uint32(0); col < uint32(inner.Columns); col++ {
				column, err := c.addExpression(ir.ExprAccessIndex{Base: expr, Index: col}, arg.span)
				if err != nil {
					return 0, err
				}
				for row := uint32(0); row < uint32(inner.Rows); row++ {
					component, err := c.addExpression(ir.ExprAccessIndex{Base: column, Index: row}, arg.span)
					if err != nil {
						return 0, err
					}
					components = append(components, component)
				}
			}
		case ir.VectorType:
			for i := uint32(0); i < uint32(inner.Size); i++ {
				component, err := c.addExpression(ir.ExprAccessIndex{Base: expr, Index: i}, arg.span)
				if err != nil {
					return 0, err
				}
				components = append(components, component)
			}
		default:
			components = append(components, expr)
		}
	}
	if len(components) > vectorSizeCount(size) {
		components = components[:vectorSizeCount(size)]
	}
	return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
}

// constructorMany lowers a constructor of several arguments: a matrix, a
// vector, a struct or an array, Rust naga's constructor_many
// (functions.rs:414-514).
func (f *Frontend) constructorMany(c *Context, ty ir.TypeHandle, args []loweredArg, span ir.Span) (ir.ExpressionHandle, *frontError) {
	switch inner := c.mod.Types[ty].Inner.(type) {
	case ir.MatrixType:
		flattened := make([]ir.ExpressionHandle, 0, len(args))
		for _, arg := range args {
			expr, err := c.forcedConversion(arg.expr, arg.span, inner.Scalar)
			if err != nil {
				return 0, err
			}
			argTy, aerr := c.resolveType(expr, arg.span)
			if aerr != nil {
				return 0, aerr
			}
			if v, ok := argTy.(ir.VectorType); ok {
				for i := uint32(0); i < uint32(v.Size); i++ {
					component, err := c.addExpression(ir.ExprAccessIndex{Base: expr, Index: i}, arg.span)
					if err != nil {
						return 0, err
					}
					flattened = append(flattened, component)
				}
				continue
			}
			flattened = append(flattened, expr)
		}
		components := make([]ir.ExpressionHandle, 0, inner.Columns)
		for i := 0; i < vectorSizeCount(inner.Columns); i++ {
			start := i * vectorSizeCount(inner.Rows)
			if start >= len(flattened) {
				break
			}
			end := min(start+vectorSizeCount(inner.Rows), len(flattened))
			columnTy := f.registerType(ir.VectorType{Size: inner.Rows, Scalar: inner.Scalar})
			column, err := c.addExpression(ir.ExprCompose{Type: columnTy, Components: flattened[start:end]}, span)
			if err != nil {
				return 0, err
			}
			components = append(components, column)
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)

	case ir.VectorType:
		return f.vectorConstructor(c, ty, inner.Size, inner.Scalar, args, span)

	case ir.ArrayType:
		// Every element is converted to the base type (functions.rs:475-485).
		components := make([]ir.ExpressionHandle, 0, len(args))
		for _, arg := range args {
			expr := arg.expr
			if scalar, ok := scalarComponents(c.mod.Types[inner.Base].Inner); ok {
				var err *frontError
				expr, err = c.implicitConversion(expr, arg.span, scalar)
				if err != nil {
					return 0, err
				}
			}
			components = append(components, expr)
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)

	case ir.StructType:
		// Each argument is converted to its member's scalar type
		// (functions.rs:486-510).
		components := make([]ir.ExpressionHandle, 0, len(args))
		for i, arg := range args {
			expr := arg.expr
			if i < len(inner.Members) {
				if scalar, ok := scalarComponents(c.mod.Types[inner.Members[i].Type].Inner); ok {
					var err *frontError
					expr, err = c.implicitConversion(expr, arg.span, scalar)
					if err != nil {
						return 0, err
					}
				}
			}
			components = append(components, expr)
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)

	default:
		return 0, errMessage(span, "Constructor: Too many arguments")
	}
}

// conversionRank is how good a conversion between two types is, Rust naga's
// Conversion (functions.rs:1507-1520). A lower value is a better conversion.
type conversionRank uint8

const (
	convertExact conversionRank = iota
	convertFloatToDouble
	convertIntToFloat
	convertIntToDouble
	convertOther
	convertNone
)

// conversionOf returns how a value of type source converts to target, Rust
// naga's conversion (functions.rs:1524-1589): scalars and same-shaped vectors
// and matrices convert, and only upwards.
func conversionOf(target, source ir.TypeInner) (conversionRank, bool) {
	var targetScalar, sourceScalar ir.ScalarType
	switch t := target.(type) {
	case ir.ScalarType:
		s, ok := source.(ir.ScalarType)
		if !ok {
			return convertNone, false
		}
		targetScalar, sourceScalar = t, s
	case ir.VectorType:
		s, ok := source.(ir.VectorType)
		if !ok || s.Size != t.Size {
			return convertNone, false
		}
		targetScalar, sourceScalar = t.Scalar, s.Scalar
	case ir.MatrixType:
		s, ok := source.(ir.MatrixType)
		if !ok || s.Columns != t.Columns || s.Rows != t.Rows {
			return convertNone, false
		}
		targetScalar, sourceScalar = t.Scalar, s.Scalar
	default:
		return convertNone, false
	}

	targetPower, targetOK := typePower(targetScalar)
	sourcePower, sourceOK := typePower(sourceScalar)
	if !targetOK || !sourceOK || targetPower < sourcePower {
		return convertNone, false
	}
	switch {
	case targetScalar.Kind == ir.ScalarFloat && targetScalar.Width == 8 &&
		sourceScalar.Kind == ir.ScalarFloat && sourceScalar.Width == 4:
		return convertFloatToDouble, true
	case targetScalar.Kind == ir.ScalarFloat && targetScalar.Width == 4 &&
		(sourceScalar.Kind == ir.ScalarSint || sourceScalar.Kind == ir.ScalarUint):
		return convertIntToFloat, true
	case targetScalar.Kind == ir.ScalarFloat && targetScalar.Width == 8 &&
		(sourceScalar.Kind == ir.ScalarSint || sourceScalar.Kind == ir.ScalarUint):
		return convertIntToDouble, true
	default:
		return convertOther, true
	}
}

// functionCall lowers a call of a named function or builtin, Rust naga's
// function_call (functions.rs:515-868): overload resolution by argument type
// and conversion quality, then a Statement::Call with the out and inout
// arguments spilled and written back.
func (f *Frontend) functionCall(c *Context, e *hirArena, name string, args []loweredArg, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	argTypes := make([]ir.TypeInner, 0, len(args))
	for _, arg := range args {
		if err := c.typifierGrow(arg.span); err != nil {
			return 0, false, err
		}
		argTypes = append(argTypes, c.getType(arg.expr))
	}

	if reason := refusedBuiltin(name); reason != "" {
		// A deliberately unsupported builtin fails here, where the call's span
		// is known, so the diagnostic carries a source location; injectBuiltin's
		// own refusal stays as a fallback for the declaration-time call sites.
		return 0, false, errMessage(span, reason)
	}
	// A builtin family the dialect's version does not have is refused here
	// unless the shader itself declared a function of that name, which the
	// builtin must not shadow.
	if reason := f.dialect.builtinRefusal(name); reason != "" && !userDefinedFunction(f.functions[name]) {
		return 0, false, errMessage(span, reason)
	}
	decl, variations := f.functionDeclaration(name, builtinRequiredVariations(argTypes...))
	f.injectBuiltin(decl, name, variations)

	var (
		chosen     *overload
		ambiguous  bool
		haveChosen bool
	)
	oldConversions := make([]conversionRank, len(args))
	for i := range oldConversions {
		oldConversions[i] = convertNone
	}

	for _, overload := range decl.overloads {
		if len(args) != len(overload.parameters) {
			continue
		}
		exact := true
		superior := 0 // 0: unknown, 1: this overload is better, -1: the previous one
		newConversions := make([]conversionRank, len(args))
		for i := range newConversions {
			newConversions[i] = convertNone
		}

		skip := false
		for i, parameter := range overload.parameters {
			arg := args[i]
			info := overload.parametersInfo[i]

			if info.depth {
				if err := sampledToDepth(c, arg.expr, arg.span); err != nil {
					f.addError(err)
				}
				c.invalidateTypifier()
			}
			if err := c.typifierGrow(arg.span); err != nil {
				return 0, false, err
			}

			parameterTy := c.mod.Types[parameter].Inner
			argTy := c.getType(arg.expr)

			// A storage image's access is a qualifier in GLSL, not part of the
			// type it must match on (functions.rs:596-649).
			if paramImage, ok := parameterTy.(ir.ImageType); ok && paramImage.Class == ir.ImageClassStorage {
				if argImage, ok := argTy.(ir.ImageType); ok && argImage.Class == ir.ImageClassStorage {
					sameSize := argImage.Dim == paramImage.Dim && argImage.Arrayed == paramImage.Arrayed
					sameFormat := paramImage.StorageFormat == argImage.StorageFormat ||
						(overload.internal && paramImage.StorageFormat.ScalarKind() == argImage.StorageFormat.ScalarKind())
					if !sameSize || !sameFormat {
						skip = true
						break
					}
					if !argImage.StorageAccess.Contains(paramImage.StorageAccess) {
						f.addError(errSemantic(span, "'%s': image needs %s access but only %s was provided",
							name, storageAccessDebug(paramImage.StorageAccess), storageAccessDebug(argImage.StorageAccess)))
					}
					newConversions[i] = convertExact
					continue
				}
			}
			if typeInnerEqual(parameterTy, argTy) {
				newConversions[i] = convertExact
				continue
			}

			// An inout argument must match exactly: its value is written back.
			if info.qualifier == paramInOut {
				skip = true
				break
			}

			var rank conversionRank
			var ok bool
			if info.qualifier.isLHS() {
				rank, ok = conversionOf(argTy, parameterTy)
			} else {
				rank, ok = conversionOf(parameterTy, argTy)
			}
			if !ok {
				skip = true
				break
			}

			exact = false
			var better int
			switch {
			case oldConversions[i] == convertExact:
				better = -1
			case oldConversions[i] == convertNone:
				better = 1
			case rank == convertFloatToDouble:
				better = 1
			case oldConversions[i] == convertFloatToDouble:
				better = -1
			case rank == convertIntToFloat && oldConversions[i] == convertIntToDouble:
				better = 1
			case rank == convertIntToDouble && oldConversions[i] == convertIntToFloat:
				better = -1
			default:
				continue
			}
			switch better {
			case 1:
				if superior == -1 {
					ambiguous = true
				} else {
					superior = 1
					newConversions[i] = rank
				}
			case -1:
				if superior == 1 {
					ambiguous = true
				} else {
					superior = -1
				}
			}
		}
		if skip {
			continue
		}

		if exact {
			chosen, haveChosen = overload, true
			ambiguous = false
			break
		}
		switch superior {
		case 1:
			chosen, haveChosen = overload, true
			oldConversions = newConversions
		case 0:
			ambiguous = true
			chosen, haveChosen = overload, true
		}
	}

	if ambiguous {
		f.addError(errSemantic(span, "Ambiguous best function for '%s'", name))
	}
	if !haveChosen {
		if refusal := subgroupRefusal(name, span); refusal != nil {
			return 0, false, refusal
		}
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}

	parametersInfo := chosen.parametersInfo
	parameters := chosen.parameters
	isVoid := chosen.void
	kind := chosen.kind
	function := chosen.function
	var macro *macroCall
	if kind == overloadMacro {
		macro = chosen.macro
	}

	arguments := make([]ir.ExpressionHandle, 0, len(args))
	var proxyWrites []proxyWrite
	for i, parameter := range parameters {
		arg := args[i]
		if parametersInfo[i].qualifier.isLHS() {
			// An out or inout argument is re-lowered in Lhs position: the
			// write-back store needs a pointer, and the value lowered for the
			// overload comparison is not one (functions.rs:789-805).
			pointer, ptrSpan, err := c.lowerExpect(e, rawArgs[i], lhsPos)
			if err != nil {
				return 0, false, err
			}
			pointerArg := loweredArg{expr: pointer, span: ptrSpan}
			if perr := f.processLHSArgument(c, parameter, &parametersInfo[i], pointerArg, arg, &proxyWrites, &arguments); perr != nil {
				return 0, false, perr
			}
			continue
		}
		handle := arg.expr
		if scalar, ok := scalarComponents(c.mod.Types[parameter].Inner); ok {
			converted, err := c.implicitConversion(handle, arg.span, scalar)
			if err != nil {
				return 0, false, err
			}
			handle = converted
		}
		arguments = append(arguments, handle)
	}

	if kind == overloadMacro {
		return macro.call(c, toLoweredArgs(arguments, span), span)
	}

	c.emitEnd()
	var result ir.ExpressionHandle
	if !isVoid {
		var err *frontError
		result, err = c.addExpression(ir.ExprCallResult{Function: function}, span)
		if err != nil {
			return 0, false, err
		}
	}
	stmt := ir.StmtCall{Function: function, Arguments: arguments}
	if !isVoid {
		r := result
		stmt.Result = &r
	}
	c.body = append(c.body, ir.Statement{Kind: stmt})
	c.emitStart()

	for _, write := range proxyWrites {
		value, err := c.addExpression(ir.ExprLoad{Pointer: write.value}, span)
		if err != nil {
			return 0, false, err
		}
		if write.convert != nil {
			value, err = c.conversion(value, span, *write.convert)
			if err != nil {
				return 0, false, err
			}
		}
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: write.target, Value: value}})
	}
	return result, !isVoid, nil
}

// toLoweredArgs pairs the final argument handles with the call's span, for a
// builtin expansion.
func toLoweredArgs(arguments []ir.ExpressionHandle, span ir.Span) []loweredArg {
	args := make([]loweredArg, len(arguments))
	for i, argument := range arguments {
		args[i] = loweredArg{expr: argument, span: span}
	}
	return args
}

// proxyWrite is one write back to an out or inout argument after the call,
// Rust naga's ProxyWrite (functions.rs:23-31).
type proxyWrite struct {
	target  ir.ExpressionHandle
	value   ir.ExpressionHandle
	convert *ir.ScalarType
}

// processLHSArgument prepares an out or inout argument, Rust naga's
// process_lhs_argument (functions.rs:873-1026): a swizzled or non-function
// address space argument is spilled into a local, and the value is written
// back through the pointer the argument lowered to. pointer is the argument
// re-lowered in Lhs position; value is the argument as the overload
// comparison saw it.
func (f *Frontend) processLHSArgument(c *Context, parameterTy ir.TypeHandle, info *parameterInfo, pointer, valueArg loweredArg, proxyWrites *[]proxyWrite, arguments *[]ir.ExpressionHandle) *frontError {
	original := pointer.expr
	originalTy, err := c.resolveType(original, pointer.span)
	if err != nil {
		return err
	}
	_, isPointer := originalTy.(ir.PointerType)
	_, isValuePointer := originalTy.(ir.ValuePointerType)

	var maybeTy *ir.TypeHandle
	switch t := originalTy.(type) {
	case ir.VectorType:
		h := f.registerType(ir.VectorType{Size: t.Size, Scalar: t.Scalar})
		maybeTy = &h
	case ir.PointerType:
		if t.Space != ir.SpaceFunction {
			base := t.Base
			maybeTy = &base
		}
	case ir.ValuePointerType:
		if t.Space != ir.SpaceFunction {
			var inner ir.TypeInner
			if t.Size != nil {
				inner = ir.VectorType{Size: *t.Size, Scalar: t.Scalar}
			} else {
				inner = t.Scalar
			}
			h := f.registerType(inner)
			maybeTy = &h
		}
	}

	value := original
	if isPointer || isValuePointer {
		value, err = c.addExpression(ir.ExprLoad{Pointer: original}, ir.Span{})
		if err != nil {
			return err
		}
	}

	if err := c.typifierGrow(valueArg.span); err != nil {
		return err
	}
	parameterInner := c.mod.Types[parameterTy].Inner
	argInner := c.getType(valueArg.expr)
	needsConversion := !typeInnerEqual(argInner, parameterInner)
	argScalar, hasScalar := scalarComponents(argInner)

	if needsConversion {
		maybeTy = &parameterTy
	}

	if maybeTy == nil {
		*arguments = append(*arguments, original)
		return nil
	}

	spillVar := ir.LocalVariableHandle(len(c.locals))
	c.locals = append(c.locals, ir.LocalVariable{Type: *maybeTy})
	spillExpr, err := c.addExpression(ir.ExprLocalVariable{Variable: spillVar}, ir.Span{})
	if err != nil {
		return err
	}
	if info.qualifier == paramInOut {
		c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: spillExpr, Value: value}})
	}
	*arguments = append(*arguments, spillExpr)

	var convert *ir.ScalarType
	if needsConversion && hasScalar {
		convert = &argScalar
	}

	if sw, ok := c.Expression(original).(ir.ExprSwizzle); ok {
		vector := sw.Vector
		if load, ok := c.Expression(vector).(ir.ExprLoad); ok {
			vector = load.Pointer
		}
		for i := 0; i < vectorSizeCount(sw.Size); i++ {
			target, err := c.addExpression(ir.ExprAccessIndex{Base: vector, Index: uint32(sw.Pattern[i])}, ir.Span{})
			if err != nil {
				return err
			}
			spillComponent, err := c.addExpression(ir.ExprAccessIndex{Base: spillExpr, Index: uint32(i)}, ir.Span{})
			if err != nil {
				return err
			}
			*proxyWrites = append(*proxyWrites, proxyWrite{target: target, value: spillComponent, convert: convert})
		}
		return nil
	}
	*proxyWrites = append(*proxyWrites, proxyWrite{target: original, value: spillExpr, convert: convert})
	return nil
}

// sampledToDepth turns a sampled image into a depth image, Rust naga's
// sampled_to_depth (builtins.rs:2213-2280): a global's type is replaced, and
// a function argument is marked as carrying a depth image.
func sampledToDepth(c *Context, image ir.ExpressionHandle, span ir.Span) *frontError {
	var ty *ir.TypeHandle
	argIndex := -1
	switch kind := c.Expression(image).(type) {
	case ir.ExprGlobalVariable:
		ty = &c.mod.GlobalVariables[kind.Variable].Type
	case ir.ExprFunctionArgument:
		idx := int(kind.Index)
		if idx >= len(c.parametersInfo) || idx >= len(c.arguments) {
			return errMessage(span, "Not a valid texture expression")
		}
		c.parametersInfo[idx].depth = true
		ty = &c.arguments[idx].Type
		argIndex = idx
	default:
		return errMessage(span, "Not a valid texture expression")
	}

	img, ok := c.mod.Types[*ty].Inner.(ir.ImageType)
	if !ok {
		return errMessage(span, "Not a valid texture expression")
	}
	if img.Class != ir.ImageClassDepth {
		img.Class = ir.ImageClassDepth
		*ty = c.fe.registerType(img)
	}
	// An image passed through a function argument also changes the
	// parameter, so a call to this function converts its argument too
	// (builtins.rs:2280-2284).
	if argIndex >= 0 && argIndex < len(c.parameters) {
		c.parameters[argIndex] = *ty
	}
	return nil
}

// typeInnerEqual reports whether two type inners are the same type, which is
// how an overload's parameter is matched to a call argument (Rust compares
// TypeInner by value, functions.rs:1080-1087).
func typeInnerEqual(a, b ir.TypeInner) bool {
	return reflect.DeepEqual(a, b)
}

// zeroLiteral is the value zero of a scalar type, Rust's Literal::zero.
func zeroLiteral(s ir.ScalarType) (ir.LiteralValue, bool) {
	switch s.Kind {
	case ir.ScalarSint:
		switch s.Width {
		case 2:
			return ir.LiteralI16(0), true
		case 4:
			return ir.LiteralI32(0), true
		case 8:
			return ir.LiteralI64(0), true
		}
	case ir.ScalarUint:
		switch s.Width {
		case 2:
			return ir.LiteralU16(0), true
		case 4:
			return ir.LiteralU32(0), true
		case 8:
			return ir.LiteralU64(0), true
		}
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return ir.LiteralF16(0), true
		case 4:
			return ir.LiteralF32(0), true
		case 8:
			return ir.LiteralF64(0), true
		}
	case ir.ScalarBool:
		return ir.LiteralBool(false), true
	}
	return nil, false
}

// userDefinedFunction reports whether a name has an overload the shader itself
// declared, which a version gate of a builtin family must not refuse.
func userDefinedFunction(decl *functionDeclaration) bool {
	if decl == nil {
		return false
	}
	for i := range decl.overloads {
		if decl.overloads[i].defined && !decl.overloads[i].internal {
			return true
		}
	}
	return false
}

// functionDeclaration returns the declaration of name and the builtin
// variations to inject for it. A name seen for the first time also asks for
// the standard overloads, which is how Rust's lookup_function entry works
// (functions.rs:535-539): the bit goes into the requested variations, not the
// declaration's.
func (f *Frontend) functionDeclaration(name string, variations builtinVariations) (*functionDeclaration, builtinVariations) {
	decl, ok := f.functions[name]
	if !ok {
		decl = &functionDeclaration{}
		f.functions[name] = decl
		variations |= variationStandard
	}
	return decl, variations
}

// lowerFunctionDeclaration lowers a function prototype or definition, Rust
// naga's function declaration parsing (parser/declarations.rs) with
// add_function and add_prototype (functions.rs:1028-1218).
func (f *Frontend) lowerFunctionDeclaration(d *declFunc) *frontError {
	var result *ir.FunctionResult
	if d.Return != nil {
		ty, err := f.resolveTypeSpec(d.Return, structLayoutStd140)
		if err != nil {
			return err
		}
		result = &ir.FunctionResult{Type: ty}
	}

	c, err := newContext(f, false)
	if err != nil {
		return err
	}
	for i := range d.Params {
		p := &d.Params[i]
		ty, rerr := f.resolveTypeSpec(&p.Typ, structLayoutStd140)
		if rerr != nil {
			return rerr
		}
		ty, rerr = f.resolveTypeArrays(ty, p.Arrays)
		if rerr != nil {
			return rerr
		}
		if aerr := c.addFunctionArg(p.Sym, ty, p.Qualifier, p.Span); aerr != nil {
			return aerr
		}
	}

	if d.Body == nil {
		f.addPrototype(c, d.Name, result, d.Span)
		return nil
	}
	if lerr := c.lowerStatements(d.Body); lerr != nil {
		return lerr
	}
	f.addFunction(c, d.Name, result, d.Span)
	return nil
}

// addFunction finishes a defined function, Rust naga's add_function
// (functions.rs:1028-1117): a matching prototype takes the body, a second
// definition is an error, and anything else adds an overload.
func (f *Frontend) addFunction(c *Context, name string, result *ir.FunctionResult, span ir.Span) {
	ensureBlockReturns(&c.body)

	decl, variations := f.functionDeclaration(name, builtinRequiredVariations(c.parameterTypes()...))
	f.injectBuiltin(decl, name, variations)

	function := c.intoFunction(name)
	function.Result = result

	for _, overload := range decl.overloads {
		if !sameParameters(overload.parameters, c.parameters) {
			continue
		}
		if overload.defined {
			f.addError(errMessage(span, "Function already defined"))
			return
		}
		overload.defined = true
		overload.parametersInfo = c.parametersInfo
		if overload.kind == overloadCall {
			f.mod.Functions[overload.function] = function
		} else {
			handle := ir.FunctionHandle(len(f.mod.Functions))
			f.mod.Functions = append(f.mod.Functions, function)
			f.functionSpans = append(f.functionSpans, span)
			overload.kind = overloadCall
			overload.function = handle
			overload.macro = nil
		}
		return
	}

	handle := ir.FunctionHandle(len(f.mod.Functions))
	f.mod.Functions = append(f.mod.Functions, function)
	decl.overloads = append(decl.overloads, &overload{
		parameters:     c.parameters,
		parametersInfo: c.parametersInfo,
		kind:           overloadCall,
		function:       handle,
		defined:        true,
		void:           result == nil,
	})
}

// addPrototype records a function prototype, Rust naga's add_prototype
// (functions.rs:1119-1186).
func (f *Frontend) addPrototype(c *Context, name string, result *ir.FunctionResult, span ir.Span) {
	decl, variations := f.functionDeclaration(name, builtinRequiredVariations(c.parameterTypes()...))
	f.injectBuiltin(decl, name, variations)

	for _, overload := range decl.overloads {
		if sameParameters(overload.parameters, c.parameters) {
			f.addError(errMessage(span, "Prototype already defined"))
			return
		}
	}

	function := ir.Function{Name: name, Arguments: c.arguments, Result: result}
	handle := ir.FunctionHandle(len(f.mod.Functions))
	f.mod.Functions = append(f.mod.Functions, function)
	f.functionSpans = append(f.functionSpans, span)
	decl.overloads = append(decl.overloads, &overload{
		parameters:     c.parameters,
		parametersInfo: c.parametersInfo,
		kind:           overloadCall,
		function:       handle,
		void:           result == nil,
	})
}

// sameParameters reports whether two parameter lists name the same types, the
// check add_function and add_prototype use (functions.rs:1080-1087).
func sameParameters(a, b []ir.TypeHandle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ensureBlockReturns appends a return to a block whose control flow can fall
// through, Rust naga's ensure_block_returns (proc/terminator.rs:7-60).
func ensureBlockReturns(block *ir.Block) {
	if len(*block) == 0 {
		*block = append(*block, ir.Statement{Kind: ir.StmtReturn{}})
		return
	}
	last := &(*block)[len(*block)-1]
	switch kind := last.Kind.(type) {
	case ir.StmtBlock:
		ensureBlockReturns(&kind.Block)
		last.Kind = kind
		return
	case ir.StmtIf:
		ensureBlockReturns(&kind.Accept)
		ensureBlockReturns(&kind.Reject)
		last.Kind = kind
		return
	case ir.StmtSwitch:
		for i := range kind.Cases {
			if !kind.Cases[i].FallThrough {
				ensureBlockReturns(&kind.Cases[i].Body)
			}
		}
		last.Kind = kind
		return
	case ir.StmtBreak, ir.StmtContinue, ir.StmtReturn, ir.StmtKill:
		return
	}
	*block = append(*block, ir.Statement{Kind: ir.StmtReturn{}})
}
