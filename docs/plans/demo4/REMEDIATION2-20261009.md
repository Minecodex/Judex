# Demo4 第二次复核修复

本文保留 R5／R6 修复时的验收与部署快照，当前 R7 修复和常用 revision 19 发布见[第三次修复与本地发布](./REMEDIATION3-20261009.md)。

2026-10-09，用户授权完善 [R5、R6](./REVIEW2-20261009.md)。两项已关闭，同一最终镜像完成业务与真实模型验证；复核前的证据保留，问题复现用例改为期望行为断言。

| 项目 | 实现 | 验证 |
| --- | --- | --- |
| R5 外部任务导航 | 任务选择只接受对象 ID；TaskDrawer 使用共享 useTaskDetail 按项目／用户查询，统一卡片、外部依赖及概览入口。流程参考的“打开来源”使用共用 objectNavigation，在查看器内退出后导航，讨论内只切换右栏；两种交互都不依赖缓存命中。保留预览只读、原画布和来源焦点，补齐实际所属计划。 | 真实 API 验证冷／热缓存、全屏预览、普通路线、讨论中的任务来源；确认任务与计划标题、详情读取、只读动作隐藏、关闭焦点、中央会话／发言任务／草稿保留。临时查询失败后同 ID 重试成功；D4-B07 回归原来源退出与独立任务详情。 |
| R6 阻塞标识 | taskDisplayStatus 集中计算显示状态，路线、标题与计划上下文共用；只有 ready 且开始硬前置未满足显示 blocked。原业务阶段、版本、统计、正式命令权限及校验保留。 | 真实 API 验证三处同任务显示等待前置，开始禁用，数据库仍 ready；逻辑测试覆盖服务端 satisfied、未缓存前序、开始／验收／双阶段、软条件、跳过和丢弃。 |

当前 38 项浏览器回归通过，含原 Demo4／R1～R4、12 组尺寸／语言／主题矩阵和 R6 状态一致性。前端逻辑 63 项通过，TypeScript 通过。真实 API 的 R5／R6 与查询失败重试分别验证，PostgreSQL／对象存储备份恢复完成。

源码沿用共享 HeroUI、样式 token、正式查询和事件刷新；没有改动后端业务阶段，也没有改变审批、交接、验收及 AI 权限边界。工作区已有其他任务的改动保留，差异检查中无关设计文档末尾空行未作为本轮修改。

证据：`.cache/demo4-review2/repair-{browser,web,status,api,retry}.log`；真实 API 场景 `.cache/e2e/judex-e2e-19392-1791509287996/`，重试场景 `.cache/e2e/judex-e2e-22504-1791509640794/`。新图 `.cache/demo4-review2/consistent-blocked-inspector.png`，外部任务与状态 API 图片在各运行的 business-results 目录。对照页 `.cache/demo4-review2/compare.html` 使用实际对应原型图，仍保留 9 位置 × 12 组合。

Go 全量测试及 go vet 通过。首轮隔离环境 judex-ui-ed97abd5 中 23 项业务通过，D4-B07 发现流程来源跳转误用预览内选中回调，旧查看器没有退出。保留失败证据，修复为稳定的对象来源导航，不放宽旧断言；该所属命名空间已清理。

## 最终制品与隔离验收

最终镜像 `judex/server:collaboration-demo4-20261009014431-322f0a65`，版本 `0.1.0-demo4.20261009014431`，镜像摘要 `sha256:e3ed998ba9a103f8d0cf33a7c95a22a1d6c3f5591b6b0bcb21f27ae3678fe500`。源码哈希 `322f0a65cb75e388d0114ad91c90de52b2a809de054278b47188b30d76efd7c2`；最终核对 583 个输入文件无漂移，458 个代码文件均不超过 2000 行，最大 1805 行。

临时命名空间 `judex-ui-e2c6e392` 通过全部 24 项真实业务：本轮 R5／R6／失败重试，既有 D4-B、C01、COOP-A／R2／R3、P05、T06、MAT06。覆盖双用户权限、正式修改会签、草稿撤审、丢弃、跳过／恢复、人工验收及 CLI／Agent 上下文。随后同镜像 GLM-5.3 C02 通过：1 次职位运行、8 次模型调用、2 次资料读取、1 次公开任务分析工具成功，没有自动批准。

证据 `.cache/k8s/judex-ui-e2c6e392/{manifest,real-model-proof}.json` 与独立的 business-results／real-model-results；命令日志 `.cache/demo4-review2/repair-k8s-final.log`。最终 exitCode=0、cleanedUp=true；原部署 UID／副本数核对无变化，本轮未缩容或发布常用部署。

最终浏览器结果 `.cache/demo4-review2/repair-browser-final.log` 为 38 项通过，前端逻辑 63 项、Go 全量测试及 go vet 通过。已有对照页使用本轮更新后的正式截图与独立原型同位置图，9 位置 × 12 组合；原 R1～R4 不回退。

常用 judex 部署当前为 `judex/server:local-materials-20261009084429-3ac0226b`，由工作区同期的资料工作更新，未包含本轮 Demo4 验收镜像。本轮完成的是重构源码及隔离制品验收，常用部署更新继续作为独立发布步骤。
