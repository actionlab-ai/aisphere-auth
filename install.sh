#!/usr/bin/env bash
set -euo pipefail

MARKER="__PAYLOAD_BELOW__"
APP_NAME="aisphere-auth"
DEFAULT_REGISTRY="sealos.hub:5000"

usage() {
  cat <<'EOF'
Usage:
  ./aisphere-auth-<version>-<arch>.run install [options]
  ./aisphere-auth-<version>-<arch>.run extract --output-dir <dir>

Install options:
  -y, --yes                         Non-interactive install
  --registry <registry>             Target offline registry, default: sealos.hub:5000
  --namespace <namespace>           Kubernetes namespace, default: aisphere-system
  --replicas <n>                    Deployment replicas, default: 1
  --skip-push                       Load and retag images, but do not push to target registry
  --skip-apply                      Render manifests, but do not kubectl apply
  --dry-run                         Print actions and render manifests only
  --output-dir <dir>                Keep extracted payload and rendered manifests

Runtime config options:
  --public-base-url <url>           AISPHERE_AUTH_PUBLIC_BASE_URL
  --cookie-domain <domain>          AISPHERE_COOKIE_DOMAIN
  --cookie-secure <true|false>      AISPHERE_COOKIE_SECURE, default: false
  --auth-mode <debug|release>       Gin mode, default: release
  --casdoor-endpoint <url>          Casdoor endpoint
  --casdoor-owner <owner>           Casdoor owner, default: skillhub
  --casdoor-application <app>       Casdoor application, default: aisphere
  --casdoor-client-id <id>          Casdoor client id
  --casdoor-client-secret <secret>  Casdoor client secret
  --casdoor-redirect-url <url>      Casdoor redirect url
  --casdoor-permission-id <id>      Casdoor permission id, default: aisphere/perm_aihub_admin
  --session-provider <memory|redis> Session provider, default: memory
  --redis-addrs <host:port,...>     Redis addresses when session-provider=redis
  --service-token <token>           Internal service token; generated if omitted
  --jwt-secret <secret>             JWT signing secret; generated if omitted

Examples:
  ./aisphere-auth-0.1.0-amd64.run install -y --registry sealos.hub:5000
  ./aisphere-auth-0.1.0-amd64.run install -y --registry 10.10.10.10:5000 --namespace aisphere-system
  ./aisphere-auth-0.1.0-amd64.run install -y --registry harbor.local:5000 --skip-apply --output-dir ./out
EOF
}

log() { echo "[INFO] $*"; }
warn() { echo "[WARN] $*" >&2; }
fail() { echo "[ERROR] $*" >&2; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"; }

payload_line() {
  awk -v marker="${MARKER}" '$0 == marker { print NR + 1; exit 0 }' "$0"
}

extract_payload() {
  local dest="$1"
  local line
  line="$(payload_line)"
  [[ -n "${line}" ]] || fail "payload marker not found"
  mkdir -p "${dest}"
  tail -n +"${line}" "$0" | tar -xzf - -C "${dest}"
}

rand_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  elif [[ -r /proc/sys/kernel/random/uuid ]]; then
    cat /proc/sys/kernel/random/uuid | tr -d '-'
  else
    date +%s%N | sha256sum | awk '{print $1}'
  fi
}

