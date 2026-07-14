# 下游二开 Fork 工作流

> 适用范围：本仓库作为 `Wei-Shaw/sub2api` 的下游二开版本时，所有代码、文档、部署模板和配置模板变更都必须遵循本工作流。

## 核心原则

本仓库采用“上游干净镜像 + 下游二开主线”的管理方式：

- `upstream` 指向原项目：`https://github.com/Wei-Shaw/sub2api.git`。
- `origin` 指向二开仓库：`https://github.com/blessonism/sub2api.git`。
- `main` 保持接近官方主线，用于同步 `origin/main` 与 `upstream/main`。
- `custom/main` 是长期二开主分支，所有业务定制都以它为基线。
- 具体开发使用 `feature/*` 分支，从 `custom/main` 拉出，完成后合回 `custom/main`。

## Trellis 任务要求

创建或接手任何会修改仓库内容的 Trellis 任务时：

1. 任务的 base branch 必须设置为 `custom/main`。
2. 任务工作分支必须从 `custom/main` 创建，命名为 `feature/<短功能名>` 或 `fix/<短问题名>`。
3. 任务的 `implement.jsonl` 和 `check.jsonl` 必须加入本文件，确保实现和检查阶段都遵守二开分支边界。
4. 如果任务只是同步官方更新，可使用 `sync/upstream-<version-or-date>` 分支。

推荐命令：

```bash
python3 ./.trellis/scripts/task.py set-base-branch <task-dir> custom/main
python3 ./.trellis/scripts/task.py set-branch <task-dir> feature/<short-name>
python3 ./.trellis/scripts/task.py add-context <task-dir> implement ".trellis/spec/guides/downstream-fork-workflow.md" "下游 fork 分支与上游同步规则"
python3 ./.trellis/scripts/task.py add-context <task-dir> check ".trellis/spec/guides/downstream-fork-workflow.md" "检查是否遵守下游 fork 分支边界"
```

## 开发前检查

开始写代码前必须确认：

```bash
git status --short --branch
git remote -v
git rev-list --left-right --count origin/main...upstream/main
```

判断规则：

- 当前应在 `feature/*`、`fix/*` 或 `custom/main`，不要直接在 `main` 上二开。
- `upstream` 的 push 地址应保持禁用或不可用，避免误推原项目。
- 若 `origin/main` 与 `upstream/main` 出现差异，先判断是官方更新、fork 未同步，还是错误提交到了 `main`。

## 跟进官方更新

上游同步必须在 `sync/upstream-*` 分支完成。预检、冲突解决和定向检查通过后，再通过以 `custom/main` 为目标的 PR 合入；不得直接在 `custom/main` 上试合并。

### 一次性启用 rerere

经仓库维护者确认后，在本仓库启用冲突解决复用，但禁止 Git 自动暂存复用结果：

```bash
git config --local rerere.enabled true
git config --local rerere.autoupdate false
git config --local --get rerere.enabled
git config --local --get rerere.autoupdate
```

预期值依次为 `true`、`false`。`rerere` 只复用历史冲突解法；每个自动应用的结果仍需人工复核并显式 `git add`。

### 1. 更新本地引用并预检

先显式拉取引用。不要使用 `git fetch --all --tags`：`origin` 与 `upstream` 存在同名但不同对象的 tag 时，批量更新本地 tag 会直接失败，并让操作者误以为远端分支没有更新。同步分支只需要分支 refs，安全做法是禁用 tag 跟随并分别更新两个 remote：

```bash
git fetch --no-tags --prune origin
git fetch --no-tags --prune upstream
```

这两个命令不会创建或改写本地 tag。确实需要验证官方版本 tag 时，应在本流程之外显式获取单个已确认 tag，且先核对本地同名 tag 指向；预检和 merge 仍必须使用同一个已解析 ref。

然后仅基于本地 refs 执行只读预检：

```bash
python3 tools/upstream_sync.py preflight \
  --base custom/main \
  --upstream upstream/main
```

预检报告必须确认双方 merge-base、直接重叠、删除/重命名/拆分风险、受影响的下游能力和建议检查。高风险不等于禁止创建同步分支，但缺少对应行为检查时不得合入 `custom/main`。

### 2. 创建同步分支并集中解决冲突

从最新的 `custom/main` 创建同步分支，再合入已经预检的同一个上游 ref：

```bash
git switch custom/main
git switch -c sync/upstream-<version-or-date>
git merge upstream/main
```

如果官方发布了稳定标签，可将预检和合并命令中的 `upstream/main` 同时替换为该标签。不要预检一个 ref 后再合并另一个 ref。

发生冲突时先检查 rerere 建议和完整差异：

```bash
git rerere status
git rerere diff
git status --short
git diff --check
```

`rerere.autoupdate=false` 时，复用结果不会自动进入暂存区。必须逐文件确认下游能力仍有数据生产者、消费端和行为测试，再显式暂存并完成 merge commit。

### 3. 运行定向检查

先生成合并结果的检查计划，再执行命中的检查。检查命令由 `tools/upstream_sync_checks.json` 统一维护，文档、Makefile 和 CI 不复制具体测试清单：

```bash
python3 tools/upstream_sync.py check \
  --base custom/main \
  --head HEAD \
  --dry-run \
  --format json \
  --output /tmp/sub2api-upstream-sync-plan.json

python3 tools/upstream_sync.py check \
  --base custom/main \
  --head HEAD \
  --jobs 3
```

前端、后端和 integration lane 可以并行运行；同一检查只能执行一次。integration 环境不可用时必须记录为未验证，不得用 mock handler/view 测试代替真实 repository integration 测试。

