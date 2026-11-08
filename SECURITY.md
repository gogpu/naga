# Security Policy

## Supported Versions

Naga is currently in early development (v0.x.x).

| Version | Supported          |
| ------- | ------------------ |
| 0.0.x   | :white_check_mark: |

## Reporting a Vulnerability

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues via:

1. **Private Security Advisory** (preferred):
   https://github.com/gogpu/naga/security/advisories/new

2. **GitHub Discussions** (for less critical issues):
   https://github.com/gogpu/naga/discussions

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

### Response Timeline

- **Initial Response**: Within 72 hours
- **Fix & Disclosure**: Coordinated with reporter

## Security Considerations

Naga is a shader compiler. Security considerations include:

1. **Input Validation** - malformed input is an error, never a panic or a crash.
2. **Resource Limits** - nesting is bounded before any recursive pass can run
   out of stack. `wgsl.Parse` refuses a source over 4 GiB, more than 200
   levels of parser recursion (about 197 nested parentheses), more than 127
   levels of braces in a function, statements nested more than 1024 levels
   deep once lowered (each `else if` is a level), and expressions more than
   4096 operators deep. Every limit is above what Rust naga compiles before it
   overflows its own stack. The DXIL backend also refuses an entry point
   larger than 65536 expressions and statements once its calls are inlined,
   or with more than 65536 bytes of function and private variables
   (`backend.ErrUnsupported`). The exact limits are in the `wgsl.Parse` and
   `dxil.Compile` documentation.
3. **Generated Code** - backend output should be valid and safe.

## Security Contact

- **GitHub Security Advisory**: https://github.com/gogpu/naga/security/advisories/new
- **Public Issues**: https://github.com/gogpu/naga/issues

---

**Thank you for helping keep Naga secure!**
