"""通过公网域名/TLS 校验入口和 V1 渠道卡片相关产物字节。"""
import hashlib
import json
import pathlib
import re
import subprocess

record = pathlib.Path(__file__).resolve().parent
source = pathlib.Path(json.loads((record / 'source-evidence.json').read_text())['source'])
dist = source / 'backend/internal/web/dist'
origin = 'https://api.edom37.online'


def fetch(path):
    return subprocess.check_output(['ssh', 'ovh', 'curl', '-fsS', '--retry', '2', '--retry-all-errors', '--connect-timeout', '10', '--max-time', '20', origin + path])


health = json.loads(fetch('/health'))
assert health == {'status': 'ok'}
homepage = fetch('/').decode()
assets = set(re.findall(r'(?:src|href)="(/assets/[^"]+)"', (dist / 'index.html').read_text()))
assert assets and all(path in homepage for path in assets)
status_js = list((dist / 'assets').glob('ChannelStatusView-*.js'))
status_css = list((dist / 'assets').glob('ChannelStatusView-*.css'))
admin_js = list((dist / 'assets').glob('ChannelMonitorView-*.js'))
assert len(status_js) == len(status_css) == len(admin_js) == 1
js = status_js[0].read_text()
css = status_css[0].read_text()
for name in ('visible_rate_multiplier', 'cache_rate_24h', 'output_tps_5m', 'primary_checked_at'):
    assert name in js, name
assert 'border:0' in css and '--monitor-shadow:' in css and 'box-shadow:var(--monitor-shadow)' in css
assert 'group_id' in admin_js[0].read_text()
for p in status_js + status_css + admin_js:
    assets.add('/assets/' + p.name)
rows = []
for path in sorted(assets):
    remote = fetch(path)
    expected = (dist / path.lstrip('/')).read_bytes()
    assert remote == expected, path
    rows.append({'path': path, 'sha256': hashlib.sha256(remote).hexdigest(), 'bytes': len(remote), 'matches_release': True})
settings = json.loads(fetch('/api/v1/settings/public'))
assert settings['code'] == 0
summary = {
    'status': 'passed', 'origin': origin, 'vantage_point': 'OVH via public hostname and TLS',
    'tls_verified': True, 'health': health, 'homepage_matches_release': True,
    'v1_metrics_bundle_matches': True, 'borderless_shadow_css_matches': True,
    'admin_linked_group_bundle_matches': True, 'assets': rows,
}
(record / 'public-validation.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps(summary, indent=2))
