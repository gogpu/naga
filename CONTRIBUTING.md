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
├── wgsl/           # WGSL frontend (lexer, parser, AST)
├── ir/             # Intermediate representation
├── spirv/          # SPIR-V backend
├── msl/            # MSL backend (Metal)
├── glsl/           # GLSL backend (OpenGL)
├── cmd/nagac/      # CLI tool
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

Components: `wgsl`, `ir`, `spirv`, `msl`, `glsl`, `cli`, `docs`, `ci`

## Testing

### Unit Tests
```bash
go test ./...
```

### With Coverage
```bash
go test -cover ./...
```

### Verbose Output
```bash
go test -v ./...
```

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
