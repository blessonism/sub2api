#!/usr/bin/env python3
"""下游分支同步上游时的只读预检与定向检查入口。"""

from __future__ import annotations

import argparse
import fnmatch
import json
import os
import signal
import subprocess
import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, replace
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any, Callable, Iterable, Sequence


SCHEMA_VERSION = 1
MAX_CAPTURE_BYTES = 16_384
DEFAULT_CONFIG = Path(__file__).with_name("upstream_sync_checks.json")
READ_ONLY_GIT_COMMANDS = {
    "cat-file",
    "diff",
    "diff-tree",
    "merge-base",
    "rev-list",
    "rev-parse",
    "show",
}


class SyncError(RuntimeError):
    """可向调用方展示的配置、Git 或命令错误。"""


@dataclass(frozen=True)
class Change:
    status: str
    old_path: str | None
    new_path: str | None
    additions: int | None = None
    deletions: int | None = None

    @property
    def paths(self) -> set[str]:
        return {path for path in (self.old_path, self.new_path) if path is not None}

    def to_dict(self) -> dict[str, Any]:
        return {
            "status": self.status,
            "old_path": self.old_path,
            "new_path": self.new_path,
            "additions": self.additions,
            "deletions": self.deletions,
        }


@dataclass(frozen=True)
class CheckDefinition:
    id: str
    description: str
    lane: str
    paths: tuple[str, ...]
    argv: tuple[str, ...]
    cwd: str
    timeout_seconds: int
    fallback: bool = False


def _decode_path(value: bytes) -> str:
    return value.decode("utf-8", errors="surrogateescape")


def parse_name_status_z(data: bytes) -> list[Change]:
    """解析 ``git diff --name-status -z``，路径永远不按换行拆分。"""

    fields = data.split(b"\0")
    if fields and fields[-1] == b"":
        fields.pop()
    changes: list[Change] = []
    index = 0
    while index < len(fields):
        status = fields[index].decode("ascii", errors="strict")
        index += 1
        if not status:
            raise SyncError("Git diff 返回了空状态字段")
        kind = status[0]
        if kind in {"R", "C"}:
            if index + 1 >= len(fields):
                raise SyncError(f"Git diff 的 {status} 记录缺少路径")
            old_path = _decode_path(fields[index])
            new_path = _decode_path(fields[index + 1])
            index += 2
        else:
            if index >= len(fields):
                raise SyncError(f"Git diff 的 {status} 记录缺少路径")
            path = _decode_path(fields[index])
            index += 1
            old_path = None if kind == "A" else path
            new_path = None if kind == "D" else path
        changes.append(Change(status=status, old_path=old_path, new_path=new_path))
    return changes


def _parse_numstat_number(value: bytes) -> int | None:
    return None if value == b"-" else int(value)


def parse_numstat_z(data: bytes) -> dict[str, tuple[int | None, int | None]]:
    """解析 ``git diff --numstat -z``，包括 rename 的三段路径格式。"""

    fields = data.split(b"\0")
    if fields and fields[-1] == b"":
        fields.pop()
    stats: dict[str, tuple[int | None, int | None]] = {}
    index = 0
    while index < len(fields):
        header = fields[index]
        index += 1
        parts = header.split(b"\t", 2)
        if len(parts) != 3:
            raise SyncError("Git numstat 返回了无法解析的记录")
        additions = _parse_numstat_number(parts[0])
        deletions = _parse_numstat_number(parts[1])
        if parts[2]:
            stats[_decode_path(parts[2])] = (additions, deletions)
            continue
        if index + 1 >= len(fields):
            raise SyncError("Git numstat 的 rename/copy 记录缺少路径")
        old_path = _decode_path(fields[index])
        new_path = _decode_path(fields[index + 1])
        index += 2
        stats[old_path] = (additions, deletions)
        stats[new_path] = (additions, deletions)
    return stats


def attach_numstat(
    changes: Sequence[Change], stats: dict[str, tuple[int | None, int | None]]
) -> list[Change]:
    result: list[Change] = []
    for change in changes:
        key = change.new_path or change.old_path
        additions, deletions = stats.get(key or "", (None, None))
        result.append(replace(change, additions=additions, deletions=deletions))
    return result


