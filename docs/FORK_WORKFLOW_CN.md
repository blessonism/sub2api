# Sub2API 二开分支管理说明

本文档记录本仓库作为 `Wei-Shaw/sub2api` 下游二开版本时的 Git 管理方式。

## 远程仓库职责

- `origin`：你的 GitHub 仓库，地址为 `https://github.com/blessonism/sub2api.git`。
- `upstream`：原开源项目仓库，地址为 `https://github.com/Wei-Shaw/sub2api.git`。
- `upstream` 只用于拉取官方更新，本地已禁用它的 push 地址，避免误推到原项目。

## 分支职责

- `main`：保持接近官方代码，用于同步 `origin/main` 和 `upstream/main`。
- `custom/main`：长期二开主分支，业务定制、配置模板、部署脚本和自定义功能都从这里开始。
- `feature/*`：具体二开功能分支，从 `custom/main` 拉出，完成后合回 `custom/main`。

## Trellis 集成

- Trellis 的长期规则入口是 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 任何会修改仓库内容的 Trellis 任务，都应把这份 guide 加入 `implement.jsonl` 和 `check.jsonl`。
- 任务 base branch 应设置为 `custom/main`，任务工作分支从 `custom/main` 拉出。

## 日常二开流程

```bash
git switch custom/main
git switch -c feature/your-change
```

开发完成后，优先合回 `custom/main`：

```bash
git switch custom/main
git merge feature/your-change
```

## 跟进官方更新

先拉取两个远程仓库的最新引用：

```bash
git fetch --all --tags --prune
```

确认你的 fork 和官方上游差异：

```bash
git rev-list --left-right --count origin/main...upstream/main
```

如果需要把官方更新合入二开分支：

```bash
git switch custom/main
git merge upstream/main
```

如果官方发布了明确版本标签，也可以按稳定版本合并：

```bash
git switch custom/main
git merge v0.1.137
```

## 生产数据边界

- 本地仓库只管理代码、文档、配置模板和迁移脚本。
- 服务器生产数据、密钥、数据库备份和真实 `.env` 不应提交到 Git。
- 升级生产前，先在测试环境或临时服务器验证容器启动、健康检查、数据库迁移和关键 API。
- 对生产执行 `git push`、数据库变更、容器重建、DNS 切换前，需要单独确认操作范围和回滚方案。
