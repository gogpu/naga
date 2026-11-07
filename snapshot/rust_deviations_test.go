package snapshot_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
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
	reasonSPVWorkgroupLayoutFree = "workgroup variables use layout-free types (VUID-StandaloneSpirv-None-10684), " +
		"copied to and from the decorated types with OpCopyLogical from SPIR-V 1.4 and with a member-wise " +
		"function below it, as Tint does; Rust uses the decorated types there"
	reasonSPVUnusedUniformPointers = "Rust declares the type of every expression it caches, also where nothing uses it, " +
		"so it has a pointer to the regular type of each uniform buffer it declares std140-compatible; " +
		"nothing uses it and we don't declare it"
	reasonCompactNoEntryPoints = "the module has no entry points; Rust's compact(KeepUnused::No) removes " +
		"everything, irutil.CompactUnused returns early and every type and function is emitted"
	reasonGLSLStd140MatCx2 = "2-row matrices in uniform buffers are declared std140-compatible, a struct's matrix " +
		"member as one vec2 member per column and any other such matrix as a struct of its columns, rebuilt on " +
		"load: std140 puts matrix columns 16 bytes apart where WGSL puts a matCx2's 8 apart, so Rust's GLSL, " +
		"which declares the matrix as is, reads the wrong offsets (its SPIR-V writer declares the same " +
		"std140-compatible types, 35975143)"
	reasonInt16SpecClamp = "i16(f16(33000.0)) is 32752, the WGSL spec's float to integral conversion (the " +
		"value of i16 closest to the truncated value that f16 represents exactly); Rust clamps to 32767 " +
		"(i16::MAX as f32, constant_evaluator.rs cast), which no f16 holds. Intentional"
	reasonHLSLAtomic64Polyfill = "atomicStore and atomicLoad of a 64-bit atomic in a storage buffer are written " +
		"InterlockedExchange64 and InterlockedOr64; Rust (f0201c9c) writes the 32-bit InterlockedExchange and " +
		"InterlockedOr, which DXC accepts with \"conversion from larger type 'uint64_t' to smaller type " +
		"'unsigned int &'\", cutting the value to 32 bits"
)

