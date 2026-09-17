# 技术设计：Responses 渠道监控流式探测

## 边界

只改 `backend/internal/service/channel_monitor_checker.go`、监控常量和对应单元测试。`runCheckForModel` 继续负责状态归类，Responses adapter 负责请求格式，新的读取逻辑只对 OpenAI Responses 生效。

## 数据流与状态

```text
challenge
  -> POST /v1/responses (stream=true)
  -> HTTP/SSE reader
  -> 首个有效 output_text / completed / failed 事件
  -> challenge 校验 + 首包/完整耗时
  -> CheckResult
  -> 现有历史落库
```

- HTTP 非 2xx：保持现有 `error` 和错误 body 摘要。
- SSE 出现 `response.failed`、提前 EOF、协议不完整或上下文总超时：`error`。
- 只有 HTTP client 自身读超时、且已有可验证输出时才允许记为 `degraded`，并保留首包与总耗时，避免掩盖真实中断。
- 收到 challenge 答案并正常完成：按完整耗时 `operational/degraded`；若首包耗时超过 degraded 阈值，即使完整响应快，也按慢响应记录。
- 2xx 但无有效文本或 challenge 不匹配：`failed`。

## 复用与实现选择

- 复用项目现有 `extractOpenAISSEDataLine`，不新增 SSE 解析依赖。
- 保留 `monitorResponseMaxBytes` 限制，流式读取累计字节超限时终止并返回可诊断错误。
- 用 `bufio.Reader.ReadBytes('\n')` 或现有同类读取模式处理 SSE，不把完整响应缓存到内存后再判定。
- 为避免 API 结构扩散，内部读取函数返回文本、原始诊断片段、状态码、首包耗时、完整耗时和错误；外层仍只构造 `CheckResult`。

## 超时

- `monitorResponseHeaderTimeout = 60s`。
- `monitorRequestTimeout` 至少为 65s，给响应头后少量读取/收尾空间。
- `runOne` 外层 deadline 使用 `monitorRequestTimeout + monitorPingTimeout + monitorRunOneBuffer`，因此自动覆盖新总超时。
- 不把 runner deadline 单独写死，避免再次出现分层超时不一致。

## 回滚

代码回滚只需恢复监控 checker 与常量改动；不涉及数据迁移和生产状态。部署仍需遵循下游 fork 工作流，构建在本地完成，OVH 只切换已构建镜像。
