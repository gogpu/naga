package snapshot_test

import (
	"fmt"
	"math"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// mapRonScalarKind maps RON scalar kind string to ir.ScalarKind.
func mapRonScalarKind(s string) (ir.ScalarKind, error) {
	switch s {
	case "Sint":
		return ir.ScalarSint, nil
	case "Uint":
		return ir.ScalarUint, nil
	case "Float":
		return ir.ScalarFloat, nil
	case "Bool":
		return ir.ScalarBool, nil
	default:
		return 0, fmt.Errorf("unknown scalar kind %q", s)
	}
}

// mapRonVecSize maps RON vector size string to ir.VectorSize.
func mapRonVecSize(s string) (ir.VectorSize, error) {
	switch s {
	case "Bi":
		return ir.Vec2, nil
	case "Tri":
		return ir.Vec3, nil
	case "Quad":
		return ir.Vec4, nil
	default:
		return 0, fmt.Errorf("unknown vector size %q", s)
	}
}

// mapRonAddressSpace maps RON address space string to ir.AddressSpace.
func mapRonAddressSpace(s string) (ir.AddressSpace, error) {
	switch s {
	case "Function":
		return ir.SpaceFunction, nil
	case "Private":
		return ir.SpacePrivate, nil
	case "WorkGroup":
		return ir.SpaceWorkGroup, nil
	case "Uniform":
		return ir.SpaceUniform, nil
	case "Handle":
		return ir.SpaceHandle, nil
	case "PushConstant":
		return ir.SpacePushConstant, nil
	case "TaskPayload":
		return ir.SpaceTaskPayload, nil
	default:
		// Storage has access flags — handled separately
		return 0, fmt.Errorf("unknown address space %q", s)
	}
}

// ronFieldStr gets a string field from RON Fields map.
func ronFieldStr(fields map[string]interface{}, key string) string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ronFieldUint gets a uint32 field from RON Fields map.
func ronFieldUint(fields map[string]interface{}, key string) (uint32, bool) {
	if v, ok := fields[key]; ok {
		switch n := v.(type) {
		case string:
			u, err := strconv.ParseUint(n, 10, 32)
			if err == nil {
				return uint32(u), true
			}
		case float64:
			return uint32(n), true
		case uint32:
			return n, true
		case int:
			return uint32(n), true
		}
	}
	return 0, false
}

// ronOptionalString extracts a string from a RON Some("...") / None generic value.
func ronOptionalString(v interface{}) string {
	if v == nil {
		return ""
	}
	// Direct string
	if s, ok := v.(string); ok {
		return s
	}
	// Some("data") parsed as map{"_tag":"Some", "_inner": map/value}
	if m, ok := v.(map[string]interface{}); ok {
		tag, _ := m["_tag"].(string)
		if tag == "None" {
			return ""
		}
		if tag == "Some" {
			inner := m["_inner"]
			if s, ok := inner.(string); ok {
				return s
			}
			// Positional tuple: []interface{}{"data"}
			if arr, ok := inner.([]interface{}); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					return s
				}
			}
			if im, ok := inner.(map[string]interface{}); ok {
				for _, v := range im {
					if s, ok := v.(string); ok {
						return s
					}
				}
			}
		}
	}
	return ""
}

// ronOptionalUint extracts a uint32 from a RON Some(N) / None generic value.
func ronOptionalUint(v interface{}) *uint32 {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		tag, _ := m["_tag"].(string)
		if tag == "None" {
			return nil
		}
		if tag == "Some" {
			inner := m["_inner"]
			if arr, ok := inner.([]interface{}); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					u, err := strconv.ParseUint(s, 10, 32)
					if err == nil {
						val := uint32(u)
						return &val
					}
				}
			}
			if im, ok := inner.(map[string]interface{}); ok {
				for _, v := range im {
					if s, ok := v.(string); ok {
						u, err := strconv.ParseUint(s, 10, 32)
						if err == nil {
							val := uint32(u)
							return &val
						}
					}
				}
			}
		}
	}
	return nil
}

