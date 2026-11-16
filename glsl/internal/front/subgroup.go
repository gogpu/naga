package front

// This file implements GLSL's subgroup builtins: the GLSL 4.60 §8.18 functions
// and the GL_KHR_shader_subgroup_* extensions. Rust naga's GLSL frontend has no
// subgroups at all (upstream #7571), so glslangValidator is the reference for
// the GLSL surface and the SPIR-V shape, and the IR comes from the WGSL and
// SPIR-V paths (ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation and
// ir.StmtSubgroupGather).
//
// What the IR expresses, the frontend lowers:
//
//   - the basic family's subgroupBarrier and subgroupMemoryBarrier* become
//     ir.StmtControlBarrier / ir.StmtMemoryBarrier at subgroup scope;
//   - subgroupAll and subgroupAny become a collective operation with
//     ir.SubgroupOperationAll / Any and the vote extension;
//   - subgroupBallot becomes ir.StmtSubgroupBallot, and the broadcast,
//     shuffle and quad families become ir.StmtSubgroupGather;
//   - the arithmetic reductions and the Add/Mul scans become collective
//     operations.
//
// What the IR (or the text backends) cannot express is refused with a message
// naming the missing capability, never lowered to a different operation:
// subgroupElect has no IR statement, subgroupAllEqual has no IR operation, the
// ballot bit and find operations have no IR expression, the clustered forms
// have no ir.CollectiveOperation, and the Min/Max/And/Or/Xor scans have no
// Metal or HLSL spelling.
//
// Every builtin is available only where glslang has it: the source must enable
// the family's extension (GL_KHR_shader_subgroup_basic is implied by each of
// the others, so any of them enables the basic family), and GLSL ES has no
// subgroup operations before 3.10. The GL 3.30 core dialect gets them through
// the same extensions, as glslang accepts them there (measured with
// glslangValidator 11:16.2.0: `#version 330` plus
// `#extension GL_KHR_shader_subgroup_arithmetic : require` compiles
// subgroupAdd).

import (
	"github.com/gogpu/naga/ir"
)

// subgroupFamily is the GL_KHR_shader_subgroup_* extension a builtin belongs
// to. The extension decides where the builtin is available (subgroupGate).
type subgroupFamily uint8

const (
	subgroupFamilyBasic subgroupFamily = iota
	subgroupFamilyVote
	subgroupFamilyBallot
	subgroupFamilyShuffle
	subgroupFamilyShuffleRelative
	subgroupFamilyQuad
	subgroupFamilyArithmetic
	subgroupFamilyClustered
)

// The extension names of the family, exactly as GLSL spells them.
const (
	subgroupExtBasic           = "GL_KHR_shader_subgroup_basic"
	subgroupExtVote            = "GL_KHR_shader_subgroup_vote"
	subgroupExtBallot          = "GL_KHR_shader_subgroup_ballot"
	subgroupExtShuffle         = "GL_KHR_shader_subgroup_shuffle"
	subgroupExtShuffleRelative = "GL_KHR_shader_subgroup_shuffle_relative"
	subgroupExtQuad            = "GL_KHR_shader_subgroup_quad"
	subgroupExtArithmetic      = "GL_KHR_shader_subgroup_arithmetic"
	subgroupExtClustered       = "GL_KHR_shader_subgroup_clustered"
)

// extension is the GL_KHR_shader_subgroup_* name the family's builtins need.
func (f subgroupFamily) extension() string {
	switch f {
	case subgroupFamilyVote:
		return subgroupExtVote
	case subgroupFamilyBallot:
		return subgroupExtBallot
	case subgroupFamilyShuffle:
		return subgroupExtShuffle
	case subgroupFamilyShuffleRelative:
		return subgroupExtShuffleRelative
	case subgroupFamilyQuad:
		return subgroupExtQuad
	case subgroupFamilyArithmetic:
		return subgroupExtArithmetic
	case subgroupFamilyClustered:
		return subgroupExtClustered
	default:
		return subgroupExtBasic
	}
}

// subgroupForm is the IR shape a builtin lowers to.
type subgroupForm uint8

