// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package glsl provides a GLSL (OpenGL Shading Language) backend and a Vulkan
// GLSL input frontend for naga.
//
// # GLSL Backend
//
// Compile generates GLSL source code from naga's IR representation.
// It supports multiple GLSL versions for different target platforms:
//
//   - GLSL ES 3.00: WebGL 2.0, Mobile OpenGL ES 3.0
//
//   - GLSL 3.30 Core: Desktop OpenGL 3.3+
//
//   - GLSL ES 3.10: Android 5.0+ with compute shaders
//
//   - GLSL 4.30 Core: Desktop OpenGL 4.3+ with compute shaders
//
//     source, info, err := glsl.Compile(module, glsl.Options{
//     LangVersion: glsl.Version330,
//     })
//
// # GLSL Input
//
// Parse reads Vulkan GLSL 440/450/460 source into the IR module the other
// frontends produce, under the WGSL frontend's error contract: one *ParseError
// whose label is the first error and whose notes are the rest. GLSL does not
// name its stage, so ParseOptions.Stage supplies it. A combined image sampler
// (uniform sampler2D) becomes an image global and a sampler global, and
// ParseInfo.CombinedSamplers reports each split; ParseOptions.SamplerBindings
// places the sampler and CombinedImageSamplerReject refuses the declaration
// instead. The GL 3.30 core and ES 3.00 dialects are accepted too: they have no layout(binding), so
// ParseOptions.ResourceBindings places a module-scope resource by its source name and the rest take the next
// free slot of their group; each dialect's feature gates, the ES precision rules and the #extension behavior
// follow the Khronos specifications.
//
// The frontend is a preview: every form it accepts is verified end to end, but
// textureQueryLod is not implemented yet and a few subgroup forms the IR cannot
// express are refused with a message naming the limitation (subgroupElect,
// subgroupAllEqual, the ballot helpers, the clustered forms, the
// Min/Max/And/Or/Xor scans, and the subgroup built-in variables); ParseInfo and
// ParseOptions may still change until the frontend is complete.
//
// # Plain Uniforms
//
// GL 3.30 core and ES 3.00 let a shader declare a uniform outside any block
// (`uniform mat4 mvp;`). The IR has no resource for such a declaration, so the
// frontend collects every plain uniform of a module into one generated uniform
// block, named frontend.DefaultBlockName, whose members are the plain uniforms
// in source declaration order with std140 offsets. ParseInfo.DefaultBlock
// reports the block's binding, its size and each member's offset and size:
// allocate Size bytes, write each member at its Offset with std140 encoding and
// bind the buffer at Binding. Use the reported offsets and never re-derive the
// layout. The block takes the slot ParseOptions.ResourceBindings gives
// frontend.DefaultBlockName, else the next free binding of group 0 at the
// position of the first plain-uniform declaration. ParseInfo.DefaultBlock is
// nil when the module has no plain uniform.
//
// Vulkan GLSL is different: the Vulkan profile has no default uniform block, so
// a plain uniform there is a *ParseError that asks for a block, as glslang
// refuses it too ("non-opaque uniforms outside a block").
//
//	module, info, err := glsl.Parse(source, glsl.ParseOptions{Stage: ir.StageFragment})
//
// The frontend is implemented in glsl/internal/front.
//
// # Texture/Sampler Handling
//
// WGSL separates textures and samplers, but GLSL combines them.
// The backend automatically generates combined sampler uniforms
// for texture-sampler pairs used together.
//
// # Reserved Words
//
// GLSL has over 500 reserved words (including future reserved).
// The backend automatically escapes conflicting identifier names
// by prefixing them with an underscore.
//
// # Concurrency
//
// Every exported function is safe for concurrent use. Compile never modifies
// its input module, so one module can be compiled for several targets at once.
package glsl
