"""校验公开站点的 TLS、健康、入口资源和分组管理页面产物。"""
import hashlib
import json
import pathlib
import re
import subprocess

record = pathlib.Path(__file__).resolve().parent
source = pathlib.Path(json.loads((record / "source-evidence.json").read_text())["source"])
dist = source / "backend/internal/web/dist"
origin = "https://api.edom37.online"

def fetch(path):
    return subprocess.check_output(["ssh", "ovh", "curl", "-fsS", "--retry", "2", "--retry-all-errors", "--connect-timeout", "10", "--max-time", "20", origin + path])

health = json.loads(fetch("/health"))
assert health == {"status": "ok"}
homepage = fetch("/").decode()
assets = set(re.findall(r'(?:src|href)="(/assets/[^"]+)"', (dist / "index.html").read_text()))
assert assets
assert all(path in homepage for path in assets), "homepage does not reference release assets"
groups = list((dist / "assets").glob("GroupsView-*.js"))
assert len(groups) == 1
assert b"system_prompt_injection" in groups[0].read_bytes()
assets.add("/assets/" + groups[0].name)
evidence = []
for path in sorted(assets):
    remote = fetch(path)
    expected = (dist / path.lstrip("/")).read_bytes()
    assert remote == expected, path
    evidence.append({"path": path, "sha256": hashlib.sha256(remote).hexdigest(), "bytes": len(remote), "matches_release": True})
settings = json.loads(fetch("/api/v1/settings/public"))
assert settings["code"] == 0
result = {"status": "passed", "origin": origin, "vantage_point": "OVH via public hostname and TLS", "local_vantage_note": "本机 health 可达，但首页由 Caddy 返回 403；通过 OVH 对同一公网域名执行 TLS、首页和资源校验。", "tls_verified": True, "health": health, "homepage_matches_release": True, "group_prompt_ui_bundle_matches": True, "settings_public_code": settings["code"], "assets": evidence}
(record / "public-validation.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps(result, indent=2))
