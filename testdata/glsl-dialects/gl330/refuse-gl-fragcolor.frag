#version 330 core

// gl_FragColor is removed in GL 3.30 core (design §15 "gl_FragColor": no;
// 3.30 core drops the deprecated builtins). glslangValidator is laxer and
// still accepts the name; the language rule decides.
void main() {
    gl_FragColor = vec4(1.0);
}
