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
}