class GitRepository:
    def __init__(self, root: Path):
        self.root = root.resolve()

    def _run(
        self,
        args: Sequence[str],
        *,
        allowed_returncodes: Iterable[int] = (0,),
    ) -> subprocess.CompletedProcess[bytes]:
        if not args or args[0] not in READ_ONLY_GIT_COMMANDS:
            raise SyncError(f"预检拒绝执行非只读 Git 命令: {' '.join(args)}")
        completed = subprocess.run(
            ["git", *args],
            cwd=self.root,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=False,
            check=False,
            timeout=30,
        )
        if completed.returncode not in set(allowed_returncodes):
            detail = completed.stderr.decode("utf-8", errors="replace").strip()
            raise SyncError(f"git {' '.join(args)} 失败: {detail or completed.returncode}")
        return completed

    def run(
        self,
        args: Sequence[str],
        *,
        allowed_returncodes: Iterable[int] = (0,),
    ) -> bytes:
        return self._run(args, allowed_returncodes=allowed_returncodes).stdout

    def resolve(self, ref: str) -> str:
        output = self.run(["rev-parse", "--verify", f"{ref}^{{commit}}"])
        return output.decode("ascii").strip()

    def merge_bases(self, left: str, right: str) -> list[str]:
        output = self.run(["merge-base", "--all", left, right])
        bases = output.decode("ascii").split()
        if not bases:
            raise SyncError(f"{left} 与 {right} 没有 merge-base")
        return bases

    def changes(self, start: str, end: str) -> list[Change]:
        names = self.run(
            ["diff", "--name-status", "-z", "-M50%", "-C50%", start, end, "--"]
        )
        numstat = self.run(["diff", "--numstat", "-z", start, end, "--"])
        return attach_numstat(parse_name_status_z(names), parse_numstat_z(numstat))

    def copy_successors(self, start: str, end: str) -> dict[str, list[str]]:
        """返回低阈值 copy 关系，作为文件拆分的人审证据。"""

        output = self.run(
            [
                "diff",
                "--name-status",
                "-z",
                "--find-copies-harder",
                "-C20%",
                start,
                end,
                "--",
            ]
        )
        relationships: dict[str, set[str]] = {}
        for change in parse_name_status_z(output):
            if change.status[0] != "C" or change.old_path is None or change.new_path is None:
                continue
            relationships.setdefault(change.old_path, set()).add(change.new_path)
        return {source: sorted(destinations) for source, destinations in relationships.items()}

    def commit_paths(self, commit: str) -> set[str]:
        output = self.run(
            ["diff-tree", "--root", "--no-commit-id", "--name-status", "-r", "-z", commit]
        )
        return change_paths(parse_name_status_z(output))

    def commit_side(self, commit: str, downstream: str, upstream: str) -> str:
        try:
            self.run(["cat-file", "-e", f"{commit}^{{commit}}"])
            in_downstream = self._is_ancestor(commit, downstream)
            in_upstream = self._is_ancestor(commit, upstream)
        except SyncError:
            return "missing"
        if in_downstream and in_upstream:
            return "common"
        if in_downstream:
            return "downstream-only"
        if in_upstream:
            return "incoming-upstream"
        return "missing"

    def _is_ancestor(self, commit: str, ref: str) -> bool:
        completed = self._run(
            ["merge-base", "--is-ancestor", commit, ref],
            allowed_returncodes=(0, 1),
        )
        return completed.returncode == 0

    def line_count(self, ref: str, path: str) -> int | None:
        try:
            content = self.run(["show", f"{ref}:{path}"])
        except SyncError:
            return None
        if not content:
            return 0
        return content.count(b"\n") + (0 if content.endswith(b"\n") else 1)

    def commit_count(self, start: str, end: str) -> int:
        output = self.run(["rev-list", "--count", f"{start}..{end}"])
        return int(output.decode("ascii").strip())


def change_paths(changes: Sequence[Change]) -> set[str]:
    paths: set[str] = set()
    for change in changes:
        paths.update(change.paths)
    return paths


def matches_any(path: str, patterns: Sequence[str]) -> bool:
    return any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns)


def _same_prefix_successors(source: str, additions: Sequence[str]) -> list[str]:
    source_path = PurePosixPath(source)
    stem = source_path.stem
    sibling_prefix = str(source_path.parent / f"{stem}_")
    module_prefix = str(source_path.parent / stem) + "/"
    return sorted(
        path
        for path in additions
        if path.startswith(sibling_prefix) or path.startswith(module_prefix)
    )


def detect_structural_signals(
    changes: Sequence[Change],
    base_line_count: Callable[[str], int | None],
    copy_successors: dict[str, list[str]] | None = None,
) -> list[dict[str, Any]]:
    additions = sorted(
        change.new_path
        for change in changes
        if change.new_path is not None and change.status[0] in {"A", "C"}
    )
    signals: list[dict[str, Any]] = []
    for change in changes:
        kind = change.status[0]
        source = change.old_path
        if source is None or kind not in {"M", "D", "R"}:
            continue
        successors = _same_prefix_successors(source, additions)
        if len(successors) < 2:
            continue
        total_lines = base_line_count(source)
        deleted_lines = change.deletions
        ratio = (
            deleted_lines / total_lines
            if deleted_lines is not None and total_lines not in (None, 0)
            else None
        )
        if kind != "D" and (ratio is None or ratio < 0.5):
            continue
        signals.append(
            {
                "kind": "file-split",
                "severity": "high",
                "source": source,
                "status": change.status,
                "deleted_lines": deleted_lines,
                "base_lines": total_lines,
                "deletion_ratio": round(ratio, 4) if ratio is not None else None,
                "successors": successors,
                "copy_successors": sorted(
                    set((copy_successors or {}).get(source, [])) & set(successors)
                ),
            }
        )
    return signals


