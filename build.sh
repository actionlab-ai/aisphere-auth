#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAME="aisphere-auth"
VERSION="$(tr -d '[:space:]' < "${ROOT_DIR}/VERSION")"
DIST_DIR="${ROOT_DIR}/dist"
BUILD_DIR="${ROOT_DIR}/.build"

usage() {
  cat <<'EOF'
Usage:
  bash build.sh --arch amd64|arm64|all

Examples:
  bash build.sh --arch amd64
  bash build.sh --arch arm64
  bash build.sh --arch all
EOF
}

ARCH=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --arch) ARCH="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "${ARCH}" ]]; then
  echo "missing --arch" >&2
  usage
  exit 1
fi

case "${ARCH}" in
  amd64|arm64|all) ;;
  *) echo "unsupported arch: ${ARCH}" >&2; exit 1 ;;
esac

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing command: $1" >&2; exit 1; }
}

need_cmd docker
need_cmd python3
need_cmd tar
need_cmd sha256sum

mkdir -p "${DIST_DIR}" "${BUILD_DIR}"

build_one() {
  local arch="$1"
  local payload="${BUILD_DIR}/payload-${arch}"
  local platform
  case "${arch}" in
    amd64) platform="linux/amd64" ;;
    arm64) platform="linux/arm64" ;;
  esac

  echo "[INFO] build ${NAME} ${VERSION} arch=${arch} platform=${platform}"
  rm -rf "${payload}"
  mkdir -p "${payload}/images" "${payload}/manifests" "${payload}/meta"

  cp "${ROOT_DIR}/VERSION" "${payload}/VERSION"
  cp -R "${ROOT_DIR}/manifests/." "${payload}/manifests/"

  python3 - "${ROOT_DIR}/images/image.json" "${arch}" "${VERSION}" > "${payload}/meta/selected-images.json" <<'PY'
import json, sys
path, arch, version = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
selected = []
for item in data:
    if item.get('arch') != arch:
        continue
    rendered = {}
    for k, v in item.items():
        if isinstance(v, str):
            v = v.replace('${VERSION}', version).replace('${ARCH}', arch)
        rendered[k] = v
    selected.append(rendered)
if not selected:
    raise SystemExit(f'no images for arch={arch}')
print(json.dumps(selected, indent=2, ensure_ascii=False))
PY

  printf 'name\tarch\tplatform\tsource_tag\ttarget_tag\ttar\n' > "${payload}/images/image-index.tsv"

  while IFS=$'\t' read -r name img_arch img_platform pull dockerfile context tag tar_name; do
    if [[ -z "${tag}" || -z "${tar_name}" ]]; then
      echo "invalid image entry: tag/tar required" >&2
      exit 1
    fi

    local source_tag="${tag}"
    if [[ -n "${dockerfile}" ]]; then
      echo "[INFO] docker buildx build --load ${tag}"
      docker buildx build --load --platform "${img_platform:-${platform}}" \
        -t "${tag}" \
        -f "${ROOT_DIR}/${dockerfile}" \
        "${ROOT_DIR}/${context}"
    else
      if [[ -z "${pull}" ]]; then
        echo "invalid image entry: pull or dockerfile required for ${name}" >&2
        exit 1
      fi
      echo "[INFO] docker pull ${pull}"
      docker pull --platform "${img_platform:-${platform}}" "${pull}"
      docker tag "${pull}" "${tag}"
      source_tag="${pull}"
    fi

    echo "[INFO] docker save ${tag} -> images/${tar_name}"
    docker save "${tag}" -o "${payload}/images/${tar_name}"
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${name}" "${img_arch}" "${img_platform:-${platform}}" "${source_tag}" "${tag}" "${tar_name}" >> "${payload}/images/image-index.tsv"
  done < <(python3 - "${payload}/meta/selected-images.json" <<'PY'
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as f:
    data = json.load(f)
for item in data:
    print('\t'.join([
        item.get('name',''),
        item.get('arch',''),
        item.get('platform',''),
        item.get('pull',''),
        item.get('dockerfile',''),
        item.get('context','.'),
        item.get('tag',''),
        item.get('tar',''),
    ]))
PY
)

  (
    cd "${payload}"
    tar -czf "${BUILD_DIR}/payload-${arch}.tar.gz" .
  )

  local out="${DIST_DIR}/${NAME}-${VERSION}-${arch}.run"
  cat "${ROOT_DIR}/install.sh" "${BUILD_DIR}/payload-${arch}.tar.gz" > "${out}"
  chmod +x "${out}"
  sha256sum "${out}" > "${out}.sha256"
  echo "[OK] ${out}"
  echo "[OK] ${out}.sha256"
}

if [[ "${ARCH}" == "all" ]]; then
  build_one amd64
  build_one arm64
else
  build_one "${ARCH}"
fi
