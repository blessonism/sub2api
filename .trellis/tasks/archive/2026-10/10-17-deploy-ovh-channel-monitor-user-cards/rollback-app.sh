#!/usr/bin/env bash
# 仅回滚应用；保留新增可空 group_id 列，不恢复数据库或覆盖后续写入。
set -euo pipefail
ssh ovh 'bash -s' <<'REMOTE'
set -euo pipefail
cd /opt/sub2api-deploy
cp /opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_10f2f9a00dca-to-a28f537673bd-20261017-022420.yml docker-compose.override.yml
docker compose config --quiet
docker compose up -d --no-deps --pull never sub2api
for attempt in $(seq 1 60); do
  if curl -fsS --max-time 5 http://127.0.0.1:8080/health; then exit 0; fi
  sleep 2
done
exit 1
REMOTE