const (
	// subgroupFormBarrier is subgroupBarrier: ir.StmtControlBarrier at subgroup
	// scope.
	subgroupFormBarrier subgroupForm = iota
	// subgroupFormMemoryBarrier is the subgroupMemoryBarrier family:
	// ir.StmtMemoryBarrier at subgroup scope.
	subgroupFormMemoryBarrier
	// subgroupFormCollective is the vote and arithmetic families:
	// ir.StmtSubgroupCollectiveOperation.
	subgroupFormCollective
	// subgroupFormBallot is subgroupBallot: ir.StmtSubgroupBallot.
	subgroupFormBallot
	// subgroupFormGather is the broadcast, shuffle and quad families:
	// ir.StmtSubgroupGather.
	subgroupFormGather
)

// subgroupGatherForm tells which ir.GatherMode a gather builtin builds.
type subgroupGatherForm uint8

const (
	subgroupGatherBroadcast subgroupGatherForm = iota
	subgroupGatherBroadcastFirst
	subgroupGatherShuffle
	subgroupGatherShuffleXor
	subgroupGatherShuffleUp
	subgroupGatherShuffleDown
	subgroupGatherQuadBroadcast
	subgroupGatherQuadSwap
)

// subgroupBuiltin is one GLSL subgroup builtin: the extension family, the IR
// shape, and for a form the frontend cannot lower the reason in gap.
type subgroupBuiltin struct {
	family subgroupFamily
	form   subgroupForm
	// arity is the number of arguments the builtin's signature takes.
	arity int
	// op and cop are the collective operation of subgroupFormCollective.
	op  ir.SubgroupOperation
	cop ir.CollectiveOperation
	// gather and direction are the gather mode of subgroupFormGather.
	gather    subgroupGatherForm
	direction ir.QuadDirection
	// gap is the refusal message body of a recognized form the frontend does
	// not implement. It is empty for every implemented form.
	gap string
}

