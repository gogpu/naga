#version 430 core

image2D output_tex;

layout(local_size_x = 8, local_size_y = 8, local_size_z = 1) in;

void main() {
    if (((gl_GlobalInvocationID[0] >= textureSize(output_tex, 0)[0]) || (gl_GlobalInvocationID[1] >= textureSize(output_tex, 0)[1]))) {
        return;
    }
    imageStore(output_tex, int(gl_GlobalInvocationID.xy), vec4(vec2((float(gl_GlobalInvocationID[0]) / float(textureSize(output_tex, 0)[0])), (float(gl_GlobalInvocationID[1]) / float(textureSize(output_tex, 0)[1])))[0], vec2((float(gl_GlobalInvocationID[0]) / float(textureSize(output_tex, 0)[0])), (float(gl_GlobalInvocationID[1]) / float(textureSize(output_tex, 0)[1])))[1], 0.5, 1.0));
}
