# Judex 文档

更新：2026-10-16。本文是统一入口；业务规则见产品与详细设计，实施状态见对应计划，运行结果见带日期的验收记录。

## 阅读入口

| 需要了解什么 | 入口 |
| --- | --- |
| 产品边界、身份、职责、审批与验收 | [产品主线](产品/README.md)、[已确认答复](产品/决策答复.md) |
| 模块、数据、API、Agent、共享目录与部署设计 | [详细设计](详细设计/README.md) |
| 会话、职位偏好、职位与流程模板 | [功能说明](功能/README.md) |
| HeroUI、主题 Token、布局、控件与聊天组件树 | [界面规范](界面/README.md)、[聊天工作区](界面/聊天工作区.md) |
| 当前实现、实际验收与剩余门槛 | [工程与验收状态](详细设计/13-工程实现与验收状态.md)、[实施计划](plans/README.md) |
| 部署、备份恢复、PR 与 CI | [运维入口](运维/README.md) |
| 尚未采用的记忆方案 | [研究候选](研究/README.md) |

## 实施与验收

| 计划 | 范围 | 状态入口 |
| --- | --- | --- |
| [V1](plans/v1/README.md) | 后端、CLI／Skill、认证、Agent、持久化及整套系统门槛 | [验收报告与未完成项](plans/v1/STATUS.md) |
| [项目协同](plans/cooperation-ui/README.md) | 项目列表、协同页面、对象范围讨论及完整设置 | [范围复核](plans/cooperation-ui/STATUS.md) |
| [计划任务与 Demo4](plans/demo4/README.md) | 卡片、抽屉、原生全屏、草稿修订与运行例外 | [最新复核](plans/demo4/STATUS.md)、[运行规则](plans/demo4/RUNTIME.md) |
| [共享资料](plans/materials/README.md) | 原件、固定版本预览、用途、CLI 上报关联及删除 | [最新修复与发布记录](plans/materials/records/RELEASE-20261009.md) |
| [真实模型与 Kimi 联测](plans/real-host/README.md) | 当前新30任务/9计划验收、157项API契约与功能复测、正常服务44升级与数据保持；保留明确未测范围 | [完整回归与升级](plans/real-host/records/FULL-REGRESSION-20261015.md) |

2026-10-09专项记录保留项目协同、Demo4 R1～R8及MAT01～MAT16时点范围；正常环境已于2026-10-16升级44、revision 24，业务数据保持。当前完整回归见上表；V1与通用多流程仍按各自声明范围验收，不把已测试样本推导为任意组合。

启动与工程命令见[根 README](../README.md)、[前端](../web/README.md)、[测试](../tests/README.md)及[开发脚本](../scripts/README.md)。正式前端唯一工程入口是 `web/`；离线原型放在各计划目录。

## 文档维护

- 当前规则放在产品、功能、界面或详细设计；每个主题保留一个主要说明入口。
- 一项改造的设计、任务、状态、原型放在 `docs/plans/<plan>/`；带日期的复核、修复和发布记录放在该计划的 `records/`。
- 验收摘要放在对应计划的 `evidence/`；大型截图、trace、日志与备份放忽略的 `.cache/` 或指定制品库。
- 更新功能后同步主要规则和计划入口。被替代的草案直接清理，有效答复和实际测试证据保留在对应主题。
- 移动或删除文件时同步 Markdown、进度 JSON、工程 README、构建和预览脚本的引用。测试通过只陈述其实际执行时点及范围。
