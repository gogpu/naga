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
└── cmd/dxilval/    # DXIL validator (Windows)
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

### Rust reference comparison

`TestRustReference` compares our output with every output Rust naga's own
snapshot driver writes for the corpus: which backends a shader has comes from
its `.toml` `targets` (Rust's defaults when absent), and pipeline constants are
applied before each backend, as Rust does. Each output either matches Rust's,
or is a recorded deviation, or the test fails — a compile error included.
`TestRustReferenceInventory` checks that the vendored references are exactly
the files Rust writes.

Known differences live in `snapshot/rust_deviations_test.go`, one entry per
output file:

- `intentional` — we deliberately differ; the reason must say why;
- `bug` — our output is wrong or worse; link the tracking task;
- `unsupported` — we cannot produce the output yet; the expected compile error
  is recorded.

Entries for `intentional` and `bug` pin a fingerprint of the compared pair, so
any change on either side fails the test until the entry is re-reviewed, and
an entry that no longer describes a difference fails too: the list only
shrinks. Before adding an entry, check that the whole diff is explained by its
reason.

### External tools

Some tests need `spirv-val` / `spirv-dis` (SPIRV-Tools, e.g. from the Vulkan
SDK) or `dxc`. Locally, a missing tool skips those checks with an explicit
message. Set `NAGA_REQUIRE_TOOLS` (comma-separated tool names, or `all`) to
turn a missing tool into a failure — CI does, so a runner without the tool
cannot pass while checking nothing.

### Test corpus

The snapshot tests run over two corpora:

- `snapshot/testdata/rust/` — the [Rust naga test suite](https://github.com/gfx-rs/wgpu/tree/trunk/naga/tests)
  (WGSL inputs, their `.toml` parameters and Rust's reference outputs),
  vendored verbatim. **Do not edit or add files there by hand**:
  `TestRustCorpusIntegrity` checks every file against `MANIFEST.json`. New
  upstream shaders arrive by re-syncing the whole corpus with
  `go run ./internal/cmd/corpussync sync` — see
  `snapshot/testdata/rust/README.md`.
- `snapshot/testdata/own/in/` — shaders written for this project. They do not
  count toward Rust parity.

### Adding a new test shader

1. Add the `.wgsl` file to `snapshot/testdata/own/in/` with a name not used by
   the Rust corpus
2. Run `UPDATE_GOLDEN=1 go test -run TestSnapshots -count=1 ./snapshot/` and
   review the new golden files
3. Run `go test ./snapshot/` to check SPIR-V and DXIL validation

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
