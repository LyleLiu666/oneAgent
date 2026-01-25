#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-${ROOT_DIR}/dist/release}"

VERSION="${VERSION:-}"
if [[ -z "${VERSION}" ]]; then
  if command -v git >/dev/null 2>&1; then
    VERSION="$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || true)"
  fi
  VERSION="${VERSION:-dev}"
fi

COMMIT="unknown"
if command -v git >/dev/null 2>&1; then
  COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || true)"
fi
DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "[release] version=${VERSION} commit=${COMMIT} date=${DATE}"

mkdir -p "${OUT_DIR}"

echo "[release] build frontend + sync assets"
cd "${ROOT_DIR}"
make sync-frontend >/dev/null

LDFLAGS=(
  "-s" "-w"
  "-X" "github.com/liu_y/oneAgent/backend/internal/buildinfo.Version=${VERSION}"
  "-X" "github.com/liu_y/oneAgent/backend/internal/buildinfo.Commit=${COMMIT}"
  "-X" "github.com/liu_y/oneAgent/backend/internal/buildinfo.Date=${DATE}"
)

TARGETS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

echo "[release] build matrix: ${TARGETS[*]}"

pushd "${ROOT_DIR}/backend" >/dev/null
for target in "${TARGETS[@]}"; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  OUT="${OUT_DIR}/oneagent_${VERSION}_${GOOS}_${GOARCH}"
  echo "[release] go build ${GOOS}/${GOARCH} -> ${OUT}"
  env CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags "${LDFLAGS[*]}" -o "${OUT}" ./cmd/oneagent
done
popd >/dev/null

CHECKSUM_FILE="${OUT_DIR}/checksums_${VERSION}.txt"
rm -f "${CHECKSUM_FILE}"
touch "${CHECKSUM_FILE}"

sha256_file() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
    return
  fi
  echo "no sha256 tool found (need shasum or sha256sum)" >&2
  exit 1
}

echo "[release] checksums -> ${CHECKSUM_FILE}"
for target in "${TARGETS[@]}"; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  FILE="oneagent_${VERSION}_${GOOS}_${GOARCH}"
  SUM="$(sha256_file "${OUT_DIR}/${FILE}")"
  echo "${SUM}  ${FILE}" >> "${CHECKSUM_FILE}"
done

echo "[release] done: ${OUT_DIR}"