def load_config(path: Path) -> tuple[dict[str, Any], dict[str, CheckDefinition]]:
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SyncError(f"无法读取检查矩阵 {path}: {exc}") from exc
    if not isinstance(raw, dict):
        raise SyncError("检查矩阵顶层必须是对象")
    if raw.get("schema_version") != SCHEMA_VERSION:
        raise SyncError(f"检查矩阵 schema_version 必须为 {SCHEMA_VERSION}")
    if not isinstance(raw.get("capabilities"), list) or not isinstance(raw.get("checks"), list):
        raise SyncError("检查矩阵必须包含 capabilities 和 checks 数组")

    checks: dict[str, CheckDefinition] = {}
    for item in raw["checks"]:
        if not isinstance(item, dict):
            raise SyncError(f"检查定义必须是对象: {item!r}")
        for field in ("id", "description", "lane"):
            if not isinstance(item.get(field), str) or not item[field]:
                raise SyncError(f"检查定义的 {field} 必须是非空字符串: {item!r}")
        for field in ("paths", "argv"):
            values = item.get(field)
            if not isinstance(values, list) or not values or not all(
                isinstance(value, str) and value for value in values
            ):
                raise SyncError(f"检查 {item['id']} 的 {field} 必须是非空字符串数组")
        cwd_value = item.get("cwd", ".")
        if not isinstance(cwd_value, str):
            raise SyncError(f"检查 {item['id']} 的 cwd 必须是字符串")
        timeout_value = item.get("timeout_seconds")
        if not isinstance(timeout_value, int) or isinstance(timeout_value, bool):
            raise SyncError(f"检查 {item['id']} 的 timeout_seconds 必须是整数")
        check = CheckDefinition(
            id=item["id"],
            description=item["description"],
            lane=item["lane"],
            paths=tuple(item["paths"]),
            argv=tuple(item["argv"]),
            cwd=cwd_value,
            timeout_seconds=timeout_value,
            fallback=item.get("fallback", False),
        )
        if check.id in checks:
            raise SyncError(f"检查 ID 重复: {check.id}")
        cwd = PurePosixPath(check.cwd)
        if cwd.is_absolute() or ".." in cwd.parts:
            raise SyncError(f"检查 {check.id} 的 cwd 必须位于仓库内")
        if not 1 <= check.timeout_seconds <= 60:
            raise SyncError(f"检查 {check.id} 的 timeout_seconds 必须在 1..60")
        if not isinstance(item.get("fallback", False), bool):
            raise SyncError(f"检查 {check.id} 的 fallback 必须是布尔值")
        checks[check.id] = check

    capability_ids: set[str] = set()
    for capability in raw["capabilities"]:
        if not isinstance(capability, dict):
            raise SyncError(f"capability 必须是对象: {capability!r}")
        capability_id = capability.get("id")
        if not isinstance(capability_id, str) or not capability_id:
            raise SyncError("capability 缺少稳定 ID")
        if capability_id in capability_ids:
            raise SyncError(f"capability ID 重复: {capability_id}")
        capability_ids.add(capability_id)
        for field in ("paths", "tracked_commits", "checks"):
            values = capability.get(field)
            if not isinstance(values, list) or not all(
                isinstance(value, str) and value for value in values
            ):
                raise SyncError(
                    f"capability {capability_id} 的 {field} 必须是非空字符串数组"
                )
        if not capability["paths"] or not capability["checks"]:
            raise SyncError(f"capability {capability_id} 的 paths/checks 不得为空")
        successors = capability.get("structural_successors", [])
        if not isinstance(successors, list):
            raise SyncError(f"capability {capability_id} 的 structural_successors 必须是数组")
        for successor in successors:
            if (
                not isinstance(successor, dict)
                or not isinstance(successor.get("source"), str)
                or not isinstance(successor.get("successors"), list)
                or not all(
                    isinstance(pattern, str) and pattern
                    for pattern in successor["successors"]
                )
            ):
                raise SyncError(f"capability {capability_id} 的 structural_successors 无效")
        unknown = set(capability.get("checks", [])) - checks.keys()
        if unknown:
            raise SyncError(f"capability {capability_id} 引用了未知检查: {sorted(unknown)}")
    return raw, checks


def _tracked_commit_records(
    git: GitRepository,
    capability: dict[str, Any],
    downstream: str,
    upstream: str,
    downstream_paths: set[str],
    upstream_paths: set[str],
) -> tuple[list[dict[str, Any]], set[str]]:
    records: list[dict[str, Any]] = []
    affected_paths: set[str] = set()
    for commit in capability.get("tracked_commits", []):
        side = git.commit_side(commit, downstream, upstream)
        paths: set[str] = set()
        intersections: set[str] = set()
        if side != "missing":
            try:
                paths = git.commit_paths(commit)
            except SyncError:
                side = "missing"
        if side == "downstream-only":
            intersections = paths & upstream_paths
        elif side == "incoming-upstream":
            intersections = paths & downstream_paths
        affected_paths.update(intersections)
        records.append(
            {
                "commit": commit,
                "side": side,
                "intersections": sorted(intersections),
            }
        )
    return records, affected_paths


