# 智力雷达最新数据源验收

本地实现、提交和 OVH 发布均已完成。代码提交 `eb5bff47e499` 已快进本地 custom/main；生产镜像 `sub2api-custom:eb5bff47e499` 为 healthy，`/health` 返回 ok。未推送 Git。

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

## 已交付与回滚

已提交 `fix(monitor): 接入最新雷达成绩并动态展示来源模型`（`eb5bff47e499`），范围为 4 个后端文件、6 个前端实现/测试/文案文件、2 个规范文件和本任务记录。上次部署 journal、归档和其他本地未跟踪工作不混入本次代码提交。

已快进本地 custom/main，从固定提交的 git archive 干净快照本地构建 linux/amd64 release 镜像并加载 OVH，只切换 sub2api 应用容器，不推送 Git。此次代码未新增数据库迁移，启动日志没有新迁移执行；回滚恢复 override 备份并切回健康镜像 `sub2api-custom:8fec104efb1f`。数据库/Redis/反代等维持当前运行实例。

因用户要求不推送，标准部署脚本的 origin/custom/main 对齐门禁不适用；发布时使用固定本地 commit、干净 git archive 快照和仅切应用的等价检查，记录 wrapper/runtime Dockerfile 差异，禁止伪造远端 refs。OVH 不执行构建。

## 生产最终验收

- 当前镜像：`sub2api-custom:eb5bff47e499`，镜像 ID：`sha256:1993f021886731dff5019c0b03f7174539cb149ca28e0a0626051d7bc437fde7`，本地/加载/运行一致；版本输出为 0.2.16 / eb5bff47e499。
- Docker health 为 healthy、restart_count=0；公网 health/公开设置正常，未鉴权雷达接口返回 401。
- 9 个公网 JS/CSS 资源（含 3 个雷达相关资源）的内容及 SHA256 与发布产物一致。
- 使用现有已登录 Dia 标签页验证实际生产雷达：6.1 Sol、6 Astra、6 Sol/Luna、5.6 Sol/Luna 可见，真实 10 月 14 日判分日可见，覆盖/暂定状态及新评分口径正确。页面本次提示 1/34 档位读取失败；未把请求失败解释为没有成绩，也未据此推算全部有成绩系列数。
- 其余 7 个容器的 ID/镜像/启动时间/重启计数/健康状态相同；应用环境变量 hash 相同；.env 与主 Compose 的 hash/权限相同。override 仅镜像行变化，权限 0600。
- 启动日志 fatal/error、旧 efficiency 采集错误、新迁移应用记录均为 0。应用启动早期首次健康连接重置，自动重试成功。
- override 备份：`/opt/sub2api-backups/docker-compose.override.predeploy-sub2api-custom_8fec104efb1f-to-eb5bff47e499-20261014-204820.yml`。旧镜像保留。
- 标准部署脚本使用临时固定提交副本；因为用户不推送，用固定 SHA/干净快照替代远端对齐门禁，并明确只执行 `docker compose up -d --no-deps --pull never sub2api`。差异见 deploy-wrapper.diff / runtime-dockerfile.diff。OVH 未执行构建命令。

发布与界面证据见 release-build-evidence.json、release-validation.json、production-ui-validation.json、server-before.json、server-after.json、deploy.log。没有读取或导出浏览器私有凭据。

生产旧镜像和 override 备份保留用于回滚；本次无用的本地临时源码、二进制、构建日志和 wrapper 在 hash 记录后清理。原有未跟踪工作和上次部署 journal 保留，不混入本次代码或归档提交。因 journal 原先已有未提交内容，本次使用 add_session.py --no-commit 追加记录，避免自动提交夹带先前记录。
