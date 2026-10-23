// Identifiers that start with gl_, which GLSL reserves, or with the prefixes
// of the names the GLSL writer gives bindings.

struct gl_Light {
    gl_color: vec4<f32>,
    gl_intensity: f32,
}

struct gl_VertexOut {
    @builtin(position) gl_pos: vec4<f32>,
    @location(0) gl_uv: vec2<f32>,
}

@group(0) @binding(0) var<uniform> gl_light: gl_Light;
var<private> gl_counter: i32;
const gl_SCALE: f32 = 2.0;

fn gl_shade(gl_l: gl_Light, _group_0_binding_0_fs: f32) -> vec4<f32> {
    let gl_c = gl_l.gl_color * gl_l.gl_intensity * gl_SCALE;
    var gl_acc = gl_c * _group_0_binding_0_fs;
    return gl_acc;
}

@vertex
fn vs_main(@location(0) gl_in: vec2<f32>) -> gl_VertexOut {
    var gl_o: gl_VertexOut;
    gl_o.gl_pos = vec4(gl_in, 0.0, 1.0);
    gl_o.gl_uv = gl_in;
    return gl_o;
}

@fragment
fn fs_main(gl_v: gl_VertexOut) -> @location(0) vec4<f32> {
    gl_counter += 1;
    let _immediates_binding_fs = f32(gl_counter);
    return gl_shade(gl_light, _immediates_binding_fs) + vec4(gl_v.gl_uv, 0.0, 0.0);
}
