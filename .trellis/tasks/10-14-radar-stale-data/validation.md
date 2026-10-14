# 智力雷达最新数据源验收

本地实现与最终检查通过，用户已确认提交和生产发布，正在执行。OVH 只读复查仍为 `sub2api-custom:8fec104efb1f`、healthy，`/health` 返回 ok。本任务尚未重启生产应用或改动生产数据。

## 最终行为

按用户“看他给的数据有哪些，就展示哪些”的要求，从公开 binding 动态发现全部模型和档位，再读取现行 RadarBench summary。有有效成绩就展示，不固定型号，也不主动排除 5.5/5.6；0 分保留，null 成绩不展示。

新评分满分 150、任务数 64。每个系列展示原分数、覆盖及暂定/完整状态；覆盖不是通过题数。日期使用真实 UTC 判分日，采集时间单独标为“读取于”。历史不可用时判分日留空。旧 IQ 数据不用于 runtime 补齐、降级或拼接新趋势，旧 DTO/JSON parser 与题库管理保留兼容。

部分失败保留成功成绩并提示 failed/requested，一分钟后允许重试；完整成功缓存一小时。429 尊重 Retry-After 并停止后续采集。单请求 8 秒、整批 20 秒、并发 3、响应 512 KiB，目录最多 256 个配置，显式检测超限并校验评分及历史口径。

## 真实源证据

2026-10-14 从 OVH 只读抓取 34 个配置，29 个成功有成绩、5 个超时，耗时 17.38 秒。成功数据包含 6 个模型：GPT-6.1 Sol、GPT-6 Astra、GPT-6 Sol/Luna、GPT-5.6 Sol/Luna。失败请求不能被解释为没有成绩。

真实捕获通过当前服务完整采集和归一化路径回放，得到 29 个系列、5 个失败、缓存 60 秒。各系列最新判分日分布：

| UTC 判分日 | 系列数 |
| --- | ---: |
| 2026-10-11 | 1 |
| 2026-10-13 | 22 |
| 2026-10-14 | 6 |

这证明存在比 10 月 1 日新的成绩，未声称所有档位都已在 10 月 14 日重测。公开捕获必要字段、失败与原始 hash 见 `research/radar-bench-live-source.json`；回放见 `live-radar-bench-replay.json` 和 `unit-radar-bench.txt`。临时真实回放测试已删除，长期回归保留有意义的契约断言。

## 最终检查

| 检查 | 结果 |
| --- | --- |
| 后端定向 unit | 18 个常驻主测试通过，另一次临时真实源回放通过 |
| golangci-lint 2.14.0 | 0 issues；unit tags，基于 custom/main，两个新增 Go 文件已纳入 diff 后复查 |
| 前端定向测试 | 4 个文件、49 个测试通过 |
| i18n completeness | 3 个测试通过；新增 zh/en 文案同步 |
| 前端 typecheck / 改动文件 ESLint | 通过 |
| 前端 production build | 通过，新资源生成于 ignored 的 backend/internal/web/dist |
| Linux AMD64 后端 embed build | 最终源码构建通过，构建证据见 build-radar-bench.json |
| 规范与跨层复核 | 后端/前端索引、动态范围、DTO、日期、缓存、UI 与错误状态已复核 |
| 格式 / diff | 通过 |

本轮未重复无关的完整测试套件。最终定向日志为 `unit-radar-bench-final.txt`、`lint-radar-bench.txt`。前端结果保留于执行阶段终端输出；生成产物 hash 记录于 `build-radar-bench.json`。源码及持久证据 hash 见 `validation-evidence.json`。

## 根因与已替代方案

2026-09-17 的动态模型优化 `b08200006` 已合入当前生产版本，未丢失。旧 efficiency JSON 实测 9,695,083 字节，超过旧 8 MiB 上限，且已有系列不会更新；降级 current.json 后留下旧日期。只提高上限仍只能得到 10 月 1 日静态快照，因为原站已换成 RadarBench。

早期旧接口回归、回放和 lint 日志仅保留为调查证据（`regression-before.txt`、`unit-after.txt`、`lint-after.txt`、`live-replay.txt`），不是最终实现验收。最终方案已接入新公开入口。

## 拟交付与回滚

拟提交 `fix(monitor): 接入最新雷达成绩并动态展示来源模型`，范围为 4 个后端文件、6 个前端实现/测试/文案文件、2 个规范文件和本任务记录。上次部署 journal、归档和其他本地未跟踪工作不混入本次代码提交。

确认后快进本地 custom/main，本地构建 linux/amd64 镜像并加载 OVH，只切换 sub2api 应用容器，不推送 Git。此次代码未新增数据库迁移，应用会短暂重启；回滚恢复 override 备份并切回健康镜像 `sub2api-custom:8fec104efb1f`。数据库/Redis/反代等维持当前运行实例。

因用户要求不推送，标准部署脚本的 origin/custom/main 对齐门禁不适用；发布时使用固定本地 commit、干净 git archive 快照和仅切应用的等价检查，记录 wrapper/runtime Dockerfile 差异，禁止伪造远端 refs。OVH 不执行构建。