// mapRonArraySize extracts ir.ArraySize from a RON generic value.
// RON: Dynamic → nil Constant, Constant(N) → &N
func mapRonArraySize(v interface{}) ir.ArraySize {
	if v == nil {
		return ir.ArraySize{}
	}
	// Plain string "Dynamic"
	if s, ok := v.(string); ok && s == "Dynamic" {
		return ir.ArraySize{Constant: nil}
	}
	// Enum variant: map{"_tag":"Constant","_inner":[...]} or map{"_tag":"Dynamic"}
	if m, ok := v.(map[string]interface{}); ok {
		tag, _ := m["_tag"].(string)
		if tag == "Dynamic" {
			return ir.ArraySize{Constant: nil}
		}
		if tag == "Constant" {
			inner := m["_inner"]
			// Inner is []interface{}{N} from positional tuple
			if arr, ok := inner.([]interface{}); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					u, err := strconv.ParseUint(s, 10, 32)
					if err == nil {
						val := uint32(u)
						return ir.ArraySize{Constant: &val}
					}
				}
			}
		}
	}
	return ir.ArraySize{}
}

// mapRonNestedScalar reads a nested scalar object like scalar: (kind: Uint, width: 4)
func mapRonNestedScalar(fields map[string]interface{}, key string) (ir.ScalarType, error) {
	scalarMap, ok := fields[key].(map[string]interface{})
	if !ok {
		return ir.ScalarType{}, fmt.Errorf("field %q is not a map", key)
	}
	kind, err := mapRonScalarKind(ronFieldStr(scalarMap, "kind"))
	if err != nil {
		return ir.ScalarType{}, err
	}
	width, _ := ronFieldUint(scalarMap, "width")
	return ir.ScalarType{Kind: kind, Width: uint8(width)}, nil
}

// mapRonTypeInnerToIR maps a ronTypeInner to an ir.TypeInner interface value.
func mapRonTypeInnerToIR(inner ronTypeInner) (ir.TypeInner, error) {
	f := inner.Fields
	switch inner.Tag {
	case "Scalar":
		kind, err := mapRonScalarKind(ronFieldStr(f, "kind"))
		if err != nil {
			return nil, err
		}
		width, _ := ronFieldUint(f, "width")
		return ir.ScalarType{Kind: kind, Width: uint8(width)}, nil

	case "Vector":
		size, err := mapRonVecSize(ronFieldStr(f, "size"))
		if err != nil {
			return nil, err
		}
		scalar, err := mapRonNestedScalar(f, "scalar")
		if err != nil {
			return nil, err
		}
		return ir.VectorType{Size: size, Scalar: scalar}, nil

	case "Matrix":
		cols, err := mapRonVecSize(ronFieldStr(f, "columns"))
		if err != nil {
			return nil, err
		}
		rows, err := mapRonVecSize(ronFieldStr(f, "rows"))
		if err != nil {
			return nil, err
		}
		scalar, err := mapRonNestedScalar(f, "scalar")
		if err != nil {
			return nil, err
		}
		return ir.MatrixType{Columns: cols, Rows: rows, Scalar: scalar}, nil

	case "Array":
		base, _ := ronFieldUint(f, "base")
		stride, _ := ronFieldUint(f, "stride")
		arrSize := mapRonArraySize(f["size"])
		return ir.ArrayType{
			Base:   ir.TypeHandle(base),
			Size:   arrSize,
			Stride: stride,
		}, nil

	case "Struct":
		span, _ := ronFieldUint(f, "span")
		membersRaw, _ := f["members"].([]interface{})
		var members []ir.StructMember
		for _, mRaw := range membersRaw {
			if mMap, ok := mRaw.(map[string]interface{}); ok {
				name := ronOptionalString(mMap["name"])
				ty, _ := ronFieldUint(mMap, "ty")
				offset, _ := ronFieldUint(mMap, "offset")
				members = append(members, ir.StructMember{
					Name:   name,
					Type:   ir.TypeHandle(ty),
					Offset: offset,
				})
			}
		}
		return ir.StructType{
			Members: members,
			Span:    span,
		}, nil

	case "Pointer":
		base, _ := ronFieldUint(f, "base")
		spaceStr := ronFieldStr(f, "space")
		var space ir.AddressSpace
		if spaceStr == "Storage" {
			space = ir.SpaceStorage
		} else {
			var err error
			space, err = mapRonAddressSpace(spaceStr)
			if err != nil {
				return nil, err
			}
		}
		return ir.PointerType{
			Base:  ir.TypeHandle(base),
			Space: space,
		}, nil

	case "Sampler":
		comparison := ronFieldStr(f, "comparison") == "true"
		return ir.SamplerType{Comparison: comparison}, nil

	case "Atomic":
		kind, err := mapRonScalarKind(ronFieldStr(f, "kind"))
		if err != nil {
			return nil, err
		}
		width, _ := ronFieldUint(f, "width")
		return ir.AtomicType{Scalar: ir.ScalarType{Kind: kind, Width: uint8(width)}}, nil

	case "Image":
		// Complex — return minimal ImageType, deep fields compared by tag only
		return ir.ImageType{}, nil

	case "AccelerationStructure":
		return ir.AccelerationStructureType{}, nil

	case "RayQuery":
		return ir.RayQueryType{}, nil

	case "BindingArray":
		base, _ := ronFieldUint(f, "base")
		size, _ := ronFieldUint(f, "size")
		sizePtr := &size
		return ir.BindingArrayType{
			Base: ir.TypeHandle(base),
			Size: sizePtr,
		}, nil

	default:
		return nil, fmt.Errorf("unknown type tag %q", inner.Tag)
	}
}

