-- 清理已废弃的旧上游成本校准表；保留上游倍率监控 upstream_relay_* 表。
DROP TABLE IF EXISTS upstream_cost_calibration_suggestions;
DROP TABLE IF EXISTS upstream_cost_calibration_results;
DROP TABLE IF EXISTS upstream_cost_calibration_runs;
DROP TABLE IF EXISTS upstream_cost_calibration_task_accounts;
DROP TABLE IF EXISTS upstream_cost_calibration_tasks;
