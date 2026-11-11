# PROVENANCE — SPIRV-Headers

- Repository: https://github.com/KhronosGroup/SPIRV-Headers
- Tag: `vulkan-sdk-1.4.341.0`
- Commit: `04f10f650d514df88b76d25e83db360142c7b174` (committer date 2026-01-21)
- Files: `spirv.h` and `spirv.core.grammar.json` from the upstream `include/spirv/unified1/`
  directory; `LICENSE` from the repository root.
- Licence: MIT-style, see `LICENSE` (the Khronos Group).
- How produced: copied from the Vulkan SDK 1.4.341.1 installation
  (`Include/spirv-headers/`) on 2026-11-11. The SDK copies carry CRLF line endings and are
  byte-identical to the tag above after CRLF → LF normalisation (SHA-256 checked); this copy keeps
  the upstream LF line endings, so it matches the tag byte for byte.
- Used by: `internal/spvnames` — the capability table is generated from these files and
  `TestTableMatchesVendoredHeaders` regenerates it and diffs the result against `table.go`.
- Refresh: take a newer tag, fetch the three files from
  `https://raw.githubusercontent.com/KhronosGroup/SPIRV-Headers/<commit>/…`, rewrite the manifest
  with
  `go run ./internal/cmd/corpussync manifest -dir testdata/upstream/github.com/KhronosGroup/SPIRV-Headers -kind vendored -repo https://github.com/KhronosGroup/SPIRV-Headers -commit <sha> -commit-date <YYYY-MM-DD>`,
  then update `HeadersRevision` in `internal/spvnames/gen` and run `go generate ./internal/spvnames`.