def _checks_for_paths(
    paths: set[str],
    checks: dict[str, CheckDefinition],
    *,
    include_fallback: bool = True,
) -> set[str]:
    return {
        check.id
        for check in checks.values()
        if include_fallback or not check.fallback
        if any(matches_any(path, check.paths) for path in paths)
    }


def _add_selection_reason(
    selected: set[str],
    reasons: dict[str, set[str]],
    check_id: str,
    reason: str,
) -> None:
    selected.add(check_id)
    reasons.setdefault(check_id, set()).add(reason)


def _check_details(
    check_ids: Iterable[str],
    checks: dict[str, CheckDefinition],
    reasons: dict[str, set[str]] | dict[str, list[str]] | None = None,
) -> list[dict[str, Any]]:
    reason_map = reasons or {}
    return [
        {
            "id": check_id,
            "description": checks[check_id].description,
            "lane": checks[check_id].lane,
            "cwd": checks[check_id].cwd,
            "argv": list(checks[check_id].argv),
            "timeout_seconds": checks[check_id].timeout_seconds,
            "fallback": checks[check_id].fallback,
            "reasons": sorted(reason_map.get(check_id, [])),
        }
        for check_id in sorted(check_ids)
    ]


def _selected_lanes(check_ids: Iterable[str], checks: dict[str, CheckDefinition]) -> list[str]:
    return sorted({checks[check_id].lane for check_id in check_ids})


def _capability_structural_patterns(capability: dict[str, Any]) -> tuple[str, ...]:
    patterns = list(capability["paths"])
    for relationship in capability.get("structural_successors", []):
        patterns.append(relationship["source"])
        patterns.extend(relationship["successors"])
    return tuple(patterns)