// mapRonTypeToIR converts a parsed RON type to our ir.Type.
func mapRonTypeToIR(rt ronType) (ir.Type, error) {
	name := ""
	if rt.Name != nil {
		name = *rt.Name
	}
	inner, err := mapRonTypeInnerToIR(rt.Inner)
	if err != nil {
		return ir.Type{}, fmt.Errorf("type %q: %w", name, err)
	}
	return ir.Type{Name: name, Inner: inner}, nil
}

// mapRonExpressionToIR maps a RON expression to the Go expression tag + key fields.
// Returns: (tag string, fields as comparable map). We compare tag + handle fields.
func mapRonExprTag(e ronExpression) string {
	return e.Tag
}

// goExprTag returns the Rust-equivalent expression tag for a Go expression.
func goExprTag(e ir.Expression) string {
	switch e.Kind.(type) {
	case ir.Literal:
		return "Literal"
	case ir.ExprConstant:
		return "Constant"
	case ir.ExprOverride:
		return "Override"
	case ir.ExprZeroValue:
		return "ZeroValue"
	case ir.ExprCompose:
		return "Compose"
	case ir.ExprAccess:
		return "Access"
	case ir.ExprAccessIndex:
		return "AccessIndex"
	case ir.ExprSplat:
		return "Splat"
	case ir.ExprSwizzle:
		return "Swizzle"
	case ir.ExprFunctionArgument:
		return "FunctionArgument"
	case ir.ExprGlobalVariable:
		return "GlobalVariable"
	case ir.ExprLocalVariable:
		return "LocalVariable"
	case ir.ExprLoad:
		return "Load"
	case ir.ExprImageSample:
		return "ImageSample"
	case ir.ExprImageLoad:
		return "ImageLoad"
	case ir.ExprImageQuery:
		return "ImageQuery"
	case ir.ExprUnary:
		return "Unary"
	case ir.ExprBinary:
		return "Binary"
	case ir.ExprSelect:
		return "Select"
	case ir.ExprDerivative:
		return "Derivative"
	case ir.ExprRelational:
		return "Relational"
	case ir.ExprMath:
		return "Math"
	case ir.ExprAs:
		return "As"
	case ir.ExprCallResult:
		return "CallResult"
	case ir.ExprArrayLength:
		return "ArrayLength"
	case ir.ExprAtomicResult:
		return "AtomicResult"
	case ir.ExprRayQueryProceedResult:
		return "RayQueryProceedResult"
	case ir.ExprRayQueryGetIntersection:
		return "RayQueryGetIntersection"
	case ir.ExprSubgroupBallotResult:
		return "SubgroupBallotResult"
	case ir.ExprSubgroupOperationResult:
		return "SubgroupOperationResult"
	case ir.ExprWorkGroupUniformLoadResult:
		return "WorkGroupUniformLoadResult"
	default:
		return fmt.Sprintf("Unknown(%T)", e.Kind)
	}
}

// deepCompareExpressions compares expression arrays by tag sequence.
func deepCompareExpressions(prefix string, rustExprs []ronExpression, ourExprs []ir.Expression) []string {
	var diffs []string

	if len(rustExprs) != len(ourExprs) {
		diffs = append(diffs, fmt.Sprintf("%s.expressions count: rust=%d go=%d", prefix, len(rustExprs), len(ourExprs)))
	}

	minLen := len(rustExprs)
	if len(ourExprs) < minLen {
		minLen = len(ourExprs)
	}

	for i := 0; i < minLen; i++ {
		rustTag := mapRonExprTag(rustExprs[i])
		goTag := goExprTag(ourExprs[i])
		if rustTag != goTag {
			diffs = append(diffs, fmt.Sprintf("%s.expr[%d]: rust=%s go=%s", prefix, i, rustTag, goTag))
		}
		// TODO: deep field comparison within each expression variant
	}

	return diffs
}

