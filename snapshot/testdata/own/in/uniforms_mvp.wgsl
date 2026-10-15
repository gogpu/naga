// Uniform buffer with mat4x4, transform vertex position.

struct Uniforms {
    model: mat4x4<f32>,
    view: mat4x4<f32>,
    projection: mat4x4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) world_pos: vec3<f32>,
}

@vertex
fn vs_main(@location(0) position: vec3<f32>) -> VertexOutput {
    let world = uniforms.model * vec4<f32>(position, 1.0);
    let clip = uniforms.projection * uniforms.view * world;

    var out: VertexOutput;
    out.position = clip;
    out.world_pos = world.xyz;
    return out;
}

@fragment
fn fs_main(@location(0) world_pos: vec3<f32>) -> @location(0) vec4<f32> {
    let color = normalize(world_pos) * 0.5 + 0.5;
    return vec4<f32>(color, 1.0);
}