def build_preflight_report(
    git: GitRepository,
    config: dict[str, Any],
    checks: dict[str, CheckDefinition],
    base_ref: str,
    upstream_ref: str,
) -> dict[str, Any]:
    started = time.monotonic()
    downstream = git.resolve(base_ref)
    upstream = git.resolve(upstream_ref)
    merge_bases = git.merge_bases(downstream, upstream)
    merge_base = merge_bases[0]
    downstream_changes = git.changes(merge_base, downstream)
    upstream_changes = git.changes(merge_base, upstream)
    downstream_paths = change_paths(downstream_changes)
    upstream_paths = change_paths(upstream_changes)
    direct_overlaps = downstream_paths & upstream_paths
    structural = detect_structural_signals(
        upstream_changes,
        lambda path: git.line_count(merge_base, path),
        git.copy_successors(merge_base, upstream),
    )

    risks: list[dict[str, Any]] = []
    if direct_overlaps:
        risks.append(
            {
                "kind": "direct-overlap",
                "severity": "medium",
                "paths": sorted(direct_overlaps),
            }
        )
    for change in upstream_changes:
        kind = change.status[0]
        touched = change.paths & downstream_paths
        if not touched or kind not in {"D", "R"}:
            continue
        risks.append(
            {
                "kind": "delete" if kind == "D" else "rename",
                "severity": "high",
                "status": change.status,
                "old_path": change.old_path,
                "new_path": change.new_path,
                "paths": sorted(touched),
            }
        )
    risks.extend(structural)

    structural_paths = {
        path
        for signal in structural
        for path in [signal["source"], *signal["successors"]]
    }
    high_risk_paths = set(structural_paths)
    for risk in risks:
        if risk.get("severity") != "high":
            continue
        high_risk_paths.update(risk.get("paths", []))
        high_risk_paths.update(risk.get("successors", []))
        for field in ("source", "old_path", "new_path"):
            if risk.get(field):
                high_risk_paths.add(risk[field])
    capability_results: list[dict[str, Any]] = []
    selected: set[str] = set()
    selection_reasons: dict[str, set[str]] = {}
    for capability in config["capabilities"]:
        tracked, tracked_paths = _tracked_commit_records(
            git,
            capability,
            downstream,
            upstream,
            downstream_paths,
            upstream_paths,
        )
        patterns = capability["paths"]
        structural_patterns = _capability_structural_patterns(capability)
        overlap_paths = {path for path in direct_overlaps if matches_any(path, patterns)}
        split_paths = {
            path for path in structural_paths if matches_any(path, structural_patterns)
        }
        affected_paths = tracked_paths | overlap_paths | split_paths
        if not affected_paths:
            continue
        capability_checks = sorted(set(capability.get("checks", [])))
        for check_id in capability_checks:
            _add_selection_reason(
                selected,
                selection_reasons,
                check_id,
                f"capability:{capability['id']}",
            )
        capability_results.append(
            {
                "id": capability["id"],
                "severity": "high" if tracked_paths or split_paths else "medium",
                "affected_paths": sorted(affected_paths),
                "tracked_commits": tracked,
                "checks": capability_checks,
            }
        )

    relevant_paths = direct_overlaps | structural_paths
    for path in sorted(relevant_paths):
        for check in checks.values():
            if not check.fallback and matches_any(path, check.paths):
                _add_selection_reason(
                    selected,
                    selection_reasons,
                    check.id,
                    f"path:{path}",
                )
    selected_checks = sorted(selected)
    warnings: list[str] = []
    if len(merge_bases) > 1:
        warnings.append("检测到多个 merge-base；报告使用第一个基线并要求人工复核")
    for capability in capability_results:
        if not capability["checks"]:
            warnings.append(f"高风险能力 {capability['id']} 没有对应检查")
    capability_patterns = tuple(
        pattern
        for capability in config["capabilities"]
        for pattern in _capability_structural_patterns(capability)
    )
    mapped_risk_paths = {
        path
        for path in relevant_paths
        if matches_any(path, capability_patterns)
        or any(
            not check.fallback and matches_any(path, check.paths)
            for check in checks.values()
        )
    }
    unmapped_paths = sorted(relevant_paths - mapped_risk_paths)
    unmapped_risk_paths = sorted(high_risk_paths - mapped_risk_paths)
    fallback_paths: dict[str, list[str]] = {}
    uncovered_risk_paths: list[str] = []
    uncovered_nonblocking_paths: list[str] = []
    fallback_warning_paths: dict[str, list[str]] = {}
    for path in unmapped_paths:
        fallback_ids = sorted(
            check.id
            for check in checks.values()
            if check.fallback and matches_any(path, check.paths)
        )
        if not fallback_ids:
            if path in high_risk_paths:
                uncovered_risk_paths.append(path)
            else:
                uncovered_nonblocking_paths.append(path)
            continue
        fallback_paths[path] = fallback_ids
        for check_id in fallback_ids:
            fallback_warning_paths.setdefault(check_id, []).append(path)
            _add_selection_reason(
                selected,
                selection_reasons,
                check_id,
                f"fallback:{path}",
            )
    for check_id, paths in sorted(fallback_warning_paths.items()):
        warnings.append(
            f"{len(paths)} 个未映射风险路径使用通用检查 {check_id}；详见 fallback_paths"
        )
    if uncovered_risk_paths:
        warnings.append(
            f"{len(uncovered_risk_paths)} 个风险路径没有通用 lane 检查；详见 uncovered_risk_paths"
        )
    if uncovered_nonblocking_paths:
        warnings.append(
            f"{len(uncovered_nonblocking_paths)} 个普通交叉路径没有通用 lane 检查；详见 uncovered_nonblocking_paths"
        )
    selected_checks = sorted(selected)
    selection_reason_lists = {
        check_id: sorted(values) for check_id, values in selection_reasons.items()
    }

    return {
        "schema_version": SCHEMA_VERSION,
        "mode": "preflight",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "refs": {
            "base_input": base_ref,
            "upstream_input": upstream_ref,
            "base": downstream,
            "upstream": upstream,
            "merge_base": merge_base,
            "merge_bases": merge_bases,
        },
        "summary": {
            "downstream_commits": git.commit_count(merge_base, downstream),
            "upstream_commits": git.commit_count(merge_base, upstream),
            "downstream_paths": len(downstream_paths),
            "upstream_paths": len(upstream_paths),
            "direct_overlaps": len(direct_overlaps),
            "estimated_conflict_candidates": len(direct_overlaps),
            "conflict_estimate_basis": "双方规范化路径交集；这是保守候选数，不等于 Git 文本冲突数",
            "analysis_duration_seconds": round(time.monotonic() - started, 3),
        },
        "changes": {
            "downstream": [change.to_dict() for change in downstream_changes],
            "upstream": [change.to_dict() for change in upstream_changes],
        },
        "direct_overlaps": sorted(direct_overlaps),
        "unmapped_direct_overlaps": sorted(direct_overlaps - mapped_risk_paths),
        "unmapped_risk_paths": unmapped_risk_paths,
        "fallback_paths": fallback_paths,
        "uncovered_risk_paths": uncovered_risk_paths,
        "uncovered_nonblocking_paths": uncovered_nonblocking_paths,
        "risks": risks,
        "capabilities": capability_results,
        "selected_checks": selected_checks,
        "selected_lanes": _selected_lanes(selected_checks, checks),
        "selection_reasons": selection_reason_lists,
        "selected_check_details": _check_details(
            selected_checks, checks, selection_reason_lists
        ),
        "warnings": warnings,
    }


def select_checks_for_range(
    git: GitRepository,
    config: dict[str, Any],
    checks: dict[str, CheckDefinition],
    base_ref: str,
    head_ref: str,
) -> tuple[dict[str, str], list[str], dict[str, list[str]]]:
    base = git.resolve(base_ref)
    head = git.resolve(head_ref)
    paths = change_paths(git.changes(base, head))
    selected: set[str] = set()
    reasons: dict[str, set[str]] = {}
    for path in sorted(paths):
        for check in checks.values():
            if matches_any(path, check.paths):
                prefix = "fallback" if check.fallback else "path"
                _add_selection_reason(selected, reasons, check.id, f"{prefix}:{path}")
    for capability in config["capabilities"]:
        if any(matches_any(path, capability["paths"]) for path in paths):
            for check_id in capability.get("checks", []):
                _add_selection_reason(
                    selected,
                    reasons,
                    check_id,
                    f"capability:{capability['id']}",
                )
    return (
        {"base_input": base_ref, "head_input": head_ref, "base": base, "head": head},
        sorted(selected),
        {check_id: sorted(values) for check_id, values in reasons.items()},
    )


