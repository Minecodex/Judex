# 实施计划

| 计划 | 设计与合同 | 状态与证据 |
| --- | --- | --- |
| [V2 群聊协作与界面重构](v2/README.md) | [合同](v2/CONTRACTS.md)、[实施状态](v2/IMPLEMENTATION.md)、[完成度复核](v2/records/REVIEW-20261020.md) | 主体与本机切换完成，整体计划复核收尾中；历史讨论和原型证据单独保留 |
| [V1 完整业务](v1/README.md) | 00～12 专项合同、任务依赖与 `progress.json` | [状态](v1/STATUS.md)、[记录](v1/records/README.md)、[验收摘要](v1/evidence/README.md) |
| [项目协同](cooperation-ui/README.md) | [设计](cooperation-ui/DESIGN.md)、页面契约、任务和 `progress.json` | [范围复核](cooperation-ui/STATUS.md)、[记录](cooperation-ui/records/README.md) |
| [Demo4 与计划任务](demo4/README.md) | [交互设计](demo4/DESIGN.md)、[运行例外](demo4/RUNTIME.md) | [最新复核](demo4/STATUS.md)、[对照验收](demo4/ACCEPTANCE.md)、[记录](demo4/records/README.md) |
| [共享资料](materials/README.md) | [正式设计](materials/DESIGN.md)、[原型说明](materials/PREVIEW.md) | [最新发布](materials/records/RELEASE-20261009.md)、[记录](materials/records/README.md) |
| [真实模型与 Kimi 联测](real-host/README.md) | [前置能力实现方案](real-host/DESIGN.md)、软件、图片、Office 与缓存观测的准入范围 | [综合场景覆盖与缺口](real-host/records/SCENARIO-GAPS-20261010.md)、[CLI / 缓存实施](real-host/records/CLI-CACHE-20261010.md)、[前置核查](real-host/READINESS.md) |

V2 是当前正式主线，V1 及其专项只保留历史时点。每轮 records 中的镜像、数量和边界按原日期阅读，不作为当前 V2 状态。

## 离线原型

在仓库根目录执行构建和预览命令。HTML 与所属计划放在一起，HTTP 路径保持不变。

| 原型文件 | 构建 | 预览 |
| --- | --- | --- |
| [V2 群聊工作区](v2/demo.html) | `npm run build:v2` | `npm run preview:v2`，5199 |
| [项目协同](cooperation-ui/demo.html) | 历史静态 HTML | 直接阅读原型文件 |
| [共享资料](materials/demo3.html) | 历史静态 HTML | 直接阅读原型文件 |
| [计划任务](demo4/demo4.html) | 历史静态 HTML | 直接阅读原型文件 |

原型只使用示例／页面本地状态。V2 原型在 `web/v2-preview/`，正式组件已独立接入真实领域；其他旧原型只保留静态 HTML，旧业务构建源码和运行器已清理。原型运行不证明服务器事务、模型或 Kubernetes 已通过验收。
