import json,pathlib,subprocess,sys
remote=r'''
import hashlib,json,pathlib,subprocess,datetime
def run(a):return subprocess.check_output(a,text=True).strip()
items=json.loads(run(['docker','inspect']+run(['docker','ps','-q']).splitlines()))
containers={}
for c in items:
 env=dict(x.split('=',1) for x in c['Config']['Env'])
 containers[c['Name'].lstrip('/') ]={'id':c['Id'],'image':c['Config']['Image'],'image_id':c['Image'],'started_at':c['State']['StartedAt'],'health':c['State'].get('Health',{}).get('Status'),'restart_count':c['RestartCount'],'env_sha256':hashlib.sha256(json.dumps(env,sort_keys=True).encode()).hexdigest()}
files={str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path('/opt/sub2api-deploy').iterdir() if p.is_file() and (p.name=='.env' or p.suffix in ['.yml','.yaml'])}
rows=run(['docker','exec','sub2api-postgres','sh','-c','exec psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "SELECT filename, checksum FROM schema_migrations ORDER BY filename;"'])
migrations=dict(r.split('|',1) for r in rows.splitlines())
print(json.dumps({'time_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'containers':containers,'config_hashes':files,'migrations':migrations,'health':json.loads(run(['curl','-fsS','--max-time','5','http://127.0.0.1:8080/health'])),'version_output':run(['docker','exec','sub2api','/app/sub2api','-version'])}))
'''
r=subprocess.run(['ssh','ovh','python3','-'],input=remote,text=True,capture_output=True,check=True)
data=json.loads(r.stdout)
pathlib.Path(sys.argv[1]).write_text(json.dumps(data,indent=2)+'\n')
print(json.dumps({'saved':sys.argv[1],'application':data['containers']['sub2api'],'containers':len(data['containers']),'migrations':len(data['migrations']),'health':data['health']},indent=2))