// deepCompareStatements compares statement arrays by tag sequence.
func deepCompareStatements(prefix string, rustStmts []ronStatement, ourStmts []ir.Statement) []string {
	var diffs []string

	// Filter out empty Emit statements from Rust (compaction artifacts)
	var filteredRust []ronStatement
	for _, s := range rustStmts {
		if s.Tag == "Emit" {
			if args, ok := s.Fields["_args"].([]interface{}); ok && len(args) == 1 {
				if m, ok := args[0].(map[string]interface{}); ok {
					start := ronFieldStr(m, "start")
					end := ronFieldStr(m, "end")
					if start == end {
						continue // Empty Emit — skip
					}
				}
			}
		}
		filteredRust = append(filteredRust, s)
	}

	// Filter empty Emits from ours too
	var filteredOurs []ir.Statement
	for _, s := range ourStmts {
		if emit, ok := s.Kind.(ir.StmtEmit); ok {
			if emit.Range.Start == emit.Range.End {
				continue
			}
		}
		filteredOurs = append(filteredOurs, s)
	}

	if len(filteredRust) != len(filteredOurs) {
		diffs = append(diffs, fmt.Sprintf("%s.statements count: rust=%d go=%d", prefix, len(filteredRust), len(filteredOurs)))
	}

	minLen := len(filteredRust)
	if len(filteredOurs) < minLen {
		minLen = len(filteredOurs)
	}

	for i := 0; i < minLen; i++ {
		rustTag := filteredRust[i].Tag
		goTag := goStmtTag(filteredOurs[i])
		if rustTag != goTag {
			diffs = append(diffs, fmt.Sprintf("%s.stmt[%d]: rust=%s go=%s", prefix, i, rustTag, goTag))
		}
	}

	return diffs
}

// goStmtTag returns the Rust-equivalent statement tag.
func goStmtTag(s ir.Statement) string {
	switch s.Kind.(type) {
	case ir.StmtEmit:
		return "Emit"
	case ir.StmtBlock:
		return "Block"
	case ir.StmtIf:
		return "If"
	case ir.StmtSwitch:
		return "Switch"
	case ir.StmtLoop:
		return "Loop"
	case ir.StmtBreak:
		return "Break"
	case ir.StmtContinue:
		return "Continue"
	case ir.StmtReturn:
		return "Return"
	case ir.StmtKill:
		return "Kill"
	case ir.StmtStore:
		return "Store"
	case ir.StmtImageStore:
		return "ImageStore"
	case ir.StmtCall:
		return "Call"
	case ir.StmtAtomic:
		return "Atomic"
	case ir.StmtRayQuery:
		return "RayQuery"
	case ir.StmtSubgroupBallot:
		return "SubgroupBallot"
	case ir.StmtSubgroupGather:
		return "SubgroupGather"
	case ir.StmtSubgroupCollectiveOperation:
		return "SubgroupCollectiveOperation"
	case ir.StmtWorkGroupUniformLoad:
		return "WorkGroupUniformLoad"
	case ir.StmtImageAtomic:
		return "ImageAtomic"
	case ir.StmtBarrier:
		return "ControlBarrier"
	default:
		return fmt.Sprintf("Unknown(%T)", s)
	}
}

// deepCompareTypes compares Rust RON types with our ir.Types.
// Returns list of human-readable diffs. Empty = perfect match.
func deepCompareTypes(rustTypes []ronType, ourTypes []ir.Type) []string {
	var diffs []string

	if len(rustTypes) != len(ourTypes) {
		diffs = append(diffs, fmt.Sprintf("types count: rust=%d go=%d", len(rustTypes), len(ourTypes)))
		// Compare up to min length
	}

	minLen := len(rustTypes)
	if len(ourTypes) < minLen {
		minLen = len(ourTypes)
	}

	for i := 0; i < minLen; i++ {
		rustIR, err := mapRonTypeToIR(rustTypes[i])
		if err != nil {
			diffs = append(diffs, fmt.Sprintf("type[%d]: mapping error: %v", i, err))
			continue
		}
		ourType := ourTypes[i]

		// Compare name
		if rustIR.Name != ourType.Name {
			diffs = append(diffs, fmt.Sprintf("type[%d].Name: rust=%q go=%q", i, rustIR.Name, ourType.Name))
		}

		// Compare inner type tag
		rustTag := typeInnerTag(rustIR)
		ourTag := typeInnerTag(ourType)
		if rustTag != ourTag {
			diffs = append(diffs, fmt.Sprintf("type[%d].Inner: rust=%s go=%s", i, rustTag, ourTag))
			continue // Different types — no point comparing fields
		}

		// Deep compare inner fields
		innerDiffs := deepCompareTypeInner(i, rustIR.Inner, ourType.Inner)
		diffs = append(diffs, innerDiffs...)
	}

	return diffs
}

