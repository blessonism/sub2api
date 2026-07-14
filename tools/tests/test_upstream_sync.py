from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest import mock


REPO_ROOT = Path(__file__).resolve().parents[2]
TOOLS_DIR = REPO_ROOT / "tools"
sys.path.insert(0, str(TOOLS_DIR))

import upstream_sync  # noqa: E402


def write_config(root: Path, checks: list[dict], capabilities: list[dict] | None = None) -> Path:
    path = root / "checks.json"
    path.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "capabilities": capabilities or [],
                "checks": checks,
            }
        ),
        encoding="utf-8",
    )
    return path


def check_definition(
    check_id: str,
    argv: list[str],
    *,
    lane: str = "frontend",
    timeout: int = 5,
) -> dict:
    return {
        "id": check_id,
        "description": check_id,
        "lane": lane,
        "paths": ["frontend/**"],
        "argv": argv,
        "cwd": ".",
        "timeout_seconds": timeout,
    }


class DiffParsingTests(unittest.TestCase):
    def test_name_status_parser_preserves_newlines_and_rename_paths(self) -> None:
        data = (
            b"M\0frontend/line\nbreak.vue\0"
            b"R087\0old name.ts\0new name.ts\0"
            b"A\0new-file.ts\0D\0deleted-file.go\0"
        )

        changes = upstream_sync.parse_name_status_z(data)

        self.assertEqual("frontend/line\nbreak.vue", changes[0].new_path)
        self.assertEqual("old name.ts", changes[1].old_path)
        self.assertEqual("new name.ts", changes[1].new_path)
        self.assertIsNone(changes[2].old_path)
        self.assertIsNone(changes[3].new_path)

    def test_numstat_parser_handles_normal_and_rename_records(self) -> None:
        data = b"4\t9\tplain.go\0" b"3\t2\t\0old.go\0new.go\0" b"-\t-\tasset.bin\0"

        stats = upstream_sync.parse_numstat_z(data)

        self.assertEqual((4, 9), stats["plain.go"])
        self.assertEqual((3, 2), stats["old.go"])
        self.assertEqual((3, 2), stats["new.go"])
        self.assertEqual((None, None), stats["asset.bin"])

    def test_structural_signal_requires_large_reduction_and_two_successors(self) -> None:
        changes = [
            upstream_sync.Change(
                status="M",
                old_path="pkg/usage_log_repo.go",
                new_path="pkg/usage_log_repo.go",
                additions=5,
                deletions=75,
            ),
            upstream_sync.Change("A", None, "pkg/usage_log_repo_query.go", 20, 0),
            upstream_sync.Change("A", None, "pkg/usage_log_repo_stats.go", 30, 0),
        ]

        signals = upstream_sync.detect_structural_signals(changes, lambda _path: 100)

        self.assertEqual(1, len(signals))
        self.assertEqual("file-split", signals[0]["kind"])
        self.assertEqual(0.75, signals[0]["deletion_ratio"])
        self.assertEqual(2, len(signals[0]["successors"]))

    def test_structural_signal_detects_locale_file_to_directory_split(self) -> None:
        changes = [
            upstream_sync.Change("D", "locales/en.ts", None, 0, 100),
            upstream_sync.Change("A", None, "locales/en/common.ts", 40, 0),
            upstream_sync.Change("A", None, "locales/en/dashboard.ts", 60, 0),
        ]

        signals = upstream_sync.detect_structural_signals(changes, lambda _path: 100)

        self.assertEqual("locales/en.ts", signals[0]["source"])

    def test_structural_signal_includes_low_threshold_copy_evidence(self) -> None:
        changes = [
            upstream_sync.Change("M", "pkg/source.go", "pkg/source.go", 1, 90),
            upstream_sync.Change("A", None, "pkg/source_a.go", 40, 0),
            upstream_sync.Change("A", None, "pkg/source_b.go", 40, 0),
        ]

        signals = upstream_sync.detect_structural_signals(
            changes,
            lambda _path: 100,
            {"pkg/source.go": ["pkg/source_a.go", "unrelated/copy.go"]},
        )

        self.assertEqual(["pkg/source_a.go"], signals[0]["copy_successors"])


