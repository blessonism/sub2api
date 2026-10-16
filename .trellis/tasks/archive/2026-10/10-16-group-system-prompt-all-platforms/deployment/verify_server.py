import hashlib,json,pathlib,re,subprocess,time,urllib.request,urllib.error
repo=pathlib.Path('/Users/suki/code/sub2api')
record=pathlib.Path(__file__).resolve().parent
info=json.loads((record/'build-evidence.json').read_text())
source=pathlib.Path(info['source'])
for attempt in range(30):
 state=subprocess.check_output(['ssh','ovh','docker','inspect','sub2api','--format','{{.State.Health.Status}}'],text=True).strip()
 if state=='healthy':break
 time.sleep(2)
assert state=='healthy'
subprocess.run(['python3',str(record/'snapshot_server.py'),str(record/'server-after.json')],check=True)
before=json.loads((record/'server-before.json').read_text());after=json.loads((record/'server-after.json').read_text())
app=after['containers']['sub2api'];old=before['containers']['sub2api']
assert app['image']==info['image'] and app['image_id']==info['image_id']
assert app['health']=='healthy' and app['restart_count']==0
assert app['env_sha256']==old['env_sha256']
assert after['health']=={'status':'ok'}
assert info['rev'] in after['version_output']
assert before['migrations']==after['migrations']
expected_migrations={p.name:hashlib.sha256(p.read_text().strip().encode()).hexdigest() for p in (source/'backend/migrations').glob('*.sql')}
assert after['migrations']==expected_migrations
assert set(before['containers'])==set(after['containers'])
for name,item in before['containers'].items():
 if name!='sub2api':assert item==after['containers'][name],name
for file,h in before['config_hashes'].items():
 if not file.endswith('docker-compose.override.yml'):assert after['config_hashes'][file]==h,file
backup=re.search(r'Override backup: (.+)',(record/'deployment-result.txt').read_text()).group(1)
remote='''import pathlib,json,urllib.request,urllib.error
old=pathlib.Path(BACKUP).read_text()
current=pathlib.Path('/opt/sub2api-deploy/docker-compose.override.yml').read_text()
assert current==old.replace('image: '+OLD_IMAGE,'image: '+NEW_IMAGE,1)
results={}
for path in ['/','/setup/status','/api/v1/settings/public','/api/v1/admin/groups','/v1/models']:
 try:r=urllib.request.urlopen('http://127.0.0.1:8080'+path,timeout=10)
 except urllib.error.HTTPError as e:r=e
 result={'http':r.status}
 body=r.read()
 if path=='/setup/status':
  data=json.loads(body);result['needs_setup']=data.get('needs_setup',data.get('data',{}).get('needs_setup'))
 if path=='/api/v1/settings/public':result['code']=json.loads(body).get('code')
 results[path]=result
print(json.dumps(results))
'''
remote='BACKUP='+repr(backup)+'\nOLD_IMAGE='+repr(old['image'])+'\nNEW_IMAGE='+repr(info['image'])+'\n'+remote
r=subprocess.run(['ssh','ovh','python3','-'],input=remote,capture_output=True,text=True,check=True)
http=json.loads(r.stdout)
for path in ['/','/setup/status','/api/v1/settings/public']:assert http[path]['http']==200
for path in ['/api/v1/admin/groups','/v1/models']:assert http[path]['http']==401
assert http['/setup/status']['needs_setup'] is False
assert http['/api/v1/settings/public']['code']==0
(record/'source-evidence.json').write_text(json.dumps(info,indent=2)+'\n')
public=(repo/'.trellis/tasks/archive/2026-10/10-16-deploy-ovh-group-system-prompt/verify_public.py').read_text()
public=public.replace('"local_vantage_note": "本机 health 可达，但首页由 Caddy 返回 403；通过 OVH 对同一公网域名执行 TLS、首页和资源校验。", ','')
(record/'verify_public.py').write_text(public)
subprocess.run(['python3',str(record/'verify_public.py')],check=True)
result={'status':'passed','image':info['image'],'image_id':info['image_id'],'health':app['health'],'restart_count':app['restart_count'],'migrations_unchanged':len(after['migrations']),'other_containers_unchanged':7,'config_and_environment_unchanged':True,'override_only_image_changed':True,'override_backup':backup,'http':http}
(record/'server-validation.json').write_text(json.dumps(result,indent=2)+'\n')
(record/'rollback-app.sh').write_text('#!/usr/bin/env bash\nset -euo pipefail\nssh ovh bash -s <<\'REMOTE\'\nset -euo pipefail\ncd /opt/sub2api-deploy\ncp '+repr(backup)+' docker-compose.override.yml\ndocker compose config --quiet\ndocker compose up -d --no-deps --pull never sub2api\ncurl -fsS --max-time 5 http://127.0.0.1:8080/health\nREMOTE\n')
print(json.dumps(result,indent=2))
