package frontend

import "github.com/gogpu/naga/ir"

// DefaultBlockName is the name of the uniform block the GLSL frontend
// synthesizes for the plain (non-block) uniforms of a GL 3.30 core or ES 3.00
// source (ADR-013). It is also the key a caller puts in
// glsl.ParseOptions.ResourceBindings to place the block, and the name the
// module's generated type and global carry.
//
// The GLSL specification reserves the gl_ prefix for the implementation, so no
// valid shader identifier can collide with it: glslang refuses
// `uniform float gl_DefaultUniformBlock;` with "'gl_DefaultUniformBlock' :
// identifiers starting with "gl_" are reserved", and it refuses the name as a
// block's too ("cannot redeclare block"). The name is a deliberate choice, not
// an accident of namespace.
const DefaultBlockName = "gl_DefaultUniformBlock"

// DefaultBlockInfo describes the one uniform block the GLSL frontend synthesizes
// for the plain (non-block) uniforms of a GL 3.30 core or ES 3.00 source
// (ADR-013).
//
// GLSL outside the Vulkan profile lets a shader declare uniforms outside any
// block — `uniform mat4 mvp;` — and naga's IR has no resource for such a
// declaration: every non-opaque resource is a descriptor (group, binding).
// The frontend therefore collects them into one generated uniform block, named
// [DefaultBlockName], whose members are the plain uniforms in source
// declaration order, laid out with the std140 rules (GLSL 4.60 §7.6.2.2 — the
// normative layout of a uniform block, and what a GL driver's lowering of the
// default block uses).
//
// The contract for a caller: allocate Size bytes, write each member at its
// Offset with std140 encoding, and bind the buffer at Binding. Use the
// reported offsets and never re-derive the layout yourself: they are the
// offsets the compiled module reads. A caller that re-derives them (from the
// declared types, or with another layout rule) writes into the wrong bytes.
//
// The reported Offsets match glslangValidator's for the same declarations in an
// explicit std140 block, member for member, including the cases where std140
// differs from a type's natural size: a vec3, matCxR or array member is aligned
// to 16 bytes, an array's element stride is rounded up to 16, and a bool member
// occupies 4 bytes.
//
// ParseInfo.DefaultBlock is nil for a module with no plain uniform, and always
// nil for Vulkan GLSL, whose plain uniforms are refused outright: the Vulkan
// profile has no default uniform block.
//
// A member keeps the type it was declared with, so a bool member stays an IR
// bool. The HLSL and MSL targets take that as it is, but the SPIR-V target then
// holds an OpTypeBool inside the block, which Vulkan forbids: spirv-val refuses
// such a module with "If OpTypeBool is stored in conjunction with OpVariable, it
// can only be used with non-externally visible shader Storage Classes". A shader
// that must compile to SPIR-V declares the flag as a uint or an int, which
// shares the member's four bytes; the module is not otherwise different. (This
// is what glslang does behind the scenes: it lowers a bool block member to a
// uint.)
type DefaultBlockInfo struct {
	// Name is the block's name in the module: [DefaultBlockName].
	Name string
	// Binding is where the block is bound: the slot
	// glsl.ParseOptions.ResourceBindings gives DefaultBlockName, else the next
	// free slot of group 0 at the position of the first plain-uniform
	// declaration.
	Binding ir.ResourceBinding
	// Size is the std140 size of the whole block: the span of the generated
	// struct, rounded up to the block's alignment (a multiple of 16, the
	// alignment of the largest member rounded up). It is at least the end of
	// the last member.
	Size uint32
	// Members are the plain uniforms in source declaration order.
	Members []DefaultBlockMember
}

// DefaultBlockMember is one plain uniform inside [DefaultBlockInfo]: its source
// name and where its value goes in the block.
type DefaultBlockMember struct {
	// Name is the source name of the uniform.
	Name string
	// Offset is the member's std140 offset from the start of the block.
	Offset uint32
	// Size is the member's std140 size, which is not the size its IR type would
	// take on its own: a vec3 is 12 bytes of a 16-byte slot, an array counts its
	// rounded-up element stride.
	Size uint32
}