class GitSafetyTests(unittest.TestCase):
    def test_repository_rejects_git_write_commands(self) -> None:
        repository = upstream_sync.GitRepository(REPO_ROOT)

        with self.assertRaisesRegex(upstream_sync.SyncError, "非只读"):
            repository.run(["merge", "upstream/main"])

    def test_commit_side_classifies_both_sides(self) -> None:
        repository = upstream_sync.GitRepository(REPO_ROOT)
        with mock.patch.object(repository, "run", return_value=b""), mock.patch.object(
            repository,
            "_is_ancestor",
            side_effect=[True, False, False, True, True, True, False, False],
        ):
            self.assertEqual("downstream-only", repository.commit_side("a", "d", "u"))
            self.assertEqual("incoming-upstream", repository.commit_side("b", "d", "u"))
            self.assertEqual("common", repository.commit_side("c", "d", "u"))
            self.assertEqual("missing", repository.commit_side("d", "d", "u"))

    def test_copy_relationships_are_parsed_from_nul_delimited_git_output(self) -> None:
        repository = upstream_sync.GitRepository(REPO_ROOT)
        output = (
            b"M\0pkg/source.go\0"
            b"C021\0pkg/source.go\0pkg/source copy\npart.go\0"
            b"A\0pkg/other.go\0"
        )

        with mock.patch.object(repository, "run", return_value=output) as run:
            relationships = repository.copy_successors("base", "head")

        self.assertEqual(
            {"pkg/source.go": ["pkg/source copy\npart.go"]}, relationships
        )
        self.assertIn("--find-copies-harder", run.call_args.args[0])


