@group(0) @binding(0) var depth: texture_depth_2d;
@group(0) @binding(1) var depth_array: texture_depth_2d_array;
@group(0) @binding(2) var depth_cube: texture_depth_cube;
@group(0) @binding(3) var depth_ms: texture_depth_multisampled_2d;
@group(0) @binding(4) var shadow: texture_depth_2d;
@group(1) @binding(0) var s: sampler;
@group(1) @binding(1) var sc: sampler_comparison;

fn read(t: texture_depth_2d, uv: vec2<f32>) -> f32 {
    return textureSample(t, s, uv) + textureLoad(t, vec2<i32>(uv), 0);
}

fn compare(t: texture_depth_2d, uv: vec2<f32>) -> f32 {
    return textureSampleCompare(t, sc, uv, 0.5);
}

fn size(t: texture_depth_2d) -> vec2<u32> {
    return textureDimensions(t);
}

@fragment
fn sampled(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let level = 1;
    return vec4<f32>(
        textureSample(depth, s, p.xy),
        textureSampleLevel(depth_array, s, p.xy, 2, level),
        textureSample(depth_cube, s, p.xyz),
        1.0,
    ) + textureGather(depth, s, p.xy);
}

@fragment
fn loaded(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let c = vec2<i32>(p.xy);
    return vec4<f32>(textureLoad(depth, c, 0), textureLoad(depth_array, c, 1, 0), textureLoad(depth_ms, c, 2), 1.0);
}

@fragment
fn compared(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(textureSampleCompare(shadow, sc, p.xy, 0.5)) + textureGatherCompare(shadow, sc, p.xy, 0.5);
}

@fragment
fn helpers(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(read(depth, p.xy), compare(shadow, p.xy), vec2<f32>(size(shadow)));
}
