# 渠道监控开放 API 文档入口

## Goal

在用户渠道监控页面增加开放 API 标识与调用说明弹窗，覆盖 V1/V2 页面并提供复制示例。

## Requirements

- TBD

## Acceptance Criteria

- [ ] TBD

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
## 需求

在用户渠道监控页面提供开放渠道状态 API 的可发现入口。用户点击入口后看到调用说明、鉴权方式、限流规则、响应字段和可复制的 curl 示例。

## 验收标准

- V1 与 V2 渠道监控页面均显示带明确文字和图标的 API 文档入口。
- 点击入口打开可关闭的说明弹窗，不影响页面刷新、筛选和详情弹窗。
- 弹窗覆盖 `docs/CHANNEL_STATUS_API.md` 中的实际路径、Bearer/x-api-key 鉴权、限流和关键响应字段。
- curl 示例可一键复制，并对复制成功/失败给出反馈。
- zh/en 文案同步，移动端内容可滚动，入口和弹窗具备可访问名称。
- 不新增后端接口，不把 API Key 写入前端或 URL。