def _truncate(data: bytes) -> str:
    if len(data) <= MAX_CAPTURE_BYTES:
        return data.decode("utf-8", errors="replace")
    suffix = b"\n... output truncated ...\n"
    return (data[: MAX_CAPTURE_BYTES - len(suffix)] + suffix).decode(
        "utf-8", errors="replace"
    )


class CheckRunner:
    def __init__(self, root: Path):
        self.root = root.resolve()
        self._lock = threading.Lock()
        self._processes: set[subprocess.Popen[bytes]] = set()

    def run(self, check: CheckDefinition) -> dict[str, Any]:
        started = time.monotonic()
        try:
            process = subprocess.Popen(
                list(check.argv),
                cwd=self.root / check.cwd,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                shell=False,
                start_new_session=True,
            )
        except OSError as exc:
            return self._result(check, started, 127, False, str(exc))
        with self._lock:
            self._processes.add(process)
        timed_out = False
        try:
            output, _ = process.communicate(timeout=check.timeout_seconds)
        except subprocess.TimeoutExpired:
            timed_out = True
            self._signal_process(process, signal.SIGKILL)
            output, _ = process.communicate()
        finally:
            with self._lock:
                self._processes.discard(process)
        return self._result(
            check,
            started,
            124 if timed_out else process.returncode,
            timed_out,
            _truncate(output),
        )

    def terminate_all(self) -> None:
        with self._lock:
            processes = list(self._processes)
        for process in processes:
            if process.poll() is None:
                self._signal_process(process, signal.SIGTERM)

    @staticmethod
    def _signal_process(process: subprocess.Popen[bytes], signal_number: int) -> None:
        try:
            if os.name == "posix":
                os.killpg(process.pid, signal_number)
            elif signal_number == signal.SIGKILL:
                process.kill()
            else:
                process.terminate()
        except ProcessLookupError:
            # 超时与子进程自然退出可能同时发生；此时目标已经结束。
            return

    @staticmethod
    def _result(
        check: CheckDefinition,
        started: float,
        returncode: int,
        timed_out: bool,
        output: str,
    ) -> dict[str, Any]:
        return {
            "id": check.id,
            "lane": check.lane,
            "status": "timeout" if timed_out else ("passed" if returncode == 0 else "failed"),
            "returncode": returncode,
            "timed_out": timed_out,
            "duration_seconds": round(time.monotonic() - started, 3),
            "output": output,
        }


def execute_checks(
    root: Path,
    selected: Sequence[CheckDefinition],
    jobs: int,
) -> list[dict[str, Any]]:
    runner = CheckRunner(root)
    results: list[dict[str, Any]] = []
    try:
        with ThreadPoolExecutor(max_workers=jobs) as executor:
            futures = {executor.submit(runner.run, check): check for check in selected}
            for future in as_completed(futures):
                check = futures[future]
                try:
                    results.append(future.result())
                except Exception as exc:  # pragma: no cover - 防御执行器内部异常
                    results.append(
                        {
                            "id": check.id,
                            "lane": check.lane,
                            "status": "failed",
                            "returncode": 125,
                            "timed_out": False,
                            "duration_seconds": 0.0,
                            "output": f"检查执行器异常: {exc}",
                        }
                    )
    except KeyboardInterrupt:
        runner.terminate_all()
        raise
    return sorted(results, key=lambda item: item["id"])


def _quote_path(path: str | None) -> str:
    return "null" if path is None else json.dumps(path, ensure_ascii=True)


def _format_command(detail: dict[str, Any]) -> str:
    argv = " ".join(json.dumps(arg, ensure_ascii=True) for arg in detail["argv"])
    return f"{_quote_path(detail['cwd'])} $ {argv}"


def _render_change(lines: list[str], change: dict[str, Any]) -> None:
    status = change["status"]
    old_path = change.get("old_path")
    new_path = change.get("new_path")
    if status[0] in {"R", "C"}:
        lines.append(f"  {status}: {_quote_path(old_path)} -> {_quote_path(new_path)}")
    else:
        lines.append(f"  {status}: {_quote_path(new_path or old_path)}")


