# Model IQ Probe（降智检测 / 糖果问题探针）契约

> 状态：Active。探测、真实记录与用户展示分别处理，不参与请求调度。

## 配置与默认值

settings key `model_iq_probe_config`（JSON），改配置不需要重启：

- `enabled=false`，`question` 默认糖果题，`expected_answer="21"`。
- `interval_minutes=360`，合法范围 5～10080；实际定时附加 ±10% 抖动。
- `target_mode=account_model`（账号 × 模型）或 `model_only`（每模型选择第一个支持账号）。
- `models=[]` 时从上游目录发现 `gpt-*` 文本模型，排除 `gpt-image-*`；`account_ids=[]` 使用全部可调度的活跃 OpenAI 账号。
- `visibility=admin_only` 或 `all_users`，缺失字段默认 `admin_only`。
- `user_display_mode=real` 或 `always_green`，缺失字段默认 `real`。
- `show_exclusion=false`，开启后为答错结果附加 `marked_for_exclusion` 展示标记。
- 普通用户可见范围与定时探测启停独立；管理员始终读取真实结果。

## 判分与执行

`ValidateModelIQAnswer` 保留“出现即通过”的既有约定：整数期望以 `-?\d+` 提取匹配，其余使用大小写不敏感的包含匹配。“不是 21 而是 20”仍通过，由管理员人工改判纠正。生效状态为 `override_status ?? status`。

- Chat Completions、Responses 和 OAuth 探测均必须发送配置的 `question`；连接测试无自定义题目时才使用 `hi`。
- 并发上限为 3。账号测试会更新 Extra 和内部缓存，每个目标须在启动并发之前获得独立账号快照及深拷贝的 Credentials / Extra。
- 单轮超时 40 分钟；leader lock key `model_iq_probe:runner:leader`，TTL 45 分钟；进程内 atomic 防重入。
- 写库失败计入 Error，返回错误与部分汇总；取消不能报告整轮成功。取消后不继续派发新目标。
- 结果表 `model_iq_probe_results` 保留 30 天，OpsCleanupService 每日物理清理。
- `actual_answer` ≤4KB，`error_message` ≤2KB；当前连接测试通道无 usage，token 三列为 NULL。

## 管理端 API 与后台任务

- `GET/PUT /api/v1/admin/model-iq-probe/config`：读写配置，保存后 Reload 调度。
- `GET /api/v1/admin/model-iq-probe/results`：model/account_id/effective status 过滤、分页。
- `POST /api/v1/admin/model-iq-probe/results/:id/override`：pass/wrong_answer/error/null。
- `POST /api/v1/admin/model-iq-probe/run`：校验启用、获取互斥锁并登记任务后立即返回 `ModelIQProbeRunState`，不再同步等待整轮汇总；重复触发返回 409，禁用返回 400。
- `GET /api/v1/admin/model-iq-probe/run`：读取相同任务状态，包括 id、status、started_at、finished_at、summary、error_message。
- 任务状态写 settings key `model_iq_probe_run_state`，各应用实例共享。任务继承应用生命周期，不继承触发 HTTP 请求的取消信号。
- 状态为 idle/running/completed/failed；每完成一个目标更新进度，写库或进度持久化失败标为 failed。应用正常退出取消并等待后台任务；终态写入在限定时间内最多重试 3 次；若仍遗留 running，状态查询仅在拿到全局执行锁且本地没有活跃任务后将其恢复为 failed。新任务启动以活跃执行标记与全局锁为准，不被旧 running 字段锁死。超过单轮超时加一分钟仍保留兜底失败判定。
- 前端运行中每 2 秒查询；页面重开恢复进度，完成后刷新结果。启动收到 409 或丢失响应时重新查询共享任务状态；临时查询失败继续恢复轮询；区分旧任务终态与新完成任务，避免把历史完成当成本次成功。禁用时不能点击“立即探测”。

## 用户端 API 与展示投影

`GET /api/v1/channel-monitors/iq-probe` 返回 `{items: [{model, latest, recent}]}`，角色只读取 JWT middleware 写入的可信 context。

- 未启用、无数据、普通用户遇到 `admin_only` 时返回空列表；私有模式不查询结果仓储。
- 不输出 account_id/account_name。回答摘要 ≤120 字符。
- 展示与执行共用精确目标选择规则，剔除已停用、已删除、未选中和不再支持对应模型的账号；`model_only` 只保留每模型当前选中账号。自动发现模式从当前账号的本地历史读取模型名，不在用户请求中访问上游目录。仓储在 SQL 窗口计算前按精确 `(account_id, model)` 对筛选，再保留每目标最新记录和每模型最近 60 条时间线，限 30 天；既不能用全局 LIMIT 挤掉最新失败，也不能按每个账号全量加载 60 条历史。
- 每模型当前状态取各账号最新生效结果中的最严重状态：wrong_answer > error > pass。旧失败被同账号新结果替代，不被其他账号的一次通过覆盖；回答和时间取对应代表记录。时间线为模型最近 60 条、旧→新。
- `always_green` 仅给普通用户把当前和历史状态投影为 pass，并移除回答摘要；数据库、管理员明细和管理员用户端视图保留真实结果。
- `show_exclusion=true` 时只为真实 wrong_answer 附加 `marked_for_exclusion=true`（包括常绿投影），error 不标记。文案“已标记剔除 / Marked for exclusion”，标记可显示于当前状态与历史悬浮提示。
- 标记不修改账号启用、权重、限流或调度状态。页面仅描述模型抽样检测，不声称分组全量保证。

## 验证

- 服务回归覆盖三类协议实际题目、并发快照隔离、写库失败、后台请求取消/重复触发/失败进度、默认私有、角色与常绿投影、跨账号失败保留及恢复、模式切换与退出范围目标过滤、终态写入重试与遗留任务恢复、其他实例活跃任务不被误判。
- handler 回归覆盖可信角色、查询参数无法越权、私有模式不读结果。
- repository SQL 回归覆盖分区查询、筛选参数、完整扫描和错误传递；真实 PostgreSQL 验证使用 integration 测试，不以 SQL mock 代替。
- 前端覆盖设置保存、异步轮询/重开/失败/卸载、剔除标记、API 字段解析、zh/en key 等价及类型检查。
