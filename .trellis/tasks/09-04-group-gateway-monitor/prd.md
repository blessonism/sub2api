# PRD：分组级渠道监控（v2 的效果 + v1 的展示）

## 背景

当前 v1 渠道监控的探测目标是一个固定 endpoint（单一上游）。当一个分组（如 pro 分组）接入多个上游账号时，真实流量走网关选号 + failover，单上游故障对用户无感；但监控探测恰好打到坏上游时会报 error，形成"监控报障、实际服务正常"的误报。另外探测遇到上游/网关限流（如 "Too many pending requests, please retry later"）时被笼统记为 error，与"渠道真挂了"无法区分。

用户诉求：**状态页保持 v1 卡片形态（适合展示给终端用户），但卡片语义升级为分组级可用性（v2 视角：failover 之后的真实体验）。**

## 方案（已与用户确认）

1. **探测语义：网关入口探测**。监控条目 endpoint = 本站网关公网地址，apiKey = 绑定目标分组的 API key。探测请求经网关完整走选号 + failover——探测成功 ⇔ 分组内至少一个上游能承接（任一上游可用=可用）。复用现有 probe 引擎，不改探测/调度/历史模型。
2. **聚合规则：任一上游可用=可用**。由网关 failover 自然保证；degraded 语义只保留给"可用但响应慢"。
3. **限流/容量错误归类**：探测错误按 v2 taxonomy 识别 `rate_or_capacity`，前端以"限流/拥挤"黄色警示区分于红色故障；可用率计算不变（限流即未承接成功）。
4. **条目标注**：新增 `target_kind`（endpoint / gateway_group），卡片显示"分组"徽标，让读者理解该卡片代表整组服务。

## 范围

- 后端：迁移（2 列）、CheckResult/ChannelMonitor 字段、checker 错误分类、repo 读写、admin/user DTO 透传。
- 前端：类型/常量、卡片限流样式 + 分组徽标、管理端表单目标类型选择 + 分组/API key 联动、i18n。
- 不做（二期）：逐账号直连探测与组内子状态、系统级站点公开 URL 配置、v2 聚合剔除探测流量。

## 验收标准

1. 配置 endpoint=本站网关 + 分组绑定 key 的监控条目后，探测结果反映 failover 后的分组可用性：坏掉部分上游不改变 operational 判定；全部上游不可用时记 error。
2. 探测错误消息命中限流/容量语义（429、Too many pending requests、rate limit、overloaded 等）时，历史与卡片展示 `rate_or_capacity` 类别（黄色"限流/拥挤"），与普通 error 视觉区分。
3. `target_kind=gateway_group` 的条目在管理端与用户状态页卡片上显示"分组"徽标。
4. 管理端创建/编辑表单可选择"本站分组"目标：联动选择分组 → 列出该分组绑定的 API key → endpoint 手填（文案提示公网地址）。
5. `go vet`、定向 go test、前端 build 通过；迁移测试覆盖新列。
