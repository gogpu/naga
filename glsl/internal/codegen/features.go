// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// Features represents required GLSL features as bitflags.
// Matches Rust naga's back::glsl::Features.
type Features uint32

const (
	FeatureBufferStorage         Features = 1 << 0
	FeatureArrayOfArrays         Features = 1 << 1
	FeatureDoubleType            Features = 1 << 2
	FeatureFullImageFormats      Features = 1 << 3
	FeatureMultisampledTextures  Features = 1 << 4
	FeatureMultisampledTexArrays Features = 1 << 5
	FeatureCubeTexturesArray     Features = 1 << 6
	FeatureComputeShader         Features = 1 << 7
	FeatureImageLoadStore        Features = 1 << 8
	FeatureConservativeDepth     Features = 1 << 9
	FeatureNoPerspective         Features = 1 << 11
	FeatureSampleQualifier       Features = 1 << 12
	FeatureClipDistance          Features = 1 << 13
	FeatureCullDistance          Features = 1 << 14
	FeatureSampleVariables       Features = 1 << 15
	FeatureDynamicArraySize      Features = 1 << 16
	FeatureMultiView             Features = 1 << 17
	FeatureTextureSamples        Features = 1 << 18
	FeatureTextureLevels         Features = 1 << 19
	FeatureImageSize             Features = 1 << 20
	FeatureDualSourceBlending    Features = 1 << 21
	FeatureInstanceIndex         Features = 1 << 22
	FeatureTextureShadowLod      Features = 1 << 23
	FeatureSubgroupOperations    Features = 1 << 24
	FeatureTextureAtomics        Features = 1 << 25
	FeatureShaderBarycentrics    Features = 1 << 26
	// FeaturePrimitiveIndex is gl_PrimitiveID in a fragment shader: core from
	// GLSL 1.50 and ES 3.20, below them from GL_ARB_geometry_shader4 and
	// GL_OES_geometry_shader (Rust naga's PRIMITIVE_INDEX).
	FeaturePrimitiveIndex Features = 1 << 27
	// FeatureDrawIndex is gl_DrawID, core in GLSL 4.60 and on older desktop
	// versions gl_DrawIDARB of GL_ARB_shader_draw_parameters. Rust naga has
	// no such feature: it writes gl_DrawID at every version, which no GLSL
	// below 4.60 declares.
	FeatureDrawIndex Features = 1 << 28
)

// featuresManager collects and writes required features.
type featuresManager struct {
	required Features
	// drawParameters is Options.DrawParameters: the host has
	// GL_ARB_shader_draw_parameters, which brings FeatureDrawIndex to every
	// desktop version.
	drawParameters bool
	// forImageLoadChecks are the features only the ImageLoad bounds checks
	// asked for, so the error for a missing one can name the policy.
	forImageLoadChecks Features
}

// featureNames are Rust naga's names of the features, in bit order, for
// error messages.
var featureNames = []struct {
	feature Features
	name    string
}{
	{FeatureBufferStorage, "BUFFER_STORAGE"},
	{FeatureArrayOfArrays, "ARRAY_OF_ARRAYS"},
	{FeatureDoubleType, "DOUBLE_TYPE"},
	{FeatureFullImageFormats, "FULL_IMAGE_FORMATS"},
	{FeatureMultisampledTextures, "MULTISAMPLED_TEXTURES"},
	{FeatureMultisampledTexArrays, "MULTISAMPLED_TEXTURE_ARRAYS"},
	{FeatureCubeTexturesArray, "CUBE_TEXTURES_ARRAY"},
	{FeatureComputeShader, "COMPUTE_SHADER"},
	{FeatureImageLoadStore, "IMAGE_LOAD_STORE"},
	{FeatureConservativeDepth, "CONSERVATIVE_DEPTH"},
	{FeatureNoPerspective, "NOPERSPECTIVE_QUALIFIER"},
	{FeatureSampleQualifier, "SAMPLE_QUALIFIER"},
	{FeatureClipDistance, "CLIP_DISTANCE"},
	{FeatureCullDistance, "CULL_DISTANCE"},
	{FeatureSampleVariables, "SAMPLE_VARIABLES"},
	{FeatureDynamicArraySize, "DYNAMIC_ARRAY_SIZE"},
	{FeatureMultiView, "MULTI_VIEW"},
	{FeatureTextureSamples, "TEXTURE_SAMPLES"},
	{FeatureTextureLevels, "TEXTURE_LEVELS"},
	{FeatureImageSize, "IMAGE_SIZE"},
	{FeatureDualSourceBlending, "DUAL_SOURCE_BLENDING"},
	{FeatureInstanceIndex, "INSTANCE_INDEX"},
	{FeatureTextureShadowLod, "TEXTURE_SHADOW_LOD"},
	{FeatureSubgroupOperations, "SUBGROUP_OPERATIONS"},
	{FeatureTextureAtomics, "TEXTURE_ATOMICS"},
	{FeatureShaderBarycentrics, "SHADER_BARYCENTRICS"},
	{FeaturePrimitiveIndex, "PRIMITIVE_INDEX"},
	{FeatureDrawIndex, "DRAW_INDEX"},
}

