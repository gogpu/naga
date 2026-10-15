package snapshot_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// deviationKind classifies a known difference between our output and Rust's.
type deviationKind int

const (
	// deviationIntentional: we deliberately differ (Reason says why).
	deviationIntentional deviationKind = iota + 1
	// deviationBug: our output is wrong or worse; Card tracks the fix.
	deviationBug
	// deviationUnsupported: we cannot produce this output yet; the compile
	// error must contain ErrorContains; Card tracks the feature.
	deviationUnsupported
)

func (k deviationKind) String() string {
	switch k {
	case deviationIntentional:
		return "intentional"
	case deviationBug:
		return "bug"
	case deviationUnsupported:
		return "unsupported"
	default:
		return fmt.Sprintf("deviationKind(%d)", int(k))
	}
}

// rustDeviation records one expected Rust output we knowingly do not match.
// It replaces the old per-shader allow-list: an entry covers exactly one
// output file and pins the difference, so a new or changed difference fails
// instead of hiding behind the entry.
type rustDeviation struct {
	Shader string        // corpus shader name
	File   string        // Rust output file name, e.g. "wgsl-globals.metal"
	Kind   deviationKind //
	Reason string        // why we differ; reviewed when the entry is added
	Card   string        // tracking task; required for bug and unsupported
	// Fingerprint pins the compared pair (referenceFingerprint) for
	// intentional and bug entries. When either side changes, the test fails
	// and the entry must be re-reviewed.
	Fingerprint string
	// ErrorContains is part of the expected compile error (unsupported).
	ErrorContains string
}

// Reasons shared by several entries.
const (
	reasonMSLThreadgroup = "workgroup variables are declared at kernel-body scope (`threadgroup T name;`) " +
		"instead of as threadgroup entry-point parameters: the pure-Go wgpu Metal HAL does not call " +
		"setThreadgroupMemoryLength:atIndex:, so parameter-based threadgroup memory would be unsized. " +
		"The rest of the output is identical to Rust's"
	reasonSPVWorkgroupLayoutFree = "workgroup variables use layout-free types plus OpCopyLogical " +
		"(VUID-StandaloneSpirv-None-10684); OpCopyLogical needs SPIR-V 1.4, so the module is 1.4 and its " +
		"OpEntryPoint lists every global the entry point uses, where Rust emits 1.1 with Input/Output only"
	reasonSPVAtomicLoadStore = "atomicLoad/atomicStore are emitted as plain OpLoad/OpStore instead of " +
		"OpAtomicLoad/OpAtomicStore (a data race on the GPU; spirv-val cannot see it)"
	reasonSPVPerEntryPoint = "Rust writes one SPIR-V module per entry point ([spv] separate_entry_points, " +
		"spv::PipelineOptions); the spirv package cannot restrict output to one entry point"
	reasonProcessOverrides = "ir.ProcessOverrides does not match Rust's process_overrides: it skips the " +
		"compact(KeepUnused::No) step, leaving every override duplicated as a `<name>_1` constant (several " +
		"with zero values instead of the resolved ones), and does not constant-fold function expressions " +
		"that use overrides"
	reasonCompactNoEntryPoints = "the module has no entry points; Rust's compact(KeepUnused::No) removes " +
		"everything, ir.CompactUnused returns early and every type and function is emitted"
)

