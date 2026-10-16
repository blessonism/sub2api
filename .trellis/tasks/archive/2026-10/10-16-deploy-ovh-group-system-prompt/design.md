# 设计和边界

用户已明确要求合并后部署到 ssh ovh。本次发布本地 custom/main 固定 SHA，不附带 Git push；远端 custom/main 落后，本次不为部署同步大量历史提交。

原工作树含此前未提交的 journal 和任务记录，保持原状。使用 git archive 固定 SHA 导出干净源码，构建不读取这些工作树修改。标准部署脚本的干净工作树/远端对齐门禁用固定提交、构建输入校验及本地分支校验替代，保留具体 unified diff。禁止伪造 origin 引用。

本地使用已有 lockfile 匹配的依赖编译前端、Go 交叉编译 linux/amd64；由于 Docker Hub 超时，运行时基于本机与生产 ID 完全一致的当前镜像，仅替换新程序与 revision 标签；资源及 entrypoint 在两提交间无变更，保存相对标准最终阶段的差异。仅本地 Docker 组装镜像。

部署脚本保留镜像加载、override 备份、失败回滚，并增加加载镜像 ID、生产旧镜像 ID/配置未变化校验，以及 --no-deps --pull never。只切换应用容器。

启动自动执行新增的 245_group_system_prompt_injection.sql。迁移为新增默认关闭的 JSONB 列和缓存失效触发函数更新。发布前数据库逻辑备份留在生产，校验 pg_restore 解析；应用回滚不自动恢复数据库，不覆盖后续业务写入。