image_path_without_registry() {
  local image="$1"
  local first rest
  first="${image%%/*}"
  if [[ "${image}" == */* ]]; then
    rest="${image#*/}"
    if [[ "${first}" == *.* || "${first}" == *:* || "${first}" == "localhost" ]]; then
      echo "${rest}"
    else
      echo "${image}"
    fi
  else
    echo "${image}"
  fi
}

retarget_image() {
  local registry="$1"
  local image="$2"
  local path
  path="$(image_path_without_registry "${image}")"
  echo "${registry%/}/${path}"
}

sed_escape() {
  printf '%s' "$1" | sed -e 's/[\\&|]/\\&/g'
}

replace_token() {
  local file="$1" key="$2" value="$3"
  sed -i "s|__${key}__|$(sed_escape "${value}")|g" "${file}"
}

run_cmd() {
  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "+ $*"
  else
    "$@"
  fi
}

cmd="${1:-help}"
if [[ $# -gt 0 ]]; then shift; fi

YES="false"
REGISTRY="${DEFAULT_REGISTRY}"
NAMESPACE="aisphere-system"
REPLICAS="1"
SKIP_PUSH="false"
SKIP_APPLY="false"
DRY_RUN="false"
OUTPUT_DIR=""
PUBLIC_BASE_URL=""
COOKIE_DOMAIN=""
COOKIE_SECURE="false"
AUTH_MODE="release"
CASDOOR_ENDPOINT="http://casdoor:8000"
CASDOOR_OWNER="skillhub"
CASDOOR_APPLICATION="aisphere"
CASDOOR_CLIENT_ID="change-me"
CASDOOR_CLIENT_SECRET="change-me"
CASDOOR_REDIRECT_URL=""
CASDOOR_PERMISSION_ID="aisphere/perm_aihub_admin"
SESSION_PROVIDER="memory"
REDIS_ADDRS="127.0.0.1:6379"
SERVICE_TOKEN=""
JWT_SECRET=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--yes) YES="true"; shift ;;
    --registry) REGISTRY="${2:-}"; shift 2 ;;
    --namespace) NAMESPACE="${2:-}"; shift 2 ;;
    --replicas) REPLICAS="${2:-}"; shift 2 ;;
    --skip-push) SKIP_PUSH="true"; shift ;;
    --skip-apply) SKIP_APPLY="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    --output-dir) OUTPUT_DIR="${2:-}"; shift 2 ;;
    --public-base-url) PUBLIC_BASE_URL="${2:-}"; shift 2 ;;
    --cookie-domain) COOKIE_DOMAIN="${2:-}"; shift 2 ;;
    --cookie-secure) COOKIE_SECURE="${2:-}"; shift 2 ;;
    --auth-mode) AUTH_MODE="${2:-}"; shift 2 ;;
    --casdoor-endpoint) CASDOOR_ENDPOINT="${2:-}"; shift 2 ;;
    --casdoor-owner) CASDOOR_OWNER="${2:-}"; shift 2 ;;
    --casdoor-application) CASDOOR_APPLICATION="${2:-}"; shift 2 ;;
    --casdoor-client-id) CASDOOR_CLIENT_ID="${2:-}"; shift 2 ;;
    --casdoor-client-secret) CASDOOR_CLIENT_SECRET="${2:-}"; shift 2 ;;
    --casdoor-redirect-url) CASDOOR_REDIRECT_URL="${2:-}"; shift 2 ;;
    --casdoor-permission-id) CASDOOR_PERMISSION_ID="${2:-}"; shift 2 ;;
    --session-provider) SESSION_PROVIDER="${2:-}"; shift 2 ;;
    --redis-addrs) REDIS_ADDRS="${2:-}"; shift 2 ;;
    --service-token) SERVICE_TOKEN="${2:-}"; shift 2 ;;
    --jwt-secret) JWT_SECRET="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

case "${cmd}" in
  help|-h|--help) usage; exit 0 ;;
  extract)
    [[ -n "${OUTPUT_DIR}" ]] || fail "extract requires --output-dir"
    extract_payload "${OUTPUT_DIR}"
    log "payload extracted to ${OUTPUT_DIR}"
    exit 0
    ;;
  install) ;;
  *) fail "unknown command: ${cmd}" ;;
esac

[[ -n "${REGISTRY}" ]] || fail "--registry is empty"
if [[ "${YES}" != "true" && "${DRY_RUN}" != "true" ]]; then
  warn "interactive confirmation is disabled; use -y to continue"
  exit 1
fi

need_cmd docker
need_cmd tar
if [[ "${SKIP_APPLY}" != "true" && "${DRY_RUN}" != "true" ]]; then
  need_cmd kubectl
fi

if [[ -z "${PUBLIC_BASE_URL}" ]]; then PUBLIC_BASE_URL="http://aisphere-auth.${NAMESPACE}.svc.cluster.local:18080"; fi
if [[ -z "${SERVICE_TOKEN}" ]]; then SERVICE_TOKEN="$(rand_secret)"; fi
if [[ -z "${JWT_SECRET}" ]]; then JWT_SECRET="$(rand_secret)"; fi
if [[ -z "${CASDOOR_REDIRECT_URL}" ]]; then CASDOOR_REDIRECT_URL="${PUBLIC_BASE_URL%/}/auth/callback/casdoor"; fi

WORK_DIR="${OUTPUT_DIR:-$(mktemp -d -t aisphere-auth-run.XXXXXX)}"
PAYLOAD_DIR="${WORK_DIR}/payload"
RENDER_DIR="${WORK_DIR}/rendered"
mkdir -p "${PAYLOAD_DIR}" "${RENDER_DIR}"

log "extract payload to ${PAYLOAD_DIR}"
extract_payload "${PAYLOAD_DIR}"

