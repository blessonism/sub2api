"""从标准安全脚本生成固定 SHA 部署副本，并保存所有差异。"""
import difflib
import json
import pathlib
import shlex
import subprocess

record = pathlib.Path(__file__).resolve().parent
evidence = json.loads((record / 'source-evidence.json').read_text())
before = json.loads((record / 'server-before.json').read_text())
backup = json.loads((record / 'backup-evidence.json').read_text())
image = 'sub2api-custom:' + evidence['rev']
image_info = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image], text=True))[0]
assert image_info['Os'] == 'linux' and image_info['Architecture'] == 'amd64'
assert image_info['Config']['Labels']['org.opencontainers.image.revision'] == evidence['commit']
assert backup['database_restore_parse'] == 'passed'
repo_root = pathlib.Path('/Users/suki/code/sub2api')
original = (repo_root / 'deploy/ovh-safe-deploy.sh').read_text()
script = original


def replace(old, new):
    global script
    assert script.count(old) == 1, old
    script = script.replace(old, new, 1)


replace('REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"', 'REPO_ROOT=' + shlex.quote(str(repo_root)))
start = script.index('if [[ -n "$(git status --porcelain)" ]]')
end = script.index('\necho "Target image:', start)
checks = f'''# 发布用户指定的本地固定提交；构建输入已逐一核对 git blob，不附带 Git push。
EXPECTED_FULL_HEAD={evidence['commit']}
[[ "$FULL_HEAD" == "$EXPECTED_FULL_HEAD" ]] || fail "local HEAD changed"
[[ "$(git rev-parse custom/main)" == "$EXPECTED_FULL_HEAD" ]] || fail "custom/main changed"
[[ "$(git branch --show-current)" == custom/main ]] || fail "local branch changed"
git diff --quiet "$EXPECTED_FULL_HEAD" -- backend frontend deploy docs/legal || fail "tracked build inputs changed"
git diff --cached --quiet || fail "index changed"
[[ "$SKIP_BUILD" -eq 1 ]] || fail "fixed release requires --skip-build"
LOCAL_IMAGE_ID="$(docker image inspect "$IMAGE" --format '{{{{.Id}}}}')"
[[ "$LOCAL_IMAGE_ID" == {image_info['Id']} ]] || fail "local image ID changed"
[[ "$(docker image inspect "$IMAGE" --format '{{{{.Os}}}}/{{{{.Architecture}}}}')" == "$PLATFORM" ]] || fail "image platform mismatch"
[[ "$(docker image inspect "$IMAGE" --format '{{{{index .Config.Labels "org.opencontainers.image.revision"}}}}')" == "$EXPECTED_FULL_HEAD" ]] || fail "image revision mismatch"
'''
script = script[:start] + checks + script[end:]
replace("HEALTH_URL='$HEALTH_URL' bash -s\" <<'REMOTE'", "HEALTH_URL='$HEALTH_URL' EXPECTED_IMAGE_ID='$LOCAL_IMAGE_ID' bash -s\" <<'REMOTE'")
replace('docker image inspect "$IMAGE" >/dev/null\n\ncd "$DEPLOY_DIR"', '''[[ "$(docker image inspect "$IMAGE" --format '{{.Id}}')" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Loaded image ID mismatch' >&2; exit 1; }

cd "$DEPLOY_DIR"''')
gates = [
    f'[[ "$OLD_IMAGE" == {shlex.quote(before["containers"]["sub2api"]["image"])} ]] || {{ echo "Production image changed" >&2; exit 1; }}',
    f'''[[ "$(docker inspect "$CONTAINER_NAME" --format '{{{{.Image}}}}')" == {before['containers']['sub2api']['image_id']} ]] || {{ echo "Production image ID changed" >&2; exit 1; }}''',
]
for path, checksum in before['config_hashes'].items():
    gates.append(f'''[[ "$(sha256sum {shlex.quote(path)} | awk '{{print $1}}')" == {checksum} ]] || {{ echo "Deployment config changed" >&2; exit 1; }}''')
for name, item in backup['files'].items():
    path = str(pathlib.PurePosixPath(backup['directory']) / name)
    gates.append(f'''[[ "$(sha256sum {shlex.quote(path)} | awk '{{print $1}}')" == {item['sha256']} ]] || {{ echo "Verified backup changed" >&2; exit 1; }}''')
replace('STAMP="$(date +%Y%m%d-%H%M%S)"', '\n'.join(gates) + '\nSTAMP="$(date +%Y%m%d-%H%M%S)"')
replace('cp docker-compose.override.yml "$BACKUP_FILE"', 'cp docker-compose.override.yml "$BACKUP_FILE"\nchmod 600 "$BACKUP_FILE"')
old = '''mv "$TMP_FILE" docker-compose.override.yml
docker compose config --quiet

rollback() {
  echo "Rolling back to $OLD_IMAGE using $BACKUP_FILE" >&2
  cp "$BACKUP_FILE" docker-compose.override.yml
  docker compose config --quiet
  docker compose up -d "$CONTAINER_NAME" || true
}

if ! docker compose up -d "$CONTAINER_NAME"; then'''
new = '''rollback() {
  echo "Rolling back to $OLD_IMAGE using $BACKUP_FILE" >&2
  cp "$BACKUP_FILE" docker-compose.override.yml
  docker compose config --quiet
  docker compose up -d --no-deps --pull never "$CONTAINER_NAME" || true
}

mv "$TMP_FILE" docker-compose.override.yml
if ! docker compose config --quiet; then
  rollback
  exit 1
fi

if ! docker compose up -d --no-deps --pull never "$CONTAINER_NAME"; then'''
replace(old, new)
replace('for _ in $(seq 1 30); do\n  if curl -fsS --max-time 5 "$HEALTH_URL"; then', '''for _ in $(seq 1 60); do
  if [[ "$(docker inspect "$CONTAINER_NAME" --format '{{.Image}}')" == "$EXPECTED_IMAGE_ID" ]] && curl -fsS --max-time 5 "$HEALTH_URL"; then''')
wrapper = pathlib.Path(evidence['build_root']) / 'ovh-safe-deploy-pinned.sh'
wrapper.write_text(script)
wrapper.chmod(0o700)
subprocess.run(['bash', '-n', str(wrapper)], check=True)
(record / 'deploy-wrapper.diff').write_text(''.join(difflib.unified_diff(original.splitlines(True), script.splitlines(True), fromfile='deploy/ovh-safe-deploy.sh', tofile='temporary/ovh-safe-deploy-pinned.sh', n=0)))
print(json.dumps({'wrapper': str(wrapper), 'image': image, 'image_id': image_info['Id'], 'backup': backup['directory']}))
