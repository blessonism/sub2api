# `2c503a333` 历史同步 dry-run 与 preflight 设计

## 结论

仅依赖本地 Git，可以在真正合并前稳定识别这次事故的两个高风险面，并自动选择 Dashboard 与 UsageView 的定向检查：

- `6a53f203a` 是第一父提交一侧独有的 Dashboard 二开提交；它与上游变更直接相交 7 个文件，其中 `usage_log_repo.go` 被上游从 4701 行缩减 4525 行，并拆出 5 个同前缀文件。该信号必须判定为“下游能力落在上游拆分源文件中”的高风险结构迁移，并选择 Dashboard repository integration tests。
- `1a3cc2a78` 实际是第二父提交（上游）一侧独有的提交，不是第一父提交一侧的下游提交。它改动的文件中有 5 个同时被下游分支改过，其中 `frontend/src/views/user/UsageView.vue` 是双方采用不同表格结构的同一消费端。该信号必须判定为“上游新行为进入下游 overlay”的高风险交叉修改，并选择用户 UsageView 渲染测试与 `latencyHealth` 阈值测试。
- 只做文件交集和 `git merge` 冲突检查不够。`2c503a333` 是可正常提交的 merge，但结果同时出现“Dashboard 字段/卡片仍在而生产者消失”和“`latency` 列仍在而槽位仍是旧 key”两类静默语义回归。preflight 应负责保守选中风险与测试；最终保证必须来自合并后的能力探针和行为测试。

建议实现为一个不 checkout、不 merge、不写索引的两阶段命令：合并前 `preflight` 生成风险报告和检查计划，合并后 `verify` 对结果提交执行能力探针和同一组定向测试。所有分析输入均来自本地 refs、commit/tree/blob 和仓库内的能力映射文件，不依赖 GitHub API、网络或数据库查询。

## 历史拓扑事实

目标 merge：

```text
M  = 2c503a333df1f7903a8ecd3471787391c1601c4c
D  = c1560bf80e1e65e305962a82d0b4830cbaa74d95  (M^1，下游 custom/main)
U  = 12d811bd76572836d6df6e1fa8aa5ff91be3b12e  (M^2，上游 main)
B  = 44ab690a018f53b5b1d53fa2c2fe285679a96eea  (merge-base D U)
```

`B..D` 有 167 个提交，`B..U` 有 115 个提交。按 tree diff 统计，`B -> D` 改动 995 个路径，`B -> U` 改动 310 个路径，精确路径交集为 72 个。数字较大说明不能把全部交集都作为阻断项，必须再按提交归属、路径结构和能力映射排序。

两个关注提交的可达性：

- `6a53f203a670df1dd6fdb95146dff93d4733f838`：是 `D` 的祖先，不是 `U` 的祖先；属于真正的下游独有能力。
- `1a3cc2a787dc79231012e57ff9e8ad8606afcc7a`：不是 `D` 的祖先，是 `U` 的祖先；属于本次将进入下游的上游新行为。
- 两者都是 `M` 的祖先，因此 merge 在提交拓扑层面“包含”了它们；事故发生在最终 tree 的行为组合上，而不是某个提交没有进入历史。

## 可复现实验

以下命令都只读取本地对象与 refs，不会修改工作树、索引或分支。

### 1. 确认父提交、merge-base 与提交规模

```bash
git show -s --format='commit %H%nparents %P%nsubject %s' 2c503a333
B=$(git merge-base 2c503a333^1 2c503a333^2)
git show -s --format='%H %s' "$B"
git rev-list --left-right --count 2c503a333^1...2c503a333^2
```

预期关键输出：

```text
parents c1560bf80e1e65e305962a82d0b4830cbaa74d95 12d811bd76572836d6df6e1fa8aa5ff91be3b12e
44ab690a018f53b5b1d53fa2c2fe285679a96eea Merge pull request #3768 ...
167 115
```

### 2. 确认关注提交属于哪一侧

```bash
for C in 6a53f203a 1a3cc2a78; do
  for SIDE in 2c503a333^1 2c503a333^2; do
    git merge-base --is-ancestor "$C" "$SIDE" \
      && echo "$C is in $SIDE" \
      || echo "$C is not in $SIDE"
  done
done
```

预期关键输出：

