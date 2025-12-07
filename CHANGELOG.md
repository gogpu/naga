# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.2-dev] - 2025-12-08

### Added
- **WGSL Parser** (`wgsl/parser.go`)
  - Recursive descent parser (~1400 lines)
  - Function definitions with attributes
  - Struct definitions with members
  - Variable declarations (var, let, const)
  - Type aliases
  - All WGSL expressions with proper precedence
  - All WGSL statements (if, for, while, loop, return, etc.)
  - Error recovery via synchronize()
- **Parser Tests** (`wgsl/parser_test.go`)
  - 19 tests covering all parser functionality
  - Vertex, fragment, and compute shader parsing
  - Expression and statement subtests
- **Community Files**
  - `CODE_OF_CONDUCT.md`
  - `SECURITY.md`
  - `.github/CODEOWNERS`
- **CHANGELOG.md** (this file)

### Changed
- Updated `.golangci.yml` to v2 format
- Formatted all Go source files

## [0.0.1-dev] - 2025-12-05

### Added
- **Repository Structure**
  - `wgsl/` - WGSL frontend package
  - `ir/` - Intermediate representation package
  - `spirv/` - SPIR-V backend package
  - `cmd/nagac/` - CLI tool skeleton
- **WGSL Lexer** (`wgsl/lexer.go`)
  - Full WGSL tokenization
  - 140+ token types
  - Line and column tracking
- **WGSL AST** (`wgsl/ast.go`)
  - Complete AST type definitions
  - Module, declarations, statements, expressions
- **Lexer Tests** (`wgsl/lexer_test.go`)
  - 8 tests covering lexer functionality
- **IR Skeleton** (`ir/ir.go`)
  - Basic type definitions
  - Module and function structures
- **SPIR-V Skeleton** (`spirv/spirv.go`)
  - OpCode definitions
- **CI Configuration** (`.github/workflows/ci.yml`)
  - Test, lint, build jobs
  - Multi-platform support
  - Fuzz testing
- **Basic Documentation**
  - `README.md` with project vision
  - `LICENSE` (MIT)
  - `.gitignore`

---

[Unreleased]: https://github.com/gogpu/naga/compare/v0.0.2-dev...HEAD
[0.0.2-dev]: https://github.com/gogpu/naga/compare/v0.0.1-dev...v0.0.2-dev
[0.0.1-dev]: https://github.com/gogpu/naga/releases/tag/v0.0.1-dev
