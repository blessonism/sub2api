# OpenAI 账号信任请求侧 service_tier 开关

## Goal

为 OpenAI 账号新增账号级开关 `openai_trust_requested_service_tier`(信任请求侧档位)。开启后,该账号的用量记录与计费档位以客户端请求的 `service_tier` 为准,不再因上游响应声明更便宜档位(如 `default`)而降级。

## 背景

OVH 生产实例对接的上游本身是另一个 sub2api 中转站(蝶祈云)。该上游背后使用 Codex OAuth 凭据,官方 Codex 私有后端即使在 Fast 实际生效时也常在响应中声明 `service_tier=default`;上游(新版)对 OAuth 账号豁免该误报并按 fast 记录计费,但返回给本站的响应体仍是原样的 `default`。本站账号是 apikey 型,无法自动获知上游背后凭据类型,响应声明被当作权威,导致:

- 使用记录服务档位显示 standard(fast 被压成 default → 前端归一为 standard);
- 用户按 standard(1x)计费,而上游按 fast(2x)向本站结算,差价由站长承担。

已有豁免 `IsOpenAIOAuthLike()` 只覆盖"直接对接 Codex OAuth"的场景,无法覆盖链式中转。需要管理员对已知上游是中转站的账号显式声明"响应档位不可信"。

## Requirements

- 后端:`Account` 新增方法读取 `Extra["openai_trust_requested_service_tier"]`(bool,默认 false)。
- 后端:`ResolveOpenAIServiceTierBilling` 在该开关开启时直接以请求侧档位作为计费档位;响应档位仍记录在观测字段(Observed)供审计,不参与降级。
- 开关语义与现有 OAuth 豁免一致:使用 `billingAccount`(shadow 凭据解析后)判断。
- 前端:账号创建/编辑弹窗在"OpenAI 自动透传"开关附近新增"信任请求侧档位"开关,仅对 openai 平台账号展示,提交写入 Extra。
- 未开启开关的账号行为完全不变(apikey + 响应 default 仍降级;OAuth + default 仍豁免)。
- 不改动 Anthropic 侧逻辑。

## Acceptance Criteria

- [ ] 开启开关的账号:请求 fast + 响应声明 default 时,usage 记录与计费档位为 fast,不降级;响应 default/standard 均不降级。
- [ ] 未开启开关:所有既有行为不变(含 apikey 降级、Codex OAuth 豁免)。
- [ ] 前端创建/编辑账号弹窗可开关并正确保存到 Extra。
- [ ] 后端单测覆盖:开关开启(不降级)、关闭(既有行为)、shadow/nil 账号安全。
- [ ] `go build` / `go test` 相关包通过;前端 lint / type-check 通过。

## Notes

### 分支偏离说明

按 `.trellis/spec/guides/downstream-fork-workflow.md`,feature 分支应从 `custom/main` 拉出。但本功能依赖 `sync/upstream-20260903` 上的 fast 策略重构(`ResolveOpenAIServiceTierBilling`、响应档位语义、f8eae4504 归一化接线等),`custom/main`(= 线上 10e321ad9)尚无这些结构。因此本任务工作分支从 `sync/upstream-20260903` 拉出,PR 目标也为该分支;待其合回 `custom/main` 后随同一批次部署。此偏离已在任务与聊天记录中说明。