// rustDeviations is the complete list of known differences from the vendored
// Rust outputs, each verified against its diff when added. Entries are
// removed when fixed: TestRustReference fails on an entry that no longer
// describes a difference.
var rustDeviations = []rustDeviation{
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
		Fingerprint: "afa5c31789b54342a4002bdb5d83260f46b7563d38105af05d9edae65f2d592a"},
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
	{Shader: "access", File: "wgsl-access.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      reasonSPVUnusedUniformPointers + "; " + notPorted("1f4ae673"),
		Fingerprint: "c6b9d70188106f954299f1bbe3c5d6947f69679590ecd6fbb41e14a2f484f145"},
	{Shader: "access", File: "wgsl-access.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "cbe4bf60b8c5171e70c1bf30a6bbb200b181a60201d773c6ce227b04e12c1e97"},
	{Shader: "access", File: "wgsl-access.hlsl", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      notPorted("786b8b11"),
		Fingerprint: "68e371f863469673cd98e0c02a4df13f11b910f88f8664a74d88c9c3820e1fec"},
	{Shader: "access", File: "wgsl-access.foo_vert.Vertex.glsl", Kind: deviationIntentional,
		Reason:      reasonGLSLStd140MatCx2,
		Fingerprint: "a9fc778ce836d7e82dcc04a0c449ffd2250246f8509aefaf83865c9263527bbe"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("7721a3e0", "56e4a389"),
		Fingerprint: "2bd5dc71a566e16bd4b835879e407493894aaff5077842c0a98fc5d3aaacf346"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("f6d9a1db", "a03be8d2"),
		Fingerprint: "5ee0f8c8d0a582059adeb38abf7a5e8b28290bee0e6f50b2205517bef193966c"},
	{Shader: "aliased-ray-query", File: "wgsl-aliased-ray-query.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "2422fa4c08e1bbcbb0716089bb6df75210b746dbbb062d6402370fb970e68ea9"},
	{Shader: "array-in-function-return-type", File: "wgsl-array-in-function-return-type.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "0dfb944bc3f437a26b8d5ab090c44d1cc4ca25846d8f1fb55a5c48deebf62850"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "d914db5e93c4bbc1eb2f380ac064ebb4b1f22d36a25e6dee05674cd56b47e541"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.test_atomic_compare_exchange_i32.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "4a319f8c7a931b27b73abe4104548f0ea6191f996cdd7bf185d9b22df8987e1b"},
	{Shader: "atomicCompareExchange", File: "wgsl-atomicCompareExchange.test_atomic_compare_exchange_u32.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "7033dbc397cdaa7c2b7d44b9369f9c8721be51c9528e3b1ef1e1be61c17a4d56"},
	{Shader: "atomicCompareExchange-int64", File: "wgsl-atomicCompareExchange-int64.hlsl", Kind: deviationIntentional,
		Reason:      reasonHLSLAtomic64Polyfill,
		Fingerprint: "94775201f4c625a2d9327c9607fde567f1089043cd9389b714163ec79f7bfad1"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("6bca1014"),
		Fingerprint: "5a5740115f1d98e6eea1e1fd3893fc80441419db1fad9872ae7ca95b0b54d3d8"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("1ef831ad", "b5bd5aed"),
		Fingerprint: "5ae02019a950cc49da288cb2bd4db399e82c46406e16ab96f08ea3003718c940"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.cs_main.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "ba2fcc995b9f98c4ca57cecd6dbe2ec674846769e38941cafb4cc43121f8e096"},
	{Shader: "atomicOps", File: "wgsl-atomicOps.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "9052add8214188636f704890426a946e3b6af43d2dfc95484e6cc3089e6b5cd4"},
	{Shader: "atomicOps-float32", File: "wgsl-atomicOps-float32.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("1ef831ad", "b5bd5aed", "948b1a6c"),
		Fingerprint: "1d9b1ecfbb4725b5dbf7d2a2c245228bd4f0f87b5e12fa01fb1535c62b466dc2"},
	{Shader: "atomicOps-int64", File: "wgsl-atomicOps-int64.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad", "6bca1014"),
		Fingerprint: "3dc37943ef5ffc2a0f7dabc54a94a4eee2e765f1a6f16143a676d0782202669b"},
	{Shader: "atomicOps-int64", File: "wgsl-atomicOps-int64.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe") + "; intentional: " + reasonHLSLAtomic64Polyfill,
		Fingerprint: "966c5b9610a0174b677408381ae29da091afc066d0130b6dfb849cc3d2f1d0d8"},
	{Shader: "atomicOps-int64-min-max", File: "wgsl-atomicOps-int64-min-max.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "a19eecfb52715dd0d2234932136120b4fde7855f864aa4b145171e7f1f022dad"},
	{Shader: "atomicTexture", File: "wgsl-atomicTexture.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4e7ffe09"),
		Fingerprint: "4a37ae746a55281b9034e2698d7313918ab32f76e8427d3e95b345a976b967d0"},
	{Shader: "atomicTexture-int64", File: "wgsl-atomicTexture-int64.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("4e7ffe09", "b5bd5aed", "948b1a6c"),
		Fingerprint: "eb24e4def4b80b992afb83ef997a52aba107be779269cd47057d420a8027c0cf"},
	{Shader: "binding-arrays", File: "wgsl-binding-arrays.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4c1ea47e"),
		Fingerprint: "3f428374ad2d8a403b042f9cfcbea7a1fcbff6ed7b4b23f96e2a954a82604382"},
	{Shader: "bits", File: "wgsl-bits.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("a071a4d5"),
		Fingerprint: "9cfef2120d2f0b2fce14982899ee291fb48932f705478b332121a3062801c15e"},
	{Shader: "bits-optimized-msl", File: "wgsl-bits-optimized-msl.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("a071a4d5"),
		Fingerprint: "618a4906769cfa97da290400137ba709c498ebff905f4a8c409f21b0c1949347"},
	{Shader: "bounds-check-zero-atomic", File: "wgsl-bounds-check-zero-atomic.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("1ef831ad"),
		Fingerprint: "1ee4bb2ed5e24d053be0e31840f3b6b368831465ded20b16f201416e0d8c7f68"},
	{Shader: "const-exprs", File: "wgsl-const-exprs.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "03d7bfc1ae796b58e8d2bdcd84e31f708c2bd576d48390aab33d427a442ec0f9"},
	{Shader: "constructors", File: "wgsl-constructors.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b9c09622"),
		Fingerprint: "9cc4a8b6c5f9adf9e3d72894fd457e33380f644e177428d15c8653baca43d2cd"},
	{Shader: "f16", File: "wgsl-f16.spvasm", Kind: deviationIntentional,
		Reason: reasonSPVUnusedUniformPointers + "; the f16 matrices of three and four rows are declared std140-compatible " +
			"too: WGSL puts their columns 8 bytes apart, which a Uniform block holds only with uniformBufferStandardLayout, " +
			"and Rust declares two-row matrices std140-compatible only",
		Fingerprint: "2c79ac76e502eb50e6d0fde1cf925315453722b3516c573c244662febff50dfb"},
	{Shader: "fragment-output", File: "wgsl-fragment-output.metal", Kind: deviationBug, Card: "BUG-MSL-MULTI-RETURN-001",
		Reason:      notPorted("0892b384"),
		Fingerprint: "ccad7255547e07f36cbd6fb5f32afef0211fff5a29b7f972466c36cf87454d25"},
	{Shader: "fragment-output", File: "wgsl-fragment-output.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("b051a87e"),
		Fingerprint: "a0f26490b1acc6d644f85bd2379f22727bf0cd4e6440b2ada79e74a18e2ad0a5"},
	{Shader: "globals", File: "wgsl-globals.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      reasonSPVUnusedUniformPointers + "; " + notPorted("6bca1014"),
		Fingerprint: "37fa83b46ae4ba3d2d1e093f15fa12dedaa3770177fbc8ccbf4dc3e392949b55"},
	{Shader: "globals", File: "wgsl-globals.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "87877e2771f0c8885fd9a990e6be655d147090486e3a043d129c8bf63c9dd8e3"},
	{Shader: "globals", File: "wgsl-globals.main.Compute.glsl", Kind: deviationIntentional,
		Reason:      reasonGLSLStd140MatCx2,
		Fingerprint: "d7b8849c22985847ee758e4f0a4e87927d119fc39f27a0e6d871c0285465ec34"},
	{Shader: "globals", File: "wgsl-globals.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe", "786b8b11"),
		Fingerprint: "ad4ba511e32c8464d0f215bdef99bcc778e32232039d4d97dce1cc6ec47eed83"},
	{Shader: "image", File: "wgsl-image.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("4c1ea47e"),
		Fingerprint: "c19c6964927501d578ff313a597cdccf644c87da5700826e1f5750beb1daa583"},
	{Shader: "interface", File: "wgsl-interface.metal", Kind: deviationBug, Card: "BUG-MSL-MULTI-RETURN-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed", "0892b384"),
		Fingerprint: "d609cf205b92964217e9b83c69c71091f3ce0c1fad2ee57730fc7d015b5e5932"},
	{Shader: "interface", File: "wgsl-interface.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "2e5af9196b9127bb1483338536bd38f47d27f5c5ba988b3236f18b36889b0dc3"},
	{Shader: "overrides-atomicCompareExchangeWeak", File: "wgsl-overrides-atomicCompareExchangeWeak.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "f483af7caf2f9f3c2df44d835e3d6cb29ef25f58beac51d988bc26cd301e389c"},
	{Shader: "overrides-ray-query", File: "wgsl-overrides-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("f6d9a1db", "a03be8d2"),
		Fingerprint: "203cb193872a4dd209b7426eebe8d733d494f0b619e8ee12f449ff4a03fef6d8"},
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
		Reason:      notPorted("7721a3e0", "56e4a389"),
		Fingerprint: "8d784c87f0e1072cf686b138fc0ad47e0d648e9e0ef6bae19074319722662000"},
	{Shader: "ray-query", File: "wgsl-ray-query.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("f6d9a1db", "a03be8d2"),
		Fingerprint: "2fbbf4a3983b4a6363977f4c9be80bddc543c6982decd60afded0b14ec796d7d"},
	{Shader: "ray-query", File: "wgsl-ray-query.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "cb8f0d2a54d6b5df257442587287064504e6ed8dbd31124f13892a8c41539c71"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("7721a3e0", "56e4a389"),
		Fingerprint: "8d784c87f0e1072cf686b138fc0ad47e0d648e9e0ef6bae19074319722662000"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("f6d9a1db", "a03be8d2"),
		Fingerprint: "6fe7d556f00ef07ef4388e92af1aa8c7843a7e394ba7196688a9220123c06c12"},
	{Shader: "ray-query-no-init-tracking", File: "wgsl-ray-query-no-init-tracking.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("c97fe3e7"),
		Fingerprint: "c519dd006261c4371354d5e4b1cc6c89e0bb97ce67589151e244b01c15197232"},
	{Shader: "skybox", File: "wgsl-skybox.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      notPorted("de9f9f4d"),
		Fingerprint: "795c7cd20304501fecb72cfa2f361f721a1c45b96d1e287e2379cfc2bf9c3356"},
	{Shader: "int16", File: "wgsl-int16.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonInt16SpecClamp + "; " + reasonMSLThreadgroup + "; " + notPorted("b5bd5aed", "b9c09622"),
		Fingerprint: "f7443a6b9b8ed7c0f9a30383012e2fcf5d1f4ad5e54b6effda30bf10e7816ce6"},
	{Shader: "int16", File: "wgsl-int16.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonInt16SpecClamp + "; " + notPorted("96b7872f"),
		Fingerprint: "19ab2200df79643455b3b46fac5e1da216b29b17fdc25e02dda51fb6eb6898cb"},
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
		Fingerprint: "0244b2eceaa063ee8301cb20e54be78ce98d49c0f88743c89b65057fa534e730"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.metal", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      reasonMSLThreadgroup + "; " + notPorted("b5bd5aed"),
		Fingerprint: "b66a14becea1eb9fa9f71d61289df82652660c58e999a131e70fcdc9f3b00c38"},
	{Shader: "workgroup-var-init", File: "wgsl-workgroup-var-init.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      notPorted("96b7872f", "a53ffffe"),
		Fingerprint: "33590ccad2599a2e7d0001ed960e7588967153a07302680f8d01991f8a736362"},

	// Corpus sync to 14328f49: new outputs and outputs whose input changed,
	// each reviewed against its full difference.
	{Shader: "binding-buffer-arrays", File: "wgsl-binding-buffer-arrays.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "upstream extended the shader (nested struct members, runtime-sized binding_array, plain storage buffer); counts-level SPIR-V: Rust declares 4 more StorageBuffer pointer types",
		Fingerprint: "9205f551a8e2f6a9a52a4ccc19e8c07bcefc4dd3f1ed8b9b533919e75a72887d"},
	{Shader: "binding-buffer-arrays", File: "wgsl-binding-buffer-arrays.metal", Kind: deviationUnsupported, Card: "P1-MSL-101",
		Reason:        "buffer binding arrays: Rust passes NagaArgumentBufferWrapper<device Foo*>* with per-element buffer sizes (size0[10]); we pass a constant wrapper reference with no sizes, so the bounds check of `far` has no size to use (before P0-MSL-100 it read `1 + 0` and returned zero for every element but the first); " + notPorted("bf5c6cff"),
		ErrorContains: "runtime-sized array inside binding_array is not supported"},
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
	{Shader: "9890-no-rq-hoist", File: "wgsl-9890-no-rq-hoist.metal", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "ray queries: we still use the intersector-based _RayQuery wrapper without initialization tracking or ray validity checks; " + notPorted("f6d9a1db", "a03be8d2"),
		Fingerprint: "ea500c8338ad7aa95a10666ca4ad366142ad51614e428ad9c1870e1d8e52e582"},
	{Shader: "9890-no-rq-hoist", File: "wgsl-9890-no-rq-hoist.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "ray query helpers are emitted in a different place and without the safety checks; " + notPorted("c97fe3e7"),
		Fingerprint: "68d6dee675e5b829732c5eef5f971ed39c368182d17accfed6baeb477003fa52"},
	{Shader: "int-div-unchecked", File: "wgsl-int-div-unchecked.spvasm", Kind: deviationBug, Card: "TEST-TOML-KEYS-001",
		Reason:      "[spv] emit_int_div_checks = false is not supported: we always emit the naga_div/naga_mod helpers",
		Fingerprint: "1bac666dcd8d28ffce5f4fe3ee8384a022e53fe0669aa9ee3b85a38b6896b644"},
	{Shader: "int-div-unchecked", File: "wgsl-int-div-unchecked.metal", Kind: deviationBug, Card: "TEST-TOML-KEYS-001",
		Reason:      "[msl] emit_int_div_checks = false is not supported: we always emit the naga_div/naga_mod helpers",
		Fingerprint: "4cfb5183e47d7e2edf8866a5e5fc2d9411db1881d8dfb089d91fb18d3772334d"},
	{Shader: "mat_cx2", File: "wgsl-mat_cx2.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      reasonSPVUnusedUniformPointers + "; " + notPorted("1f4ae673"),
		Fingerprint: "2f976ffc231c30d0a8d7680661da1d668835c312d22b6c536adba9241e11a99c"},
	{Shader: "mat_cx2", File: "wgsl-mat_cx2.hlsl", Kind: deviationBug, Card: "BUG-UNIFORM-MAT2-001",
		Reason:      "mat2x2 helpers are named __mat2x2 instead of __mat2x2_f32, and arrays of mat2x2 in a cbuffer are indexed directly instead of through the column helpers (wrong cbuffer layout)",
		Fingerprint: "75f1a125fc76a394bc91ca390afe139cce7c22fc71f005b19aa76c4b975e3b99"},
	{Shader: "mat_cx2_f16", File: "wgsl-mat_cx2_f16.spvasm", Kind: deviationIntentional,
		Reason:      reasonSPVUnusedUniformPointers,
		Fingerprint: "fa004ff5b57449f383eaef09de1f8e7a6627a9c4808bab5716282aa6489c4e70"},
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
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes coherent device",
		Fingerprint: "f862f87ddada4d42474b79a4f0c95e8070c4b146d0ae9bddb97a3c4a7093ee33"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.main.Compute.glsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes a coherent buffer qualifier",
		Fingerprint: "4717b0e58914928931955b4f4488cad8875f5080e94faaa4577de9eb88acb35a"},
	{Shader: "memory-decorations-coherent", File: "wgsl-memory-decorations-coherent.hlsl", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "@coherent memory decoration is not supported (parsed and dropped): Rust writes globallycoherent",
		Fingerprint: "34dd72cec082c83049c91a3ca7f80e3e6a2d7379415a97aa20e8ff3478bfcc9d"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.spvasm", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "SPIR-V mesh shaders: the fragment entry point's @per_primitive input (PerPrimitiveEXT) is refused; " + notPorted("6bca1014"),
		ErrorContains: "per_primitive is not supported"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.metal", Kind: deviationUnsupported, Card: "PARITY-UPSTREAM-001",
		Reason:        "the msl package has no task/mesh stages; " + notPorted("b5bd5aed"),
		ErrorContains: "unsupported shader stage"},
	{Shader: "mesh-shader", File: "wgsl-mesh-shader.hlsl", Kind: deviationBug, Card: "BUG-HLSL-BUILTIN-SEMANTIC-001",
		Reason:      "mesh outputs: triangle_indices and cull_primitive get SV_Position (BuiltInToSemantic fallback); " + notPorted("a53ffffe"),
		Fingerprint: "fe3824f47fbf116ba16fc85f6e92d3aefe8472bf56f42bdf5c4c10f66269afb0"},
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
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.ms_main.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "SPIR-V mesh shaders: no MeshShadingEXT, the module has no mesh entry point interface; " + notPorted("6bca1014"),
		Fingerprint: "9bcea5cba24289cab32d49b14087d13417a6216b470917dd33e3715d9704e636"},
	{Shader: "primitive-index-mesh-shader", File: "wgsl-primitive-index-mesh-shader.fs_main.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "SPIR-V mesh shaders: no MeshShadingEXT, the module has no mesh entry point interface; " + notPorted("6bca1014"),
		Fingerprint: "fd27dd5559f743eca48d6e04fd3df066eda7ff56f3571d5fdb011cdde7bf40eb"},
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
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.spvasm", Kind: deviationBug, Card: "PARITY-UPSTREAM-001",
		Reason:      "workgroup zero-initialization tests the shader's LocalInvocationId input where Rust adds a LocalInvocationIndex input (counts-level SPIR-V: one decoration and one entry point interface variable fewer); " + notPorted("96b7872f"),
		Fingerprint: "66059db3a1bc8fcad16a27f2c2c907acb5d0682ce9cc8e32fe5a305ca4ba5a26"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.metal", Kind: deviationBug, Card: "BUG-MSL-WGULOAD-ATOMIC-001",
		Reason:      "workgroupUniformLoad(&atomic) reads the atomic object as a plain value (uint x = wg) instead of atomic_load_explicit, invalid MSL; the array type is named type_2 instead of type_4 (type arena order); zero-init through local_invocation_id; " + reasonMSLThreadgroup + "; " + notPorted("96b7872f"),
		Fingerprint: "50c48ae55069c0361d6c5660c2f71322c986695f7b159eebeadac403acbb9b04"},
	{Shader: "workgroup-uniform-load-atomic", File: "wgsl-workgroup-uniform-load-atomic.hlsl", Kind: deviationBug, Card: "BUG-ZERO-INIT-INDEX-001",
		Reason:      "zero-init through SV_GroupThreadID; " + notPorted("96b7872f"),
		Fingerprint: "469371c141e9d3aa1f05d07116676322349456259b673300b3ef80ffe215c915"},
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
		ErrorContains: "12:37: expected ;, got ,"},
	"cooperative-matrix-mixed-accumulator": {Card: "PARITY-UPSTREAM-001",
		Reason:        "cooperative matrix types (coop_mat8x8<f32, A>, enable wgpu_cooperative_matrix) are not parsed",
		ErrorContains: "12:58: expected ;, got ,"},
	"debug-printf": {Card: "PARITY-UPSTREAM-001",
		Reason:        "debugPrintf with a string literal format argument is not parsed",
		ErrorContains: "5:17: unexpected token Error in expression"},
	"ray-tracing-pipeline": {Card: "PARITY-UPSTREAM-001",
		Reason:        "ray tracing pipeline stages, ray payload address spaces and traceRay are not supported",
		ErrorContains: "unknown address space: `ray_payload`"},
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
	// The recorded text is "<line>:<column>: <message>" for a parse error, so
	// an error that moves is a change too.
	described := ""
	if err != nil {
		described = err.Error()
		var pe *wgsl.ParseError
		if errors.As(err, &pe) {
			if loc, ok := pe.Location(shader.source); ok {
				described = fmt.Sprintf("%d:%d: %s", loc.Line, loc.Column, pe.Message)
			}
		}
	}
	switch {
	case err == nil:
		t.Errorf("%s now lowers: remove it from rustUnsupportedShaders (%s)", shader.name, u.Card)
	case !strings.Contains(described, u.ErrorContains):
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