// String returns the names of the features joined by " | ", as Rust naga's
// Display for Features.
func (f Features) String() string {
	return strings.Join(f.Names(), " | ")
}

// checkAvailability returns an error naming the required features that
// version v lacks, as Rust naga's FeaturesManager::check_availability does:
// writing them anyway gives GLSL the driver rejects. A feature with only a
// core version is missing on every GLSL ES version; with an ES version too,
// it is missing below it.
func (fm *featuresManager) checkAvailability(v Version) error {
	var missing Features
	core := func(f Features, coreVersion int) {
		if fm.required&f != 0 && (v.ES || v.versionLessThan(coreVersion)) {
			missing |= f
		}
	}
	coreES := func(f Features, coreVersion, esVersion int) {
		if fm.required&f == 0 {
			return
		}
		if v.ES && v.versionLessThan(esVersion) || !v.ES && v.versionLessThan(coreVersion) {
			missing |= f
		}
	}
	coreES(FeatureComputeShader, 420, 310)
	coreES(FeatureBufferStorage, 400, 310)
	core(FeatureDoubleType, 150)
	coreES(FeatureCubeTexturesArray, 130, 310)
	coreES(FeatureMultisampledTextures, 150, 300)
	coreES(FeatureMultisampledTexArrays, 150, 310)
	coreES(FeatureArrayOfArrays, 120, 310)
	coreES(FeatureImageLoadStore, 130, 310)
	coreES(FeatureConservativeDepth, 130, 300)
	core(FeatureNoPerspective, 130)
	coreES(FeatureSampleQualifier, 400, 320)
	coreES(FeatureClipDistance, 130, 300)
	coreES(FeatureCullDistance, 450, 300)
	coreES(FeatureSampleVariables, 400, 300)
	coreES(FeatureDynamicArraySize, 400, 310)
	coreES(FeatureDualSourceBlending, 330, 300)
	coreES(FeatureSubgroupOperations, 430, 310)
	coreES(FeatureTextureAtomics, 420, 310)
	if v.isWebGL() {
		coreES(FeatureMultiView, 140, 300)
	} else {
		coreES(FeatureMultiView, 140, 310)
	}
	// Only desktop GLSL can query the number of samples and levels of an
	// image, so GLSL ES cannot bounds-check the sample or level of a
	// texelFetch either.
	core(FeatureTextureSamples, 150)
	core(FeatureTextureLevels, 130)
	coreES(FeatureImageSize, 430, 310)
	coreES(FeatureTextureShadowLod, 200, 300)
	if !fm.drawParameters {
		core(FeatureDrawIndex, 460)
	} else if v.ES {
		missing |= fm.required & FeatureDrawIndex // the ARB extension is desktop GLSL only
	}
	if missing == 0 {
		return nil
	}
	return &MissingFeaturesError{Version: v, Features: missing, ForImageLoadChecks: missing&fm.forImageLoadChecks != 0}
}

// MissingFeaturesError is Rust naga's Error::MissingFeatures: the entry
// point needs Features that Version lacks. ForImageLoadChecks reports that
// the ImageLoad bounds checks asked for some of them, so ImageLoad set to
// Unchecked avoids the error. glsl.Compile returns it as
// glsl.MissingFeaturesError.
type MissingFeaturesError struct {
	Version            Version
	Features           Features
	ForImageLoadChecks bool
}

func (e *MissingFeaturesError) Error() string {
	msg := fmt.Sprintf("GLSL %s doesn't support the required feature(s): %s", e.Version, e.Features)
	if e.ForImageLoadChecks {
		msg += " (the ImageLoad bounds checks need them; set BoundsCheckPolicies.ImageLoad to Unchecked " +
			"for this target, as wgpu-hal does below desktop GLSL 4.30)"
	}
	return msg
}

