"""只读上线验证；仅返回运行状态、迁移和配置哈希。"""
import hashlib
import json
import pathlib
import re
import subprocess

record = pathlib.Path(__file__).resolve().parent
source = pathlib.Path(json.loads((record / "source-evidence.json").read_text())["source"])
backup = re.search(r"Override backup: (.+)", (record / "deployment-result.txt").read_text()).group(1)
remote = r'''
import subprocess,json,hashlib,pathlib,datetime,time,shlex,urllib.request,urllib.error
def run(argv):return subprocess.check_output(argv,text=True).strip()
for attempt in range(30):
 state=run(['docker','inspect','sub2api','--format','{{.State.Health.Status}}'])
 if state=='healthy':break
 time.sleep(2)
assert state=='healthy',state
items=json.loads(run(['docker','inspect']+run(['docker','ps','-q']).splitlines()))
containers={}
for c in items:
 containers[c['Name'].lstrip('/') ]={'id':c['Id'],'image':c['Config']['Image'],'image_id':c['Image'],'started_at':c['State']['StartedAt'],'health':c['State'].get('Health',{}).get('Status'),'env_sha256':hashlib.sha256(json.dumps(c['Config']['Env'],sort_keys=True).encode()).hexdigest()}
files={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path('/opt/sub2api-deploy').iterdir() if p.is_file() and (p.name=='.env' or p.suffix in ['.yml','.yaml'])}
def query(sql):return run(['docker','exec','sub2api-postgres','sh','-c','exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c '+shlex.quote(sql)])
rows=query('SELECT filename, checksum FROM schema_migrations ORDER BY filename;')
applied={r.split('|',1)[0]:r.split('|',1)[1] for r in rows.splitlines()}
column=json.loads(query("SELECT row_to_json(x) FROM (SELECT data_type,is_nullable,column_default FROM information_schema.columns WHERE table_schema='public' AND table_name='groups' AND column_name='system_prompt_injection') x;"))
default_closed=query("SELECT NOT EXISTS (SELECT 1 FROM groups WHERE system_prompt_injection <> jsonb_build_object('enabled',false,'mode','append','content',''));")
trigger=query("SELECT position('OLD.system_prompt_injection IS NOT DISTINCT FROM NEW.system_prompt_injection' in pg_get_functiondef('enqueue_group_auth_cache_invalidation()'::regprocedure)) > 0;")
override=pathlib.Path('/opt/sub2api-deploy/docker-compose.override.yml').read_text()
old=pathlib.Path(OVERRIDE_BACKUP).read_text()
expected=old.replace('image: sub2api-custom:eb5bff47e499','image: sub2api-custom:7f4d33b06144',1)
version=run(['docker','exec','sub2api','/app/sub2api','-version'])
def request(path):
 try:
  response=urllib.request.urlopen('http://127.0.0.1:8080'+path,timeout=10)
 except urllib.error.HTTPError as e:
  response=e
 return response.status,response.read()
status,body=request('/health')
health=json.loads(body)
checks={}
for path in ['/','/setup/status','/api/v1/settings/public','/api/v1/admin/groups','/v1/models']:
 status,body=request(path)
 item={'http':status}
 if path=='/setup/status':
  data=json.loads(body)
  item['needs_setup']=data.get('needs_setup',data.get('data',{}).get('needs_setup'))
 if path=='/api/v1/settings/public':item['code']=json.loads(body).get('code')
 checks[path]=item
print(json.dumps({'time_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'containers':containers,'config_hashes':files,'migrations':applied,'new_column':column,'all_existing_group_configs_default_closed':default_closed=='t','durable_invalidation_trigger_updated':trigger=='t','override_only_image_changed':override==expected,'override_backup':OVERRIDE_BACKUP,'version_output':version,'health':health,'http':checks}))
'''
remote = "OVERRIDE_BACKUP=" + repr(backup) + "\n" + remote
r = subprocess.run(["ssh", "ovh", "python3 -"], input=remote, text=True, capture_output=True)
if r.returncode:
    print(r.stderr)
    raise SystemExit(r.returncode)
data = json.loads(r.stdout)
(record / "server-after.json").write_text(json.dumps(data, indent=2) + "\n")
before = json.loads((record / "server-before.json").read_text())
build = json.loads((record / "build-evidence.json").read_text())
assert data["containers"]["sub2api"]["image_id"] == build["image_id"]
assert data["containers"]["sub2api"]["health"] == "healthy"
environment = json.loads((record / "environment-validation.json").read_text())
assert environment["runtime_matches_effective_compose"]
assert not environment["different_env_keys"]
others = {k: v for k, v in before["containers"].items() if k != "sub2api"}
assert set(before["containers"]) == set(data["containers"])
for name, old in others.items():
    assert data["containers"][name] == old, name
for path, checksum in before["config_hashes"].items():
    if not path.endswith("docker-compose.override.yml"):
        assert data["config_hashes"][path] == checksum, path
assert data["override_only_image_changed"]
target = {p.name: hashlib.sha256(p.read_text().strip().encode()).hexdigest() for p in (source / "backend/migrations").glob("*.sql")}
assert data["migrations"] == target
assert data["new_column"]["data_type"] == "jsonb" and data["new_column"]["is_nullable"] == "NO"
assert data["all_existing_group_configs_default_closed"]
assert data["durable_invalidation_trigger_updated"]
assert data["health"] == {"status": "ok"}
assert "7f4d33b06144" in data["version_output"]
for path in ["/", "/setup/status", "/api/v1/settings/public"]:
    assert data["http"][path]["http"] == 200, (path, data["http"][path])
assert data["http"]["/setup/status"]["needs_setup"] is False
assert data["http"]["/api/v1/settings/public"]["code"] == 0
for path in ["/api/v1/admin/groups", "/v1/models"]:
    assert data["http"][path]["http"] == 401
result = {
    "status": "passed",
    "image": build["image"],
    "image_id": build["image_id"],
    "health": data["containers"]["sub2api"]["health"],
    "migrations": len(target),
    "existing_groups_default_closed": True,
    "durable_trigger_updated": True,
    "unchanged_other_containers": list(others),
    "environment_definitions_unchanged": True,
    "runtime_environment_matches_effective_compose": True,
    "environment_validation": environment,
    "override_only_image_changed": True,
    "override_backup": backup,
    "http": data["http"],
}
(record / "server-validation.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps(result, indent=2))
