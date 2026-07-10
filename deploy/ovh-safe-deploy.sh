#!/usr/bin/env bash
# 从本地或 CI 安全部署 custom/main 到 OVH。
# 生产机只允许加载镜像和切换 sub2api 容器，禁止在 OVH 上构建镜像。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SSH_HOST="${SSH_HOST:-ovh}"
PLATFORM="${SUB2API_PLATFORM:-linux/amd64}"
IMAGE_REPO="${SUB2API_IMAGE_REPO:-sub2api-custom}"
DEPLOY_DIR="${SUB2API_DEPLOY_DIR:-/opt/sub2api-deploy}"
BACKUP_DIR="${SUB2API_BACKUP_DIR:-/opt/sub2api-backups}"
CONTAINER_NAME="${SUB2API_CONTAINER_NAME:-sub2api}"
HEALTH_URL="${SUB2API_HEALTH_URL:-http://127.0.0.1:8080/health}"

REV=""
YES=0
SKIP_BUILD=0

usage() {
  cat <<'EOF'
Usage: deploy/ovh-safe-deploy.sh [options]

Build the target image outside OVH, load it over SSH, then switch only the
sub2api application container on OVH.

Options:
  --rev <12-char-rev>     Deploy a specific local commit. Defaults to HEAD.
  --ssh-host <host>       SSH host alias. Defaults to ovh.
  --platform <platform>   Docker build platform. Defaults to linux/amd64.
  --skip-build            Reuse an existing local image tag.
  --yes                   Do not prompt before loading/switching.
  -h, --help              Show this help.

Environment overrides:
  SSH_HOST, SUB2API_PLATFORM, SUB2API_IMAGE_REPO, SUB2API_DEPLOY_DIR,
  SUB2API_BACKUP_DIR, SUB2API_CONTAINER_NAME, SUB2API_HEALTH_URL
EOF
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --rev)
      REV="${2:-}"
      [[ -n "$REV" ]] || fail "--rev requires a value"
      shift 2
      ;;
    --ssh-host)
      SSH_HOST="${2:-}"
      [[ -n "$SSH_HOST" ]] || fail "--ssh-host requires a value"
      shift 2
      ;;
    --platform)
      PLATFORM="${2:-}"
      [[ -n "$PLATFORM" ]] || fail "--platform requires a value"
      shift 2
      ;;
    --skip-build)
      SKIP_BUILD=1
      shift
      ;;
    --yes)
      YES=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

cd "$REPO_ROOT"

require_command git
require_command docker
require_command ssh
require_command awk

if [[ "$(pwd -P)" == /opt/sub2api-src* || -d /opt/sub2api-deploy ]]; then
  fail "refusing to build/deploy from the OVH production host; run this script from local workstation or CI"
fi

docker buildx version >/dev/null 2>&1 || fail "docker buildx is required"

[[ -n "$REV" ]] || REV="$(git rev-parse --short=12 HEAD)"
FULL_HEAD="$(git rev-parse HEAD)"
SHORT_HEAD="$(git rev-parse --short=12 HEAD)"
IMAGE="${IMAGE_REPO}:${REV}"

[[ "$REV" == "$SHORT_HEAD" ]] || fail "requested rev $REV does not match checked-out HEAD $SHORT_HEAD"

if [[ -n "$(git status --porcelain)" ]]; then
  fail "working tree is not clean; deploy from a clean checkout of custom/main"
fi

git fetch origin custom/main
REMOTE_REV="$(git rev-parse --short=12 origin/custom/main)"
[[ "$REV" == "$REMOTE_REV" ]] || fail "HEAD $REV is not origin/custom/main $REMOTE_REV"

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$BRANCH" != "custom/main" && -z "${CI:-}" ]]; then
  fail "deploy from custom/main; current branch is $BRANCH"
fi

