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

同步官方更新时先拉取引用：

```bash
git fetch --all --tags --prune
```

如果官方主线需要同步到 fork 的 `main`：

```bash
git switch main
git merge --ff-only upstream/main
```

把官方更新合入二开主线：

```bash
git switch custom/main
git merge main
```

如果官方发布了稳定标签，优先按标签合并到 `custom/main`：

```bash
git switch custom/main
git merge <official-version-tag>
```

合并后必须跑与变更范围匹配的检查，再考虑推送或部署。

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
