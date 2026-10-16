#!/usr/bin/env bash
# Builds spirv-val and spirv-dis from a pinned SPIRV-Tools release into "$1/bin".
#
# The version matches the Vulkan SDK used for development (SDK 1.4.341.1 ships
# SPIRV-Tools v2026.1), so validation strictness and disassembly are the same
# locally and in CI. Khronos publishes no binaries for a tag, so the tools are
# built from source; CI caches the result.
set -euo pipefail

prefix=${1:?usage: install-spirv-tools.sh <prefix>}

tools_tag=v2026.1
tools_sha=fbe4f3ad913c44fe8700545f8ffe35d1382b7093
# spirv_headers_revision from SPIRV-Tools' DEPS at the tag above.
headers_sha=04f10f650d514df88b76d25e83db360142c7b174

work=${RUNNER_TEMP:-${TMPDIR:-/tmp}}/spirv-tools-build
rm -rf "$work"
mkdir -p "$work"

git clone --quiet --depth 1 --branch "$tools_tag" https://github.com/KhronosGroup/SPIRV-Tools.git "$work/src"
got=$(git -C "$work/src" rev-parse HEAD)
if [ "$got" != "$tools_sha" ]; then
	echo "SPIRV-Tools $tools_tag is $got, want $tools_sha" >&2
	exit 1
fi

headers="$work/src/external/spirv-headers"
git init --quiet "$headers"
git -C "$headers" fetch --quiet --depth 1 https://github.com/KhronosGroup/SPIRV-Headers.git "$headers_sha"
git -C "$headers" checkout --quiet FETCH_HEAD

cmake -S "$work/src" -B "$work/build" -DCMAKE_BUILD_TYPE=Release -DSPIRV_SKIP_TESTS=ON -DSPIRV_WERROR=OFF
cmake --build "$work/build" --config Release --parallel --target spirv-val spirv-dis

mkdir -p "$prefix/bin"
find "$work/build" -type f \( -name spirv-val -o -name spirv-dis -o -name spirv-val.exe -o -name spirv-dis.exe \) \
	-exec cp {} "$prefix/bin/" \;

"$prefix/bin/spirv-val" --version
"$prefix/bin/spirv-dis" --version
