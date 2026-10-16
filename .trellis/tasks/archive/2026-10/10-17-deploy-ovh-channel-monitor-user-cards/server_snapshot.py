"""只读生产快照，只记录配置/环境哈希和数据库结构。"""
import datetime
import hashlib
import json
import pathlib
import shlex
import subprocess
import urllib.error
import urllib.request


def run(args):
    return subprocess.check_output(args, text=True).strip()


def query(sql):
    return run([
        'docker', 'exec', 'sub2api-postgres', 'sh', '-c',
        'exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c ' + shlex.quote(sql),
    ])


def digest(value):
    return hashlib.sha256(value).hexdigest()


containers = {}
items = json.loads(run(['docker', 'inspect'] + run(['docker', 'ps', '-q']).splitlines()))
for c in items:
    env = dict(item.split('=', 1) for item in c['Config']['Env'])
    containers[c['Name'].lstrip('/')] = {
        'id': c['Id'], 'image': c['Config']['Image'], 'image_id': c['Image'],
        'started_at': c['State']['StartedAt'],
        'health': c['State'].get('Health', {}).get('Status'),
        'restart_count': c['RestartCount'],
        'env_sha256': digest(json.dumps(env, sort_keys=True).encode()),
    }
app = next(c for c in items if c['Name'] == '/sub2api')
config_dir = pathlib.Path('/opt/sub2api-deploy')
config_hashes = {
    str(p): digest(p.read_bytes()) for p in config_dir.iterdir()
    if p.is_file() and (p.name == '.env' or p.suffix in ('.yml', '.yaml'))
}
override = (config_dir / 'docker-compose.override.yml').read_text()
image_line = next(line.strip() for line in override.splitlines() if line.strip().startswith('image: sub2api-custom:'))
normalized_override = override.replace(image_line, 'image: RELEASE_IMAGE', 1)
migrations = dict(line.split('|', 1) for line in query('SELECT filename, checksum FROM schema_migrations ORDER BY filename;').splitlines())
column = query("SELECT row_to_json(x) FROM (SELECT data_type,is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name='channel_monitors' AND column_name='group_id') x;")
foreign_key = query("SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='channel_monitors'::regclass AND contype='f' AND pg_get_constraintdef(oid) LIKE 'FOREIGN KEY (group_id)%';")
http = {}
for path in ('/health', '/', '/setup/status', '/api/v1/settings/public', '/api/v1/channel-monitors', '/api/v1/admin/channel-monitors'):
    try:
        response = urllib.request.urlopen('http://127.0.0.1:8080' + path, timeout=10)
    except urllib.error.HTTPError as exc:
        response = exc
    body = response.read()
    item = {'http': response.status}
    if path == '/health':
        item['body'] = json.loads(body)
    if path == '/setup/status':
        data = json.loads(body)
        item['needs_setup'] = data.get('needs_setup', data.get('data', {}).get('needs_setup'))
    http[path] = item
print(json.dumps({
    'time_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'containers': containers, 'config_hashes': config_hashes,
    'override_normalized_sha256': digest(normalized_override.encode()),
    'app_mounts': [{k: m[k] for k in ('Type', 'Source', 'Destination')} for m in app['Mounts']],
    'migrations': migrations, 'group_id_column': json.loads(column) if column else None,
    'group_id_foreign_key': foreign_key,
    'version': run(['docker', 'exec', 'sub2api', '/app/sub2api', '-version']),
    'http': http,
}))