// Names returns Rust naga's names of the features, in bit order.
func (f Features) Names() []string {
	var names []string
	for _, n := range featureNames {
		if f&n.feature != 0 {
			names = append(names, n.name)
		}
	}
	return names
}

func (fm *featuresManager) request(f Features) {
	fm.required |= f
}

func (fm *featuresManager) contains(f Features) bool {
	return fm.required&f == f
}

// writeExtensions writes all required GL extension directives.
// Matches Rust naga's FeaturesManager::write.
func (fm *featuresManager) writeExtensions(w *Writer) {
	opts := w.options

	if fm.contains(FeatureComputeShader) && !opts.LangVersion.ES {
		w.WriteLine("#extension GL_ARB_compute_shader : require")
	}

	if fm.contains(FeatureBufferStorage) && !opts.LangVersion.ES {
		w.WriteLine("#extension GL_ARB_shader_storage_buffer_object : require")
	}

	if fm.contains(FeatureDoubleType) && !opts.LangVersion.ES && opts.LangVersion.versionLessThan(400) {
		w.WriteLine("#extension GL_ARB_gpu_shader_fp64 : require")
	}

	if fm.contains(FeatureCubeTexturesArray) {
		if opts.LangVersion.ES {
			w.WriteLine("#extension GL_EXT_texture_cube_map_array : require")
		} else if opts.LangVersion.versionLessThan(400) {
			w.WriteLine("#extension GL_ARB_texture_cube_map_array : require")
		}
	}

	if fm.contains(FeatureMultisampledTexArrays) && opts.LangVersion.ES {
		w.WriteLine("#extension GL_OES_texture_storage_multisample_2d_array : require")
	}

	if fm.contains(FeatureImageLoadStore) {
		if !opts.LangVersion.ES && opts.LangVersion.versionLessThan(420) {
			w.WriteLine("#extension GL_ARB_shader_image_load_store : require")
		}
	}

	if fm.contains(FeatureConservativeDepth) {
		if opts.LangVersion.ES {
			w.WriteLine("#extension GL_EXT_conservative_depth : require")
		} else if opts.LangVersion.versionLessThan(420) {
			w.WriteLine("#extension GL_ARB_conservative_depth : require")
		}
	}

	if (fm.contains(FeatureClipDistance) || fm.contains(FeatureCullDistance)) && opts.LangVersion.ES {
		w.WriteLine("#extension GL_EXT_clip_cull_distance : require")
	}

	if fm.contains(FeatureSampleVariables) && opts.LangVersion.ES {
		w.WriteLine("#extension GL_OES_sample_variables : require")
	}

	if fm.contains(FeatureMultiView) {
		if opts.LangVersion.ES && opts.LangVersion.isWebGL() {
			w.WriteLine("#extension GL_OVR_multiview2 : require")
		} else {
			w.WriteLine("#extension GL_EXT_multiview : require")
		}
	}

	if fm.contains(FeatureTextureSamples) {
		w.WriteLine("#extension GL_ARB_shader_texture_image_samples : require")
	}

	if fm.contains(FeatureTextureLevels) && !opts.LangVersion.ES && opts.LangVersion.versionLessThan(430) {
		w.WriteLine("#extension GL_ARB_texture_query_levels : require")
	}

	if fm.contains(FeatureDualSourceBlending) && opts.LangVersion.ES {
		w.WriteLine("#extension GL_EXT_blend_func_extended : require")
	}

	if fm.contains(FeatureInstanceIndex) && fm.drawParameters {
		w.WriteLine("#extension GL_ARB_shader_draw_parameters : require")
	}

	if fm.contains(FeatureTextureShadowLod) {
		w.WriteLine("#extension GL_EXT_texture_shadow_lod : require")
	}

	if fm.contains(FeatureSubgroupOperations) {
		w.WriteLine("#extension GL_KHR_shader_subgroup_basic : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_vote : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_arithmetic : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_ballot : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_shuffle : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_shuffle_relative : require")
		w.WriteLine("#extension GL_KHR_shader_subgroup_quad : require")
	}

	if fm.contains(FeatureTextureAtomics) {
		w.WriteLine("#extension GL_OES_shader_image_atomic : require")
	}

	if fm.contains(FeatureShaderBarycentrics) {
		w.WriteLine("#extension GL_EXT_fragment_shader_barycentric : require")
	}

	if fm.contains(FeaturePrimitiveIndex) {
		if opts.LangVersion.ES && opts.LangVersion.versionLessThan(320) {
			w.WriteLine("#extension GL_OES_geometry_shader : require")
		} else if !opts.LangVersion.ES && opts.LangVersion.versionLessThan(150) {
			w.WriteLine("#extension GL_ARB_geometry_shader4 : require")
		}
	}
}

