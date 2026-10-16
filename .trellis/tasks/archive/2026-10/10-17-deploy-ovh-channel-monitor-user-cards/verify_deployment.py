"""校验版本、迁移、容器隔离、配置哈希和公开 HTTP。"""
import hashlib
import json
import pathlib
import subprocess

record = pathlib.Path(__file__).resolve().parent
evidence = json.loads((record / 'source-evidence.json').read_text())
build = json.loads((record / 'build-evidence.json').read_text())
before = json.loads((record / 'server-before.json').read_text())
wait = '''set -eu
for attempt in $(seq 1 30); do
  if [ "$(docker inspect sub2api --format '{{.State.Health.Status}}')" = healthy ]; then exit 0; fi
  sleep 2
done
exit 1
'''
subprocess.run(['ssh', 'ovh', 'bash -s'], input=wait, text=True, check=True)
result = subprocess.run(['ssh', 'ovh', 'python3 -'], input=(record / 'server_snapshot.py').read_text(), text=True, capture_output=True)
if result.returncode:
    print(result.stderr)
    raise SystemExit(result.returncode)
after = json.loads(result.stdout)
(record / 'server-after.json').write_text(json.dumps(after, indent=2) + '\n')
app = after['containers']['sub2api']
assert app['image_id'] == build['image_id']
assert app['image'] == build['image']
assert app['health'] == 'healthy' and app['restart_count'] == 0
assert app['env_sha256'] == before['containers']['sub2api']['env_sha256']
assert set(before['containers']) == set(after['containers'])
others = []
for name, old in before['containers'].items():
    if name == 'sub2api':
        continue
    assert after['containers'][name] == old, name
    others.append(name)
for path, digest in before['config_hashes'].items():
    if not path.endswith('docker-compose.override.yml'):
        assert after['config_hashes'][path] == digest, path
assert before['override_normalized_sha256'] == after['override_normalized_sha256']
source = pathlib.Path(evidence['source'])
target = {p.name: hashlib.sha256(p.read_text().strip().encode()).hexdigest() for p in (source / 'backend/migrations').glob('*.sql')}
assert after['migrations'] == target
assert set(after['migrations']) - set(before['migrations']) == {'246_channel_monitor_group_id.sql'}
assert after['group_id_column'] == {'data_type': 'bigint', 'is_nullable': 'YES'}
assert 'REFERENCES groups(id) ON DELETE SET NULL' in after['group_id_foreign_key']
assert evidence['rev'] in after['version']
for path in ('/health', '/', '/setup/status', '/api/v1/settings/public'):
    assert after['http'][path]['http'] == 200, path
assert after['http']['/health']['body'] == {'status': 'ok'}
assert after['http']['/setup/status']['needs_setup'] is False
for path in ('/api/v1/channel-monitors', '/api/v1/admin/channel-monitors'):
    assert after['http'][path]['http'] == 401, path
summary = {
    'status': 'passed', 'image': app['image'], 'image_id': app['image_id'],
    'health': app['health'], 'restart_count': app['restart_count'],
    'runtime_environment_unchanged': True, 'other_containers_unchanged': others,
    'base_config_and_env_unchanged': True, 'override_only_image_changed': True,
    'migrations': len(target), 'new_migrations': ['246_channel_monitor_group_id.sql'],
    'group_id_column': after['group_id_column'], 'group_id_foreign_key': after['group_id_foreign_key'],
    'http': after['http'],
}
(record / 'server-validation.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps(summary, indent=2))
