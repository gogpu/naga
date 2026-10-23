package snapshot_test

import (
	"fmt"
	"strings"
)

// upstreamChanges names the gfx-rs/wgpu commits whose effect on the vendored
// Rust outputs we have not ported. Each was found by walking the history of
// an output that differs: the commits after the last upstream revision equal
// to the reference we previously vendored (TEST-CORPUS-001, sync to 14328f49).
var upstreamChanges = map[string]string{
	"1ef831ad": "handle all type resolution in the lowerer",
	"1f4ae673": "[naga test] Run hlsl_mat_cxN.wgsl snapshot tests on SPIR-V too",
	"24cee9e4": "test(naga): add runtime_{f,i}32 to math-functions' sign coverage",
	"2ef10891": "Fix constant evaluation of sign(0.0f) (#8942)",
	"35975143": "[naga spv-out] Handle 2-row matrices in uniform buffers",
	"4c1ea47e": "fix(dx12): Fix textureNum{Levels,Layers,Samples} functions (#9542)",
	"4d11f5f8": "Made traces use .metal extension instead of .msl (#8880)",
	"4e7ffe09": "Workaround Metal driver bug with atomic texture writes (#9185)",
	"55534165": "[spv-out] Some small ray query fixes (#9731)",
	"56e4a389": "[spv-out] ray tracing pipelines (#9085)",
	"6604345c": "fix(naga msl-out): Fix invalid MSL when vertex data and shader type are both f16 (#9166)",
	"6bca1014": "Various SPIR-V fixes for mesh shaders (#8756)",
	"74827c41": "[[max_total_threads_per_threadgroup]] MSL attribute (#9327)",
	"7721a3e0": "[spv-out] handle rayQueryTerminate correctly (#8581)",
	"786b8b11": "[hlsl] handle matCx2<f16> in the uniform address space properly",
	"948b1a6c": "Remove unnecessary object data threadgroup barriers in MSL writer (#9396)",
	"96b7872f": "Fix local_invocation_id and local_invocation_index handling (#9099)",
	"a03be8d2": "[msl] Make ray queries safer (#9442)",
	"a071a4d5": "firstLeadingBit returns signed semantics result (#9299)",
	"a53ffffe": "Mesh shader HLSL writer (#8752)",
	"ae19a470": "fix(hlsl-out): cast float types back from sign's output",
	"b051a87e": "fix(naga): Require explicit flat interpolation for integer shader I/O",
	"b5bd5aed": "Mesh shaders MSL writer (#8739)",
	"b9c09622": "fix(naga): Fix macOS 10.12 nested initializers",
	"bf5c6cff": "Implement bindless buffers on metal (#9081)",
	"c97fe3e7": "[HLSL] Make ray queries safer (#8763)",
	"cb02f74a": "fix(naga): avoid vector splat construction from invalid scalar type (#8829)",
	"cdc9d0c4": "fix(glsl-out): avoid accidental predecrement in atomicSub negation (#9924)",
	"de9f9f4d": "fix(naga msl-out): Support sampler lod_clamp & max_anisotropy (#9842)",
	"f0201c9c": "Fix: Polyfill atomic load/store correctly in GLSL/HLSL (#9242) (#9387)",
	"f6d9a1db": "[msl-out] Switch to intersection queries (#9304)",
	"fc9060f5": "BarycentricNoPerspective (#8852)",
	"fdd903e6": "Add supported caps info to naga backends (#9068)",
	"0892b384": "fix(naga msl-out): Fix identifier collision for multiple fragment returns (#9684)",
	"10fe6409": "Partial snorm10-10-10-2 vertex format support (#10226)",
}

// notPorted is the reason of a deviation that consists of upstream changes
// we have not ported. An unknown commit is a programming error.
func notPorted(commits ...string) string {
	parts := make([]string, len(commits))
	for i, c := range commits {
		subject, ok := upstreamChanges[c]
		if !ok {
			panic(fmt.Sprintf("notPorted: commit %s is not in upstreamChanges", c))
		}
		parts[i] = c + " " + subject
	}
	return "upstream changes not ported: " + strings.Join(parts, "; ")
}