class ConfigAndSelectionTests(unittest.TestCase):
    def test_config_rejects_unknown_capability_check(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = write_config(
                root,
                [],
                [
                    {
                        "id": "capability",
                        "paths": ["frontend/**"],
                        "tracked_commits": [],
                        "checks": ["missing"],
                    }
                ],
            )

            with self.assertRaisesRegex(upstream_sync.SyncError, "未知检查"):
                upstream_sync.load_config(config)

    def test_config_rejects_non_object_and_string_array_fields(self) -> None:
        valid_check = check_definition("valid", ["true"])
        valid_capability = {
            "id": "capability",
            "paths": ["frontend/**"],
            "tracked_commits": [],
            "checks": ["valid"],
        }
        invalid_cases = [
            (["not-an-object"], [valid_capability], "必须是对象"),
            ([{**valid_check, "paths": "frontend/**"}], [valid_capability], "paths"),
            ([{**valid_check, "argv": "true"}], [valid_capability], "argv"),
            ([{**valid_check, "cwd": ["frontend"]}], [valid_capability], "cwd"),
            ([{**valid_check, "fallback": "yes"}], [valid_capability], "fallback"),
            ([valid_check], ["not-an-object"], "必须是对象"),
            (
                [valid_check],
                [{**valid_capability, "tracked_commits": "abc"}],
                "tracked_commits",
            ),
            ([valid_check], [{**valid_capability, "checks": "valid"}], "checks"),
        ]

        for check_items, capabilities, message in invalid_cases:
            with self.subTest(message=message, check_items=check_items):
                with tempfile.TemporaryDirectory() as directory:
                    config = Path(directory) / "invalid.json"
                    config.write_text(
                        json.dumps(
                            {
                                "schema_version": 1,
                                "capabilities": capabilities,
                                "checks": check_items,
                            }
                        ),
                        encoding="utf-8",
                    )
                    with self.assertRaisesRegex(upstream_sync.SyncError, message):
                        upstream_sync.load_config(config)

    def test_default_matrix_covers_tool_and_dashboard_chain(self) -> None:
        config, checks = upstream_sync.load_config(
            TOOLS_DIR / "upstream_sync_checks.json"
        )

        self.assertIn("upstream-sync-tool-tests", checks)
        dashboard = next(
            item
            for item in config["capabilities"]
            if item["id"] == "dashboard-operational-metrics"
        )
        self.assertIn(
            "backend/internal/repository/dashboard_aggregation_repo.go",
            dashboard["paths"],
        )
        self.assertIn("frontend/src/api/admin/dashboard.ts", dashboard["paths"])
        capability_ids = {item["id"] for item in config["capabilities"]}
        self.assertTrue(
            {
                "activity-center",
                "token-leaderboards",
                "upstream-relay-monitoring",
                "announcement-email-broadcast",
                "group-time-rate-strategy",
                "user-public-group-account-binding",
            }.issubset(capability_ids)
        )
        self.assertTrue(checks["frontend-typecheck"].fallback)
        backend_fallbacks = {
            check_id
            for check_id, check in checks.items()
            if check.fallback and check.lane == "backend"
        }
        self.assertEqual(
            {
                "backend-ent-compile",
                "backend-handler-compile",
                "backend-repository-compile",
                "backend-server-compile",
                "backend-service-compile",
            },
            backend_fallbacks,
        )
        for check in checks.values():
            if check.argv[:2] == ("pnpm", "test:run"):
                self.assertNotIn("--", check.argv)
        representative_checks = {
            "frontend/src/components/user/activities/LotteryCampaignActivity.vue": "activity-center-frontend",
            "frontend/src/views/user/LeaderboardView.vue": "leaderboard-frontend",
            "backend/internal/service/upstream_relay_group_monitoring.go": "upstream-monitoring-backend",
            "backend/internal/service/announcement_email_worker.go": "announcement-email-backend",
            "backend/internal/service/group_time_rate.go": "group-time-rate-backend",
            "backend/internal/service/user_group_account_binding.go": "user-group-account-binding-backend",
        }
        for path, expected_check in representative_checks.items():
            with self.subTest(path=path):
                self.assertIn(
                    expected_check,
                    upstream_sync._checks_for_paths({path}, checks),
                )

    def test_check_path_selection_is_deduplicated(self) -> None:
        checks = {
            "view": upstream_sync.CheckDefinition(
                id="view",
                description="view",
                lane="frontend",
                paths=("frontend/**", "frontend/src/**"),
                argv=("true",),
                cwd=".",
                timeout_seconds=5,
            )
        }

        selected = upstream_sync._checks_for_paths(
            {"frontend/src/View.vue", "frontend/src/View.spec.ts"}, checks
        )

        self.assertEqual({"view"}, selected)

    def test_preflight_warns_for_unmapped_direct_overlap(self) -> None:
        class FakeGit:
            def resolve(self, ref: str) -> str:
                return ref

            def merge_bases(self, _left: str, _right: str) -> list[str]:
                return ["merge-base"]

            def changes(self, _start: str, _end: str) -> list[upstream_sync.Change]:
                return [
                    upstream_sync.Change("M", "mapped/file.go", "mapped/file.go", 1, 1),
                    upstream_sync.Change("M", "unmapped/file.go", "unmapped/file.go", 1, 1),
                ]

            def copy_successors(self, _start: str, _end: str) -> dict[str, list[str]]:
                return {}

            def line_count(self, _ref: str, _path: str) -> int:
                return 10

            def commit_count(self, _start: str, _end: str) -> int:
                return 1

        checks = {
            "mapped": upstream_sync.CheckDefinition(
                id="mapped",
                description="mapped",
                lane="backend",
                paths=("mapped/**",),
                argv=("true",),
                cwd=".",
                timeout_seconds=5,
            )
        }

        report = upstream_sync.build_preflight_report(
            FakeGit(),  # type: ignore[arg-type]
            {"capabilities": []},
            checks,
            "downstream",
            "upstream",
        )

        self.assertEqual(["unmapped/file.go"], report["unmapped_direct_overlaps"])
        self.assertIn(
            "1 个普通交叉路径没有通用 lane 检查；详见 uncovered_nonblocking_paths",
            report["warnings"],
        )
        self.assertEqual([], report["uncovered_risk_paths"])

    def test_preflight_exit_code_blocks_uncovered_high_risk_paths(self) -> None:
        self.assertEqual(
            2,
            upstream_sync._preflight_exit_code(
                {"uncovered_risk_paths": ["backend/high-risk.go"], "warnings": []}
            ),
        )
        self.assertEqual(
            0,
            upstream_sync._preflight_exit_code(
                {
                    "uncovered_risk_paths": [],
                    "uncovered_nonblocking_paths": ["README.md"],
                    "warnings": [],
                }
            ),
        )

    def test_preflight_selects_fallback_for_unmapped_risk_path(self) -> None:
        class FakeGit:
            def resolve(self, ref: str) -> str:
                return ref

            def merge_bases(self, _left: str, _right: str) -> list[str]:
                return ["merge-base"]

            def changes(self, _start: str, _end: str) -> list[upstream_sync.Change]:
                return [
                    upstream_sync.Change(
                        "M", "backend/new_area.go", "backend/new_area.go", 1, 1
                    )
                ]

            def copy_successors(self, _start: str, _end: str) -> dict[str, list[str]]:
                return {}

            def line_count(self, _ref: str, _path: str) -> int:
                return 10

            def commit_count(self, _start: str, _end: str) -> int:
                return 1

        checks = {
            "backend-compile": upstream_sync.CheckDefinition(
                id="backend-compile",
                description="fallback",
                lane="backend",
                paths=("backend/**",),
                argv=("true",),
                cwd=".",
                timeout_seconds=5,
                fallback=True,
            )
        }

        report = upstream_sync.build_preflight_report(
            FakeGit(),  # type: ignore[arg-type]
            {"capabilities": []},
            checks,
            "downstream",
            "upstream",
        )

        self.assertEqual(["backend-compile"], report["selected_checks"])
        self.assertEqual(
            ["backend-compile"], report["fallback_paths"]["backend/new_area.go"]
        )
        self.assertIn(
            "fallback:backend/new_area.go",
            report["selection_reasons"]["backend-compile"],
        )


class CheckCommandTests(unittest.TestCase):
    def run_with_report(
        self,
        root: Path,
        check: dict,
        *extra: str,
    ) -> tuple[int, dict]:
        config = write_config(root, [check])
        plan = root / "plan.json"
        plan.write_text(
            json.dumps({"schema_version": 1, "selected_checks": [check["id"]]}),
            encoding="utf-8",
        )
        output = root / "nested" / "result.json"
        args = [
            "check",
            "--report",
            str(plan),
            "--config",
            str(config),
            "--format",
            "json",
            "--output",
            str(output),
            *extra,
        ]
        with mock.patch.object(upstream_sync, "_discover_repository", return_value=root):
            return_code = upstream_sync.run_cli(args)
        return return_code, json.loads(output.read_text(encoding="utf-8"))

    def test_dry_run_never_executes_selected_command_and_creates_output_parent(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, report = self.run_with_report(
                root,
                check_definition("never-run", ["command-that-does-not-exist"]),
                "--dry-run",
            )

        self.assertEqual(0, code)
        self.assertEqual(["never-run"], report["selected_checks"])
        self.assertEqual("planned", report["results"][0]["status"])

    def test_failure_is_propagated_as_nonzero(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, report = self.run_with_report(
                root,
                check_definition("fails", [sys.executable, "-c", "raise SystemExit(7)"]),
            )

        self.assertEqual(1, code)
        self.assertEqual(7, report["results"][0]["returncode"])
        self.assertEqual("failed", report["results"][0]["status"])

    def test_timeout_is_propagated_as_nonzero(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            code, report = self.run_with_report(
                root,
                check_definition(
                    "times-out",
                    [sys.executable, "-c", "import time; time.sleep(5)"],
                    timeout=1,
                ),
            )

        self.assertEqual(1, code)
        self.assertEqual(124, report["results"][0]["returncode"])
        self.assertTrue(report["results"][0]["timed_out"])

    def test_lane_filter_only_keeps_requested_lane(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = write_config(
                root,
                [
                    check_definition("frontend", ["true"], lane="frontend"),
                    check_definition("backend", ["true"], lane="backend"),
                ],
            )
            plan = root / "plan.json"
            plan.write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "selected_checks": ["frontend", "backend"],
                    }
                ),
                encoding="utf-8",
            )
            output = root / "result.json"
            with mock.patch.object(upstream_sync, "_discover_repository", return_value=root):
                code = upstream_sync.run_cli(
                    [
                        "check",
                        "--report",
                        str(plan),
                        "--config",
                        str(config),
                        "--lane",
                        "backend",
                        "--dry-run",
                        "--format",
                        "json",
                        "--output",
                        str(output),
                    ]
                )
            report = json.loads(output.read_text(encoding="utf-8"))

        self.assertEqual(0, code)
        self.assertEqual(["backend"], report["selected_checks"])
        self.assertEqual(["backend"], report["selected_lanes"])

    def test_independent_checks_execute_in_parallel(self) -> None:
        def check(check_id: str) -> upstream_sync.CheckDefinition:
            return upstream_sync.CheckDefinition(
                id=check_id,
                description=check_id,
                lane="frontend",
                paths=("frontend/**",),
                argv=(sys.executable, "-c", "import time; time.sleep(0.5)"),
                cwd=".",
                timeout_seconds=5,
            )

        with tempfile.TemporaryDirectory() as directory:
            started = time.monotonic()
            results = upstream_sync.execute_checks(
                Path(directory), [check("one"), check("two")], jobs=2
            )
            duration = time.monotonic() - started

        self.assertLess(duration, 0.9)
        self.assertEqual(["passed", "passed"], [item["status"] for item in results])

    def test_empty_selection_succeeds_with_explicit_warning(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = write_config(root, [])
            plan = root / "plan.json"
            plan.write_text(
                json.dumps({"schema_version": 1, "selected_checks": []}),
                encoding="utf-8",
            )
            output = root / "result.json"
            with mock.patch.object(upstream_sync, "_discover_repository", return_value=root):
                code = upstream_sync.run_cli(
                    [
                        "check",
                        "--report",
                        str(plan),
                        "--config",
                        str(config),
                        "--format",
                        "json",
                        "--output",
                        str(output),
                    ]
                )
            report = json.loads(output.read_text(encoding="utf-8"))

        self.assertEqual(0, code)
        self.assertEqual([], report["results"])
        self.assertIn("完整 CI", report["warnings"][0])

    def test_executor_exception_becomes_failed_result(self) -> None:
        check = upstream_sync.CheckDefinition(
            id="broken-runner",
            description="broken-runner",
            lane="backend",
            paths=("backend/**",),
            argv=("true",),
            cwd=".",
            timeout_seconds=5,
        )

        with tempfile.TemporaryDirectory() as directory, mock.patch.object(
            upstream_sync.CheckRunner, "run", side_effect=RuntimeError("boom")
        ):
            results = upstream_sync.execute_checks(Path(directory), [check], jobs=1)

        self.assertEqual("failed", results[0]["status"])
        self.assertEqual(125, results[0]["returncode"])
        self.assertIn("boom", results[0]["output"])


class ReportRenderingTests(unittest.TestCase):
    def test_preflight_text_expands_and_escapes_all_risk_evidence(self) -> None:
        report = {
            "refs": {"merge_base": "a" * 40, "base": "b" * 40, "upstream": "c" * 40},
            "summary": {
                "downstream_commits": 1,
                "downstream_paths": 1,
                "upstream_commits": 1,
                "upstream_paths": 4,
                "direct_overlaps": 1,
            },
            "direct_overlaps": ["path/with\nnewline.go"],
            "risks": [
                {
                    "kind": "direct-overlap",
                    "severity": "medium",
                    "paths": ["path/with\nnewline.go"],
                },
                {
                    "kind": "rename",
                    "severity": "high",
                    "status": "R090",
                    "old_path": "old.go",
                    "new_path": "new.go",
                    "paths": ["old.go"],
                },
                {
                    "kind": "file-split",
                    "severity": "high",
                    "source": "source.go",
                    "deleted_lines": 90,
                    "base_lines": 100,
                    "deletion_ratio": 0.9,
                    "successors": ["source_part.go", "source_more.go"],
                    "copy_successors": ["source_part.go"],
                },
            ],
            "capabilities": [],
            "selected_lanes": [],
            "selected_checks": [],
            "warnings": [],
            "changes": {
                "downstream": [
                    {
                        "status": "M",
                        "old_path": "path/with\nnewline.go",
                        "new_path": "path/with\nnewline.go",
                    }
                ],
                "upstream": [
                    {
                        "status": "R090",
                        "old_path": "old.go",
                        "new_path": "new.go",
                    }
                ],
            },
            "selected_check_details": [
                {
                    "id": "view",
                    "lane": "frontend",
                    "cwd": "frontend",
                    "argv": ["pnpm", "test:run", "--", "view.spec.ts"],
                    "reasons": ["path:view.vue"],
                }
            ],
        }

        rendered = upstream_sync.render_preflight_text(report)

        self.assertIn('"path/with\\nnewline.go"', rendered)
        self.assertIn('[HIGH] rename R090: "old.go" -> "new.go"', rendered)
        self.assertIn("deleted lines: 90/100 (90.0%)", rendered)
        self.assertIn('C20 copy: "source_part.go"', rendered)
        self.assertIn("upstream additions/deletions/renames/copies: 1", rendered)
        self.assertIn('view [frontend]: "frontend" $ "pnpm"', rendered)
        self.assertIn("reason: path:view.vue", rendered)


def historical_object_available() -> bool:
    completed = subprocess.run(
        ["git", "cat-file", "-e", "2c503a333^{commit}"],
        cwd=REPO_ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        shell=False,
        check=False,
    )
    return completed.returncode == 0


@unittest.skipUnless(historical_object_available(), "历史同步对象不在本地仓库")
class HistoricalSyncTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.git = upstream_sync.GitRepository(REPO_ROOT)
        cls.config, cls.checks = upstream_sync.load_config(
            TOOLS_DIR / "upstream_sync_checks.json"
        )

    def test_preflight_selects_both_regression_oracles(self) -> None:
        report = upstream_sync.build_preflight_report(
            self.git,
            self.config,
            self.checks,
            "2c503a333^1",
            "2c503a333^2",
        )

        self.assertIn("dashboard-repository-integration", report["selected_checks"])
        self.assertIn("user-usage-view", report["selected_checks"])
        capability_ids = {item["id"] for item in report["capabilities"]}
        self.assertIn("dashboard-operational-metrics", capability_ids)
        self.assertIn("usage-latency-column", capability_ids)
        split_sources = {
            risk.get("source")
            for risk in report["risks"]
            if risk["kind"] == "file-split"
        }
        self.assertIn("backend/internal/repository/usage_log_repo.go", split_sources)
        dashboard_split = next(
            risk
            for risk in report["risks"]
            if risk.get("source") == "backend/internal/repository/usage_log_repo.go"
        )
        self.assertIn(
            "backend/internal/repository/usage_log_repo_insert.go",
            dashboard_split["copy_successors"],
        )
        delete_paths = {
            risk.get("old_path") for risk in report["risks"] if risk["kind"] == "delete"
        }
        self.assertIn("frontend/src/i18n/locales/en.ts", delete_paths)
        rendered = upstream_sync.render_preflight_text(report)
        self.assertIn('backend/internal/repository/usage_log_repo.go', rendered)
        self.assertIn('[HIGH] delete D: "frontend/src/i18n/locales/en.ts" -> null', rendered)

    def test_check_range_dry_run_selection_uses_stable_fields(self) -> None:
        refs, selected, reasons = upstream_sync.select_checks_for_range(
            self.git,
            self.config,
            self.checks,
            "2c503a333^1",
            "2c503a333^2",
        )

        self.assertEqual("2c503a333^1", refs["base_input"])
        self.assertIn("dashboard-repository-integration", selected)
        self.assertIn("user-usage-view", selected)
        self.assertIn("dashboard-repository-integration", reasons)


if __name__ == "__main__":
    unittest.main()
