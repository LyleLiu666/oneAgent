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
  "windows/amd64"
)

echo "[release] build matrix: ${TARGETS[*]}"

pushd "${ROOT_DIR}/backend" >/dev/null
for target in "${TARGETS[@]}"; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  OUT="${OUT_DIR}/oneagent_${VERSION}_${GOOS}_${GOARCH}"
  if [[ "${GOOS}" == "windows" ]]; then
    OUT="${OUT}.exe"
  fi
  echo "[release] go build ${GOOS}/${GOARCH} -> ${OUT}"
  env CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags "${LDFLAGS[*]}" -o "${OUT}" ./cmd/oneagent
done
popd >/dev/null

PORTABLE_GIT_URL="${PORTABLE_GIT_URL:-}"
BUNDLE_PORTABLE_GIT="${BUNDLE_PORTABLE_GIT:-1}"

resolve_portable_git_url() {
  if [[ -n "${PORTABLE_GIT_URL}" ]]; then
    echo "${PORTABLE_GIT_URL}"
    return
  fi

  curl -sSL --fail "https://api.github.com/repos/git-for-windows/git/releases/latest" | \
    python3 -c 'import json, re, sys; data=json.load(sys.stdin); pat=re.compile("^PortableGit-.*-64-bit\\\\.7z\\\\.exe$"); assets=data.get("assets", []); print(next((a.get("browser_download_url","") for a in assets if pat.match(a.get("name","") or "")), ""))'
}

ensure_portable_git_downloaded() {
  local dest="$1"
  if [[ -f "${dest}" ]]; then
    return
  fi

  local url
  url="$(resolve_portable_git_url)"
  if [[ -z "${url}" ]]; then
    echo "[release] ERROR: cannot resolve PortableGit download url; set PORTABLE_GIT_URL=..." >&2
    exit 1
  fi

  echo "[release] download PortableGit -> ${dest}"
  curl -L --fail --retry 3 --retry-delay 2 -o "${dest}" "${url}"
}

echo "[release] package windows zips (PortableGit: ${BUNDLE_PORTABLE_GIT})"
for target in "${TARGETS[@]}"; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"
  if [[ "${GOOS}" != "windows" ]]; then
    continue
  fi

  BIN="${OUT_DIR}/oneagent_${VERSION}_${GOOS}_${GOARCH}.exe"
  if [[ ! -f "${BIN}" ]]; then
    echo "[release] ERROR: missing windows binary: ${BIN}" >&2
    exit 1
  fi

  PKG_DIR="${OUT_DIR}/oneagent_${VERSION}_${GOOS}_${GOARCH}"
  rm -rf "${PKG_DIR}"
  mkdir -p "${PKG_DIR}/bundled"
  cp "${BIN}" "${PKG_DIR}/oneagent.exe"

  if [[ "${BUNDLE_PORTABLE_GIT}" == "1" ]]; then
    PORTABLE_GIT_CACHE="${OUT_DIR}/PortableGit.7z.exe"
    ensure_portable_git_downloaded "${PORTABLE_GIT_CACHE}"
    cp "${PORTABLE_GIT_CACHE}" "${PKG_DIR}/bundled/PortableGit.7z.exe"
    cp "${ROOT_DIR}/scripts/NOTICE_GIT_FOR_WINDOWS.txt" "${PKG_DIR}/bundled/NOTICE_GIT_FOR_WINDOWS.txt"
  fi

  ZIP_NAME="oneagent_${VERSION}_${GOOS}_${GOARCH}.zip"
  rm -f "${OUT_DIR}/${ZIP_NAME}"
  (
    cd "${OUT_DIR}"
    zip -r "${ZIP_NAME}" "$(basename "${PKG_DIR}")" >/dev/null
  )
done

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
  if [[ "${GOOS}" == "windows" ]]; then
    FILE="${FILE}.zip"
  fi
  if [[ "${GOOS}" == "windows" ]]; then
    SUM="$(sha256_file "${OUT_DIR}/${FILE}")"
    echo "${SUM}  ${FILE}" >> "${CHECKSUM_FILE}"
    continue
  fi

  SUM="$(sha256_file "${OUT_DIR}/${FILE}")"
  echo "${SUM}  ${FILE}" >> "${CHECKSUM_FILE}"
done

echo "[release] done: ${OUT_DIR}"
