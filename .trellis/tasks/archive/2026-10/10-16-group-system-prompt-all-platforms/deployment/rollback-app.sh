#!/usr/bin/env bash
set -euo pipefail
ssh ovh bash -s <<'REMOTE'
set -euo pipefail
cd /opt/sub2api-deploy
cp '/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_7f4d33b06144-to-10f2f9a00dca-20261017-000319.yml' docker-compose.override.yml
docker compose config --quiet
docker compose up -d --no-deps --pull never sub2api
curl -fsS --max-time 5 http://127.0.0.1:8080/health
REMOTE
