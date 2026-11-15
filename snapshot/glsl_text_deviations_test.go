package snapshot_test

// glslTextDeviations records every way our GLSL-sourced MSL and HLSL text
// knowingly differs from the pinned naga-cli's derived goldens (design §18.2).
// The entries use the WGSL corpus's machinery: a fingerprint pins the exact
// pair of texts, so a difference that changes — for the better or the worse —
// fails the test and must be re-reviewed, and an entry that stops applying
// fails it too. A difference is a frontend or a backend defect until the entry
// says why it is not.
var glslTextDeviations = []rustDeviation{
	{Shader: "images.frag", File: "glsl-images.frag.hlsl", Kind: deviationIntentional,
		Reason: "the pinned naga writes NagaRWDimensions2D twice, because its IR gives the " +
			"`writeonly readonly` storage image imgWriteReadOnly an empty StorageAccess " +
			"(verified in its SPIR-V: both NonReadable and NonWritable decorate the variable) and its " +
			"wrapper key carries the access. We normalize the empty access to LOAD|STORE — ADR-010 E2 " +
			"makes an empty set 'unset', which the validator refuses — so one wrapper serves both images. " +
			"The duplicate definitions have the same name and parameter type: dxc rejects the Rust golden " +
			"with \"redefinition of 'NagaRWDimensions2D'\", ours compiles",
		Fingerprint: "787ab539ca587aaf73307a99c588e7f48bc9f81302fc8b540920434f841126ad"},
	{Shader: "896-push-constant.frag", File: "glsl-896-push-constant.frag.metal", Kind: deviationIntentional,
		Reason:      reasonGLSLNamedTypeKept,
		Fingerprint: "5509a181b92b15eac787c314f6c44fdfc4e73f34e586637e2a8d00891fa111a4"},
	{Shader: "896-push-constant.frag", File: "glsl-896-push-constant.frag.hlsl", Kind: deviationIntentional,
		Reason:      reasonGLSLNamedTypeKept,
		Fingerprint: "f66e62875904b5ff7308cd85791414d548c47b9a38d7e95a485b88eb1edee57e"},
	{Shader: "bevy-pbr.frag", File: "glsl-bevy-pbr.frag.metal", Kind: deviationIntentional,
		Reason:      reasonGLSLStructOrder,
		Fingerprint: "f330282ae3a331d6e58cd99dddaa88cebde2a53324396fae4fcf728515541b2b"},
	{Shader: "bevy-pbr.frag", File: "glsl-bevy-pbr.frag.hlsl", Kind: deviationIntentional,
		Reason:      reasonGLSLStructOrder,
		Fingerprint: "b0466f229d62a7c6b76bedf40365ae12e863d62b39fccace6f9a8c520d582324"},
}

// Reason texts shared by deviations above.
const (
	// reasonGLSLNamedTypeKept covers 896-push-constant.frag: the unused
	// push-constant global is removed, the block's named type is not.
	reasonGLSLNamedTypeKept = "our compaction keeps every named type — the typifier resolves the " +
		"predeclared modf/frexp result structs by name until they are modeled (card P2-IR-034) — so the " +
		"block's type survives after its unused global is removed, and the backend declares the struct; Rust " +
		"removes the unused named type under KeepUnused::No. The declaration is unreferenced and valid; it " +
		"disappears with P2-IR-034"
	// reasonGLSLStructOrder covers bevy-pbr.frag: same declarations, and
	// their order differs.
	reasonGLSLStructOrder = "struct declarations follow the type arena, Rust names them on first use; the " +
		"retained named types of the module (card P2-IR-034) put CameraViewProj before CameraPosition. The " +
		"structs, their members and every use are identical, only the declaration order differs"
)