// collectFeatures scans the module and entry point to determine required features.
// Matches Rust naga's Writer::collect_required_features.
func (w *Writer) collectFeatures() {
	ep := w.getSelectedEntryPoint()
	if ep == nil {
		return
	}

	// Compute shader
	if ep.Stage == ir.StageCompute {
		w.features.request(FeatureComputeShader)
	}

	// Writer flags can request features
	w.features.drawParameters = w.options.DrawParameters
	if w.options.TextureShadowLod {
		w.features.request(FeatureTextureShadowLod)
	}

	// Early depth test
	// Rust back/glsl/features.rs:335-346.
	switch ep.EarlyDepthTest.(type) {
	case ir.EarlyDepthForce:
		if w.supportsEarlyDepthTest() {
			w.features.request(FeatureImageLoadStore)
		}
	case ir.EarlyDepthAllow:
		w.features.request(FeatureConservativeDepth)
	}

	// Scan types for features
	for _, typ := range w.module.Types {
		switch inner := typ.Inner.(type) {
		case ir.ScalarType:
			if inner.Kind == ir.ScalarFloat && inner.Width == 8 {
				w.features.request(FeatureDoubleType)
			}
		case ir.ImageType:
			if inner.Arrayed && inner.Dim == ir.DimCube {
				w.features.request(FeatureCubeTexturesArray)
			}
			if inner.Multisampled {
				w.features.request(FeatureMultisampledTextures)
				// Note: FeatureTextureSamples only requested when ImageQuery::NumSamples is used
				if inner.Arrayed {
					w.features.request(FeatureMultisampledTexArrays)
				}
			}
		}
	}

	// Scan the globals the entry point uses for features; Rust naga skips
	// the others (ep_info[handle].is_empty()), which the GLSL of this entry
	// point does not declare.
	for handle, global := range w.module.GlobalVariables {
		if w.reachable != nil && !w.reachable.hasGlobal(ir.GlobalVariableHandle(handle)) {
			continue
		}
		switch global.Space {
		case ir.SpaceWorkGroup:
			w.features.request(FeatureComputeShader)
		case ir.SpaceStorage:
			w.features.request(FeatureBufferStorage)
			// The readonly qualifier of a read-only buffer block is one of
			// the memory qualifiers, which desktop GLSL below 420 only has
			// through GL_ARB_shader_image_load_store.
			if !global.Access.Contains(ir.StorageAccessStore) {
				w.features.request(FeatureImageLoadStore)
			}
		}
	}

	// Scan entry point varyings for features
	w.scanVaryingFeatures(ep)

	// Scan expressions for features
	w.scanExpressionFeatures(ep)
}

// scanVaryingFeatures checks entry point IO for required features.
func (w *Writer) scanVaryingFeatures(ep *ir.EntryPoint) {
	fn := &ep.Function

	for _, arg := range fn.Arguments {
		w.checkVaryingBinding(arg.Binding, arg.Type)
	}
	if fn.Result != nil {
		w.checkVaryingBinding(fn.Result.Binding, fn.Result.Type)
	}
}

