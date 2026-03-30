// === Entry Point: fragment (fragment) ===
#version 330 core
layout(location = 0) out vec4 _fs2p_location0;

void derivatives() {
    float x = dFdx(0.0);
    float y = dFdy(0.0);
    float width = fwidth(0.0);
    return;
}

void main() {
    derivatives();
    _fs2p_location0 = vec4(0.0);
    return;
}


// === Entry Point: compute (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void barriers() {
    memoryBarrierBuffer();
    barrier();
    memoryBarrierShared();
    barrier();
    memoryBarrierImage();
    barrier();
    return;
}

void main() {
    barriers();
    return;
}

