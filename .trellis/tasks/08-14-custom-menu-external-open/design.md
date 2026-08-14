# 自定义菜单打开方式 — 技术设计

## Architecture

在现有 `custom_menu_items` JSON 上增加可选字段 `open_mode`，不新增 settings key、不改表结构。

```
管理员 SettingsView
  → PUT /api/v1/admin/settings
  → setting_handler_update 校验并规范化
  → settings.custom_menu_items JSON

公开设置 / 管理员设置
  → AppSidebar 按 open_mode 分流
      embed     → router-link /custom/:id → CustomPageView iframe
      external  → <a href url target=_blank>  （与购买套餐相同）
```

## Contract

`CustomMenuItem` 增加：

| 字段 | 类型 | 规则 |
|---|---|---|
| `open_mode` | `""` \| `"embed"` \| `"external"` | 空或缺失 = `embed` |

保存时规范化：空字符串写成 `embed`。非法值返回 400。

与 URL 的组合：

| URL | embed | external |
|---|---|---|
| `https://...` / `http://...` | 允许 | 允许 |
| `md:<slug>` | 允许 | 拒绝 |

前后端类型必须同步：

- `backend/internal/handler/dto/settings.go` `CustomMenuItem`
- `frontend/src/types/index.ts` `CustomMenuItem`
- `SettingsView.vue` 本地 form 类型

`json.Unmarshal` 对缺失字段会得到空字符串，因此旧数据无需迁移即可保持嵌入。

## Data flow

1. **写**：Settings 表单带上 `open_mode`；`addMenuItem()` 默认 `embed`。
2. **校验**：沿用现有 label / URL / visibility / id 规则，再校验 `open_mode` 与 `md:` 互斥。
3. **读**：公开设置仍过滤 `visibility=admin`；`open_mode` 原样下发。
4. **侧栏**：把 `open_mode === 'external'` 且 URL 为绝对 http(s) 的项映射为 `NavItem.externalUrl = item.url`。
5. **CSP**：`GetFrameSrcOrigins` 只收集会被 iframe 的 URL。`external` 和 `md:` 不进 `frame-src`。

## Sidebar wiring

现有 `externalUrl` 只覆盖用户侧栏和管理员「我的账户」。管理员主菜单仍一律 `router-link`（`AppSidebar.vue:80-100`）。

实现时三处都要能渲染外链：

- 普通用户 `userNavItems`
- 管理员个人区 `personalNavItems`
- 管理员主菜单（含简单模式下追加的自定义项）

映射伪代码：

```ts
externalUrl: isExternalMenuItem(item) ? item.url : undefined
```

`isExternalMenuItem`：`open_mode === 'external'` 且 URL 不是 `md:` 前缀。不要把 `md:` 写进 `href`。

## `/custom/:id` fallback

侧栏不再把 `external` 项指到 `/custom/:id`，但旧书签或手动输入仍可能进来。`CustomPageView` 在 `open_mode=external` 时禁止 iframe，改为展示外链按钮（可沿用现有「新标签打开」控件），避免把直跳目标嵌进本站。

## Compatibility

- 旧 JSON 无 `open_mode` → 嵌入，行为不变。
- 购买套餐继续走 `purchase_subscription_*`，本任务不改。
- 不预置「无限画布」项；上线后管理员自行新增。

## Trade-offs

- 复用自定义菜单，而不是再加一套 `canvas_*` 设置：少一组 key，名称/图标/排序仍可调。代价是管理员要会用自定义菜单。
- 直跳一律新标签，与购买套餐一致；不做「当前页跳转」选项。
- 不在首次部署写死画布 URL，避免把环境相关地址提交进仓库。

## Rollback

去掉前端对 `open_mode` 的分流后，侧栏会重新把这些项指到 `/custom/:id`。后端多出来的 JSON 字段可忽略。不需要回滚数据库。
