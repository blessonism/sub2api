#!/usr/bin/env bash
# 从本地执行，仅回滚应用，不恢复数据库。
set -euo pipefail
ssh ovh 'set -euo pipefail
cd /opt/sub2api-deploy
cp "/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_eb5bff47e499-to-7f4d33b06144-20261016-215640.yml" docker-compose.override.yml
docker compose config --quiet
docker compose up -d --no-deps --pull never sub2api
'