// deepCompareTypeInner compares the inner fields of two types of the same kind.
func deepCompareTypeInner(idx int, rust, ours ir.TypeInner) []string {
	prefix := fmt.Sprintf("type[%d]", idx)
	var diffs []string

	switch r := rust.(type) {
	case ir.ScalarType:
		o := ours.(ir.ScalarType)
		if r.Kind != o.Kind {
			diffs = append(diffs, fmt.Sprintf("%s.Scalar.Kind: rust=%d go=%d", prefix, r.Kind, o.Kind))
		}
		if r.Width != o.Width {
			diffs = append(diffs, fmt.Sprintf("%s.Scalar.Width: rust=%d go=%d", prefix, r.Width, o.Width))
		}

	case ir.VectorType:
		o := ours.(ir.VectorType)
		if r.Size != o.Size {
			diffs = append(diffs, fmt.Sprintf("%s.Vector.Size: rust=%d go=%d", prefix, r.Size, o.Size))
		}
		if r.Scalar != o.Scalar {
			diffs = append(diffs, fmt.Sprintf("%s.Vector.Scalar: rust=%+v go=%+v", prefix, r.Scalar, o.Scalar))
		}

	case ir.MatrixType:
		o := ours.(ir.MatrixType)
		if r.Columns != o.Columns {
			diffs = append(diffs, fmt.Sprintf("%s.Matrix.Columns: rust=%d go=%d", prefix, r.Columns, o.Columns))
		}
		if r.Rows != o.Rows {
			diffs = append(diffs, fmt.Sprintf("%s.Matrix.Rows: rust=%d go=%d", prefix, r.Rows, o.Rows))
		}
		if r.Scalar != o.Scalar {
			diffs = append(diffs, fmt.Sprintf("%s.Matrix.Scalar: rust=%+v go=%+v", prefix, r.Scalar, o.Scalar))
		}

	case ir.ArrayType:
		o := ours.(ir.ArrayType)
		if r.Base != o.Base {
			diffs = append(diffs, fmt.Sprintf("%s.Array.Base: rust=%d go=%d", prefix, r.Base, o.Base))
		}
		// Compare ArraySize by value, not pointer address
		rDynamic := r.Size.Constant == nil
		oDynamic := o.Size.Constant == nil
		if rDynamic != oDynamic {
			diffs = append(diffs, fmt.Sprintf("%s.Array.Size.Dynamic: rust=%v go=%v", prefix, rDynamic, oDynamic))
		} else if !rDynamic && *r.Size.Constant != *o.Size.Constant {
			diffs = append(diffs, fmt.Sprintf("%s.Array.Size.Constant: rust=%d go=%d", prefix, *r.Size.Constant, *o.Size.Constant))
		}
		if r.Stride != o.Stride {
			diffs = append(diffs, fmt.Sprintf("%s.Array.Stride: rust=%d go=%d", prefix, r.Stride, o.Stride))
		}

	case ir.StructType:
		o := ours.(ir.StructType)
		if r.Span != o.Span {
			diffs = append(diffs, fmt.Sprintf("%s.Struct.Span: rust=%d go=%d", prefix, r.Span, o.Span))
		}
		if len(r.Members) != len(o.Members) {
			diffs = append(diffs, fmt.Sprintf("%s.Struct.Members count: rust=%d go=%d", prefix, len(r.Members), len(o.Members)))
		}
		for j := 0; j < len(r.Members) && j < len(o.Members); j++ {
			rm, om := r.Members[j], o.Members[j]
			mp := fmt.Sprintf("%s.Struct.Members[%d]", prefix, j)
			if rm.Name != om.Name {
				diffs = append(diffs, fmt.Sprintf("%s.Name: rust=%q go=%q", mp, rm.Name, om.Name))
			}
			if rm.Type != om.Type {
				diffs = append(diffs, fmt.Sprintf("%s.Type: rust=%d go=%d", mp, rm.Type, om.Type))
			}
			if rm.Offset != om.Offset {
				diffs = append(diffs, fmt.Sprintf("%s.Offset: rust=%d go=%d", mp, rm.Offset, om.Offset))
			}
		}

	case ir.PointerType:
		o := ours.(ir.PointerType)
		if r.Base != o.Base {
			diffs = append(diffs, fmt.Sprintf("%s.Pointer.Base: rust=%d go=%d", prefix, r.Base, o.Base))
		}
		if r.Space != o.Space {
			diffs = append(diffs, fmt.Sprintf("%s.Pointer.Space: rust=%d go=%d", prefix, r.Space, o.Space))
		}

	case ir.AtomicType:
		o := ours.(ir.AtomicType)
		if r.Scalar != o.Scalar {
			diffs = append(diffs, fmt.Sprintf("%s.Atomic.Scalar: rust=%+v go=%+v", prefix, r.Scalar, o.Scalar))
		}

	case ir.SamplerType:
		o := ours.(ir.SamplerType)
		if r.Comparison != o.Comparison {
			diffs = append(diffs, fmt.Sprintf("%s.Sampler.Comparison: rust=%v go=%v", prefix, r.Comparison, o.Comparison))
		}

	case ir.BindingArrayType:
		o := ours.(ir.BindingArrayType)
		if r.Base != o.Base {
			diffs = append(diffs, fmt.Sprintf("%s.BindingArray.Base: rust=%d go=%d", prefix, r.Base, o.Base))
		}

		// Image, AccelerationStructure, RayQuery — tag match is enough for now
	}

	return diffs
}