// rustDeviations is the complete list of known differences from the vendored
// Rust outputs, each verified against its diff when added. Entries are
// removed when fixed: TestRustReference fails on an entry that no longer
// describes a difference.
var rustDeviations = []rustDeviation{
	// SPIR-V: per-entry-point modules.
	{Shader: "interface", File: "wgsl-interface.compute.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "interface", File: "wgsl-interface.fragment.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "interface", File: "wgsl-interface.vertex.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "interface", File: "wgsl-interface.vertex_two_structs.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "overrides", File: "wgsl-overrides.main.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "overrides-atomicCompareExchangeWeak", File: "wgsl-overrides-atomicCompareExchangeWeak.f.spvasm",
		Kind: deviationUnsupported, Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001",
		ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "overrides-ray-query", File: "wgsl-overrides-ray-query.main.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "separate-entry-points", File: "wgsl-separate-entry-points.compute.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "separate-entry-points", File: "wgsl-separate-entry-points.fragment.spvasm", Kind: deviationUnsupported,
		Reason: reasonSPVPerEntryPoint, Card: "FEAT-SPV-PIPELINE-001", ErrorContains: "per-entry-point SPIR-V output"},

	// SPIR-V: other differences.
	{Shader: "6772-unpack-expr-accesses", File: "wgsl-6772-unpack-expr-accesses.spvasm", Kind: deviationBug,
		Reason: "no Int8 capability or 8-bit types: pack4xI8/unpack4xI8 use the polyfill where Rust uses native Int8",
		Card:   "NAGA-SPV-009", Fingerprint: "df47f356c54c28518f3eca03a71758d32fdef86db528bfe1827f6edda014c744"},
	{Shader: "bits", File: "wgsl-bits.spvasm", Kind: deviationBug,
		Reason: "no Int8 capability or 8-bit types: pack4xI8/unpack4xI8 use the polyfill where Rust uses native Int8",
		Card:   "NAGA-SPV-009", Fingerprint: "9dc54aeb5e94e96d4917abe62879474a5c177232a76fd59ba7e0603efb1440a5"},
	{Shader: "binding-buffer-arrays", File: "wgsl-binding-buffer-arrays.spvasm", Kind: deviationBug,
		Reason: "spirv.Options has no binding map, so [[spv.binding_map]] binding_array_size = 10 is ignored and " +
			"the binding array keeps its declared length 1; no ReadZeroSkipWrite bounds check is emitted for the " +
			"binding-array index (Rust: OpULessThan + OpSelect + OpBranchConditional, x2); 3 NonUniform " +
			"decorations where Rust emits 2",
		Card: "BUG-SPV-BINDING-ARRAY-001", Fingerprint: "b2c9f81bf6f648311c7331e16b15925a8e2351bcdb07e6b1009bc67530e10bbf"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.spvasm", Kind: deviationBug,
		Reason: reasonSPVAtomicLoadStore + "; also " + reasonSPVWorkgroupLayoutFree,
		Card:   "BUG-SPV-ATOMIC-001", Fingerprint: "6c0e37a05e339c480f185bb7ad231ae21c62671a63ff445c260c22bf43c2d74b"},
	{Shader: "atomicOps-int64", File: "wgsl-atomicOps-int64.spvasm", Kind: deviationBug,
		Reason: reasonSPVAtomicLoadStore + "; also " + reasonSPVWorkgroupLayoutFree + "; and atomic Scope " +
			"operands use u32 constants where Rust uses i32 (equivalent per spec)",
		Card: "BUG-SPV-ATOMIC-001", Fingerprint: "d353f3fc9350e17fb03dd01d98f2523cd89a7c9be886500626afb5fe4ab16f26"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.spvasm", Kind: deviationBug,
		Reason: reasonSPVWorkgroupLayoutFree + "; also w_mem.arr is loaded and copied twice (the first " +
			"OpLoad + OpCopyLogical is dead), and Rust has one more, empty, entry block",
		Card: "NAGA-SPV-009", Fingerprint: "a3576c00f2feeb05e4eabb9617c1d7b5c2d2eeb98e45c15a18f6e1a4e040a177"},

	// MSL.
	{Shader: "abstract-types-operators", File: "wgsl-abstract-types-operators.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "a3779b5219b5ccd2994c9287500984b47744d4978ed0f27a81afa997515059ee"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "178f9df241a45e67acbd8911eddf2c78317bbf43a0f91366f573aba3b0c9907c"},
	{Shader: "globals", File: "wgsl-globals.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "b092f4a19ac0f3d8e347db22d69453a02a5e903e9c71e80501ea949334d0b1cc"},
	{Shader: "interface", File: "wgsl-interface.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "344fef2ba86b8219a12bfb1a205e2ad41e7adfda8f806bea21d0d294b53d7392"},
	{Shader: "policy-mix", File: "wgsl-policy-mix.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "dfee9e4187ff66f1ebba821003035555eb70d51f989546ec289f906b8b71349b"},
	{Shader: "workgroup-uniform-load", File: "wgsl-workgroup-uniform-load.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "85b951eee9e2fc17d8e65f73a0a719eec5518d5243e7d0636bd1a1707600e629"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.metal", Kind: deviationIntentional,
		Reason: reasonMSLThreadgroup, Fingerprint: "f1665755567d6983b2631f63db4b27a42d4c3cb19f5b570bde93a01b3d71c1d4"},
	{Shader: "overrides", File: "wgsl-overrides.metal", Kind: deviationBug,
		Reason: reasonProcessOverrides, Card: "BUG-IR-OVERRIDES-001",
		Fingerprint: "c433993184a6c7edc90da009849b0540418fc1edbc8c36819076fea71aec41bb"},
	{Shader: "overrides-ray-query", File: "wgsl-overrides-ray-query.metal", Kind: deviationBug,
		Reason: reasonProcessOverrides, Card: "BUG-IR-OVERRIDES-001",
		Fingerprint: "6a0d9bdeac8d66b6431fd4412ec1bf6f6d2f2ecbec6a6d50a8aeced987fa4177"},
	{Shader: "overrides-atomicCompareExchangeWeak", File: "wgsl-overrides-atomicCompareExchangeWeak.metal",
		Kind: deviationBug, Card: "BUG-IR-OVERRIDES-001",
		Reason:      reasonProcessOverrides + "; also " + reasonMSLThreadgroup,
		Fingerprint: "8a2517f3ffa7b7cf9f20919a4f19d8cf5fc48d672e627cd4af22cee805c93e86"},

	// Own shader with Rust-derived references (not part of parity).
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.spvasm", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "59638ee72c61512f71d16de1c5a780fde41c12980e4af8ff9d54365f5e183ee3"},
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.metal", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "bf25348238f50d14efd99b4b9fe83405cb2eb934988242266e63dd4954320e18"},
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.hlsl", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "e0c67c25ed1172a301d7e42fa2dc037fa0c99b05f93d8f99354dcf2e234238a2"},
}

// findDeviation returns the entry for one output, or nil.
func findDeviation(shader, file string) *rustDeviation {
	for i := range rustDeviations {
		if rustDeviations[i].Shader == shader && rustDeviations[i].File == file {
			return &rustDeviations[i]
		}
	}
	return nil
}

// referenceFingerprint identifies a compared pair: the SHA-256 of the
// reference and our output, line endings normalized. Any change on either
// side changes it.
func referenceFingerprint(reference, ours string) string {
	h := sha256.New()
	for _, part := range []string{reference, ours} {
		part = strings.ReplaceAll(part, "\r\n", "\n")
		fmt.Fprintf(h, "%d:", len(part)) // length prefix: parts cannot bleed into each other
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// referenceResult is the outcome of comparing one output with Rust's.
type referenceResult int

const (
	resultPass referenceResult = iota + 1
	resultDeviation
	resultFail
)

// referenceVerdict is the judged outcome; message explains a failure.
type referenceVerdict struct {
	result  referenceResult
	kind    deviationKind // set for resultDeviation
	message string
}

// judgeReference decides the outcome of one comparison. compileErr is the
// error producing our output (nil on success); differs and fingerprint
// describe the comparison when it ran.
func judgeReference(dev *rustDeviation, compileErr error, differs bool, fingerprint string) referenceVerdict {
	fail := func(format string, args ...any) referenceVerdict {
		return referenceVerdict{result: resultFail, message: fmt.Sprintf(format, args...)}
	}
	if dev != nil && dev.Kind == deviationUnsupported {
		switch {
		case compileErr == nil:
			return fail("now compiles: remove the unsupported deviation and compare with Rust")
		case !strings.Contains(compileErr.Error(), dev.ErrorContains):
			return fail("compile error changed (deviation expects %q): %v", dev.ErrorContains, compileErr)
		default:
			return referenceVerdict{result: resultDeviation, kind: deviationUnsupported}
		}
	}
	if compileErr != nil {
		return fail("compile error: %v", compileErr)
	}
	switch {
	case dev == nil && !differs:
		return referenceVerdict{result: resultPass}
	case dev == nil:
		return fail("differs from Rust (fingerprint %s)", fingerprint)
	case !differs:
		return fail("matches Rust now: remove the %v deviation", dev.Kind)
	case dev.Fingerprint != fingerprint:
		return fail("%v deviation no longer matches: re-review the difference (fingerprint %s, recorded %s)",
			dev.Kind, fingerprint, dev.Fingerprint)
	default:
		return referenceVerdict{result: resultDeviation, kind: dev.Kind}
	}
}