// subgroupBuiltinFunction returns what the GLSL subgroup builtin name denotes.
// The switch is the catalog of the whole surface the frontend recognizes:
// GLSL 4.60 §8.18 and GL_KHR_shader_subgroup. An implemented form carries its
// IR mapping; a recognized form the IR or a text backend cannot express
// carries the reason in gap. The snapshot matrix's coverage test reads this
// switch out of the source (glsl_subgroup_matrix_test.go), so a name added here
// fails the test until a row drives it; a name left out falls through to the
// generic unknown-function path.
func subgroupBuiltinFunction(name string) (subgroupBuiltin, bool) {
	switch name {
	// --- GL_KHR_shader_subgroup_basic ---------------------------------------

	case "subgroupBarrier":
		return subgroupBuiltin{family: subgroupFamilyBasic, form: subgroupFormBarrier}, true
	case "subgroupMemoryBarrier", "subgroupMemoryBarrierBuffer",
		"subgroupMemoryBarrierShared", "subgroupMemoryBarrierImage":
		// The IR's subgroup memory barrier names the union of the memory
		// classes at subgroup scope: ir.BarrierFlags' SubGroup bit makes the
		// SPIR-V writer emit AcquireRelease | UniformMemory | WorkgroupMemory |
		// ImageMemory, as glslang compiles subgroupBarrier and
		// subgroupMemoryBarrier. The single class each suffixed form names is
		// therefore already ordered, which is stronger than the class alone and
		// never weaker; the barrel's scope is the part the IR models.
		return subgroupBuiltin{family: subgroupFamilyBasic, form: subgroupFormMemoryBarrier}, true
	case "subgroupElect":
		return subgroupBuiltin{family: subgroupFamilyBasic, gap: subgroupElectGapText}, true
	case "subgroupMemoryBarrierAtomicCounter":
		return subgroupBuiltin{family: subgroupFamilyBasic, gap: subgroupAtomicCounterGapText}, true

	// --- GL_KHR_shader_subgroup_vote ----------------------------------------

	case "subgroupAll":
		return subgroupBuiltin{family: subgroupFamilyVote, form: subgroupFormCollective, arity: 1,
			op: ir.SubgroupOperationAll, cop: ir.CollectiveReduce}, true
	case "subgroupAny":
		return subgroupBuiltin{family: subgroupFamilyVote, form: subgroupFormCollective, arity: 1,
			op: ir.SubgroupOperationAny, cop: ir.CollectiveReduce}, true
	case "subgroupAllEqual":
		return subgroupBuiltin{family: subgroupFamilyVote, gap: subgroupAllEqualGapText}, true

	// --- GL_KHR_shader_subgroup_ballot --------------------------------------

	case "subgroupBallot":
		return subgroupBuiltin{family: subgroupFamilyBallot, form: subgroupFormBallot, arity: 1}, true
	case "subgroupBroadcast":
		return subgroupBuiltin{family: subgroupFamilyBallot, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherBroadcast}, true
	case "subgroupBroadcastFirst":
		return subgroupBuiltin{family: subgroupFamilyBallot, form: subgroupFormGather, arity: 1,
			gather: subgroupGatherBroadcastFirst}, true
	case "subgroupBallotBitExtract", "subgroupBallotBitCount",
		"subgroupBallotInclusiveBitCount", "subgroupBallotExclusiveBitCount",
		"subgroupBallotFindLSB", "subgroupBallotFindMSB":
		return subgroupBuiltin{family: subgroupFamilyBallot, gap: subgroupBallotBitsGapText}, true

	// --- GL_KHR_shader_subgroup_shuffle and _shuffle_relative ----------------

	case "subgroupShuffle":
		return subgroupBuiltin{family: subgroupFamilyShuffle, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherShuffle}, true
	case "subgroupShuffleXor":
		return subgroupBuiltin{family: subgroupFamilyShuffle, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherShuffleXor}, true
	case "subgroupShuffleUp":
		return subgroupBuiltin{family: subgroupFamilyShuffleRelative, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherShuffleUp}, true
	case "subgroupShuffleDown":
		return subgroupBuiltin{family: subgroupFamilyShuffleRelative, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherShuffleDown}, true

	// --- GL_KHR_shader_subgroup_quad ----------------------------------------

	case "subgroupQuadBroadcast":
		return subgroupBuiltin{family: subgroupFamilyQuad, form: subgroupFormGather, arity: 2,
			gather: subgroupGatherQuadBroadcast}, true
	case "subgroupQuadSwapHorizontal":
		return subgroupBuiltin{family: subgroupFamilyQuad, form: subgroupFormGather, arity: 1,
			gather: subgroupGatherQuadSwap, direction: ir.QuadDirectionX}, true
	case "subgroupQuadSwapVertical":
		return subgroupBuiltin{family: subgroupFamilyQuad, form: subgroupFormGather, arity: 1,
			gather: subgroupGatherQuadSwap, direction: ir.QuadDirectionY}, true
	case "subgroupQuadSwapDiagonal":
		return subgroupBuiltin{family: subgroupFamilyQuad, form: subgroupFormGather, arity: 1,
			gather: subgroupGatherQuadSwap, direction: ir.QuadDirectionDiagonal}, true

	// --- GL_KHR_shader_subgroup_arithmetic ----------------------------------

	case "subgroupAdd":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationAdd, ir.CollectiveReduce), true
	case "subgroupMul":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationMul, ir.CollectiveReduce), true
	case "subgroupMin":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationMin, ir.CollectiveReduce), true
	case "subgroupMax":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationMax, ir.CollectiveReduce), true
	case "subgroupAnd":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationAnd, ir.CollectiveReduce), true
	case "subgroupOr":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationOr, ir.CollectiveReduce), true
	case "subgroupXor":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationXor, ir.CollectiveReduce), true
	case "subgroupInclusiveAdd":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan), true
	case "subgroupExclusiveAdd":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationAdd, ir.CollectiveExclusiveScan), true
	case "subgroupInclusiveMul":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationMul, ir.CollectiveInclusiveScan), true
	case "subgroupExclusiveMul":
		return subgroupCollective(subgroupFamilyArithmetic, ir.SubgroupOperationMul, ir.CollectiveExclusiveScan), true
	case "subgroupInclusiveMin", "subgroupInclusiveMax", "subgroupInclusiveAnd",
		"subgroupInclusiveOr", "subgroupInclusiveXor",
		"subgroupExclusiveMin", "subgroupExclusiveMax", "subgroupExclusiveAnd",
		"subgroupExclusiveOr", "subgroupExclusiveXor":
		return subgroupBuiltin{family: subgroupFamilyArithmetic, gap: subgroupScanGapText}, true

	// --- GL_KHR_shader_subgroup_clustered -----------------------------------

	case "subgroupClusteredAdd", "subgroupClusteredMul", "subgroupClusteredMin",
		"subgroupClusteredMax", "subgroupClusteredAnd", "subgroupClusteredOr",
		"subgroupClusteredXor":
		return subgroupBuiltin{family: subgroupFamilyClustered, gap: subgroupClusteredGapText}, true
	}
	return subgroupBuiltin{}, false
}