// --- Field-level expression comparison ---

// parseRonLiteral converts RON literal map{"_tag":"U32","_inner":["1"]} to "U32(1)"
func parseRonLiteral(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	tag, _ := m["_tag"].(string)
	inner := m["_inner"]
	if arr, ok := inner.([]interface{}); ok && len(arr) > 0 {
		val := fmt.Sprintf("%v", arr[0])

		// F16 in RON is stored as raw u16 bits in a tuple: F16((15360))
		// The parser may produce a nested array or map for the tuple.
		// Extract the raw bits and convert to float for comparison.
		if tag == "F16" {
			rawBits := extractF16RawBits(arr[0])
			if rawBits >= 0 {
				f := f16BitsToFloat32(uint16(rawBits))
				return "F16(" + formatFloat32(f) + ")"
			}
		}

		return fmt.Sprintf("%s(%s)", tag, val)
	}
	return fmt.Sprintf("%s(?)", tag)
}

// extractF16RawBits extracts u16 raw bits from RON F16 tuple representation.
// The inner can be: (15360) parsed as array [15360], or string "15360", etc.
func extractF16RawBits(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		n, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return n
		}
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case []interface{}:
		if len(val) > 0 {
			return extractF16RawBits(val[0])
		}
	case map[string]interface{}:
		// Tuple parsed as map with _inner
		if inner, ok := val["_inner"]; ok {
			return extractF16RawBits(inner)
		}
		// Or positional _args
		if args, ok := val["_args"]; ok {
			return extractF16RawBits(args)
		}
	}
	return -1
}

// f16BitsToFloat32 converts raw f16 (IEEE 754 half) bits to float32.
func f16BitsToFloat32(bits uint16) float32 {
	sign := uint32((bits >> 15) & 0x1)
	exp := uint32((bits >> 10) & 0x1f)
	mantissa := uint32(bits & 0x3ff)

	var f32bits uint32
	if exp == 0 {
		if mantissa == 0 {
			f32bits = sign << 31
		} else {
			exp = 127 - 15 + 1
			for mantissa&0x400 == 0 {
				mantissa <<= 1
				exp--
			}
			mantissa &= 0x3ff
			f32bits = (sign << 31) | (exp << 23) | (mantissa << 13)
		}
	} else if exp == 0x1f {
		f32bits = (sign << 31) | (0xff << 23) | (mantissa << 13)
	} else {
		f32bits = (sign << 31) | ((exp + 127 - 15) << 23) | (mantissa << 13)
	}

	return math.Float32frombits(f32bits)
}

