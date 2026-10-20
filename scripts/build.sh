#!/usr/bin/env bash
# Cross-platform release build for amdl.
#
# Produces the same artifacts as .github/workflows/release.yml so a release can
# be cut from a local machine without GitHub Actions — useful when the runner is
# unavailable, or to verify a version injection before pushing a tag.
#
# Usage:
#   ./scripts/build.sh                 # native target only
#   ./scripts/build.sh --all           # every supported target
#   ./scripts/build.sh --targets windows/amd64,linux/amd64
#   ./scripts/build.sh --version v0.2.0 --out dist
#
# The version, commit and build date are injected through -ldflags exactly like
# the CI workflow, so `amdl --version` reports the same thing either way.

set -euo pipefail

# ---------------------------------------------------------------------------
# defaults
# ---------------------------------------------------------------------------

OUT_DIR="dist"
VERSION_OVERRIDE=""
ALL_TARGETS=0
CUSTOM_TARGETS=""
REPO_OWNER=""
REPO_NAME=""

# The default target list mirrors release.yml. Android is separate because it
# needs cgo plus a matching NDK toolchain, which a local machine rarely has.
DEFAULT_TARGETS=(
  "windows/amd64"
  "windows/arm64"
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
)
ANDROID_TARGET="android/arm64"

# ---------------------------------------------------------------------------
# argument parsing
# ---------------------------------------------------------------------------

usage() {
  cat <<'EOF'
Cross-platform release build for amdl.

Produces the same artifacts as .github/workflows/release.yml, so a release can
be cut from a local machine without GitHub Actions.

Usage:
  ./scripts/build.sh                                  # native target only
  ./scripts/build.sh --all                            # every supported target
  ./scripts/build.sh --targets windows/amd64,linux/amd64
  ./scripts/build.sh --version 0.2.0 --out dist

Options:
  --all               build every supported target (Android is skipped without an NDK)
  --targets LIST      comma-separated os/arch list
  --version VERSION   override the version (default: contents of VERSION)
  --out DIR           output directory (default: dist)
  --repo OWNER NAME   self-update target repository (default: git remote origin)
  -h, --help          show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --all)      ALL_TARGETS=1; shift ;;
    --targets)  CUSTOM_TARGETS="$2"; shift 2 ;;
    --version)  VERSION_OVERRIDE="$2"; shift 2 ;;
    --out)      OUT_DIR="$2"; shift 2 ;;
    --repo)     REPO_OWNER="$2"; REPO_NAME="$3"; shift 3 ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      echo >&2
      usage >&2
      exit 2
      ;;
  esac
done

# ---------------------------------------------------------------------------
# locate the module root (works from any cwd)
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f go.mod ]]; then
  echo "error: go.mod not found in $ROOT_DIR" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# version metadata
# ---------------------------------------------------------------------------

# The VERSION file drives the release, matching the CI workflow. An explicit
# --version overrides it for a one-off build.
if [[ -n "$VERSION_OVERRIDE" ]]; then
  VERSION="$VERSION_OVERRIDE"
else
  VERSION="$(tr -d '[:space:]' < VERSION)"
fi

# The tag is what self-update compares against, so it must carry the v prefix.
# A user-supplied --version may or may not include it; normalise either way.
TAG="v${VERSION#v}"

# version.Compare treats a bare "dev" as older than any release, which is wrong
# for a build that does carry a real version, so always pass the tag.
COMMIT="$(git rev-parse HEAD 2>/dev/null || echo none)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Default the self-update repository to the configured origin so a fork builds
# binaries that update from the fork, exactly as CI does.
if [[ -z "$REPO_OWNER" ]]; then
  ORIGIN_URL="$(git config --get remote.origin.url 2>/dev/null || true)"
  if [[ "$ORIGIN_URL" =~ github\.com[:/]([^/]+)/([^/.]+) ]]; then
    REPO_OWNER="${BASH_REMATCH[1]}"
    REPO_NAME="${BASH_REMATCH[2]}"
  else
    REPO_OWNER="zhaarey"
    REPO_NAME="apple-music-downloader"
  fi
fi

echo "amdl release build"
echo "  version    : ${TAG}"
echo "  commit     : ${COMMIT}"
echo "  built at   : ${BUILD_DATE}"
echo "  repository : ${REPO_OWNER}/${REPO_NAME}"
echo "  output     : ${OUT_DIR}"
echo

