# Contributing to Naga

Thank you for your interest in contributing to Naga!

## Contribution Policy

**All contributions must be submitted via Pull Request.** Direct pushes to `main` are not allowed.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/naga`
3. Add upstream: `git remote add upstream https://github.com/gogpu/naga`
4. Create a branch: `git checkout -b feat/your-feature`
5. Make your changes
6. Run tests: `go test ./...`
7. Run linter: `golangci-lint run`
8. Commit: `git commit -m "feat: add your feature"`
9. Push: `git push origin feat/your-feature`
10. Open a Pull Request

## Pull Request Process

1. **Create PR** — Open a PR against `main` branch
2. **CI Checks** — All checks must pass (tests, linting, build)
3. **Review** — Wait for maintainer review
4. **Address Feedback** — Make requested changes if any
5. **Merge** — Maintainer merges after approval

### PR Requirements

- [ ] All tests pass (`go test ./...`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Documentation updated (if applicable)
- [ ] Related issue referenced (if applicable)

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/naga
cd naga

# Add upstream remote
git remote add upstream https://github.com/gogpu/naga

# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
golangci-lint run

# Run pre-release checks
bash scripts/pre-release-check.sh
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Use `golangci-lint` for linting
- Write tests for new functionality
- Document public APIs with godoc comments

## Project Structure

```
naga/
├── wgsl/           # WGSL frontend (lexer, parser, AST → IR)
├── ir/             # Intermediate representation
├── spirv/          # SPIR-V backend (Vulkan)
├── msl/            # MSL backend (Metal)
├── glsl/           # GLSL backend (OpenGL)
├── hlsl/           # HLSL backend (DirectX 11/12)
├── dxil/           # DXIL backend (DirectX 12, experimental)
├── snapshot/       # Snapshot test infrastructure + golden files
│   └── testdata/
│       ├── in/             # Input WGSL shaders + TOML configs
│       ├── golden/         # Our golden outputs (msl/, glsl/, hlsl/, spv/)
│       └── reference/      # Rust naga reference outputs for comparison
├── cmd/nagac/      # CLI compiler
├── cmd/spvdis/     # SPIR-V disassembler
├── cmd/dxilval/    # DXIL validator (Windows)
└── scripts/        # Development scripts
```

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(component): add new feature
fix(component): fix bug
docs: update documentation
test: add tests
refactor: code refactoring
chore: maintenance tasks
```

Components: `wgsl`, `ir`, `spirv`, `msl`, `glsl`, `hlsl`, `dxil`, `snapshot`, `cli`, `docs`, `ci`

## Testing

### Unit Tests
```bash
go test ./...
```

### Rust Reference Comparison (main quality gate)
```bash
# Compare our output against Rust naga reference — ALL must pass
go test -run TestRustReference -count=1 ./snapshot/

# Single shader, single backend
go test -run "TestRustReference/atomicOps/msl" -v -count=1 ./snapshot/
```

### Update Golden Files
```bash
# After intentional output changes, regenerate goldens
UPDATE_GOLDEN=1 go test -run TestSnapshots -count=1 ./snapshot/
```

### SPIR-V Binary Validation
```bash
# All shaders must pass spirv-val (requires spirv-tools installed)
go test -run TestSpirvValBinarySummary -v -count=1 ./snapshot/
```

### With Coverage
```bash
go test -cover ./...
```

## Snapshot Test Infrastructure

### File naming convention

Golden file names in `snapshot/testdata/golden/` match Rust naga test shader
names **exactly, 1:1**. This is intentional — `TestRustReference` pairs our
golden output with the Rust reference output by name.

The Rust naga test suite uses **mixed naming conventions** (camelCase, kebab-case,
snake_case) that accumulated organically over years:

```
atomicOps.msl               ← camelCase (Rust upstream name)
workgroup-var-init.msl       ← kebab-case (Rust upstream name)
workgroup_memory.msl         ← snake_case (Rust upstream name)
types_with_comments.msl      ← snake_case (Rust upstream name)
```

**Do not rename** golden files to unify the convention — this would break the
1:1 mapping with Rust naga references. When adding new test shaders, use the
same name as in the Rust naga test suite.

### Reference allow-list

Some shaders produce intentionally different output from Rust naga. These are
recorded in `referenceAllowList` in `snapshot/snapshot_test.go` with a reason
string. When adding a new allow-list entry, document **why** the divergence
exists and confirm it does not affect correctness.

### Adding a new test shader

1. Copy the `.wgsl` file (and `.toml` config if any) from the
   [Rust naga test suite](https://github.com/gfx-rs/wgpu/tree/trunk/naga/tests/in/wgsl)
   to `snapshot/testdata/in/` — **keep the original filename**
2. Run `UPDATE_GOLDEN=1 go test -run TestSnapshots -count=1 ./snapshot/`
3. Run `go test -run TestRustReference -count=1 ./snapshot/` to verify parity
4. If the output intentionally diverges, add the shader to `referenceAllowList`

## Reporting Issues

- Use [GitHub Issues](https://github.com/gogpu/naga/issues)
- Search existing issues first
- Include Go version and OS
- Provide minimal reproduction (WGSL code if applicable)
- Include full error messages

## Questions?

- Open a [GitHub Discussion](https://github.com/gogpu/naga/discussions)
- Check existing issues and discussions first

---

Thank you for contributing to Naga!