INDEX="${PAYLOAD_DIR}/images/image-index.tsv"
[[ -f "${INDEX}" ]] || fail "image index not found: ${INDEX}"

SELECTED_IMAGE=""
log "load images"
while IFS=$'\t' read -r name arch platform source_tag target_tag tar_name; do
  [[ "${name}" == "name" ]] && continue
  [[ -n "${tar_name}" ]] || continue
  tar_path="${PAYLOAD_DIR}/images/${tar_name}"
  [[ -f "${tar_path}" ]] || fail "image tar not found: ${tar_path}"
  new_tag="$(retarget_image "${REGISTRY}" "${target_tag}")"
  log "docker load ${tar_name}"
  run_cmd docker load -i "${tar_path}"
  log "docker tag ${target_tag} ${new_tag}"
  run_cmd docker tag "${target_tag}" "${new_tag}"
  if [[ "${SKIP_PUSH}" != "true" ]]; then
    log "docker push ${new_tag}"
    run_cmd docker push "${new_tag}"
  fi
  if [[ "${name}" == "aisphere-auth" && -z "${SELECTED_IMAGE}" ]]; then
    SELECTED_IMAGE="${new_tag}"
  fi
done < "${INDEX}"

[[ -n "${SELECTED_IMAGE}" ]] || fail "aisphere-auth image not found in image index"

MANIFEST_SRC="${PAYLOAD_DIR}/manifests/aisphere-auth.yaml.tmpl"
MANIFEST_OUT="${RENDER_DIR}/aisphere-auth.yaml"
cp "${MANIFEST_SRC}" "${MANIFEST_OUT}"

replace_token "${MANIFEST_OUT}" "NAMESPACE" "${NAMESPACE}"
replace_token "${MANIFEST_OUT}" "IMAGE" "${SELECTED_IMAGE}"
replace_token "${MANIFEST_OUT}" "REPLICAS" "${REPLICAS}"
replace_token "${MANIFEST_OUT}" "SERVICE_TOKEN" "${SERVICE_TOKEN}"
replace_token "${MANIFEST_OUT}" "JWT_SECRET" "${JWT_SECRET}"
replace_token "${MANIFEST_OUT}" "PUBLIC_BASE_URL" "${PUBLIC_BASE_URL}"
replace_token "${MANIFEST_OUT}" "COOKIE_DOMAIN" "${COOKIE_DOMAIN}"
replace_token "${MANIFEST_OUT}" "COOKIE_SECURE" "${COOKIE_SECURE}"
replace_token "${MANIFEST_OUT}" "AUTH_MODE" "${AUTH_MODE}"
replace_token "${MANIFEST_OUT}" "CASDOOR_ENDPOINT" "${CASDOOR_ENDPOINT}"
replace_token "${MANIFEST_OUT}" "CASDOOR_OWNER" "${CASDOOR_OWNER}"
replace_token "${MANIFEST_OUT}" "CASDOOR_APPLICATION" "${CASDOOR_APPLICATION}"
replace_token "${MANIFEST_OUT}" "CASDOOR_CLIENT_ID" "${CASDOOR_CLIENT_ID}"
replace_token "${MANIFEST_OUT}" "CASDOOR_CLIENT_SECRET" "${CASDOOR_CLIENT_SECRET}"
replace_token "${MANIFEST_OUT}" "CASDOOR_REDIRECT_URL" "${CASDOOR_REDIRECT_URL}"
replace_token "${MANIFEST_OUT}" "CASDOOR_PERMISSION_ID" "${CASDOOR_PERMISSION_ID}"
replace_token "${MANIFEST_OUT}" "SESSION_PROVIDER" "${SESSION_PROVIDER}"
replace_token "${MANIFEST_OUT}" "REDIS_ADDRS" "${REDIS_ADDRS}"

log "rendered manifest: ${MANIFEST_OUT}"
if [[ "${SKIP_APPLY}" != "true" ]]; then
  log "kubectl apply -f ${MANIFEST_OUT}"
  run_cmd kubectl apply -f "${MANIFEST_OUT}"
else
  log "skip kubectl apply"
fi

log "done"
log "image=${SELECTED_IMAGE}"
log "namespace=${NAMESPACE}"
if [[ -z "${OUTPUT_DIR}" ]]; then
  log "temporary work dir: ${WORK_DIR}"
else
  log "output dir: ${OUTPUT_DIR}"
fi

exit 0

__PAYLOAD_BELOW__
