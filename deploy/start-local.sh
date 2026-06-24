#!/usr/bin/env bash
# 一键启动本地 Docker 开发环境。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.dev.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-deploy}"
APP_CONTAINER="${SUB2API_APP_CONTAINER:-sub2api-dev}"
POSTGRES_CONTAINER="${SUB2API_POSTGRES_CONTAINER:-sub2api-postgres-dev}"
REDIS_CONTAINER="${SUB2API_REDIS_CONTAINER:-sub2api-redis-dev}"

usage() {
  cat <<EOF
用法:
  ./deploy/start-local.sh [--logs|--no-logs|--rebuild|--status|--stop|--help]

选项:
  --logs      启动后跟随 sub2api 日志（默认）
  --no-logs   启动后只输出状态，不跟随日志
  --rebuild   基于当前代码重建镜像并启动
  --status    查看本地 sub2api 容器状态
  --stop      停止本地 sub2api 容器
  --help      显示帮助
EOF
}

log() {
  printf '[sub2api-local] %s\n' "$*"
}

die() {
  printf '[sub2api-local] 错误: %s\n' "$*" >&2
  exit 1
}

require_docker() {
  command -v docker >/dev/null 2>&1 || die "未找到 docker，请先安装 Docker Desktop。"
  docker info >/dev/null 2>&1 || die "Docker 未运行，请先启动 Docker Desktop。"
}

container_exists() {
  docker container inspect "$1" >/dev/null 2>&1
}

container_env_value() {
  local container="$1"
  local key="$2"
  docker inspect "${container}" \
    --format "{{range .Config.Env}}{{println .}}{{end}}" \
    | awk -F= -v key="${key}" '$1 == key {print substr($0, length(key) + 2); exit}'
}

write_env_line() {
  local file="$1"
  local key="$2"
  local value="$3"
  printf '%s=%s\n' "${key}" "${value}" >> "${file}"
}

create_temp_env_from_existing_containers() {
  container_exists "${APP_CONTAINER}" || return 1
  container_exists "${POSTGRES_CONTAINER}" || return 1

  local postgres_password
  postgres_password="$(container_env_value "${POSTGRES_CONTAINER}" POSTGRES_PASSWORD)"
  [ -n "${postgres_password}" ] || postgres_password="$(container_env_value "${APP_CONTAINER}" DATABASE_PASSWORD)"
  [ -n "${postgres_password}" ] || return 1

  local env_file
  env_file="$(mktemp "${TMPDIR:-/tmp}/sub2api-local-env.XXXXXX")"
  chmod 600 "${env_file}"

  write_env_line "${env_file}" POSTGRES_PASSWORD "${postgres_password}"
  for key in POSTGRES_USER POSTGRES_DB REDIS_PASSWORD REDIS_DB ADMIN_EMAIL ADMIN_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY TZ RUN_MODE SERVER_PORT BIND_HOST; do
    local value
    value="$(container_env_value "${APP_CONTAINER}" "${key}")"
    if [ -z "${value}" ]; then
      value="$(container_env_value "${POSTGRES_CONTAINER}" "${key}")"
    fi
    if [ -n "${value}" ]; then
      write_env_line "${env_file}" "${key}" "${value}"
    fi
  done

  printf '%s\n' "${env_file}"
}

start_existing_containers() {
  container_exists "${POSTGRES_CONTAINER}" || return 1
  container_exists "${REDIS_CONTAINER}" || return 1
  container_exists "${APP_CONTAINER}" || return 1

  log "检测到已有本地开发容器，直接恢复启动。"
  docker start "${POSTGRES_CONTAINER}" "${REDIS_CONTAINER}" >/dev/null
  log "等待 PostgreSQL / Redis 进入可用状态..."
  sleep 3
  docker start "${APP_CONTAINER}" >/dev/null
  return 0
}

start_with_compose() {
  local rebuild="$1"
  local env_file="${SCRIPT_DIR}/.env"
  local temp_env_file=""
  local env_args=()

  if [ -f "${env_file}" ]; then
    env_args=(--env-file "${env_file}")
  elif [ -n "${POSTGRES_PASSWORD:-}" ]; then
    log "未找到 deploy/.env，使用当前 shell 环境变量启动。"
  else
    if [ "${rebuild}" != "true" ]; then
      return 1
    fi
    temp_env_file="$(create_temp_env_from_existing_containers)" || return 1
    env_file="${temp_env_file}"
    env_args=(--env-file "${env_file}")
    log "未找到 deploy/.env，已从旧容器提取必要环境变量用于本次重建。"
  fi

  log "使用 docker-compose.dev.yml 启动本地开发环境。"
  local compose_status=0
  if [ "${rebuild}" = "true" ]; then
    docker compose --project-name "${PROJECT_NAME}" "${env_args[@]}" -f "${COMPOSE_FILE}" up -d --build || compose_status=$?
  else
    docker compose --project-name "${PROJECT_NAME}" "${env_args[@]}" -f "${COMPOSE_FILE}" up -d || compose_status=$?
  fi

  if [ -n "${temp_env_file}" ]; then
    rm -f "${temp_env_file}"
  fi
  return "${compose_status}"
}

show_status() {
  docker ps -a \
    --filter "name=^/${APP_CONTAINER}$" \
    --filter "name=^/${POSTGRES_CONTAINER}$" \
    --filter "name=^/${REDIS_CONTAINER}$" \
    --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
}

stop_local() {
  local stopped=false
  for name in "${APP_CONTAINER}" "${POSTGRES_CONTAINER}" "${REDIS_CONTAINER}"; do
    if container_exists "${name}"; then
      docker stop "${name}" >/dev/null || true
      stopped=true
    fi
  done
  if [ "${stopped}" = "true" ]; then
    log "本地开发容器已停止。"
  else
    log "未找到本地开发容器。"
  fi
}

main() {
  local follow_logs=true
  local rebuild=false
  local action="start"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --logs)
        follow_logs=true
        ;;
      --no-logs)
        follow_logs=false
        ;;
      --rebuild)
        rebuild=true
        ;;
      --status)
        action="status"
        ;;
      --stop)
        action="stop"
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        die "未知参数: $1"
        ;;
    esac
    shift
  done

  require_docker

  case "${action}" in
    status)
      show_status
      exit 0
      ;;
    stop)
      stop_local
      exit 0
      ;;
  esac

  if ! start_with_compose "${rebuild}"; then
    if [ "${rebuild}" = "true" ]; then
      die "无法重建：没有 deploy/.env，也无法从旧容器提取 POSTGRES_PASSWORD。"
    fi
    if ! start_existing_containers; then
      die "没有 deploy/.env，也没有可恢复的旧容器。请先运行 deploy/docker-deploy.sh 生成 .env，或提供 POSTGRES_PASSWORD 后重试。"
    fi
  fi

  log "启动完成。访问地址: http://127.0.0.1:8080"
  show_status

  if [ "${follow_logs}" = "true" ]; then
    log "正在跟随 ${APP_CONTAINER} 日志，按 Ctrl+C 退出日志跟随。"
    docker logs -f "${APP_CONTAINER}"
  fi
}

main "$@"
