## Sub2API 二开协作补充

- 本仓库是 `Wei-Shaw/sub2api` 的下游二开仓库；所有会修改仓库内容的任务都必须先读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- `main` 只用于保持接近官方主线，业务二开默认从 `custom/main` 拉出 `feature/*` 或 `fix/*` 分支。
- Trellis 任务进入实现或检查阶段前，应把 `.trellis/spec/guides/downstream-fork-workflow.md` 加入对应任务的 `implement.jsonl` 和 `check.jsonl`。
- 回答用户时使用简体中文；代码注释和项目文档优先使用简体中文。

<!-- TRELLIS:START -->
# Trellis Instructions

These instructions are for AI assistants working in this project.

This project is managed by Trellis. The working knowledge you need lives under `.trellis/`:

- `.trellis/workflow.md` — development phases, when to create tasks, skill routing
- `.trellis/spec/` — package- and layer-scoped coding guidelines (read before writing code in a given layer)
- `.trellis/workspace/` — per-developer journals and session traces
- `.trellis/tasks/` — active and archived tasks (PRDs, research, jsonl context)

If a Trellis command is available on your platform (e.g. `/trellis:finish-work`, `/trellis:continue`), prefer it over manual steps. Not every platform exposes every command.

If you're using Codex or another agent-capable tool, additional project-scoped helpers may live in:
- `.agents/skills/` — reusable Trellis skills
- `.codex/agents/` — optional custom subagents

Managed by Trellis. Edits outside this block are preserved; edits inside may be overwritten by a future `trellis update`.

<!-- TRELLIS:END -->