// subgroupCollective builds the catalog entry of a collective operation
// (a reduction or scan of one argument).
func subgroupCollective(family subgroupFamily, op ir.SubgroupOperation, cop ir.CollectiveOperation) subgroupBuiltin {
	return subgroupBuiltin{family: family, form: subgroupFormCollective, arity: 1, op: op, cop: cop}
}

// The refusal bodies of the forms the frontend recognizes but cannot lower.
// Each names the missing capability, as the atomic path's refusals do.
const (
	// subgroupElectGapText: the IR has no elect statement. glslang emits
	// OpGroupNonUniformElect, which no ir.Statement or ir.ExpressionKind
	// names, so the frontend cannot represent the result.
	subgroupElectGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"the IR has no subgroup elect statement (ir has SubgroupBallot, SubgroupCollectiveOperation and " +
		"SubgroupGather, and OpGroupNonUniformElect has no counterpart) (card P1-GLSLIN-017)"

	// subgroupAllEqualGapText: ir.SubgroupOperation has no AllEqual, and
	// glslang emits OpGroupNonUniformAllEqual for the form.
	subgroupAllEqualGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"ir.SubgroupOperation has All and Any but no AllEqual, and glslang emits OpGroupNonUniformAllEqual " +
		"for it (card P1-GLSLIN-017)"

	// subgroupBallotBitsGapText: the IR builds the ballot mask but has no bit
	// extract, bit count or find operation. glslang emits
	// OpGroupNonUniformBallotBitExtract, OpGroupNonUniformBallotBitCount and
	// OpGroupNonUniformBallotFindLSB / FindMSB for them.
	subgroupBallotBitsGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"ir has StmtSubgroupBallot for the mask but no ballot bit extract, bit count or find LSB/MSB " +
		"statement or expression (card P1-GLSLIN-017)"

	// subgroupClusteredGapText: ir.CollectiveOperation has no ClusteredReduce.
	// glslang accepts the clustered forms and emits
	// OpGroupNonUniformIAdd ... ClusteredReduce with the cluster size operand.
	subgroupClusteredGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"ir.CollectiveOperation has Reduce, InclusiveScan and ExclusiveScan but no ClusteredReduce and no " +
		"cluster-size operand, so the clustered forms cannot be represented (card P1-GLSLIN-017)"

	// subgroupScanGapText: the Min/Max/And/Or/Xor scans are expressible in the
	// IR, but no text backend spells them: Metal's simd functions have no
	// prefix form for them (the MSL writer refuses the scan rather than
	// emitting its reduction, which would silently change the result), and the
	// HLSL writer refuses every scan other than the Add and Mul ones. Refusing
	// the form is the honest surface until the backends write the scan.
	subgroupScanGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"the Min/Max/And/Or/Xor scans are expressible in the IR, but Metal's simd functions have no prefix " +
		"form for them and the HLSL writer refuses every scan other than the Add and Mul ones, so no text " +
		"backend writes the scan (card P1-GLSLIN-017)"

	// subgroupAtomicCounterGapText: no atomic counters in Vulkan GLSL and no
	// atomic-counter memory class in ir.BarrierFlags.
	subgroupAtomicCounterGapText = "subgroup operations are not implemented by the GLSL frontend: " +
		"atomic counters do not exist in Vulkan GLSL (glslang has no subgroupMemoryBarrierAtomicCounter " +
		"overload), and ir.BarrierFlags has no atomic-counter memory class (card P1-GLSLIN-017)"
)

// subgroupExtensions records which GL_KHR_shader_subgroup_* families the
// source's #extension directives enable.
type subgroupExtensions struct {
	basic, vote, ballot, shuffle, shuffleRelative, quad, arithmetic, clustered bool
}