# ---------------------------------------------------------------------------
# target selection
# ---------------------------------------------------------------------------

targets=()
if [[ "$ALL_TARGETS" -eq 1 ]]; then
  targets=("${DEFAULT_TARGETS[@]}" "$ANDROID_TARGET")
elif [[ -n "$CUSTOM_TARGETS" ]]; then
  IFS=',' read -r -a targets <<< "$CUSTOM_TARGETS"
else
  # Native build only: derive it from `go env` so the default is always useful.
  native_os="$(go env GOOS)"
  native_arch="$(go env GOARCH)"
  targets=("${native_os}/${native_arch}")
fi

# ---------------------------------------------------------------------------
# per-target naming
# ---------------------------------------------------------------------------

# Mirrors the CI artifact names (amdl_<os>_<arch>[.exe]).
artifact_name() {
  local goos="$1" goarch="$2"
  local name="amdl_${goos}_${goarch}"
  if [[ "$goos" == "windows" ]]; then
    name="${name}.exe"
  fi
  echo "$name"
}

# cgo needs a C compiler for the *target*, which only Android sets up in CI.
# Any other target is built with cgo off so no toolchain is required.
cgo_for_target() {
  [[ "$1" == "android" ]] && echo 1 || echo 0
}

# ---------------------------------------------------------------------------
# build
# ---------------------------------------------------------------------------

mkdir -p "$OUT_DIR"
built=()

for target in "${targets[@]}"; do
  # A target must contain exactly one slash, otherwise "bogus" would silently
  # become os=bogus arch=bogus and only fail much later inside the go toolchain
  # with a confusing message.
  if [[ "$target" != */*/* && "$target" == */* ]]; then
    goos="${target%%/*}"
    goarch="${target##*/}"
  else
    echo "error: bad target '$target' (expected os/arch, e.g. windows/amd64)" >&2
    exit 2
  fi

  if [[ -z "$goos" || -z "$goarch" ]]; then
    echo "error: bad target '$target' (expected os/arch, e.g. windows/amd64)" >&2
    exit 2
  fi

  name="$(artifact_name "$goos" "$goarch")"
  out="$OUT_DIR/$name"

  echo "building $name ..."

  # Android via NDK when it is present; otherwise skip with a clear message
  # rather than failing the whole build with an obscure compiler error.
  extra_env=()
  if [[ "$goos" == "android" ]]; then
    ndk_cc="${ANDROID_NDK_CC:-}"
    if [[ -z "$ndk_cc" ]]; then
      ndk_cc="$(command -v aarch64-linux-android24-clang || true)"
    fi
    if [[ -z "$ndk_cc" ]]; then
      echo "  skipped: no aarch64-linux-android24-clang on PATH (set ANDROID_NDK_CC)" >&2
      continue
    fi
    extra_env+=("CC=$ndk_cc")
  fi

  # -trimpath keeps local absolute paths out of the binary, which matters for a
  # published artifact; -s -w strips the symbol table and DWARF data.
  env GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="$(cgo_for_target "$goos")" "${extra_env[@]}" \
    go build -trimpath \
      -ldflags="-s -w \
        -X 'amdl/internal/version.Version=${TAG}' \
        -X 'amdl/internal/version.Commit=${COMMIT}' \
        -X 'amdl/internal/version.BuildDate=${BUILD_DATE}' \
        -X 'amdl/internal/updater.RepoOwner=${REPO_OWNER}' \
        -X 'amdl/internal/updater.RepoName=${REPO_NAME}'" \
      -o "$out" .

  built+=("$name")
done

if [[ ${#built[@]} -eq 0 ]]; then
  echo "error: nothing was built" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# checksums
# ---------------------------------------------------------------------------

# checksums.txt uses the same format as the release workflow (sha256sum), so a
# locally produced file is interchangeable with the published one.
(
  cd "$OUT_DIR"
  # Only hash the binaries, never the checksum file itself.
  sha256sum $(printf '%s\n' "${built[@]}") > checksums.txt
)

echo
echo "done. artifacts in $OUT_DIR:"
ls -lh "$OUT_DIR" | tail -n +2
echo
echo "verify with:  sha256sum -c $OUT_DIR/checksums.txt"