```text
6a53f203a is in 2c503a333^1
6a53f203a is not in 2c503a333^2
1a3cc2a78 is not in 2c503a333^1
1a3cc2a78 is in 2c503a333^2
```

### 3. 计算精确文件交集

```bash
B=$(git merge-base 2c503a333^1 2c503a333^2)
comm -12 \
  <(git diff --name-only "$B" 2c503a333^1 | sort) \
  <(git diff --name-only "$B" 2c503a333^2 | sort)
```

完整输出有 72 个路径。与事故直接相关的交集至少包括：

```text
backend/internal/handler/admin/dashboard_handler.go
backend/internal/pkg/usagestats/usage_log_types.go
backend/internal/repository/usage_log_repo.go
frontend/src/components/admin/usage/UsageFilters.vue
frontend/src/i18n/locales/en.ts
frontend/src/i18n/locales/zh.ts
frontend/src/types/index.ts
frontend/src/views/admin/UsageView.vue
frontend/src/views/admin/__tests__/DashboardView.spec.ts
frontend/src/views/admin/__tests__/UsageView.spec.ts
frontend/src/views/admin/ops/components/OpsErrorLogTable.vue
frontend/src/views/user/UsageView.vue
```

实际实现必须使用 `--raw -z` 或 `--name-status -z` 解析，不能依赖换行和 `sort` 处理任意文件名；上面只作为人工复现实验。

### 4. 关联下游 Dashboard 提交与上游变更

```bash
B=$(git merge-base 2c503a333^1 2c503a333^2)
comm -12 \
  <(git diff-tree --no-commit-id --name-only -r 6a53f203a | sort -u) \
  <(git diff --name-only "$B" 2c503a333^2 | sort -u)
```

预期得到 7 个直接相交路径：

```text
backend/internal/handler/admin/dashboard_handler.go
backend/internal/pkg/usagestats/usage_log_types.go
backend/internal/repository/usage_log_repo.go
frontend/src/i18n/locales/en.ts
frontend/src/i18n/locales/zh.ts
frontend/src/types/index.ts
frontend/src/views/admin/__tests__/DashboardView.spec.ts
```

其中仓储文件的结构变化可复现为：

```bash
B=$(git merge-base 2c503a333^1 2c503a333^2)
git show "$B:backend/internal/repository/usage_log_repo.go" | wc -l
git diff --numstat "$B" 2c503a333^2 -- \
  backend/internal/repository/usage_log_repo.go
git diff --name-status "$B" 2c503a333^2 -- \
  'backend/internal/repository/usage_log_repo_*.go'
git diff --find-copies-harder -C20% --summary "$B" 2c503a333^2 -- \
  backend/internal/repository/usage_log_repo.go \
  'backend/internal/repository/usage_log_repo_*.go'
```

预期关键输出：

```text
4701
38  4525  backend/internal/repository/usage_log_repo.go
A backend/internal/repository/usage_log_repo_dashboard.go
A backend/internal/repository/usage_log_repo_insert.go
A backend/internal/repository/usage_log_repo_query.go
A backend/internal/repository/usage_log_repo_stats.go
A backend/internal/repository/usage_log_repo_trend.go
copy backend/internal/repository/{usage_log_repo.go => usage_log_repo_insert.go} (20%)
copy backend/internal/repository/{usage_log_repo.go => usage_log_repo_stats.go} (26%)
```

这同时满足“源文件删除比例 96%（4525/4701）”“同目录新增至少两个同 stem 文件”“低阈值 copy 检测命中”的拆分信号。即使 Git 的常规 `-M50% -C50%` 只把源文件报告为 `M`，preflight 也不能把它当普通小改动。

同一范围还有两个明确删除：`frontend/src/i18n/locales/en.ts` 与 `zh.ts` 均删除 100%，并在各自同名目录新增 12 个模块文件。这应报告为 locale split；它不是本次两项回归的直接断点，但会影响 `6a53f203a` 中写入旧单文件的文案，属于必须人工复核或由 locale collision/equivalence tests 覆盖的高风险迁移。

### 5. 关联上游 UsageView 优化与下游 overlay

```bash
B=$(git merge-base 2c503a333^1 2c503a333^2)
comm -12 \
  <(git diff-tree --no-commit-id --name-only -r 1a3cc2a78 | sort -u) \
  <(git diff --name-only "$B" 2c503a333^1 | sort -u)
```