def render_preflight_text(report: dict[str, Any]) -> str:
    refs = report["refs"]
    summary = report["summary"]
    lines = [
        f"merge-base: {refs['merge_base'][:12]}",
        f"downstream: {refs['base'][:12]} ({summary['downstream_commits']} commits, {summary['downstream_paths']} paths)",
        f"upstream: {refs['upstream'][:12]} ({summary['upstream_commits']} commits, {summary['upstream_paths']} paths)",
        f"direct intersections: {summary['direct_overlaps']}",
        "estimated conflict candidates: "
        f"{summary.get('estimated_conflict_candidates', summary['direct_overlaps'])} "
        "(normalized path overlap upper bound)",
    ]
    for path in report["direct_overlaps"]:
        lines.append(f"  - {_quote_path(path)}")
    lines.append("")
    changes = report.get("changes", {})
    downstream_changes = changes.get("downstream", [])
    upstream_changes = changes.get("upstream", [])
    lines.append(f"downstream changed files: {len(downstream_changes)}")
    for change in downstream_changes:
        _render_change(lines, change)
    upstream_structural_changes = [
        change
        for change in upstream_changes
        if change["status"][0] in {"A", "D", "R", "C"}
    ]
    lines.append(f"upstream additions/deletions/renames/copies: {len(upstream_structural_changes)}")
    for change in upstream_structural_changes:
        _render_change(lines, change)
    lines.append("")
    for risk in report["risks"]:
        kind = risk["kind"]
        if kind == "direct-overlap":
            continue
        if kind in {"delete", "rename"}:
            lines.append(
                f"[{risk['severity'].upper()}] {kind} {risk['status']}: "
                f"{_quote_path(risk['old_path'])} -> {_quote_path(risk['new_path'])}"
            )
            for path in risk["paths"]:
                lines.append(f"  downstream overlap: {_quote_path(path)}")
            continue
        if kind == "file-split":
            deleted = risk["deleted_lines"]
            total = risk["base_lines"]
            ratio = risk["deletion_ratio"]
            ratio_text = "unknown" if ratio is None else f"{ratio:.1%}"
            lines.append(
                f"[{risk['severity'].upper()}] file-split: {_quote_path(risk['source'])}"
            )
            lines.append(f"  deleted lines: {deleted}/{total} ({ratio_text})")
            for path in risk["successors"]:
                lines.append(f"  successor: {_quote_path(path)}")
            for path in risk.get("copy_successors", []):
                lines.append(f"  C20 copy: {_quote_path(path)}")
    if len(report["risks"]) > 1:
        lines.append("")
    for capability in report["capabilities"]:
        lines.append(f"[{capability['severity'].upper()}] {capability['id']}")
        for tracked in capability["tracked_commits"]:
            lines.append(f"  tracked commit: {tracked['commit'][:12]} ({tracked['side']})")
        lines.append(f"  affected paths: {len(capability['affected_paths'])}")
        lines.append(f"  checks: {', '.join(capability['checks']) or '(missing)'}")
    lines.extend(
        [
            "",
            f"selected lanes: {', '.join(report['selected_lanes']) or '(none)'}",
            f"selected checks: {', '.join(report['selected_checks']) or '(none)'}",
        ]
    )
    for detail in report.get("selected_check_details", []):
        lines.append(f"  {detail['id']} [{detail['lane']}]: {_format_command(detail)}")
        for reason in detail.get("reasons", []):
            lines.append(f"    reason: {reason}")
    for warning in report["warnings"]:
        lines.append(f"warning: {warning}")
    return "\n".join(lines) + "\n"


def render_check_text(report: dict[str, Any]) -> str:
    lines = [
        f"selected lanes: {', '.join(report['selected_lanes']) or '(none)'}",
        f"selected checks: {', '.join(report['selected_checks']) or '(none)'}",
    ]
    for detail in report.get("selected_check_details", []):
        lines.append(f"  {detail['id']} [{detail['lane']}]: {_format_command(detail)}")
        for reason in detail.get("reasons", []):
            lines.append(f"    reason: {reason}")
    for result in report["results"]:
        lines.append(
            f"[{result['status'].upper()}] {result['id']} ({result['duration_seconds']:.3f}s)"
        )
        if result["status"] not in {"passed", "planned"} and result.get("output"):
            lines.append(result["output"].rstrip())
    if "duration_seconds" in report:
        lines.append(f"stage duration: {report['duration_seconds']:.3f}s")
    for warning in report.get("warnings", []):
        lines.append(f"warning: {warning}")
    return "\n".join(lines) + "\n"


def emit_report(report: dict[str, Any], output_format: str, output: Path | None) -> None:
    if output_format == "json":
        rendered = json.dumps(report, ensure_ascii=True, indent=2, sort_keys=True) + "\n"
    elif report["mode"] == "preflight":
        rendered = render_preflight_text(report)
    else:
        rendered = render_check_text(report)
    if output is None:
        sys.stdout.write(rendered)
        return
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(rendered, encoding="utf-8")


def _preflight_exit_code(report: dict[str, Any]) -> int:
    if report.get("uncovered_risk_paths"):
        return 2
    if any(
        "高风险能力" in warning and "没有对应检查" in warning
        for warning in report.get("warnings", [])
    ):
        return 2
    return 0


def _discover_repository() -> Path:
    completed = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        shell=False,
        check=False,
        timeout=10,
    )
    if completed.returncode != 0:
        raise SyncError("当前目录不在 Git 仓库中")
    return Path(completed.stdout.strip())


