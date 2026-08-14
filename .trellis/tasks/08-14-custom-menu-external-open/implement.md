# 自定义菜单打开方式 — 实现清单

## 工作分支

从 `custom/main` 拉出 `feature/custom-menu-external-open`。不要在 `main` 或直接在 `custom/main` 上改。

## 顺序

1. **契约**
   - `backend/internal/handler/dto/settings.go`：`CustomMenuItem` 增加 `OpenMode string \`json:"open_mode,omitempty"\``。
   - `frontend/src/types/index.ts`：`open_mode?: 'embed' | 'external'`。
2. **后端校验**
   - `backend/internal/handler/admin/setting_handler_update.go`：规范化空值为 `embed`；只允许 `embed` / `external`；`external` + `md:` 返回 400。
   - `backend/internal/service/setting_public.go`：`GetFrameSrcOrigins` / `parseCustomMenuItemURLs` 排除 `external` 项。
3. **后端测试**
   - 给更新设置加用例：合法 `external`、缺省回落 `embed`、非法值、`external`+`md:`。
   - 若更新逻辑没有可单测的提取函数，把规范化/校验抽成小函数再测，避免只测巨型 handler。
4. **管理端表单**
   - `SettingsView.vue`：每项增加打开方式选择；`addMenuItem()` 默认 `embed`；保存载荷带上该字段。
   - `frontend/src/i18n/locales/{zh,en}/admin/settings.ts`：中英 key 对齐，说明「新标签直跳不会停在本站」。
5. **用户侧栏**
   - `AppSidebar.vue`：自定义项按 `open_mode` 填 `externalUrl`。
   - 管理员主菜单补上与用户侧栏相同的 `<a v-if="item.externalUrl">` 分支。
6. **站内回退**
   - `CustomPageView.vue`：`external` 项不渲染 iframe。
7. **前端测试**
   - `AppSidebar.spec.ts`：`open_mode=external` 渲染外链；缺省/embed 仍走 `/custom/:id`。
   - `SettingsView.spec.ts`：保存载荷包含 `open_mode`。
   - 如有 `CustomPageView` 测试，补 `external` 不 iframe。

## 验证命令

```bash
cd frontend && pnpm exec vitest run \
  src/components/layout/__tests__/AppSidebar.spec.ts \
  src/views/admin/__tests__/SettingsView.spec.ts

cd frontend && pnpm exec vue-tsc --noEmit --pretty false

# 后端：以实现时实际测试文件名为准
cd backend && go test ./internal/handler/admin/ ./internal/handler/dto/ ./internal/service/ -count=1
```

## 风险点

- `AppSidebar.vue` 管理员主菜单目前没有 `externalUrl` 分支，漏改则管理员可见外链仍会进 iframe。
- `SettingsView.vue` 极大，只改自定义菜单卡片和 form 类型，不要顺手重排整个设置页。
- `api_contract_test.go` 里 `custom_menu_items` 现在是空数组；若契约快照含菜单项结构，同步加字段。
- 不要改 `purchase_subscription_*`。

## 回退

还原本任务文件即可。settings JSON 多出的 `open_mode` 可留着，旧代码会忽略。
