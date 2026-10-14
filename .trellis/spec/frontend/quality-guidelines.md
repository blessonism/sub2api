# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)

### Scenario: 轻量账号列表与异步参数快照

#### 1. Scope / Trigger
- 修改 `AccountsView` 的轻量列表、账号弹窗、批量操作或 `useTableLoader` 时。

#### 2. Signatures
- 首次列表参数 `lite=1`；完整详情 `adminAPI.accounts.getById(id)`；`useTableLoader` 的 `fetchFn`。

#### 3. Contracts
- 轻量行缺少 `groups` 时从分组目录按 `group_ids` 展示；编辑、测试、统计弹窗打开前加载完整账号。
- `fetchFn` 接收当前参数的独立浅拷贝，后续刷新移除 `lite` 不能改变已经发出的请求参数。
- 批量刷新部分失败时保留失败账号的选择；没有错误明细时保留原选择，便于重试。
- 菜单内滚动保持菜单打开，表格滚动关闭菜单。

#### 4. Validation & Error Matrix
- 完整详情加载失败 → 展示错误并保持弹窗关闭。
- 测试 mock 经 `vi.restoreAllMocks` 恢复后仍须返回有效 Promise；列表 mock 使用 `vi.fn(async () => [])` 等真实默认实现。

#### 5. Good / Base / Bad Cases
- Good：首次轻量加载后，后台完整行刷新不改变首次调用的 `lite` 参数。
- Base：完整行的分组数据直接展示。
- Bad：把轻量行直接交给凭据编辑弹窗，或把可变筛选对象交给异步请求。

#### 6. Tests Required
- `AccountsView.lite.spec.ts` 保留分组展示、详情成功/失败、菜单滚动和首次/后续请求参数断言。
- `AccountsView.selectAllResults.spec.ts` 保留部分失败选择及重试回归；完整套件不得有未处理异常。

#### 7. Wrong vs Correct
- Wrong：`fetchFn(..., toRaw(params), ...)`；仅给 mock 添加会被 restore 清除的 `mockResolvedValue`。
- Correct：`fetchFn(..., { ...toRaw(params) }, ...)`；为 mock 提供可恢复的默认函数实现。
