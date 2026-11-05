#!/usr/bin/env python3
"""可删除探针：验证「一步一条 generation、一轮一条 trace」在 Langfuse 上的最终渲染。

它只做三件事，全部可通过 DELETE /api/public/traces/<id> 清理：
  1. 先发 3 条 generation（此时 trace 尚不存在）；
  2. 再补 1 条 trace-create，时间戳取「该轮首步」；
  3. 回读 trace，断言 observations 数、input/output、总费用、时间戳都对。

用真实 ingestion API，不用 SDK，确保验证的是生产同一条链路。

用法：
    LF_HOST=https://infuse.dshvip.site \
    LF_PUBLIC_KEY=pk-lf-... LF_SECRET_KEY=sk-lf-... \
        python3 deploy/langfuse/probe-turn-card.py
"""

from __future__ import annotations

import base64
import json
import os
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone

HOST = os.environ.get("LF_HOST", "").rstrip("/")
PK = os.environ.get("LF_PUBLIC_KEY", "")
SK = os.environ.get("LF_SECRET_KEY", "")

if not (HOST and PK and SK):
    sys.exit("需要 LF_HOST / LF_PUBLIC_KEY / LF_SECRET_KEY")

AUTH = "Basic " + base64.b64encode(f"{PK}:{SK}".encode()).decode()


def call(method: str, path: str, body: dict | None = None) -> tuple[int, str]:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(HOST + path, data=data, method=method)
    req.add_header("Authorization", AUTH)
    # Cloudflare 会把 python-urllib 的默认 UA 当成爬虫（403 error code: 1010）。
    req.add_header("User-Agent", "sub2api-observability-probe/1.0")
    if data:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, resp.read().decode()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read().decode()


def ts(dt: datetime) -> str:
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def main() -> int:
    now = datetime.now(timezone.utc)
    stamp = int(now.timestamp())
    trace_id = f"tr-probe-turn-{stamp}"
    session_id = f"probe-turn-session-{stamp}"
    user_id = f"probe-turn-user-{stamp}"

    # 该轮的第一步在 6 分钟前，最后一步在 2 分钟前：卡片时间应取首步时间。
    t0 = now - timedelta(minutes=6)
    steps = [
        ("先看一下仓库结构", 0.001),
        ("", 0.002),  # 纯工具步：没有正文
        ("改完再跑测试，全部通过", 0.003),
    ]

    def generation(idx: int, text: str, cost: float) -> dict:
        started = t0 + timedelta(minutes=2 * idx)
        gen_body = {
            "id": f"gn-{trace_id}-{idx}",
            "traceId": trace_id,
            "name": "probe-model",
            "model": "probe-model",
            "costDetails": {"total": cost},
            "usageDetails": {"input": 100 + idx, "output": 20 + idx, "total": 120 + 2 * idx},
            "startTime": ts(started),
            "endTime": ts(started + timedelta(seconds=5)),
            "level": "DEFAULT",
            "metadata": {"probe": "turn-card", "thread_step": idx + 1, "turn_index": 7},
        }
        # 纯工具步没有正文：不发 output 字段，验证卡片仍能取到后面那条。
        if text:
            gen_body["output"] = text
        return {
            "id": f"ev-{trace_id}-g{idx}",
            "type": "generation-create",
            "timestamp": ts(started),
            "body": gen_body,
        }

    # 1) 先发 generation：此刻 trace 不存在，验证 Langfuse 会暂存而不是丢弃。
    batch = {
        "batch": [generation(i, text, cost) for i, (text, cost) in enumerate(steps)]
    }
    code, body = call("POST", "/api/public/ingestion", batch)
    print(f"[1] generation 先行: HTTP {code} {body[:160]}")
    if code >= 400:
        return 1

    # 2) 再补卡片：时间戳取该轮首步。
    card = {
        "batch": [
            {
                "id": f"ev-{trace_id}-t",
                "type": "trace-create",
                "timestamp": ts(t0),
                "body": {
                    "id": trace_id,
                    "name": "sub2api.turn · probe · t7",
                    "timestamp": ts(t0),
                    "userId": user_id,
                    "sessionId": session_id,
                    "tags": ["thread:th-probe"],
                    "input": "把观测改成一轮一张卡，先 review 再部署",
                    "output": "改完再跑测试，全部通过",
                    "metadata": {
                        "cost_source": "actual_charge",
                        "turn_steps": len(steps),
                        "turn_index": 7,
                        "turn_cost": sum(c for _, c in steps),
                        "session_source": "client",
                        "session_title": "把观测改成一轮一张卡",
                    },
                },
            }
        ]
    }
    code, body = call("POST", "/api/public/ingestion", card)
    print(f"[2] 卡片后补:       HTTP {code} {body[:160]}")
    if code >= 400:
        return 1

    # 3) 回读：ingestion 是异步的，给它几秒。
    time.sleep(6)
    code, body = call("GET", f"/api/public/traces/{trace_id}")
    print(f"[3] 回读 trace:     HTTP {code}")
    if code >= 400:
        print(body[:400])
        return 1

    trace = json.loads(body)
    observations = trace.get("observations") or []
    problems: list[str] = []

    if len(observations) != len(steps):
        problems.append(f"observations = {len(observations)}, want {len(steps)}（generation 挂了没）")
    if trace.get("input") != "把观测改成一轮一张卡，先 review 再部署":
        problems.append(f"input = {trace.get('input')!r}（用户消息没有落到卡片上）")
    if trace.get("output") != "改完再跑测试，全部通过":
        problems.append(f"output = {trace.get('output')!r}（回答没有落到卡片上）")
    want_cost = round(sum(c for _, c in steps), 6)
    got_cost = round(float(trace.get("totalCost") or 0), 6)
    if abs(got_cost - want_cost) > 1e-6:
        problems.append(f"totalCost = {got_cost}, want {want_cost}（各步费用没有汇总到卡片）")
    if trace.get("sessionId") != session_id:
        problems.append(f"sessionId = {trace.get('sessionId')!r}")
    if not str(trace.get("timestamp", "")).startswith(ts(t0)[:19]):
        problems.append(f"timestamp = {trace.get('timestamp')!r}, want 约 {ts(t0)}（应取该轮首步）")

    print("\n--- 判读 ---")
    if problems:
        for p in problems:
            print("  ✗", p)
    else:
        print("  ✓ 卡片聚合、正文分层、费用汇总、时间戳全部符合预期")

    code, body = call("DELETE", f"/api/public/traces/{trace_id}")
    print(f"\n[清理] DELETE trace: HTTP {code}")
    # 删除在 Langfuse 侧是异步的（要等 ClickHouse 标记），隔几秒再复查。
    for _ in range(6):
        time.sleep(5)
        code, _ = call("GET", f"/api/public/traces/{trace_id}")
        if code == 404:
            print("[清理] 复查 trace:   HTTP 404（已删除）")
            break
    else:
        print(f"[清理] 复查 trace:   HTTP {code}（仍在，稍后再确认）")

    return 0 if not problems else 2


if __name__ == "__main__":
    raise SystemExit(main())
