# 实施计划

| 计划 | 设计与合同 | 状态与证据 |
| --- | --- | --- |
| [V1 完整业务](v1/README.md) | 00～12 专项合同、任务依赖与 `progress.json` | [状态](v1/STATUS.md)、[记录](v1/records/README.md)、[验收摘要](v1/evidence/README.md) |
| [项目协同](cooperation-ui/README.md) | [设计](cooperation-ui/DESIGN.md)、页面契约、任务和 `progress.json` | [范围复核](cooperation-ui/STATUS.md)、[记录](cooperation-ui/records/README.md) |
| [Demo4 与计划任务](demo4/README.md) | [交互设计](demo4/DESIGN.md)、[运行例外](demo4/RUNTIME.md) | [最新复核](demo4/STATUS.md)、[对照验收](demo4/ACCEPTANCE.md)、[记录](demo4/records/README.md) |
| [共享资料](materials/README.md) | [正式设计](materials/DESIGN.md)、[原型说明](materials/PREVIEW.md) | [最新发布](materials/records/RELEASE-20261009.md)、[记录](materials/records/README.md) |
| [真实模型与 Kimi 联测](real-host/README.md) | [前置能力实现方案](real-host/DESIGN.md)、软件、图片、Office 与缓存观测的准入范围 | [综合场景覆盖与缺口](real-host/records/SCENARIO-GAPS-20261010.md)、[CLI / 缓存实施](real-host/records/CLI-CACHE-20261010.md)、[前置核查](real-host/READINESS.md) |

编制时源码快照不作为当前代码状态。每轮 `records` 中的数量、镜像和未执行项按原日期阅读；当前计划入口明确其覆盖范围，整套 V1 尚未关闭的门槛仍保留。

## 离线原型

在仓库根目录执行构建和预览命令。HTML 与所属计划放在一起，HTTP 路径保持不变。

| 原型文件 | 构建 | 预览 |
| --- | --- | --- |
| [项目协同](cooperation-ui/demo.html) | `node web/cooperation-preview/build.mjs` | `node tests/cooperation-preview/server.mjs`，5195 |
| [共享资料](materials/demo3.html) | `node web/material-preview/build.mjs` | `node tests/material-preview/server.mjs`，5197 |
| [计划任务](demo4/demo4.html) | `node web/task-route-preview/build.mjs` | `node tests/task-route-preview/server.mjs`，5396 |

原型只使用示例／页面本地状态，正式与 demo 业务体验继续共用 `web/` 中的组件树。原型运行不证明服务器事务、模型或 Kubernetes 已通过验收。
