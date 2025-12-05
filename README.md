# naga

[![Go Reference](https://pkg.go.dev/badge/github.com/gogpu/naga.svg)](https://pkg.go.dev/github.com/gogpu/naga)
[![Go Report Card](https://goreportcard.com/badge/github.com/gogpu/naga)](https://goreportcard.com/report/github.com/gogpu/naga)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Pure Go Shader Compiler** — WGSL to SPIR-V, GLSL, and more.

> 🚧 **Coming Soon** — Active development in progress!

---

## ✨ Features

- **Pure Go** — No CGO, no external dependencies
- **WGSL Frontend** — Parse WebGPU Shading Language
- **SPIR-V Backend** — Generate Vulkan-compatible bytecode
- **GLSL Backend** — OpenGL shader output (planned)
- **Validation** — Catch errors before GPU execution

## 🎯 Vision

Port the excellent [naga](https://github.com/gfx-rs/naga) Rust shader compiler to pure Go, enabling:

- Shader compilation without Rust toolchain
- Runtime shader generation in Go applications
- WebAssembly deployment
- Integration with [gogpu](https://github.com/gogpu/gogpu) ecosystem

## 📦 Installation

```bash
go get github.com/gogpu/naga
```

## 🚀 Usage (Planned API)

```go
package main

import (
    "github.com/gogpu/naga"
    "github.com/gogpu/naga/wgsl"
    "github.com/gogpu/naga/spirv"
)

func main() {
    // Parse WGSL shader
    source := `
        @vertex
        fn vs_main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
            return vec4<f32>(pos, 1.0);
        }

        @fragment
        fn fs_main() -> @location(0) vec4<f32> {
            return vec4<f32>(1.0, 0.0, 0.0, 1.0);
        }
    `

    module, err := wgsl.Parse(source)
    if err != nil {
        panic(err)
    }

    // Validate
    if err := naga.Validate(module); err != nil {
        panic(err)
    }

    // Generate SPIR-V
    spirvBytes, err := spirv.Generate(module)
    if err != nil {
        panic(err)
    }

    // Use with Vulkan/WebGPU...
}
```

## 🏗️ Architecture

```
naga/
├── wgsl/          # WGSL frontend
│   ├── lexer.go   # Tokenizer
│   ├── parser.go  # Parser
│   └── ast.go     # AST types
├── ir/            # Intermediate representation
├── spirv/         # SPIR-V backend
├── glsl/          # GLSL backend (future)
├── hlsl/          # HLSL backend (future)
└── cmd/naga/      # CLI tool
```

## 🗺️ Roadmap

**Phase 1: WGSL Parser**
- [ ] Lexer (tokenizer)
- [ ] AST types
- [ ] Parser
- [ ] Error messages with source locations

**Phase 2: IR & Validation**
- [ ] Intermediate representation
- [ ] Type checking
- [ ] Semantic validation

**Phase 3: SPIR-V Backend**
- [ ] SPIR-V binary writer
- [ ] Type emission
- [ ] Function emission
- [ ] Built-in functions

**Phase 4: Additional Backends**
- [ ] GLSL output
- [ ] HLSL output (future)
- [ ] MSL output (future)

## 📚 References

- [WGSL Specification](https://www.w3.org/TR/WGSL/)
- [SPIR-V Specification](https://registry.khronos.org/SPIR-V/)
- [naga (Rust)](https://github.com/gfx-rs/naga) — Original implementation

## 🔗 Related Projects

| Project | Description |
|---------|-------------|
| [gogpu/gogpu](https://github.com/gogpu/gogpu) | Graphics framework |
| [go-webgpu/webgpu](https://github.com/go-webgpu/webgpu) | WebGPU bindings |

## 🤝 Contributing

We welcome contributions! Especially help with:
- WGSL parser implementation
- SPIR-V spec expertise
- Test cases from real shaders

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>naga</b> — Shaders in Pure Go
</p>
