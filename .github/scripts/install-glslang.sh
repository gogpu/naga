#!/usr/bin/env bash
# Builds glslangValidator from a pinned glslang source tag into "$1/bin".
#
# The tag matches the Vulkan SDK used for development (SDK 1.4.341.1 ships the
# glslang at vulkan-sdk-1.4.341.0, reporting "11:16.2.0"), so the reference
# verdicts the corpus matrices re-check are produced by the same compiler
# locally and in CI. Khronos publishes release binaries only for the newest
# tags, so the tool is built from source; CI caches the result.
#
# This script runs on the CI runner image (python3 and cmake are part of
# ubuntu-latest); on the development machine the Vulkan SDK already provides
# glslangValidator, and this machine has no Python by policy.
set -euo pipefail

prefix=${1:?usage: install-glslang.sh <prefix>}

glslang_tag=vulkan-sdk-1.4.341.0
glslang_sha=f0bd0257c308b9a26562c1a30c4748a0219cc951

work=${RUNNER_TEMP:-${TMPDIR:-/tmp}}/glslang-build
rm -rf "$work"
mkdir -p "$work"

git clone --quiet --depth 1 --branch "$glslang_tag" https://github.com/KhronosGroup/glslang.git "$work/src"
got=$(git -C "$work/src" rev-parse HEAD)
if [ "$got" != "$glslang_sha" ]; then
	echo "glslang $glslang_tag is $got, want $glslang_sha" >&2
	exit 1
fi

# The repository's own dependency fetch: the known-good commits of
# SPIRV-Headers and SPIRV-Tools pinned at this tag.
(cd "$work/src" && python3 ./update_glslang_sources.py)

cmake -S "$work/src" -B "$work/build" -DCMAKE_BUILD_TYPE=Release \
	-DENABLE_OPT=OFF -DBUILD_TESTING=OFF -DENABLE_CTEST=OFF
cmake --build "$work/build" --config Release --parallel --target glslangValidator

mkdir -p "$prefix/bin"
found=$(find "$work/build" -type f \( -name glslangValidator -o -name glslangValidator.exe \) | head -1)
if [ -z "$found" ]; then
	echo "glslangValidator not found in the build tree" >&2
	exit 1
fi
cp "$found" "$prefix/bin/"

"$prefix/bin/glslangValidator" --version