def _load_selected_from_report(path: Path) -> tuple[list[str], dict[str, list[str]]]:
    try:
        report = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SyncError(f"无法读取计划报告 {path}: {exc}") from exc
    if report.get("schema_version") != SCHEMA_VERSION:
        raise SyncError("计划报告 schema_version 不受支持")
    selected = report.get("selected_checks")
    if not isinstance(selected, list) or not all(isinstance(item, str) for item in selected):
        raise SyncError("计划报告缺少 selected_checks 字符串数组")
    raw_reasons = report.get("selection_reasons", {})
    if not isinstance(raw_reasons, dict) or not all(
        isinstance(check_id, str)
        and isinstance(values, list)
        and all(isinstance(value, str) for value in values)
        for check_id, values in raw_reasons.items()
    ):
        raise SyncError("计划报告的 selection_reasons 必须是字符串数组映射")
    return sorted(set(selected)), {
        check_id: sorted(set(values)) for check_id, values in raw_reasons.items()
    }


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    preflight = subparsers.add_parser("preflight", help="只读分析下游与待合入上游 ref")
    preflight.add_argument("--base", default="custom/main", help="下游目标 ref")
    preflight.add_argument("--upstream", default="upstream/main", help="待合入上游 ref")
    preflight.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    preflight.add_argument("--format", choices=("text", "json"), default="text")
    preflight.add_argument("--output", type=Path)

    check = subparsers.add_parser("check", help="选择并并发执行受影响检查")
    check.add_argument("--base", default="custom/main", help="变更起点 ref")
    check.add_argument("--head", default="HEAD", help="变更终点 ref")
    check.add_argument("--report", type=Path, help="复用 preflight/check dry-run JSON 报告")
    check.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    check.add_argument("--lane", action="append", help="只执行指定 lane，可重复")
    check.add_argument("--jobs", type=int, default=3)
    check.add_argument("--dry-run", action="store_true")
    check.add_argument("--format", choices=("text", "json"), default="text")
    check.add_argument("--output", type=Path)
    return parser


def run_cli(argv: Sequence[str] | None = None) -> int:
    started = time.monotonic()
    args = _build_parser().parse_args(argv)
    root = _discover_repository()
    git = GitRepository(root)
    config, checks = load_config(args.config)
    if args.command == "preflight":
        report = build_preflight_report(git, config, checks, args.base, args.upstream)
        emit_report(report, args.format, args.output)
        return _preflight_exit_code(report)

    if args.jobs < 1:
        raise SyncError("--jobs 必须大于等于 1")
    refs: dict[str, str] | None = None
    if args.report:
        selected_ids, selection_reasons = _load_selected_from_report(args.report)
    else:
        refs, selected_ids, selection_reasons = select_checks_for_range(
            git, config, checks, args.base, args.head
        )
    unknown = set(selected_ids) - checks.keys()
    if unknown:
        raise SyncError(f"计划报告引用了未知检查: {sorted(unknown)}")
    lanes = set(args.lane or [])
    known_lanes = {check.lane for check in checks.values()}
    unknown_lanes = lanes - known_lanes
    if unknown_lanes:
        raise SyncError(f"未知 lane: {sorted(unknown_lanes)}")
    selected = [
        checks[check_id]
        for check_id in selected_ids
        if not lanes or checks[check_id].lane in lanes
    ]
    warnings: list[str] = []
    if not selected:
        if lanes:
            warnings.append(
                f"指定 lane {', '.join(sorted(lanes))} 没有命中定向检查；仍需依赖现有完整 CI"
            )
        else:
            warnings.append("变更范围没有命中定向检查；仍需依赖现有完整 CI")
    if args.dry_run:
        results = [
            {
                "id": check.id,
                "lane": check.lane,
                "status": "planned",
                "returncode": None,
                "timed_out": False,
                "duration_seconds": 0.0,
                "output": "",
                "description": check.description,
                "cwd": check.cwd,
                "argv": list(check.argv),
                "reasons": selection_reasons.get(check.id, []),
            }
            for check in selected
        ]
    else:
        results = execute_checks(root, selected, min(args.jobs, max(1, len(selected))))
        for result in results:
            check = checks[result["id"]]
            result.update(
                {
                    "description": check.description,
                    "cwd": check.cwd,
                    "argv": list(check.argv),
                    "reasons": selection_reasons.get(check.id, []),
                }
            )
    filtered_ids = [check.id for check in selected]
    filtered_reasons = {
        check_id: selection_reasons.get(check_id, []) for check_id in filtered_ids
    }
    report = {
        "schema_version": SCHEMA_VERSION,
        "mode": "check",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "refs": refs,
        "dry_run": args.dry_run,
        "selected_checks": filtered_ids,
        "selected_lanes": _selected_lanes(filtered_ids, checks),
        "selection_reasons": filtered_reasons,
        "selected_check_details": _check_details(
            filtered_ids, checks, filtered_reasons
        ),
        "results": results,
        "warnings": warnings,
        "duration_seconds": round(time.monotonic() - started, 3),
    }
    emit_report(report, args.format, args.output)
    return 1 if any(result["status"] in {"failed", "timeout"} for result in results) else 0


def main() -> int:
    try:
        return run_cli()
    except SyncError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 3
    except KeyboardInterrupt:
        print("error: 检查被中断", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
