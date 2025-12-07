<h1 align="center">naga</h1>

<p align="center">
  <strong>Pure Go Shader Compiler</strong><br>
  WGSL to SPIR-V, GLSL, and more. Zero CGO.
</p>

<p align="center">
  <a href="https://github.com/gogpu/naga/actions/workflows/ci.yml"><img src="https://github.com/gogpu/naga/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/gogpu/naga"><img src="https://codecov.io/gh/gogpu/naga/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://pkg.go.dev/github.com/gogpu/naga"><img src="https://pkg.go.dev/badge/github.com/gogpu/naga.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/gogpu/naga"><img src="https://goreportcard.com/badge/github.com/gogpu/naga" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/gogpu/naga"><img src="https://img.shields.io/badge/CGO-none-success" alt="Zero CGO"></a>
</p>

<p align="center">
  <sub>Part of the <a href="https://github.com/gogpu">GoGPU</a> ecosystem</sub>
</p>

> **Status:** WGSL parser complete, IR and backends in progress.

---

## ✨ Features

- **Pure Go** — No CGO, no external dependencies
- **WGSL Frontend** — Full lexer and parser for WebGPU Shading Language
- **SPIR-V Backend** — Generate Vulkan-compatible bytecode (in progress)
- **GLSL Backend** — OpenGL shader output (planned)
- **Validation** — Catch errors before GPU execution (planned)

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

## 🚀 Usage

### Parsing WGSL

```go
package main

import (
    "fmt"
    "github.com/gogpu/naga/wgsl"
)

func main() {
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

    // Tokenize
    lexer := wgsl.NewLexer(source)
    tokens, err := lexer.Tokenize()
    if err != nil {
        panic(err)
    }

    // Parse to AST
    parser := wgsl.NewParser(tokens)
    module, err := parser.Parse()
    if err != nil {
        panic(err)
    }

    fmt.Printf("Parsed %d functions\n", len(module.Functions))
    // Output: Parsed 2 functions
}
```

## 🏗️ Architecture

```
naga/
├── wgsl/           # WGSL frontend
│   ├── token.go    # Token types (140+)
│   ├── lexer.go    # Tokenizer
│   ├── ast.go      # AST types
│   └── parser.go   # Recursive descent parser
├── ir/             # Intermediate representation
├── spirv/          # SPIR-V backend
├── glsl/           # GLSL backend (future)
├── hlsl/           # HLSL backend (future)
└── cmd/nagac/      # CLI tool
```

## 🗺️ Roadmap

**Phase 1: WGSL Parser** ✅
- [x] Lexer (tokenizer) — 140+ token types
- [x] AST types — Complete type definitions
- [x] Parser — Recursive descent, all WGSL constructs
- [x] Error recovery — Synchronization on errors

**Phase 2: IR & Validation**
- [ ] Intermediate representation
- [ ] Type checking
- [ ] Semantic validation
- [ ] Error messages with source locations

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

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Especially help with:
- IR implementation
- SPIR-V backend
- Test cases from real shaders

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <b>naga</b> — Shaders in Pure Go
</p>
