# feature: 连接器展开分组概览

## Goal

管理员在上游倍率监控的连接器状态区域，可以展开某个连接器，直接查看该连接器下属分组的核心运行态，而不进入候选映射表或快照弹窗。

## Requirements

- 连接器行增加展开 / 收起按钮。
- 展开区不是表格，不展示表头。
- 展开区只读，不迁移候选映射操作按钮。
- 每个下属分组只展示：分组名称、倍率、健康状态、Priority、今日用量。
- 展开数据复用当前已加载候选映射数据，不新增后端接口。

## Acceptance Criteria

- [x] 点击连接器展开按钮后，能看到该连接器下的分组紧凑概览。
- [x] 展开区不出现候选映射操作按钮。
- [x] 展开区没有表头，只显示数据行。
- [x] 展示字段限定为分组名称、倍率、健康状态、Priority、今日用量。
- [x] 针对性前端测试通过。

## Out of Scope

- 不移动候选映射表中的编辑、启停、删除、探测等操作。
- 不新增或修改后端 API。
- 不改候选映射表主体结构。

## Technical Notes

- 预计修改 `frontend/src/views/admin/UpstreamRelayGroupMonitoringView.vue` 和对应测试。
- 当前仓库为 `Wei-Shaw/sub2api` 下游二开，base branch 为 `custom/main`。