// formatFloat32 formats a float32 to match Rust's Display for f32.
// Uses shortest representation, ensuring at least one decimal place.
func formatFloat32(f float32) string {
	s := strconv.FormatFloat(float64(f), 'f', -1, 32)
	// Ensure at least one decimal place for consistency with Rust
	if !containsDot(s) {
		s += ".0"
	}
	return s
}

// formatFloat64 formats a float64 to match Rust's Display for f64.
func formatFloat64(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !containsDot(s) {
		s += ".0"
	}
	return s
}

func containsDot(s string) bool {
	for _, c := range s {
		if c == '.' {
			return true
		}
	}
	return false
}

func formatLiteral(v ir.LiteralValue) string {
	switch l := v.(type) {
	case ir.LiteralU32:
		return fmt.Sprintf("U32(%d)", uint32(l))
	case ir.LiteralI32:
		return fmt.Sprintf("I32(%d)", int32(l))
	case ir.LiteralF32:
		return "F32(" + formatFloat32(float32(l)) + ")"
	case ir.LiteralF64:
		return "F64(" + formatFloat64(float64(l)) + ")"
	case ir.LiteralBool:
		if bool(l) {
			return "Bool(true)"
		}
		return "Bool(false)"
	case ir.LiteralU64:
		return fmt.Sprintf("U64(%d)", uint64(l))
	case ir.LiteralI64:
		return fmt.Sprintf("I64(%d)", int64(l))
	case ir.LiteralF16:
		return "F16(" + formatFloat32(float32(l)) + ")"
	case ir.LiteralAbstractInt:
		return fmt.Sprintf("AbstractInt(%d)", int64(l))
	case ir.LiteralAbstractFloat:
		return "AbstractFloat(" + formatFloat64(float64(l)) + ")"
	default:
		return fmt.Sprintf("?(%v)", v)
	}
}

func binaryOpName(op ir.BinaryOperator) string {
	names := map[ir.BinaryOperator]string{
		ir.BinaryAdd: "Add", ir.BinarySubtract: "Subtract",
		ir.BinaryMultiply: "Multiply", ir.BinaryDivide: "Divide",
		ir.BinaryModulo: "Modulo",
		ir.BinaryEqual:  "Equal", ir.BinaryNotEqual: "NotEqual",
		ir.BinaryLess: "Less", ir.BinaryLessEqual: "LessEqual",
		ir.BinaryGreater: "Greater", ir.BinaryGreaterEqual: "GreaterEqual",
		ir.BinaryAnd: "And", ir.BinaryInclusiveOr: "InclusiveOr",
		ir.BinaryExclusiveOr: "ExclusiveOr",
		ir.BinaryLogicalAnd:  "LogicalAnd", ir.BinaryLogicalOr: "LogicalOr",
		ir.BinaryShiftLeft: "ShiftLeft", ir.BinaryShiftRight: "ShiftRight",
	}
	if n, ok := names[op]; ok {
		return n
	}
	return fmt.Sprintf("Op(%d)", op)
}