预期得到 5 个路径：

```text
frontend/src/components/admin/usage/UsageFilters.vue
frontend/src/views/admin/UsageView.vue
frontend/src/views/admin/__tests__/UsageView.spec.ts
frontend/src/views/admin/ops/components/OpsErrorLogTable.vue
frontend/src/views/user/UsageView.vue
```

其中 `frontend/src/views/user/UsageView.vue` 必须提升为高风险：上游提交在此把列 key 从 `first_token`、`duration` 改为 `latency`，而下游分支对同一页面做过大规模内联表格/管理员代看等 overlay。文件可文本合并不等于上游公共组件假设能适用于下游内联组件。

### 6. 验证历史 merge 的静默回归形态

```bash
git grep -n -E \
  'TodayActiveUsers|YesterdayActiveUsers|TotalUserBalance|SubscriptionRemainingValue' \
  2c503a333 -- \
  backend/internal/repository \
  backend/internal/pkg/usagestats \
  frontend/src/views/admin/DashboardView.vue

git grep -n -E \
  'cell-latency|cell-first_token|cell-duration|key: .latency.' \
  2c503a333 -- frontend/src/views/user/UsageView.vue
```

预期现象：

- Dashboard 四字段仍出现在 DTO、repository integration test 与前端卡片中，但在 repository 实现文件中没有赋值或查询生产者。
- 用户页列定义为 `latency`，但仍只存在 `cell-first_token` 与 `cell-duration`，没有 `cell-latency`。

这证明两个回归都不会被“merge 无冲突”“编译通过”或“字段仍存在”充分排除。

## 建议的 preflight 算法

### 输入与不可变条件

输入：

- `downstream_ref`：待接收上游更新的下游头，例如 `custom/main`。
- `upstream_ref`：本地已 fetch 的上游头，例如 `upstream/main`。
- 仓库内能力映射：记录稳定能力 ID、关注提交、路径选择器、结果探针和测试命令。

只读约束：

- 只调用 `rev-parse`、`merge-base`、`rev-list`、`log`、`diff`、`diff-tree`、`show`、`cat-file`、`ls-tree`、`grep` 等读取对象的 Git 子命令。
- 不调用 `checkout`、`switch`、`merge`、`read-tree`、`update-index`、`commit-tree`；不要用 `git merge-tree --write-tree`，因为它会向 object database 写入对象。
- 输出写到 stdout；如需要机器产物，由调用者显式重定向到任务/CI artifact，不让分析命令改变 Git 状态。

### 阶段 A：构建两侧变更模型

1. 解析 `D`、`U`，计算全部 merge-base；正常情况要求唯一 `B`，多个 merge-base 时报告 criss-cross merge 并提高风险级别。
2. 计算 `B -> D` 与 `B -> U` 的 `--raw -z -M50% -C50%` 清单，保留状态、旧路径、新路径、blob OID、mode，而不是只保留文件名。
3. 用 `git log --cherry-pick --right-only --no-merges U...D` 枚举下游独有补丁；保留 merge commits 作为单独的拓扑事件，但不要把 merge diff 当能力来源。
4. 对映射中的关注提交分别执行 `merge-base --is-ancestor <commit> D/U`，分类为 `downstream-only`、`incoming-upstream`、`already-common` 或 `unknown/missing`。不能仅凭作者或 commit subject 判断归属。

### 阶段 B：建立路径谱系与结构信号

对每个下游独有提交及已登记能力，收集该提交的 old/new paths，并与上游 change model 比较：

1. **精确交集**：路径同名，完整覆盖所有双方都改了同一路径的情况。
2. **rename/copy 交集**：任一侧路径命中 `R*`/`C*` 的旧路径或新路径；能力路径沿新路径继续追踪。
3. **delete 信号**：能力触达路径在上游为 `D`，无论是否有文本冲突都标记高风险。
4. **split 信号**：满足任一强信号即标记高风险，弱信号组合则标记中风险：
   - `--find-copies-harder -C20%` 显示源 blob 被复制到新增文件；
   - 源文件被删除，且同目录出现至少 2 个与源 basename stem 相同的新增文件；
   - 源文件保留但删除行数 / base 总行数不低于 50%，且同目录出现至少 2 个同 stem 新文件；
   - 源文件保留但 blob size 降低不低于 50%，用于 numstat 无法处理的非文本文件，只作为弱信号。
