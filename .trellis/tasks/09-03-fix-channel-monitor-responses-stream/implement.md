# 实施计划

1. 以 `custom/main` 为任务基线，核对当前工作区用户未提交改动，确认只在监控 checker、常量和测试文件落补丁。
2. 将 Responses adapter 改为流式请求；实现首包、SSE 终态、challenge、读取错误和字节上限处理，并统一 60 秒相关超时。
3. 在 `channel_monitor_checker_body_test.go` 增加 checker 回归测试：流式有效答案、慢完成、`response.failed`、无输出、读取超时；保留既有非流式 provider 测试。
4. 运行定向 Go 测试和 `gofmt`/`git diff --check`，检查 diff 不触及用户已有修改。
5. 通过质量检查后再决定是否需要进入生产构建/部署；本任务默认不执行生产变更。

## Validation

```bash
gofmt -w backend/internal/service/channel_monitor_checker.go backend/internal/service/channel_monitor_const.go backend/internal/service/channel_monitor_checker_body_test.go
go test -tags unit ./backend/internal/service -run 'ChannelMonitor|channelMonitor|MonitorChecker' -count=1
git diff --check
```

## 风险点

- SSE 只收到文本但提前 EOF 时仍记为 error；仅客户端独立读超时且已有有效输出时允许 degraded。
- 60 秒响应头等待会延长真实故障暴露时间，因此必须保留“无首包/失败事件/总超时”错误路径。
- 当前分支含用户未提交改动，禁止 reset、checkout 或批量格式化无关文件。