// deepCompareExpressionFields compares expression fields (not just tags).
// Returns diffs for handle/value mismatches within same-tagged expressions.
func deepCompareExpressionFields(prefix string, rustExprs []ronExpression, ourExprs []ir.Expression) []string {
	var diffs []string

	minLen := len(rustExprs)
	if len(ourExprs) < minLen {
		minLen = len(ourExprs)
	}

	for i := 0; i < minLen; i++ {
		re := rustExprs[i]
		oe := ourExprs[i]
		goTag := goExprTag(oe)
		if re.Tag != goTag {
			continue // Tag diff already reported by deepCompareExpressions
		}

		ep := fmt.Sprintf("%s.expr[%d]", prefix, i)
		f := re.Fields

		switch k := oe.Kind.(type) {
		case ir.ExprFunctionArgument:
			if args, ok := f["_args"].([]interface{}); ok && len(args) > 0 {
				if idx, ok := args[0].(string); ok && fmt.Sprintf("%d", k.Index) != idx {
					diffs = append(diffs, fmt.Sprintf("%s FunctionArgument: rust=%s go=%d", ep, idx, k.Index))
				}
			}
		case ir.ExprLocalVariable:
			if args, ok := f["_args"].([]interface{}); ok && len(args) > 0 {
				if idx, ok := args[0].(string); ok && fmt.Sprintf("%d", k.Variable) != idx {
					diffs = append(diffs, fmt.Sprintf("%s LocalVariable: rust=%s go=%d", ep, idx, k.Variable))
				}
			}
		case ir.ExprGlobalVariable:
			if args, ok := f["_args"].([]interface{}); ok && len(args) > 0 {
				if idx, ok := args[0].(string); ok && fmt.Sprintf("%d", k.Variable) != idx {
					diffs = append(diffs, fmt.Sprintf("%s GlobalVariable: rust=%s go=%d", ep, idx, k.Variable))
				}
			}
		case ir.Literal:
			if args, ok := f["_args"].([]interface{}); ok && len(args) > 0 {
				rustLit := parseRonLiteral(args[0])
				goLit := formatLiteral(k.Value)
				if rustLit != goLit {
					diffs = append(diffs, fmt.Sprintf("%s Literal: rust=%s go=%s", ep, rustLit, goLit))
				}
			}
		case ir.ExprBinary:
			if op, ok := f["op"].(string); ok {
				goOp := binaryOpName(k.Op)
				if op != goOp {
					diffs = append(diffs, fmt.Sprintf("%s Binary.Op: rust=%s go=%s", ep, op, goOp))
				}
			}
			if left, ok := ronFieldUint(f, "left"); ok && left != uint32(k.Left) {
				diffs = append(diffs, fmt.Sprintf("%s Binary.Left: rust=%d go=%d", ep, left, k.Left))
			}
			if right, ok := ronFieldUint(f, "right"); ok && right != uint32(k.Right) {
				diffs = append(diffs, fmt.Sprintf("%s Binary.Right: rust=%d go=%d", ep, right, k.Right))
			}
		case ir.ExprLoad:
			if ptr, ok := ronFieldUint(f, "pointer"); ok && ptr != uint32(k.Pointer) {
				diffs = append(diffs, fmt.Sprintf("%s Load.Pointer: rust=%d go=%d", ep, ptr, k.Pointer))
			}
		case ir.ExprAccessIndex:
			if base, ok := ronFieldUint(f, "base"); ok && base != uint32(k.Base) {
				diffs = append(diffs, fmt.Sprintf("%s AccessIndex.Base: rust=%d go=%d", ep, base, k.Base))
			}
			if idx, ok := ronFieldUint(f, "index"); ok && idx != k.Index {
				diffs = append(diffs, fmt.Sprintf("%s AccessIndex.Index: rust=%d go=%d", ep, idx, k.Index))
			}
		case ir.ExprAccess:
			if base, ok := ronFieldUint(f, "base"); ok && base != uint32(k.Base) {
				diffs = append(diffs, fmt.Sprintf("%s Access.Base: rust=%d go=%d", ep, base, k.Base))
			}
			if idx, ok := ronFieldUint(f, "index"); ok && idx != uint32(k.Index) {
				diffs = append(diffs, fmt.Sprintf("%s Access.Index: rust=%d go=%d", ep, idx, k.Index))
			}
		case ir.ExprCompose:
			if ty, ok := ronFieldUint(f, "ty"); ok && ty != uint32(k.Type) {
				diffs = append(diffs, fmt.Sprintf("%s Compose.Type: rust=%d go=%d", ep, ty, k.Type))
			}
		case ir.ExprSplat:
			if val, ok := ronFieldUint(f, "value"); ok && val != uint32(k.Value) {
				diffs = append(diffs, fmt.Sprintf("%s Splat.Value: rust=%d go=%d", ep, val, k.Value))
			}
		case ir.ExprAs:
			if expr, ok := ronFieldUint(f, "expr"); ok && expr != uint32(k.Expr) {
				diffs = append(diffs, fmt.Sprintf("%s As.Expr: rust=%d go=%d", ep, expr, k.Expr))
			}
		case ir.ExprCallResult:
			if fn, ok := ronFieldUint(f, "function"); ok && fn != uint32(k.Function) {
				diffs = append(diffs, fmt.Sprintf("%s CallResult.Function: rust=%d go=%d", ep, fn, k.Function))
			}
		case ir.ExprZeroValue:
			if ty, ok := ronFieldUint(f, "ty"); ok && ty != uint32(k.Type) {
				diffs = append(diffs, fmt.Sprintf("%s ZeroValue.Type: rust=%d go=%d", ep, ty, k.Type))
			}
		}
	}

	return diffs
}