5. **incoming-vs-overlay 信号**：对已登记的上游能力提交，若提交路径与 `B -> D` 相交，则说明上游新行为要进入下游自定义消费端；按双方改动规模、是否触达模板/路由/数据生产者提高风险。

拆分判断必须输出证据而非只输出分数，例如 `4525/4701 deleted; 5 added siblings; C20 hit 2`，以便人工快速确认。阈值是召回优先的风险提示，不应自动决定如何解决冲突。

### 阶段 C：能力与检查选择

能力映射不应只用宽泛目录匹配；至少同时支持 `tracked_commits`、`paths`、`structural_successors`、`result_probes` 和 `checks`。本次历史样本需要两项：

#### `dashboard-operational-metrics`

触发条件：

- `6a53f203a` 为 `downstream-only`；并且该提交任一路径与上游直接相交、被删除/重命名，或出现结构 successor。
- 路径兜底匹配 `backend/internal/repository/usage_log_repo*.go`、Dashboard stats DTO/handler/API/type/view。

结果探针：

- 四个字段在公开契约中存在。
- repository 的结果 tree 中每个字段都至少有生产者；不能把 test、DTO tag 或前端消费者算作生产者。
- `GetDashboardStats` 与 range fallback 均应经过运营指标填充，`TodayActiveUsers` 与 `ActiveUsers` 口径一致。

建议定向检查：

```bash
cd backend
go test -tags=integration ./internal/repository \
  -run 'TestUsageLogRepoSuite/(TestDashboardStats_TodayTotalsAndPerformance|TestDashboardStats_OperationalMetrics|TestDashboardStatsWithRange_Fallback|TestDashboardAggregationConsistency)$' \
  -count=1 -timeout=60s
```

前端卡片契约检查可并行执行：

```bash
cd frontend
pnpm test:run -- src/views/admin/__tests__/DashboardView.spec.ts
```

integration 环境不可用时必须报告“未验证”，不能用 mock handler/view test 代替真实 SQL 行为检查。

#### `usage-latency-column`

触发条件：

- `1a3cc2a78` 为 `incoming-upstream`，且其改动路径与下游 overlay 相交。
- 路径兜底匹配用户/管理员 UsageView、UsageTable、`latencyHealth` 和相关测试。

结果探针：

- 如果用户页列定义含 `key: 'latency'`，同一渲染边界必须存在 `cell-latency`，或明确委托给含该槽位的公共 `UsageTable`。
- 结果不得只剩无列对应的 `cell-first_token` / `cell-duration`。
- 延迟分档工具和长时长格式行为由测试校验，不在 shell 中重复实现 Vue 语义。

建议定向检查：

```bash
cd frontend
pnpm test:run -- \
  src/views/user/__tests__/UsageView.spec.ts \
  src/utils/__tests__/latencyHealth.spec.ts
```

历史 `2c503a333` 上的旧 `UsageView.spec.ts` 不渲染 `cell-latency`，因此即使被正确选中也可能通过。必须使用 `5a18ff2bb` 中新增的视图级延迟列断言，或等价的当前测试，检查选择才真正形成门禁。这是“选择器正确但测试 oracle 不完整”的典型边界。

### 阶段 D：稳定输出

建议同时提供 human-readable 与 JSON 输出，便于本地复核和 CI 分片。历史 dry-run 的 human-readable 核心应类似：

```text
base: 44ab690a0
downstream: c1560bf80 (167 commits, 995 paths)
upstream: 12d811bd7 (115 commits, 310 paths)
direct intersections: 72

[HIGH] dashboard-operational-metrics
  tracked commit: 6a53f203a (downstream-only)
  direct intersections: 7
  split: backend/internal/repository/usage_log_repo.go
         deleted 4525/4701 lines; 5 same-stem additions; C20 copies detected
  checks: dashboard-repository-integration, dashboard-view

[HIGH] usage-latency-column
  tracked commit: 1a3cc2a78 (incoming-upstream)
  downstream overlay intersections: 5
  critical consumer: frontend/src/views/user/UsageView.vue
  checks: user-usage-view, latency-health

[HIGH] locale-module-split
  deleted: frontend/src/i18n/locales/{en,zh}.ts
  successors: frontend/src/i18n/locales/{en,zh}/**
  checks: locale collision/equivalence
```