// subgroupExtensionsFrom reads the #extension directives into the family set.
// Every one of the seven family extensions requires
// GL_KHR_shader_subgroup_basic (GL_KHR_shader_subgroup_vote: "This extension
// requires GL_KHR_shader_subgroup_basic"), so enabling any of them enables the
// basic family's builtins, as glslang resolves them (measured: subgroupBarrier
// compiles with only GL_KHR_shader_subgroup_arithmetic requested).
func subgroupExtensionsFrom(exts []ppExtension) subgroupExtensions {
	s := subgroupExtensions{
		basic:           extensionEnabled(exts, subgroupExtBasic),
		vote:            extensionEnabled(exts, subgroupExtVote),
		ballot:          extensionEnabled(exts, subgroupExtBallot),
		shuffle:         extensionEnabled(exts, subgroupExtShuffle),
		shuffleRelative: extensionEnabled(exts, subgroupExtShuffleRelative),
		quad:            extensionEnabled(exts, subgroupExtQuad),
		arithmetic:      extensionEnabled(exts, subgroupExtArithmetic),
		clustered:       extensionEnabled(exts, subgroupExtClustered),
	}
	s.basic = s.basic || s.vote || s.ballot || s.shuffle || s.shuffleRelative ||
		s.quad || s.arithmetic || s.clustered
	return s
}

// enabled reports whether the family's builtins are available.
func (s subgroupExtensions) enabled(f subgroupFamily) bool {
	switch f {
	case subgroupFamilyVote:
		return s.vote
	case subgroupFamilyBallot:
		return s.ballot
	case subgroupFamilyShuffle:
		return s.shuffle
	case subgroupFamilyShuffleRelative:
		return s.shuffleRelative
	case subgroupFamilyQuad:
		return s.quad
	case subgroupFamilyArithmetic:
		return s.arithmetic
	case subgroupFamilyClustered:
		return s.clustered
	default:
		return s.basic
	}
}

// subgroupGate returns the version or extension rule a use of the builtin
// violates, or nil. The messages are glslang's: "'subgroupAdd' : required
// extension not requested: GL_KHR_shader_subgroup_arithmetic" when the family
// is not enabled (the Vulkan GLSL dialects require the directive too, even in
// #version 460), and for GLSL ES 3.00 the version rule of the dialect.
func (f *Frontend) subgroupGate(name string, family subgroupFamily, span ir.Span) *frontError {
	if f.dialect.es() && f.dialect.version < 310 {
		return errMessage(span, f.dialect.unsupported(name, " (subgroup operations require GLSL ES 3.10)"))
	}
	if !f.subgroups.enabled(family) {
		return errMessage(span, "'"+name+"' : required extension not requested: "+family.extension())
	}
	return nil
}

// subgroupDenial returns the refusal of a recognized subgroup builtin, or nil
// when it can be lowered. The dialect and extension rule runs first, as
// glslang resolves availability before the form's own limits, so a source that
// does not enable the extension sees the extension rule rather than a backend
// limitation it has not reached.
func (f *Frontend) subgroupDenial(name string, b subgroupBuiltin, span ir.Span) *frontError {
	if err := f.subgroupGate(name, b.family, span); err != nil {
		return err
	}
	if b.gap != "" {
		return errMessage(span, "'"+name+"' : "+b.gap)
	}
	return nil
}

// subgroupCall lowers a call of a subgroup builtin. The dialect and extension
// gate runs first, as the atomic path checks its own gate before any operand,
// so a GLSL ES 3.00 source or one without the extension meets the availability
// rule rather than a type complaint.
func (f *Frontend) subgroupCall(c *Context, e *hirArena, name string, b subgroupBuiltin, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	if err := f.subgroupDenial(name, b, span); err != nil {
		return 0, false, err
	}
	if len(rawArgs) != b.arity {
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}

	switch b.form {
	case subgroupFormBarrier:
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtControlBarrier{Flags: ir.BarrierSubGroup}})
		return 0, false, nil
	case subgroupFormMemoryBarrier:
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtMemoryBarrier{Flags: ir.BarrierSubGroup}})
		return 0, false, nil
	case subgroupFormBallot:
		return f.subgroupBallotCall(c, e, rawArgs[0], span)
	case subgroupFormCollective:
		return f.subgroupCollectiveCall(c, e, name, b, rawArgs[0], span)
	default:
		return f.subgroupGatherCall(c, e, name, b, rawArgs, span)
	}
}