预检中的 `estimated conflict candidates` 是双方规范化路径交集数，用作人工处理量的保守上界，不等同于 Git 最终报告的文本冲突数。每个选中检查同时输出命中能力、路径或 fallback 原因以及矩阵中的实际 argv；未登记的交叉路径优先落到同 lane 的通用检查，高风险路径连通用检查也没有时预检返回非零，普通文档交叉只提示人工复核。

#### 高频冲突复核手册

- **i18n overlay**：上游把 `locales/{en,zh}.ts` 拆成目录时，将二开文案放回对应模块或 `custom.ts`，不要恢复已经删除的聚合文件；复核 key collision、overlay 优先级以及中英文 key 等价。
- **仓储拆分**：源 repository 大幅缩减或拆成同 stem 文件时，按“公开 DTO -> handler/service -> repository 生产者 -> integration 行为断言”逐项追踪。字段仍能编译或前端卡片仍存在，不能证明 SQL 聚合生产者仍在。
- **路由**：合并共享的 `routes/admin.go`、`routes/user.go` 或前端 `router/index.ts` 时，同时检查路由注册、鉴权 middleware、旧 alias 和 lazy import；不得只以 handler/view 文件存在作为验收。
- **DTO 与动态 key**：字段重命名必须同时核对 JSON tag、TypeScript 类型、表格 column key 和动态 slot 名。`key: 'latency'` 之类的列必须存在同边界的 `cell-latency` 或明确委托给提供该 slot 的公共组件。

这些手册只规定复核顺序；测试 argv 仍只在 `tools/upstream_sync_checks.json` 维护。

### 4. 通过专用门禁和完整 CI

获得 `git push` 的明确确认后，推送 `sync/upstream-*` 并创建目标为 `custom/main` 的 PR：

- `.github/workflows/downstream-sync.yml` 生成检查计划，并只运行命中的 frontend、backend、integration lane；job summary 记录选测原因和各 lane 耗时。
- 三个快速 lane 成功后，专用 workflow 通过 reusable workflow 调用现有 `.github/workflows/backend-ci.yml` 一次；完整测试、lint 和 integration 定义仍只保留在现有 CI，不复制到专用 workflow。
- `backend-ci.yml` 的普通 push/PR 触发会跳过 `sync/upstream-*`，避免快速门禁前并发重复运行完整套件；其他分支行为不变。
- 定向检查和现有完整 CI 均通过，且高风险能力已人工复核后，才能合回 `custom/main`。

如需维护官方镜像分支，在同步任务之外将 `main` 快进到同一个上游 ref：

```bash
git switch main
git merge --ff-only upstream/main
```

`main` 只保持官方镜像，不承载冲突解决或业务二开提交。

### CLI 安全边界

`preflight` 只读取调用时已经存在的本地 refs 和 Git 对象；`check` 基于同一批本地信息选择检查，并在非 dry-run 模式下启动矩阵中声明的测试命令。两者都不会执行 `fetch`、`merge`、`checkout`、`switch`、`reset`、`commit` 或 `push`，也不会自动解决冲突。需要更新 refs 或产生 merge commit 时，必须由操作者在命令外显式执行。

命令签名、报告字段、退出码、fallback 和必测场景以 [上游同步快车道可执行契约](../tools/upstream-sync-fast-path.md) 为准。

除 stdout 外，CLI 自身只会在显式传入 `--output` 时写入报告文件；被执行的测试仍可能生成正常的构建缓存或临时文件。CLI 不会修改索引、分支或 Git 对象数据库。因此报告中的 `upstream/main` 是否最新，取决于此前独立执行的 `git fetch`。

### 回滚顺序

按同步所处阶段选择最小影响的回滚方式：

1. merge 尚未提交：执行 `git merge --abort`，回到同步分支合并前状态；`custom/main` 不受影响。
2. merge 已提交或同步分支已共享：在同步分支使用 `git revert -m 1 <merge-commit>` 创建反向提交，不改写共享历史。
3. 已合入 `custom/main`：回退对应 PR 或 merge commit，重新通过完整 CI；禁止对共享的 `custom/main` 使用强制重置。
4. 仅需停用 rerere：执行 `git config --local --unset rerere.enabled` 和 `git config --local --unset rerere.autoupdate`。这不会修改提交历史、工作树或已经产生的冲突解决结果。

## 二开实现边界

为了降低后续跟进官方版本的冲突成本：

- 优先通过配置、环境变量、部署模板或独立模块实现定制。
- 必须改核心逻辑时，保持提交小而清晰，并在任务 PRD 或实现记录中写明原因。
- 不要把生产服务器的真实 `.env`、密钥、数据库备份、运行时数据提交到仓库。
- 本地仓库只管理代码、文档、配置模板、迁移脚本和可复现的部署说明。

## 高风险操作

以下操作必须在任务记录或聊天中获得明确确认后再执行：

- `git push`
- `git commit`
- 改动生产数据库或执行迁移
- 重建生产容器或切换生产流量
- 修改 DNS、Cloudflare、反代或服务器防火墙

确认前必须说明操作类型、影响范围和回滚方式。

## OVH 生产部署边界

OVH 生产机资源有限，部署下游二开版本时必须遵守：

- 禁止在 OVH 生产机上执行 `docker build`、`pnpm run build`、`pnpm exec vite build`、`go build` 等构建命令。
- 构建必须在本地工作站或 CI 完成，镜像标签使用 `sub2api-custom:<12位commit>`。
- OVH 只允许加载/拉取已构建镜像、备份 override、切换 `sub2api` 应用容器、健康检查和回滚。
- 默认使用 `deploy/ovh-safe-deploy.sh`；如必须偏离该脚本，需在任务记录中说明原因和等价安全措施。
