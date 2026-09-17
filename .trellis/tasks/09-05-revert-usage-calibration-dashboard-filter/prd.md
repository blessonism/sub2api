# 撤销使用记录校准并排除余额校准仪表盘统计

## Goal

管理员可撤销单次使用记录校准；余额校准不进入使用记录仪表盘聚合，避免零 Token 却产生扣费的误导。

## Requirements

- 使用记录校准历史中的每条未撤销记录必须提供“撤销”操作。
- 撤销必须保留原校准审计记录，并记录撤销时间及执行管理员。
- 撤销余额相关校准时，目标用户余额必须按原校准影响反向恢复；已撤销记录不可重复撤销。
- 所有使用记录与使用仪表盘的实际成本、用户排行聚合不得把余额校准计入，避免出现 `0 token` 但产生扣费的展示。
- 已撤销校准不得继续影响 Token 分摊、余额消费统计、用户排序等仍依赖校准数据的聚合。
- 撤销失败、目标用户不存在、余额恢复会导致非法负余额等情况必须返回明确错误，不能静默成功。

## Acceptance Criteria

- [ ] 管理员可从校准历史撤销一条未撤销记录，界面显示成功反馈并刷新历史与用户余额。
- [ ] 已撤销记录显示撤销状态且撤销按钮不可用；重复调用接口返回冲突/业务错误。
- [ ] 撤销包含余额影响的校准后，用户余额恢复到校准前值，缓存同步失效。
- [ ] 使用记录列表/统计和仪表盘统计中，余额校准不再增加 `total_actual_cost` 或同义实际成本字段。
- [ ] 已撤销校准不再参与 Token 分摊、余额消费统计、排行榜与用户排序聚合。
- [ ] 后端定向测试、前端 UsageView 测试和类型/静态检查通过。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