// subgroupBallotCall lowers subgroupBallot(predicate) to ir.StmtSubgroupBallot.
// The predicate is a bool value; the result is the vec4<u32> mask the IR types
// ir.ExprSubgroupBallotResult as.
func (f *Frontend) subgroupBallotCall(c *Context, e *hirArena, rawPredicate hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	predicate, _, err := c.lowerExpect(e, rawPredicate, rhsPos)
	if err != nil {
		return 0, false, err
	}
	if terr := c.typifierGrow(span); terr != nil {
		return 0, false, terr
	}
	if !f.subgroupBoolScalar(c, predicate) {
		return 0, false, errSemantic(span, "Unknown function '%s'", "subgroupBallot")
	}

	c.emitEnd()
	result, rerr := c.addExpression(ir.ExprSubgroupBallotResult{}, span)
	if rerr != nil {
		return 0, false, rerr
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtSubgroupBallot{
		Result:    result,
		Predicate: exprPtr(predicate),
	}})
	c.emitStart()
	return result, true, nil
}

// subgroupCollectiveCall lowers a vote or arithmetic collective operation. The
// result type is the argument's own type, as the IR's
// ir.ExprSubgroupOperationResult carries it.
func (f *Frontend) subgroupCollectiveCall(c *Context, e *hirArena, name string, b subgroupBuiltin, rawArgument hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	argument, argumentSpan, err := c.lowerExpect(e, rawArgument, rhsPos)
	if err != nil {
		return 0, false, err
	}
	if terr := c.typifierGrow(span); terr != nil {
		return 0, false, terr
	}
	switch b.op {
	case ir.SubgroupOperationAll, ir.SubgroupOperationAny:
		if !f.subgroupBoolScalar(c, argument) {
			return 0, false, errSemantic(span, "Unknown function '%s'", name)
		}
	default:
		if !f.subgroupNumericValue(c, argument) {
			return 0, false, errSemantic(span, "Unknown function '%s'", name)
		}
	}

	resultType, err := c.resolveTypeHandle(argument, argumentSpan)
	if err != nil {
		return 0, false, err
	}

	c.emitEnd()
	result, rerr := c.addExpression(ir.ExprSubgroupOperationResult{Type: resultType}, span)
	if rerr != nil {
		return 0, false, rerr
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtSubgroupCollectiveOperation{
		Op:           b.op,
		CollectiveOp: b.cop,
		Argument:     argument,
		Result:       result,
	}})
	c.emitStart()
	return result, true, nil
}

// subgroupGatherCall lowers the broadcast, shuffle and quad families to
// ir.StmtSubgroupGather. The operand order is (value) or (value, index).
func (f *Frontend) subgroupGatherCall(c *Context, e *hirArena, name string, b subgroupBuiltin, rawArgs []hirHandle, span ir.Span) (ir.ExpressionHandle, bool, *frontError) {
	argument, argumentSpan, err := c.lowerExpect(e, rawArgs[0], rhsPos)
	if err != nil {
		return 0, false, err
	}
	if terr := c.typifierGrow(span); terr != nil {
		return 0, false, terr
	}
	if !f.subgroupGatherValue(c, argument) {
		return 0, false, errSemantic(span, "Unknown function '%s'", name)
	}

	var index ir.ExpressionHandle
	if b.arity == 2 {
		rawIndex := rawArgs[1]
		if index, _, err = c.lowerExpect(e, rawIndex, rhsPos); err != nil {
			return 0, false, err
		}
		if terr := c.typifierGrow(span); terr != nil {
			return 0, false, terr
		}
		if !f.subgroupIndexScalar(c, index) {
			return 0, false, errSemantic(span, "Unknown function '%s'", name)
		}
	}

	var mode ir.GatherMode
	switch b.gather {
	case subgroupGatherBroadcast:
		mode = ir.GatherBroadcast{Index: index}
	case subgroupGatherBroadcastFirst:
		mode = ir.GatherBroadcastFirst{}
	case subgroupGatherShuffle:
		mode = ir.GatherShuffle{Index: index}
	case subgroupGatherShuffleXor:
		mode = ir.GatherShuffleXor{Mask: index}
	case subgroupGatherShuffleUp:
		mode = ir.GatherShuffleUp{Delta: index}
	case subgroupGatherShuffleDown:
		mode = ir.GatherShuffleDown{Delta: index}
	case subgroupGatherQuadBroadcast:
		mode = ir.GatherQuadBroadcast{Index: index}
	default:
		mode = ir.GatherQuadSwap{Direction: b.direction}
	}

	resultType, err := c.resolveTypeHandle(argument, argumentSpan)
	if err != nil {
		return 0, false, err
	}

	c.emitEnd()
	result, rerr := c.addExpression(ir.ExprSubgroupOperationResult{Type: resultType}, span)
	if rerr != nil {
		return 0, false, rerr
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtSubgroupGather{
		Mode:     mode,
		Argument: argument,
		Result:   result,
	}})
	c.emitStart()
	return result, true, nil
}

