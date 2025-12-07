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

1. **Input Validation** - WGSL parser should handle malformed input gracefully
2. **Resource Limits** - Parser should have limits to prevent DoS via deeply nested structures
3. **Generated Code** - SPIR-V output should be valid and safe

## Security Contact

- **GitHub Security Advisory**: https://github.com/gogpu/naga/security/advisories/new
- **Public Issues**: https://github.com/gogpu/naga/issues

---

**Thank you for helping keep Naga secure!**