// checkVaryingBinding checks a single varying binding for required features.
func (w *Writer) checkVaryingBinding(binding *ir.Binding, typeHandle ir.TypeHandle) {
	if binding == nil {
		// Struct — check members
		if int(typeHandle) < len(w.module.Types) {
			if st, ok := w.module.Types[typeHandle].Inner.(ir.StructType); ok {
				for _, m := range st.Members {
					w.checkVaryingBinding(m.Binding, m.Type)
				}
			}
		}
		return
	}
	switch b := (*binding).(type) {
	case ir.LocationBinding:
		if b.Interpolation != nil && *b.Interpolation == ir.InterpolationLinear {
			w.features.request(FeatureNoPerspective)
		}
		if b.Sampling != nil && *b.Sampling == ir.SamplingSample {
			w.features.request(FeatureSampleQualifier)
		}
		if b.BlendSrc != nil {
			w.features.request(FeatureDualSourceBlending)
		}
	case ir.BuiltinBinding:
		if b.Builtin == ir.BuiltinSampleIndex {
			w.features.request(FeatureSampleVariables)
		}
		if b.Builtin == ir.BuiltinBarycentric || b.Builtin == ir.BuiltinBarycentricNoPerspective {
			w.features.request(FeatureShaderBarycentrics)
		}
		if b.Builtin == ir.BuiltinPrimitiveIndex {
			w.features.request(FeaturePrimitiveIndex)
		}
		if b.Builtin == ir.BuiltinDrawIndex {
			w.features.request(FeatureInstanceIndex) // Rust: INSTANCE_INDEX for DrawIndex too
			w.features.request(FeatureDrawIndex)
		}
		if b.Builtin == ir.BuiltinViewIndex {
			w.features.request(FeatureMultiView)
		}
		if b.Builtin == ir.BuiltinInstanceIndex {
			w.features.request(FeatureInstanceIndex)
		}
		if b.Builtin == ir.BuiltinCullDistance {
			// Rust naga's features.rs requests CULL_DISTANCE for the built-in.
			w.features.request(FeatureCullDistance)
		}
	}
}

// scanExpressionFeatures scans all expressions for required features.
func (w *Writer) scanExpressionFeatures(ep *ir.EntryPoint) {
	scanFn := func(fn *ir.Function) {
		for _, expr := range fn.Expressions {
			switch k := expr.Kind.(type) {
			case ir.ExprSubgroupBallotResult:
				w.features.request(FeatureSubgroupOperations)
			case ir.ExprSubgroupOperationResult:
				w.features.request(FeatureSubgroupOperations)
			case ir.ExprImageQuery:
				switch k.Query.(type) {
				case ir.ImageQueryNumSamples:
					w.features.request(FeatureTextureSamples)
				case ir.ImageQueryNumLevels:
					w.features.request(FeatureTextureLevels)
				case ir.ImageQuerySize:
					// Check if image is a storage image — needs IMAGE_SIZE feature.
					// Resolve the image type through the expression chain.
					if int(k.Image) < len(fn.Expressions) {
						imgExpr := fn.Expressions[k.Image]
						if gv, ok := imgExpr.Kind.(ir.ExprGlobalVariable); ok {
							if int(gv.Variable) < len(w.module.GlobalVariables) {
								g := &w.module.GlobalVariables[gv.Variable]
								if int(g.Type) < len(w.module.Types) {
									if imgType, ok := w.module.Types[g.Type].Inner.(ir.ImageType); ok {
										if imgType.Class == ir.ImageClassStorage {
											w.features.request(FeatureImageSize)
										}
									}
								}
							}
						}
					}
				}
			case ir.ExprImageLoad:
				// Bounds-checked image loads with sample/level need extension.
				// Matches Rust naga: only when BoundsCheckPolicy != Unchecked.
				if w.options.BoundsCheckPolicies.ImageLoad != backend.BoundsCheckUnchecked {
					if k.Sample != nil {
						w.features.request(FeatureTextureSamples)
						w.features.forImageLoadChecks |= FeatureTextureSamples
					}
					if k.Level != nil {
						w.features.request(FeatureTextureLevels)
						w.features.forImageLoadChecks |= FeatureTextureLevels
					}
				}
			case ir.ExprImageSample:
				_ = k // scanned for shadow LOD below
			default:
				_ = k
			}
		}
		// Check statements for image atomics
		w.scanStatementsForFeatures(fn.Body)
	}

	// Scan entry point function
	scanFn(&ep.Function)

	// Scan all reachable functions
	for handle, fn := range w.module.Functions {
		if w.reachable != nil && !w.reachable.hasFunction(ir.FunctionHandle(handle)) {
			continue
		}
		scanFn(&fn)
	}
}

// scanStatementsForFeatures checks statements for features.
func (w *Writer) scanStatementsForFeatures(block ir.Block) {
	for _, stmt := range block {
		switch stmt.Kind.(type) {
		case ir.StmtImageAtomic:
			w.features.request(FeatureTextureAtomics)
		}
	}
}

// isWebGL returns true if this is a WebGL version.
func (v Version) isWebGL() bool {
	return v.ES && v.Major == 3 && v.Minor == 0
	// This is a simplification — WebGL 2.0 is ES 3.0
}