// subgroupValueScalar returns the scalar type of a scalar or vector value; ok
// is false for a matrix, a pointer, an array, a struct or any other type the
// subgroup builtins do not take.
func subgroupValueScalar(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	}
	return ir.ScalarType{}, false
}

// subgroupBoolScalar reports whether the operand is a bool scalar, the type a
// vote builtin and subgroupBallot take (GLSL 4.60 §8.18: `bool subgroupAll(bool
// value)`, not a bvec).
func (f *Frontend) subgroupBoolScalar(c *Context, expr ir.ExpressionHandle) bool {
	scalar, isScalar := c.getType(expr).(ir.ScalarType)
	return isScalar && scalar.Kind == ir.ScalarBool && scalar.Width == 1
}

// subgroupNumericValue reports whether the operand is a scalar or vector of an
// integer or float type, the value an arithmetic collective takes (GLSL 4.60
// §8.18: int, uint, float, double and their vectors).
func (f *Frontend) subgroupNumericValue(c *Context, expr ir.ExpressionHandle) bool {
	scalar, ok := subgroupValueScalar(c.getType(expr))
	if !ok {
		return false
	}
	switch scalar.Kind {
	case ir.ScalarSint, ir.ScalarUint, ir.ScalarFloat:
		return true
	}
	return false
}

// subgroupGatherValue reports whether the operand is a scalar or vector, the
// value a broadcast, shuffle or quad builtin takes. glslang accepts a bool
// value there too (measured: subgroupShuffle(true, 0u) compiles).
func (f *Frontend) subgroupGatherValue(c *Context, expr ir.ExpressionHandle) bool {
	_, ok := subgroupValueScalar(c.getType(expr))
	return ok
}

// subgroupIndexScalar reports whether the operand is a 32-bit integer scalar,
// the lane index, delta or mask a gather builtin takes. glslang accepts both
// int and uint there (measured: subgroupShuffle(1u, 1) compiles).
func (f *Frontend) subgroupIndexScalar(c *Context, expr ir.ExpressionHandle) bool {
	scalar, isScalar := c.getType(expr).(ir.ScalarType)
	if !isScalar || scalar.Width != 4 {
		return false
	}
	return scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint
}

// subgroupBuiltinVariable reports whether name is one of the four scalar
// subgroup built-in variables of GL_KHR_shader_subgroup_basic, which
// builtinDataFor maps to their ir.Builtin values.
func subgroupBuiltinVariable(name string) bool {
	switch name {
	case "gl_SubgroupSize", "gl_SubgroupInvocationID", "gl_SubgroupID", "gl_NumSubgroups":
		return true
	}
	return false
}

// subgroupVariableGate applies the availability rules of the subgroup built-in
// variables, which are the builtins': the source must enable the basic family's
// extension — glslang, "'gl_SubgroupSize' : required extension not requested:
// GL_KHR_shader_subgroup_basic" (probed 2026-11-16) — and GLSL ES before 3.10
// has no subgroup operations at all. A name that is not a subgroup variable is
// not gated.
func (f *Frontend) subgroupVariableGate(name string, span ir.Span) *frontError {
	if !subgroupBuiltinVariable(name) {
		return nil
	}
	return f.subgroupGate(name, subgroupFamilyBasic, span)
}

// subgroupVariableRefusal returns the diagnostic for a subgroup built-in
// variable the IR cannot express, or nil when name is not one. The basic
// family's four scalar variables are wired through builtinDataFor and the
// entry point; the ballot family's mask variables have no ir.Builtin value, so
// a use is refused with the gap named instead of an unknown-name text.
func subgroupVariableRefusal(name string, span ir.Span) *frontError {
	switch name {
	case "gl_SubgroupEqMask", "gl_SubgroupGeMask", "gl_SubgroupGtMask",
		"gl_SubgroupLeMask", "gl_SubgroupLtMask":
		return errSemantic(span, "'%s' : the subgroup ballot mask built-in variables of "+
			"GL_KHR_shader_subgroup_ballot are not implemented by the GLSL frontend: ir.Builtin has no "+
			"subgroup ballot-mask value (card P1-GLSLIN-017)", name)
	}
	return nil
}
