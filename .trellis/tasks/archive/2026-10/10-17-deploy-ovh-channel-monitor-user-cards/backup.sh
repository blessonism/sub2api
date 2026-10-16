#!/usr/bin/env bash
set -euo pipefail
umask 077

[[ "$(docker inspect sub2api --format '{{.Image}}')" == sha256:c4d1904fae842bd21927d4329b9cd0767225b443a44464d40e4948c5c7c0f882 ]]
curl -fsS --max-time 5 http://127.0.0.1:8080/health >/dev/null
backup_dir="/opt/sub2api-backups/predeploy-a28f537673bd-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$backup_dir"
docker exec sub2api-postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc -Z1' > "$backup_dir/database.dump"
test -s "$backup_dir/database.dump"
# 完整解析备份中的数据块，输出丢弃；不连接或覆盖生产数据库。
docker exec -i sub2api-postgres pg_restore --file=/dev/null < "$backup_dir/database.dump"
tar -czf "$backup_dir/deployment-config.tar.gz" -C /opt/sub2api-deploy .env docker-compose.yml docker-compose.override.yml
tar -czf "$backup_dir/app-data.tar.gz" -C /opt/sub2api-deploy data
tar -tzf "$backup_dir/deployment-config.tar.gz" >/dev/null
tar -tzf "$backup_dir/app-data.tar.gz" >/dev/null
python3 - "$backup_dir" <<'PY'
import hashlib,json,pathlib,sys
p=pathlib.Path(sys.argv[1])
files={f.name:{'bytes':f.stat().st_size,'mode':oct(f.stat().st_mode & 0o777),'sha256':hashlib.file_digest(f.open('rb'),'sha256').hexdigest()} for f in p.iterdir() if f.is_file()}
print(json.dumps({'directory':str(p),'directory_mode':oct(p.stat().st_mode & 0o777),'database_restore_parse':'passed','files':files}))
PY