echo "Target image: $IMAGE"
echo "Target commit: $FULL_HEAD"
echo "SSH host: $SSH_HOST"
echo "Platform: $PLATFORM"
echo ""
echo "This will build outside OVH, load the image via SSH, back up override, switch only $CONTAINER_NAME, and health-check $HEALTH_URL."
echo "It will not restart PostgreSQL, Redis, Caddy/Nginx, or modify .env/DNS/firewall."

if [[ "$YES" -ne 1 ]]; then
  read -r -p "Type deploy to continue: " CONFIRM
  [[ "$CONFIRM" == "deploy" ]] || fail "cancelled"
fi

ssh "$SSH_HOST" \
  "CONTAINER_NAME='$CONTAINER_NAME' HEALTH_URL='$HEALTH_URL' bash -s" <<'REMOTE_PREFLIGHT'
set -euo pipefail

docker inspect "$CONTAINER_NAME" >/dev/null
curl -fsS --max-time 5 "$HEALTH_URL" >/dev/null
REMOTE_PREFLIGHT

if [[ "$SKIP_BUILD" -ne 1 ]]; then
  docker buildx build \
    --platform "$PLATFORM" \
    --load \
    --build-arg "COMMIT=$REV" \
    -t "$IMAGE" \
    -f "${REPO_ROOT}/deploy/Dockerfile" \
    "$REPO_ROOT"
else
  docker image inspect "$IMAGE" >/dev/null
fi

docker save "$IMAGE" | ssh "$SSH_HOST" 'docker load'

ssh "$SSH_HOST" \
  "REV='$REV' IMAGE_REPO='$IMAGE_REPO' DEPLOY_DIR='$DEPLOY_DIR' BACKUP_DIR='$BACKUP_DIR' CONTAINER_NAME='$CONTAINER_NAME' HEALTH_URL='$HEALTH_URL' bash -s" <<'REMOTE'
set -euo pipefail

IMAGE="${IMAGE_REPO}:${REV}"
docker image inspect "$IMAGE" >/dev/null

cd "$DEPLOY_DIR"
OLD_IMAGE="$(docker inspect "$CONTAINER_NAME" --format "{{.Config.Image}}")"
STAMP="$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
OLD_IMAGE_SAFE="$(printf "%s" "$OLD_IMAGE" | tr "/:" "__")"
BACKUP_FILE="${BACKUP_DIR}/docker-compose.override.predeploy-${OLD_IMAGE_SAFE}-to-${REV}-${STAMP}.yml"
cp docker-compose.override.yml "$BACKUP_FILE"

TMP_FILE="$(mktemp)"
if ! awk -v image="$IMAGE" '
  BEGIN { replaced = 0 }
  /^[[:space:]]*image:[[:space:]]*sub2api-custom:/ && replaced == 0 {
    indent = $0
    sub(/image:.*/, "", indent)
    print indent "image: " image
    replaced = 1
    next
  }
  { print }
  END { if (replaced == 0) exit 42 }
' docker-compose.override.yml > "$TMP_FILE"; then
  rm -f "$TMP_FILE"
  echo "ERROR: override does not contain a sub2api-custom image line; no change applied" >&2
  exit 1
fi

mv "$TMP_FILE" docker-compose.override.yml
docker compose config --quiet

rollback() {
  echo "Rolling back to $OLD_IMAGE using $BACKUP_FILE" >&2
  cp "$BACKUP_FILE" docker-compose.override.yml
  docker compose config --quiet
  docker compose up -d "$CONTAINER_NAME" || true
}

if ! docker compose up -d "$CONTAINER_NAME"; then
  rollback
  exit 1
fi

for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 "$HEALTH_URL"; then
    echo ""
    echo "Deployment healthy"
    echo "New image: $IMAGE"
    echo "Old image: $OLD_IMAGE"
    echo "Override backup: $BACKUP_FILE"
    exit 0
  fi
  sleep 2
done

rollback
exit 1
REMOTE
