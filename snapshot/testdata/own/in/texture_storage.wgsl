// Storage texture: textureStore with compute shader.

@group(0) @binding(0)
var output_tex: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let dims = textureDimensions(output_tex);
    if gid.x >= dims.x || gid.y >= dims.y {
        return;
    }

    let uv = vec2<f32>(f32(gid.x) / f32(dims.x), f32(gid.y) / f32(dims.y));
    let color = vec4<f32>(uv.x, uv.y, 0.5, 1.0);

    textureStore(output_tex, vec2<i32>(gid.xy), color);
}
