# Design

## Data Model

新增 `user_group_account_bindings`：`user_id`、`group_id` 组成主键，`account_ids BIGINT[]` 非空，`fallback_to_group BOOLEAN`，以及时间戳。用户和分组使用外键级联删除；账号软删除或移出分组时由候选集交集自然失效。

## Service Contract

共享解析器按 `(user_id, original_group_id)` 读取绑定并缓存；管理员保存后立即失效对应用户缓存。读取错误不缓存并向调度层返回错误。

管理员用户详情增加 `group_account_bindings`。更新请求字段省略时不修改，空对象时清空全部，非空对象按用户整体替换。保存前验证分组类型及账号归属。

## Scheduling

调度从 `ctxkey.UserID` 获取可信用户 ID，并在任何内部 fallback group 解析前保存原始分组 ID。所有候选路径统一应用绑定白名单，非绑定粘性账号和模型路由账号视为不可用。

严格模式只运行白名单候选流程。回退模式先运行白名单流程；若结果未立即取得并发槽位或无候选，则排除绑定账号后运行原分组流程，避免再次等待同一批账号。

## Frontend

弹窗打开时并行读取用户详情和分组。公开分组可开启账号限制，账号通过现有账号分页接口按分组懒加载；必须至少选择一个账号。策略使用严格/回退单选控件。