退出码建议：无命中为 `0`；只有低/中风险且检查计划完整为 `0`；存在高风险但缺少结果探针或检查为 `2`；Git 对象/refs 缺失或分析失败为 `3`。高风险本身不必阻止创建 sync 分支，但必须阻止未验证的 merge 进入 `custom/main`。

## 充分性、下界与误报边界

### 可以保证的部分

- 在 refs 固定且 diff 解析正确的前提下，集合交集会完整找出所有“双方改动相同规范化路径”的情况；这是精确集合运算，不依赖启发式。
- 对 Git 已识别的 rename/copy，old/new path 双向展开后不会因只比较新路径而漏报。
- 对本历史样本，`usage_log_repo.go` 同时满足行删除比例、同 stem 新文件和低阈值 copy 三类信号；`en.ts` / `zh.ts` 同时满足完整删除与模块目录新增。因此建议算法必然选中这些结构迁移。
- 只要能力映射包含上述两个 tracked commit 和 checks，历史样本必然选择 Dashboard 与 UsageView 两组检查，不依赖 commit subject 的中文关键词。

### 无法仅靠 Git 保证的下界

本地 Git 只能观察提交图、路径、blob 与文本差异，不能从任意合并结果自动证明业务语义等价。两个文件可以零交集却通过 API/动态 key 发生跨文件契约破坏，也可以有大量交集但行为完全等价。因此任何只靠 diff/冲突数的算法都无法保证不出现静默回归；至少需要一个记录下游能力边界的映射，以及能观察业务结果的探针或行为测试。

本样本已经给出匹配反例：Git 成功产生 `2c503a333`，类型/字段仍在，但生产者和动态槽位契约分别失配。故最小充分门禁不是“preflight 无高风险”，而是“preflight 选中的每个高风险能力都有结果探针且其行为测试通过”。

### 预期误报

- 大文件格式化、代码生成或纯移动会产生高删除比例和大量同目录新增；会被 split 启发式选中。证据输出和能力映射用于快速降级，而不是自动拒绝合并。
- `-C20% --find-copies-harder` 召回高但可能把共享样板误认为 copy；必须与同目录/stem、删除比例组合使用。
- 一个提交触达某文件不代表其所有功能都受上游该文件的任意改动影响。commit-path 交集是保守候选，行为探针决定是否真正阻断。
- `B -> D` 的 995 个路径含长期二开和生成产物，宽目录规则会选出过多测试。应优先使用 tracked commit + 精确路径，目录规则只作兜底。
- 二进制、生成代码、submodule 和 mode-only changes 不适用行数阈值，需要按对象类型分别报告。

### 预期漏报

- 上游在完全不同路径改变 API、schema、动态字符串或运行时注册关系，且能力映射未声明这些依赖时，纯路径算法可能漏报。
- 拆分后的每个新文件与源文件相似度都低于 20%，且文件名/目录也全部改变时，Git 启发式无法可靠建立谱系。
- 测试被选中但断言只检查字段存在或 mock 数据，仍可能像历史 UsageView test 一样给出假阴性。

降低漏报的正确方向是持续维护少量稳定的能力映射和行为测试，不是无限降低 copy 阈值或运行全部测试。后者会显著增加误报与耗时，却仍不能弥补缺失的业务 oracle。

## 对实现任务的直接建议

1. 首版优先实现 refs/merge-base、NUL-safe diff、commit side classification、精确交集、rename/delete 和上述 split 启发式；不要在首版尝试通用 AST 语义分析。
2. 用仓库内声明式映射登记 `dashboard-operational-metrics` 与 `usage-latency-column`，将检查原因一起输出，避免脚本中散落 commit/hash/path 条件。
3. 检查执行器按能力去重命令，并行运行互不依赖的前端与后端检查；单个测试命令显式设置不超过 60 秒的 timeout。
4. 将 `5a18ff2bb` 中的 Dashboard fallback 与用户 UsageView 渲染回归测试合入目标基线后，再把两项设置为阻断门禁；否则工具只能正确报警/选测，无法对旧测试缺口做出强保证。
5. CI 全量检查只在定向门禁通过后运行一次。preflight 的目标是缩短失败反馈路径，不是取消最终全量验收。
