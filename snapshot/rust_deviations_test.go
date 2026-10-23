package snapshot_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
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
	reasonSPVPerEntryPoint = "Rust writes one SPIR-V module per entry point ([spv] separate_entry_points, " +
		"spv::PipelineOptions); the spirv package cannot restrict output to one entry point"
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

	// Own shader with Rust-derived references (not part of parity).
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.spvasm", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "3f5cef028aac29a741d56970e98587bc3a6ab8de98f93f4177f88949b2a98749"},
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.metal", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "bf25348238f50d14efd99b4b9fe83405cb2eb934988242266e63dd4954320e18"},
	{Shader: "ptr-deref-test", File: "wgsl-ptr-deref-test.hlsl", Kind: deviationBug,
		Reason: reasonCompactNoEntryPoints, Card: "BUG-IR-COMPACT-001",
		Fingerprint: "e0c67c25ed1172a301d7e42fa2dc037fa0c99b05f93d8f99354dcf2e234238a2"},

	// Corpus sync to 14328f49: outputs whose reference changed upstream while
	// ours stayed byte-identical to the previous reference (the difference is
	// exactly the listed upstream commits).
	{Shader: "abstract-types-const", File: "wgsl-abstract-types-const.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "a5d85b1ea5cf436a0d0ecdb2001d2ea8cb13264b75caab0a16cab92c66e9476d"},
	{Shader: "abstract-types-function-calls", File: "wgsl-abstract-types-function-calls.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "d48857fb9e3fd58cd8637fb705aa5d5457173c474432811aa7c83d75ac99e699"},
	{Shader: "abstract-types-let", File: "wgsl-abstract-types-let.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "77b1c89d5167e0c35993f4203c62b88e52e926c38958dd2c1818b411b0edaf2f"},
	{Shader: "abstract-types-operators", File: "wgsl-abstract-types-operators.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "3ca90467b90eea8fe43a89a20fec6f0df889f6485914c60f339b9254bbacfbff"},
	{Shader: "abstract-types-return", File: "wgsl-abstract-types-return.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "b6d82bb89ac9c0073063bcda8d50ee4c5ff7e23a03b74a16fc48b8f4aee5d9a4"},
	{Shader: "abstract-types-var", File: "wgsl-abstract-types-var.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "cc66aa85c91e01df3793623675f8198edc1f7ee92c17e4cb2dcb33640240a6c4"},
	{Shader: "access", File: "wgsl-access.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      notPorted("35975143", "1f4ae673"),
		Fingerprint: "e759e867f617d7eae23904feae6305c2ce88f7380cb08039a0ad4168815bd56c"},
	{Shader: "access", File: "wgsl-access.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "cbe4bf60b8c5171e70c1bf30a6bbb200b181a60201d773c6ce227b04e12c1e97"},
	{Shader: "access", File: "wgsl-access.hlsl", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      notPorted("786b8b11"),
		Fingerprint: "68e371f863469673cd98e0c02a4df13f11b910f88f8664a74d88c9c3820e1fec"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("7721a3e0", "56e4a389", "55534165"),
		Fingerprint: "2bd5dc71a566e16bd4b835879e407493894aaff5077842c0a98fc5d3aaacf346"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41", "f6d9a1db", "a03be8d2"),
		Fingerprint: "60d114a3a24d7c1fca95b4bc466d2f13828cbfce203e1d79b209fea8225beab8"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "2422fa4c08e1bbcbb0716089bb6df75210b746dbbb062d6402370fb970e68ea9"},
	{Shader: "array-in-function-return-type", File: "wgsl-array-in-function-return-type.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "0dfb944bc3f437a26b8d5ab090c44d1cc4ca25846d8f1fb55a5c48deebf62850"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "d914db5e93c4bbc1eb2f380ac064ebb4b1f22d36a25e6dee05674cd56b47e541"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.test_atomic_compare_exchange_i32.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("1ef831ad", "f0201c9c"),
		Fingerprint: "3841958d61ec071b05b983d622317a87fe7d76c3ed5c95f3eda01c76a3995050"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.test_atomic_compare_exchange_u32.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("1ef831ad", "f0201c9c"),
		Fingerprint: "d44482ed2c4b69d3d241a58e240129f326a9c61c3d6bd829ec10491f8debeb56"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("f0201c9c"),
		Fingerprint: "04a63f14629e823514ccdd834a2a7af4eb33aaf177a6551898fa771c20826e44"},
	{Shader: "atomicCompareExchange-int64", File: "wgsl-atomicCompareExchange-int64.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("f0201c9c"),
		Fingerprint: "c3b64a563e7ab333d4d33fb11ff72d7a841034b6b9b3f10b87c66b25fde07820"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("6bca1014"),
		Fingerprint: "5a5740115f1d98e6eea1e1fd3893fc80441419db1fad9872ae7ca95b0b54d3d8"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("1ef831ad", "b5bd5aed"),
		Fingerprint: "5ae02019a950cc49da288cb2bd4db399e82c46406e16ab96f08ea3003718c940"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.cs_main.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("1ef831ad", "f0201c9c", "cdc9d0c4"),
		Fingerprint: "f05bb510273b9b6063c1c96cc7811645ad0fbc8aa4d2ce1f48b62cff2ef33555"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("96b7872f", "a53ffffe", "f0201c9c"),
		Fingerprint: "97e4371ac120c664592f0e79d3800dc1185adc9ca217f47382b054544f0d8dcd"},
	{Shader: "atomicOps-float32", File: "wgsl-atomicOps-float32.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("1ef831ad", "74827c41", "b5bd5aed", "948b1a6c"),
		Fingerprint: "1aa73ef53b5bc3aebab327ce9d199a9c431aa66c9b580b541041c494b682613c"},
	{Shader: "atomicOps-int64", File: "wgsl-atomicOps-int64.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad", "6bca1014"),
		Fingerprint: "3dc37943ef5ffc2a0f7dabc54a94a4eee2e765f1a6f16143a676d0782202669b"},
	{Shader: "atomicOps-int64", File: "wgsl-atomicOps-int64.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("96b7872f", "a53ffffe", "f0201c9c"),
		Fingerprint: "928c580571de28b244ad890bed2a9923d4a7a1d199f653d8eeaf5c47375ffb66"},
	{Shader: "atomicOps-int64-min-max", File: "wgsl-atomicOps-int64-min-max.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad", "74827c41"),
		Fingerprint: "0484bdf2a2e749f5a5e63d9303927b5333bfed8faa7efbecfe5778d65380d007"},
	{Shader: "atomicTexture", File: "wgsl-atomicTexture.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4e7ffe09", "74827c41"),
		Fingerprint: "af960f8798ca07abea5dbf11799035cfb71c11718ea0d51458e902df2cad10bd"},
	{Shader: "atomicTexture-int64", File: "wgsl-atomicTexture-int64.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("4e7ffe09", "74827c41", "b5bd5aed", "948b1a6c"),
		Fingerprint: "562a10754925d41d5f9f2a25eff1ca5a981ffecaea31df63adf6a78536c17625"},
	{Shader: "binding-arrays", File: "wgsl-binding-arrays.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4c1ea47e"),
		Fingerprint: "3f428374ad2d8a403b042f9cfcbea7a1fcbff6ed7b4b23f96e2a954a82604382"},
	{Shader: "bits", File: "wgsl-bits.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("a071a4d5"),
		Fingerprint: "9cfef2120d2f0b2fce14982899ee291fb48932f705478b332121a3062801c15e"},
	{Shader: "bits-optimized-msl", File: "wgsl-bits-optimized-msl.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("a071a4d5", "74827c41"),
		Fingerprint: "995954f7a28c74f506bded068d306f60351ee5c8f613075d86ed0137f10a5663"},
	{Shader: "bounds-check-zero-atomic", File: "wgsl-bounds-check-zero-atomic.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "1ee4bb2ed5e24d053be0e31840f3b6b368831465ded20b16f201416e0d8c7f68"},
	{Shader: "const-exprs", File: "wgsl-const-exprs.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "03d7bfc1ae796b58e8d2bdcd84e31f708c2bd576d48390aab33d427a442ec0f9"},
	{Shader: "constructors", File: "wgsl-constructors.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "9cc4a8b6c5f9adf9e3d72894fd457e33380f644e177428d15c8653baca43d2cd"},
	{Shader: "f16", File: "wgsl-f16.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      notPorted("35975143"),
		Fingerprint: "1732c1d7d43674a201c2b5224cec7bcc87a1bad694427dacfa919ef7c571e00d"},
	{Shader: "f16", File: "wgsl-f16.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "sign() of a float is not cast back to the float type (HLSL sign returns int); " + notPorted("ae19a470"),
		Fingerprint: "82a5677093ac582f802788b8337d7bd588cf47beed4fd6e6d3214955a2e0e4db"},
	{Shader: "fragment-output", File: "wgsl-fragment-output.metal", Kind: deviationBug, Card: "BUG-MSL-MULTI-RETURN-001",
		Reason:      notPorted("0892b384"),
		Fingerprint: "ccad7255547e07f36cbd6fb5f32afef0211fff5a29b7f972466c36cf87454d25"},
	{Shader: "fragment-output", File: "wgsl-fragment-output.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b051a87e"),
		Fingerprint: "a0f26490b1acc6d644f85bd2379f22727bf0cd4e6440b2ada79e74a18e2ad0a5"},
	{Shader: "functions-optimized-by-version", File: "wgsl-functions-optimized-by-version.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41"),
		Fingerprint: "94ed7241cd0df6cb8cbea9c8c31d344bf9771de852faeef34f09ca110ca0c7a5"},
	{Shader: "globals", File: "wgsl-globals.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      notPorted("35975143", "6bca1014"),
		Fingerprint: "1e2a75676c4715b7c79bcdc1200ba93e815bd8ab2fd42552a5b47713621ba791"},
	{Shader: "globals", File: "wgsl-globals.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "87877e2771f0c8885fd9a990e6be655d147090486e3a043d129c8bf63c9dd8e3"},
	{Shader: "globals", File: "wgsl-globals.main.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("f0201c9c"),
		Fingerprint: "7d17136d781ca652f753c0330f934bbf596fc711b3b9c96fe132f7b86274603c"},
	{Shader: "globals", File: "wgsl-globals.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      notPorted("96b7872f", "a53ffffe", "f0201c9c", "786b8b11"),
		Fingerprint: "e9af820e456ec48cb0dd123516ba47c76037885238812a5099520056a80f71e1"},
	{Shader: "image", File: "wgsl-image.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4c1ea47e"),
		Fingerprint: "c19c6964927501d578ff313a597cdccf644c87da5700826e1f5750beb1daa583"},
	{Shader: "int64", File: "wgsl-int64.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41"),
		Fingerprint: "49ee023c28dfae32db81816a29b622b2efbf1e9f1c3dfb219fba3ba98e47da53"},
	{Shader: "interface", File: "wgsl-interface.metal", Kind: deviationBug, Card: "BUG-MSL-MULTI-RETURN-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("74827c41", "b5bd5aed", "0892b384"),
		Fingerprint: "ec4bc8d00a608664484e1e5dbe865562db163b6b641626418c90c14d5b84b0c2"},
	{Shader: "interface", File: "wgsl-interface.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "2e5af9196b9127bb1483338536bd38f47d27f5c5ba988b3236f18b36889b0dc3"},
	{Shader: "overrides-atomicCompareExchangeWeak", File: "wgsl-overrides-atomicCompareExchangeWeak.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "f483af7caf2f9f3c2df44d835e3d6cb29ef25f58beac51d988bc26cd301e389c"},
	{Shader: "overrides-ray-query", File: "wgsl-overrides-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41", "f6d9a1db", "a03be8d2"),
		Fingerprint: "f84957f44d488704e8b02190d14399e66e70d1882ae334cdc87cd3918355b1dd"},
	{Shader: "pointer-function-arg", File: "wgsl-pointer-function-arg.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "c8e0982568ef817bc02f785b091c26e8cb030f548e5840e0a99976c997fefcf3"},
	{Shader: "pointer-function-arg-restrict", File: "wgsl-pointer-function-arg-restrict.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "224a47d70f34da8df961826cd903b3a02e5d9dbc3ccb98b1eae7bdf581d5b00d"},
	{Shader: "pointer-function-arg-rzsw", File: "wgsl-pointer-function-arg-rzsw.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "e86bd0b6898a41fcb191aa9ef77a0c3940f8ab2a4e5799414efd6d63578b48d3"},
	{Shader: "policy-mix", File: "wgsl-policy-mix.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b9c09622", "b5bd5aed"),
		Fingerprint: "fc8353887a3ba305ae3a0174b1fa98d2dcd644b6846ae3e43b3100ce1729c5d4"},
	{Shader: "ray-query", File: "wgsl-ray-query.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("7721a3e0", "56e4a389", "55534165"),
		Fingerprint: "8d784c87f0e1072cf686b138fc0ad47e0d648e9e0ef6bae19074319722662000"},
	{Shader: "ray-query", File: "wgsl-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41", "f6d9a1db", "a03be8d2"),
		Fingerprint: "77f6b78ff15a60c42046ab4152d029439f8215f3c7753c29819c29391d7500fa"},
	{Shader: "ray-query", File: "wgsl-ray-query.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "cb8f0d2a54d6b5df257442587287064504e6ed8dbd31124f13892a8c41539c71"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("7721a3e0", "56e4a389", "55534165"),
		Fingerprint: "8d784c87f0e1072cf686b138fc0ad47e0d648e9e0ef6bae19074319722662000"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41", "f6d9a1db", "a03be8d2"),
		Fingerprint: "ed8c7b56d79adf5da2285a93596e3d0592ee76e13a7532c96b11c1eaabe78b82"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "c519dd006261c4371354d5e4b1cc6c89e0bb97ce67589151e244b01c15197232"},
	{Shader: "skybox", File: "wgsl-skybox.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("de9f9f4d"),
		Fingerprint: "795c7cd20304501fecb72cfa2f361f721a1c45b96d1e287e2379cfc2bf9c3356"},
	{Shader: "subgroup-operations", File: "wgsl-subgroup-operations.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f"),
		Fingerprint: "49c367de90782bd0b98a39cdf91dfe178b7b285e5620585937121a77a2c12a1d"},
	{Shader: "workgroup-uniform-load", File: "wgsl-workgroup-uniform-load.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "ec4a89e67b6884ab83cf4a85891929f3c074f19d813473644dcd7fac2bfef94d"},
	{Shader: "workgroup-uniform-load", File: "wgsl-workgroup-uniform-load.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "f278a18cf662e19ee4b0d76f533e153fde6089635386232b84972d187e3b6479"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.spvasm", Kind: deviationBug, Card: "NAGA-SPV-009",
		Reason:      reasonSPVWorkgroupLayoutFree + "; also Rust has one more, empty, entry block" + "; " + notPorted("6bca1014"),
		Fingerprint: "73becac2176e1a8cb3c6072c52d062cac8082dd0fffbd64ab489ac6a2745fd9a"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "b66a14becea1eb9fa9f71d61289df82652660c58e999a131e70fcdc9f3b00c38"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "33590ccad2599a2e7d0001ed960e7588967153a07302680f8d01991f8a736362"},

	// Corpus sync to 14328f49: new outputs and outputs whose input changed,
	// each reviewed against its full difference.
	{Shader: "barycentrics", File: "wgsl-barycentrics.metal", Kind: deviationBug, Card: "BUG-WGSL-UNKNOWN-BUILTIN-001",
		Reason:      "@builtin(barycentric_no_perspective) is unknown to our lowerer, which silently maps it to position ([[position]]); Rust also passes barycentrics through the stage_in struct; " + notPorted("fc9060f5"),
		Fingerprint: "6fb012193bd314757566d2f1e7fffecfc16842ba6b583f3db7b18e2ab66b9bc7"},
	{Shader: "barycentrics", File: "wgsl-barycentrics.hlsl", Kind: deviationBug, Card: "BUG-WGSL-UNKNOWN-BUILTIN-001",
		Reason:      "@builtin(barycentric_no_perspective) is silently lowered as position: SV_Position instead of noperspective SV_Barycentrics; " + notPorted("fc9060f5"),
		Fingerprint: "dc9e6f672205d8ed2285a8d04a4f8b05317b8cf2c68e6b771dade469ab3e855d"},
	{Shader: "barycentrics", File: "wgsl-barycentrics.fs_main_no_perspective.Fragment.glsl", Kind: deviationBug, Card: "BUG-WGSL-UNKNOWN-BUILTIN-001",
		Reason:      "@builtin(barycentric_no_perspective) is silently lowered as position: gl_FragCoord instead of gl_BaryCoordNoPerspEXT (and no GL_EXT_fragment_shader_barycentric); " + notPorted("fc9060f5"),
		Fingerprint: "b35f1a9bb448578e7f1e840d4bd56a107ff8d77cc936f359ee0d91ecb38c5c0d"},
	{Shader: "binding-buffer-arrays", File: "wgsl-binding-buffer-arrays.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "upstream extended the shader (nested struct members, runtime-sized binding_array, plain storage buffer); counts-level SPIR-V: Rust declares 4 more StorageBuffer pointer types",
		Fingerprint: "9205f551a8e2f6a9a52a4ccc19e8c07bcefc4dd3f1ed8b9b533919e75a72887d"},
	{Shader: "binding-buffer-arrays", File: "wgsl-binding-buffer-arrays.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "buffer binding arrays: Rust passes NagaArgumentBufferWrapper<device Foo*>* with per-element buffer sizes (size0[10]); we pass a constant wrapper reference with no sizes; " + notPorted("bf5c6cff"),
		Fingerprint: "a1a0085277b122210372a941db88011289083e41cf9375a88e2df63df59ef25c"},
	{Shader: "math-functions", File: "wgsl-math-functions.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "sign() of a float is not cast back to the float type (HLSL sign returns int); " + notPorted("ae19a470"),
		Fingerprint: "12bd192dd25554894ab56b285474b8aaeb1d2c43c9f135ad09ced71d05c005cb"},
	{Shader: "float-sign", File: "wgsl-float-sign.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "sign() of a float is not cast back to the float type (HLSL sign returns int); " + notPorted("ae19a470"),
		Fingerprint: "a0041daa6a538326ebd4317df64b9ec1b379f463a2bd59137e4a69b2e67f8b28"},
	{Shader: "subgroup-operations", File: "wgsl-subgroup-operations.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41"),
		Fingerprint: "232b965af5eb26d37e77dd60acd4f88ace3c21cfca232917d3bf0330fb9a0882"},
	{Shader: "subgroup-barrier", File: "wgsl-subgroup-barrier.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("74827c41"),
		Fingerprint: "8c19f2ac027ea2eef8932e0756b7d831fd58e44b16942e380de75f30d2aafda0"},
	{Shader: "8820-multiple-local-invocation-index-id", File: "wgsl-8820-multiple-local-invocation-index-id.spvasm", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      "with local_invocation_id/index both in an input struct, we add a third interface variable for workgroup zero-initialization; " + notPorted("96b7872f"),
		Fingerprint: "e78a140a0b8f2c54d479f7422b1c44d7c87fc88455e88bd14ad01f39002b920b"},
	{Shader: "8820-multiple-local-invocation-index-id", File: "wgsl-8820-multiple-local-invocation-index-id.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      "workgroup zero-initialization tests local_invocation_id instead of the existing local_invocation_index; " + reasonMSLThreadgroup + "; " + notPorted("96b7872f"),
		Fingerprint: "09b667371c70d37d4025a2bb3a12457e481111a53d166ab0e561e44958711c17"},
	{Shader: "8820-multiple-local-invocation-index-id", File: "wgsl-8820-multiple-local-invocation-index-id.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      "zero-initialization adds a second SV_GroupThreadID parameter although the input struct already has one (duplicate system-value semantic) instead of using SV_GroupIndex; " + notPorted("96b7872f"),
		Fingerprint: "660a7d9fe699b550b14c74a708d894f161ae0f16fcd50908fd01e80616bf12e8"},
	{Shader: "9883-atomic_sub-negative", File: "wgsl-9883-atomic_sub-negative.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      "workgroup zero-initialization through SV_GroupThreadID instead of SV_GroupIndex; " + notPorted("96b7872f"),
		Fingerprint: "66de003652187c829d1226917e7230be118469856bac7ddeefe2d94436115811"},
	{Shader: "9883-atomic_sub-negative", File: "wgsl-9883-atomic_sub-negative.main.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      "atomicSub(a, -1) is written atomicAdd(a, --1): a predecrement of a literal, invalid GLSL; " + notPorted("cdc9d0c4"),
		Fingerprint: "9a1497ec9dcb3eea7b21b62bd3ba4ed7735f1c62ed7fc48bf4e1e1bae65e0fc1"},
	{Shader: "primitive-index", File: "wgsl-primitive-index.func.Fragment.glsl", Kind: deviationBug, Card: "BUG-GLSL-PRIMITIVE-ID-001",
		Reason:      "gl_PrimitiveID (int) is assigned to a uint without uint(...)",
		Fingerprint: "b706539460818957df5e2192c27b42eeb4a9e44f30245d07bb143107e852ee6d"},
	{Shader: "primitive-index-gles", File: "wgsl-primitive-index-gles.func.Fragment.glsl", Kind: deviationBug, Card: "BUG-GLSL-PRIMITIVE-ID-001",
		Reason:      "gl_PrimitiveID (int) is assigned to a uint without uint(...), and GL_OES_geometry_shader is not required on GLSL ES",
		Fingerprint: "2ccf12550d9072126eb3d9e45e7f6c2b39ae818b26a6e47ea3a46b0db3b6d323"},
	{Shader: "9717-texture-sample-level-offset", File: "wgsl-9717-texture-sample-level-offset.hlsl", Kind: deviationBug, Card: "BUG-HLSL-SAMPLE-OFFSET-001",
		Reason:      "SampleLevel offsets are always int2: a 1D offset is widened and a 3D offset is truncated to int2",
		Fingerprint: "d114a65036d938889c2bcb12c90f1f4eba3a0a02c5c54d4be11091d0d471e91d"},
	{Shader: "9890-no-rq-hoist", File: "wgsl-9890-no-rq-hoist.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "ray queries: we still use the intersector-based _RayQuery wrapper without initialization tracking or ray validity checks; " + notPorted("f6d9a1db", "a03be8d2", "74827c41"),
		Fingerprint: "be748a3025f23c6f0fcbc8b2c6d6679ff0fba18bfdb424ef4246fdc5a3d6e583"},
	{Shader: "9890-no-rq-hoist", File: "wgsl-9890-no-rq-hoist.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "ray query helpers are emitted in a different place and without the safety checks; " + notPorted("c97fe3e7"),
		Fingerprint: "68d6dee675e5b829732c5eef5f971ed39c368182d17accfed6baeb477003fa52"},
	{Shader: "clip-distances", File: "wgsl-clip-distances.metal", Kind: deviationBug, Card: "BUG-MSL-CLIP-DISTANCES-001",
		Reason:      "@builtin(clip_distances) is emitted as a plain struct member without [[clip_distance]], so clipping is lost",
		Fingerprint: "05ed12440716b5f6abcefb3d27b49b2a3c92056e1d16ef5b22d4ab641e5daabd"},
	{Shader: "draw-index", File: "wgsl-draw-index.spvasm", Kind: deviationBug, Card: "BUG-WGSL-UNKNOWN-BUILTIN-001",
		Reason:      "@builtin(draw_index) is unknown to our lowerer and silently becomes position (BuiltIn FragCoord in the fragment stage); Rust emits BuiltIn DrawIndex with DrawParameters",
		Fingerprint: "ea9f9123eacd298f33558c4e17b751271253c753cffbd0e390bee1621aa7aae4"},
	{Shader: "int-div-unchecked", File: "wgsl-int-div-unchecked.spvasm", Kind: deviationBug, Card: "TEST-TOML-KEYS-001",
		Reason:      "[spv] emit_int_div_checks = false is not supported: we always emit the naga_div/naga_mod helpers",
		Fingerprint: "1bac666dcd8d28ffce5f4fe3ee8384a022e53fe0669aa9ee3b85a38b6896b644"},
	{Shader: "int-div-unchecked", File: "wgsl-int-div-unchecked.metal", Kind: deviationBug, Card: "TEST-TOML-KEYS-001",
		Reason:      "[msl] emit_int_div_checks = false is not supported: we always emit the naga_div/naga_mod helpers",
		Fingerprint: "4cfb5183e47d7e2edf8866a5e5fc2d9411db1881d8dfb089d91fb18d3772334d"},
	{Shader: "mat2-uniform-alignment", File: "wgsl-mat2-uniform-alignment.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "2-row matrices in uniform buffers use the WGSL layout directly; Rust rewrites them to std140-compatible types; " + notPorted("35975143"),
		Fingerprint: "79535975bb6d6483dab2dd4860968d8af0ddeb827c17fa406a0ee5bcdafa2819"},
	{Shader: "mat_cx2", File: "wgsl-mat_cx2.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "2-row matrices in uniform buffers use the WGSL layout directly; Rust rewrites them to std140-compatible types; " + notPorted("35975143", "1f4ae673"),
		Fingerprint: "752b058ad74d91354cd1b9ef6a1e35e810ea29e96c1af2b34e118a72c2aabadb"},
	{Shader: "mat_cx2", File: "wgsl-mat_cx2.hlsl", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "mat2x2 helpers are named __mat2x2 instead of __mat2x2_f32, and arrays of mat2x2 in a cbuffer are indexed directly instead of through the column helpers (wrong cbuffer layout)",
		Fingerprint: "75f1a125fc76a394bc91ca390afe139cce7c22fc71f005b19aa76c4b975e3b99"},
	{Shader: "mat_cx2_f16", File: "wgsl-mat_cx2_f16.spvasm", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "2-row f16 matrices in uniform buffers use the WGSL layout directly; Rust rewrites them to std140-compatible types; " + notPorted("35975143"),
		Fingerprint: "c78f6b43654c8f948c6e507f32c3752217041f48f1d117c35b2a3bf4327e51c1"},
	{Shader: "mat_cx2_f16", File: "wgsl-mat_cx2_f16.hlsl", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "no helper types for matCx2<f16> in the uniform address space; " + notPorted("786b8b11"),
		Fingerprint: "9fc8dc7f6eae8576ff3d5898cbdb1d27f4c15e5d9eac1f31bcac0015395ef45d"},
	{Shader: "memory-decorations", File: "wgsl-memory-decorations.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent / @volatile memory decorations are not supported (parsed and dropped): Rust decorates the variables Coherent / Volatile",
		Fingerprint: "c0916764fc1db960b7aea5da5ed7534c19efdbc2af9d69e5cf5f3614cdb58d29"},
	{Shader: "memory-decorations", File: "wgsl-memory-decorations.main.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent / @volatile memory decorations are not supported (parsed and dropped): Rust writes coherent / volatile buffer qualifiers",
		Fingerprint: "83eafadf5be6365b35b64aebf5e60211b44384e9ea18022f582e90514ea2bd78"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust decorates the variable Coherent",
		Fingerprint: "3fa2ba0e5aabb4b46542c8dc1d6d844eda595af98bbbd34f77025520b1eebb7b"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes coherent device; " + notPorted("74827c41"),
		Fingerprint: "1b35caf460d7fe5744b0478ceef68ec17b22961ec9eafb4419118211d650267a"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.main.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes a coherent buffer qualifier",
		Fingerprint: "4717b0e58914928931955b4f4488cad8875f5080e94faaa4577de9eb88acb35a"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes globallycoherent",
		Fingerprint: "34dd72cec082c83049c91a3ca7f80e3e6a2d7379415a97aa20e8ff3478bfcc9d"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.spvasm", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "SPIR-V mesh shaders: compile error; " + notPorted("6bca1014"),
		ErrorContains: "global variable not found"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.hlsl", Kind: deviationBug, Card: "BUG-HLSL-BUILTIN-SEMANTIC-001",
		Reason:      "mesh outputs: triangle_indices and cull_primitive get SV_Position (BuiltInToSemantic fallback), no primitive-rate qualifier; " + notPorted("a53ffffe"),
		Fingerprint: "a4553444b2337c38998e0b6aa5444e4e9bd0336b2a319e8ff33972644c98f212"},
	{Shader: "mesh-shader-empty", File: "wgsl-mesh-shader-empty.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "SPIR-V mesh shaders: no MeshShadingEXT, the module has Linkage and no entry point interface; " + notPorted("6bca1014"),
		Fingerprint: "2a0ed1ef7296eb7d2dc9a7431349476d0d9b27f308a2aff328a9b5685761b4b0"},
	{Shader: "mesh-shader-empty", File: "wgsl-mesh-shader-empty.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "mesh-shader-empty", File: "wgsl-mesh-shader-empty.hlsl", Kind: deviationBug, Card: "BUG-HLSL-BUILTIN-SEMANTIC-001",
		Reason:      "mesh outputs get SV_Position on every member (BuiltInToSemantic fallback); " + notPorted("a53ffffe"),
		Fingerprint: "c916441d243b9918e082c3200fc1541434a557f1dcf082351e74adeabd7b49c8"},
	{Shader: "mesh-shader-lines", File: "wgsl-mesh-shader-lines.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "SPIR-V mesh shaders: no MeshShadingEXT, the module has Linkage and no entry point interface; " + notPorted("6bca1014"),
		Fingerprint: "3bc0ad1ca0744acd24d54e9add0de63f86b468a2666339454f55727e1c7d9d98"},
	{Shader: "mesh-shader-lines", File: "wgsl-mesh-shader-lines.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "mesh-shader-lines", File: "wgsl-mesh-shader-lines.hlsl", Kind: deviationBug, Card: "BUG-HLSL-BUILTIN-SEMANTIC-001",
		Reason:      "mesh outputs get SV_Position on every member (BuiltInToSemantic fallback); " + notPorted("a53ffffe"),
		Fingerprint: "dda9fc5557fde1f3818696ab9c31da9eec99bbeee5598e80d5e4d8aa18a27561"},
	{Shader: "mesh-shader-points", File: "wgsl-mesh-shader-points.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "SPIR-V mesh shaders: no MeshShadingEXT, the module has Linkage and no entry point interface; " + notPorted("6bca1014"),
		Fingerprint: "0b2dccc220a3f7b84ab79d299f2b8dcddcdcfa72ff5e8d5cfb261ae9dbe64b46"},
	{Shader: "mesh-shader-points", File: "wgsl-mesh-shader-points.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.ms_main.spvasm", Kind: deviationUnsupported, Card: "FEAT-SPV-PIPELINE-001",
		Reason:        reasonSPVPerEntryPoint,
		ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.fs_main.spvasm", Kind: deviationUnsupported, Card: "FEAT-SPV-PIPELINE-001",
		Reason:        reasonSPVPerEntryPoint,
		ErrorContains: "per-entry-point SPIR-V output"},
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.hlsl", Kind: deviationBug, Card: "BUG-HLSL-BUILTIN-SEMANTIC-001",
		Reason:      "mesh outputs: triangle_indices gets SV_Position (BuiltInToSemantic fallback) where Rust writes no semantic; " + notPorted("a53ffffe"),
		Fingerprint: "318e7e6e2d5374f21d5da744caf73320cf0b8456160b67e1b5ed865b7ce46ed1"},
	{Shader: "msl-9683-multiple-returns", File: "wgsl-msl-9683-multiple-returns.metal", Kind: deviationBug, Card: "BUG-MSL-MULTI-RETURN-001",
		Reason:      "two returns in a fragment entry point both declare `const auto _tmp`: a redefinition, invalid MSL; " + notPorted("0892b384"),
		Fingerprint: "91432dba0859fa834025ce1251364f04a30e25ddfd16eebfa2edafd2d559d099"},
	{Shader: "packed-vec3-bitcast", File: "wgsl-packed-vec3-bitcast.metal", Kind: deviationBug, Card: "BUG-MSL-PACKED-BITCAST-001",
		Reason:      "bitcast of a packed_int3 struct member uses as_type on the packed value without first converting it to int3",
		Fingerprint: "4e7aa2c76801214ed43713a3d8d407cdfa27e499c9b37f38ee7919785cb4a31e"},
	{Shader: "per-vertex", File: "wgsl-per-vertex.spvasm", Kind: deviationBug, Card: "BUG-PER-VERTEX-001",
		Reason:      "@interpolate(per_vertex) is dropped: no PerVertexKHR decoration or FragmentBarycentricKHR capability",
		Fingerprint: "274e0a4708f507cb7c219c02c53cf0d4d13409282e065a50513677f0994ca46b"},
	{Shader: "per-vertex", File: "wgsl-per-vertex.metal", Kind: deviationBug, Card: "BUG-PER-VERTEX-001",
		Reason:      "@interpolate(per_vertex) is dropped: the array is interpolated (center_perspective) instead of read with vertex_value<T>::get",
		Fingerprint: "5752b8e262f59b30653ee79ab87c2e1d714e008aa67fb8d402d241c465e94089"},
	{Shader: "per-vertex", File: "wgsl-per-vertex.hlsl", Kind: deviationBug, Card: "BUG-PER-VERTEX-001",
		Reason:      "@interpolate(per_vertex) is dropped: an interpolated float[3] input instead of nointerpolation + GetAttributeAtVertex",
		Fingerprint: "f50cf0d71348f753961b6bae88abea1087304ae4c8a9ce1ac5b11e75d9eb5a37"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "workgroup zero-initialization tests the shader's LocalInvocationId input where Rust adds a LocalInvocationIndex input (counts-level SPIR-V: one decoration and one entry point interface variable fewer); " + notPorted("96b7872f"),
		Fingerprint: "66059db3a1bc8fcad16a27f2c2c907acb5d0682ce9cc8e32fe5a305ca4ba5a26"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.metal", Kind: deviationBug, Card: "BUG-MSL-WGULOAD-ATOMIC-001",
		Reason:      "workgroupUniformLoad(&atomic) reads the atomic object as a plain value (uint x = wg) instead of atomic_load_explicit, invalid MSL; the array type is named type_2 instead of type_4 (type arena order); zero-init through local_invocation_id; " + reasonMSLThreadgroup + "; " + notPorted("96b7872f"),
		Fingerprint: "50c48ae55069c0361d6c5660c2f71322c986695f7b159eebeadac403acbb9b04"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.test_atomic_workgroup_uniform_load.Compute.glsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      "atomicStore on a workgroup atomic struct member is a plain assignment instead of atomicExchange; " + notPorted("f0201c9c"),
		Fingerprint: "5f10ec5848ca82a9679b453fcceb1122a1a3ab0e589c30a05523f1650d5e60ea"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.hlsl", Kind: deviationBug, Card: "BUG-ATOMIC-POLYFILL-001",
		Reason:      "atomicStore on a workgroup atomic struct member is a plain assignment instead of InterlockedExchange; zero-init through SV_GroupThreadID; " + notPorted("f0201c9c", "96b7872f"),
		Fingerprint: "06d149dbf4e35b34b49fc8143a47ce7a1e3f579b9c37a3e4d0440ff4b8d9f301"},
}

// unsupportedShader records a corpus shader we cannot lower yet. None of its
// outputs is compared; lowering must fail with ErrorContains.
type unsupportedShader struct {
	Reason        string
	Card          string
	ErrorContains string
}

// rustUnsupportedShaders lists the Rust corpus shaders that fail to lower.
// A listed shader that lowers, or fails differently, fails the tests that
// lower it, so an entry cannot outlive its cause.
var rustUnsupportedShaders = map[string]unsupportedShader{
	"cooperative-matrix": {Card: "PARITY-UPSTREAM-001",
		Reason:        "cooperative matrix types (coop_mat8x8<f32, A>, enable wgpu_cooperative_matrix) are not parsed",
		ErrorContains: "line 12, column 37: expected ;, got ,"},
	"cooperative-matrix-mixed-accumulator": {Card: "PARITY-UPSTREAM-001",
		Reason:        "cooperative matrix types (coop_mat8x8<f32, A>, enable wgpu_cooperative_matrix) are not parsed",
		ErrorContains: "line 12, column 58: expected ;, got ,"},
	"debug-printf": {Card: "PARITY-UPSTREAM-001",
		Reason:        "debugPrintf with a string literal format argument is not parsed",
		ErrorContains: "line 5, column 17: unexpected token Error in expression"},
	"int16": {Card: "PARITY-UPSTREAM-001",
		Reason:        "16-bit integer types (i16, u16) are not supported",
		ErrorContains: "unknown type: i16"},
	"ray-tracing-pipeline": {Card: "PARITY-UPSTREAM-001",
		Reason:        "ray tracing pipeline stages and traceRay are not supported",
		ErrorContains: "unknown function: traceRay"},
}

// lowerCorpusShader lowers a corpus shader. ok is false when the shader is a
// recorded unsupported shader; the recorded error is checked and any problem
// reported on t. Other lowering errors of Rust shaders are fatal: Rust lowers
// every corpus shader.
func lowerCorpusShader(t *testing.T, shader shaderFile) (*ir.Module, bool) {
	t.Helper()
	module, err := tryCompileToIR(shader.source)
	u, unsupported := rustUnsupportedShaders[shader.name]
	if !unsupported || shader.origin != originRust {
		if err != nil {
			t.Fatalf("lowering failed: %v", err)
		}
		return module, true
	}
	switch {
	case err == nil:
		t.Errorf("%s now lowers: remove it from rustUnsupportedShaders (%s)", shader.name, u.Card)
	case !strings.Contains(err.Error(), u.ErrorContains):
		t.Errorf("%s: lowering error changed (rustUnsupportedShaders expects %q): %v", shader.name, u.ErrorContains, err)
	default:
		t.Logf("unsupported shader (%s): %s", u.Card, u.Reason)
	}
	return nil, false
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
