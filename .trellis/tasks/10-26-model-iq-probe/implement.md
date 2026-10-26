# 执行清单

1. [x] 后端配置：`domain_constants.go` 常量 + `model_iq_probe_service.go`（配置 Load/Save/Defaults/校验 + 判分 + 目标解析）
2. [x] 后端存储：`ent/schema/model_iq_probe_result.go` + `migrations/249_model_iq_probe_results.sql` + `go generate ./ent`
3. [x] 后端仓库：`repository/model_iq_probe_repo.go`（Insert/ListPage/ListRecent/OverrideStatus/DeleteBefore）
4. [x] 后端执行器：`model_iq_probe_runner.go`（timer 循环 + leader lock + Reload + RunOnce + 单目标执行）
5. [x] 后端 handler/路由：`handler/admin/model_iq_probe_handler.go` + 用户端 iq-probe 端点 + routes 注册（admin.go/user.go）
6. [x] 后端清理挂钩：`ops_cleanup_service.go` 调 CleanupOldResults；wire 注册（service/wire.go + cmd/server/wire*.go）
7. [x] 后端测试：判分/目标解析/配置校验单测 + repo 集成测试；`go build ./...` + 相关 `go test`（service 包仅存量 codex ticket 用例失败，与本任务无关，clean custom/main 同样失败）
8. [x] 前端 API：`api/modelIqProbe.ts` + `api/admin/modelIqProbe.ts` + adminAPI 注册
9. [x] 前端用户端：`CandyProbePanel.vue` + `ChannelStatusV1View.vue` 接入 + zh/en custom.ts overlay
10. [x] 前端管理端：`ModelIqMonitorView.vue` + 路由 + 侧边栏 + zh/en nav & admin i18n
11. [x] 前端测试：组件/API spec + `pnpm typecheck` + `pnpm check:i18n`（全量 412 文件 / 3235 测试通过）
12. [ ] 收尾：spec 更新（已完成 model-iq-probe.md）→ 三段式提交（用户确认后）
