# 自定义菜单支持新标签直跳

## Goal

让管理员在「自定义菜单页面」里把某一项设成和外链充值一样的新标签直跳。用户点击后离开 Sub2API，不再进入站内 `/custom/:id` iframe。典型用法是用户侧栏增加「无限画布」，跳转链接由管理员配置。

## Background

- 现有自定义菜单只能进站内页并 iframe 嵌入，见 `frontend/src/views/user/CustomPageView.vue:96-111` 与 `frontend/src/components/layout/AppSidebar.vue:834-839`。
- 「购买套餐」已经用 `NavItem.externalUrl` 渲染为 `<a target="_blank" rel="noopener noreferrer">`，见 `AppSidebar.vue:148-161` 与 `824-826`。
- 自定义菜单项契约在 `backend/internal/handler/dto/settings.go:11-19` 与 `frontend/src/types/index.ts:193-201`，目前没有打开方式字段。
- 管理员在 **系统设置 → 自定义菜单页面** 配置名称、URL、图标、可见角色、排序，上限 20 项。
- 本仓库是下游二开，工作分支必须从 `custom/main` 拉出。

## Requirements

1. 自定义菜单项增加打开方式 `open_mode`：`embed`（站内嵌入，默认）或 `external`（新标签直跳）。
2. 缺少或空的 `open_mode` 视为 `embed`，已有帮助中心等嵌入项行为不变。
3. 管理员可在设置页为每一项选择打开方式，并能保存、回读。
4. `open_mode=external` 的项在用户侧栏、管理员「我的账户」侧栏、管理员侧栏都按购买套餐同一套方式打开：新标签、`noopener noreferrer`。
5. `md:` Markdown 菜单项只能嵌入；保存 `external` + `md:` 必须被后端拒绝。
6. `external` 项的 URL 仍必须是绝对 `http(s)` URL。
7. 管理员用现有自定义菜单新增「无限画布」（或任意名称），填目标 URL，选新标签直跳，用户即可使用。本任务不预置菜单项、不写死画布 URL。
8. 直达 `/custom/:id` 的 `external` 项不得再 iframe 外链。

## Acceptance Criteria

- [x] AC1：设置页可为菜单项选择「站内嵌入 / 新标签直跳」，保存后再打开仍是所选值。
- [x] AC2：`open_mode=external` 的用户可见项在用户侧栏是 `<a href="{url}" target="_blank" rel="noopener noreferrer">`，点击不进入 `/custom/:id`。
- [x] AC3：未设 `open_mode` 或值为 `embed` 的项仍进入 `/custom/:id` 并 iframe，行为与现在一致。
- [x] AC4：后端拒绝非法 `open_mode`，以及 `external` 配 `md:` URL。
- [x] AC5：管理员可见的 `external` 项在管理员侧栏同样新标签直跳。
- [x] AC6：前端有设置保存载荷与侧栏接线测试；后端有打开方式校验测试。

## Out of Scope

- 内嵌 tldraw / Excalidraw 等白板产品。
- 再做一套独立的「无限画布」开关和 URL 设置（不复制购买套餐那套字段）。
- 改购买套餐 / 站内 `/purchase` 支付流程。
- 预置「无限画布」菜单项或默认 URL。
- 改自定义菜单数量、文案长度等既有上限。

## Technical Notes

- 字段落在现有 `custom_menu_items` JSON 上，不需要新 settings key，也不需要 SQL 迁移。
- 打开方式的权威校验在管理员更新设置的入口；前端只做展示和选择。
- 详细数据流、侧栏接线和回退见 `design.md`。
