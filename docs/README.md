# Judex 文档

更新：2026-10-21。当前正式版本为 V2 群聊协作。历史计划和记录保留自身执行时点，不能覆盖当前产品或验收范围。

| 内容 | 入口 |
| --- | --- |
| 产品与行为边界 | [产品主线](产品/README.md) |
| 类型、权限矩阵、API／SSE／CLI、岗位与恢复算法 | [V2 合同](plans/v2/CONTRACTS.md) |
| 实现进度、工程任务、实际验收和本机切换 | [V2 实施状态](plans/v2/IMPLEMENTATION.md)、[任务](plans/v2/TASKS.md) |
| 本机 V2 切换与完成度复核 | [最新收尾](plans/v2/records/FOURTH-FOLLOWUP-20261020.md)、[当前同版机器记录](plans/v2/records/fourth-followup-acceptance-20261020.json)、[第四次复核历史](plans/v2/records/FOURTH-REVIEW-20261020.md)、[第三次收尾验收](plans/v2/records/THIRD-FOLLOWUP-20261020.md)、[同版机器记录](plans/v2/records/third-followup-acceptance-20261020.json)、[第三次复核历史](plans/v2/records/THIRD-REVIEW-20261020.md)、[最终收尾验收](plans/v2/records/FINAL-FOLLOWUP-20261020.md)、[最终同版记录](plans/v2/records/final-followup-acceptance-20261020.json)、[历史第二次复核](plans/v2/records/SECOND-REVIEW-20261020.md)、[首次切换](plans/v2/records/RELEASE-20261020.md) |
| 模块、数据、文件、Agent 与部署设计 | [详细设计](详细设计/README.md) |
| HeroUI、token、双语主题与聊天交互 | [界面规范](界面/README.md)、[聊天工作区](界面/聊天工作区.md) |
| v2.15 正式界面改造记录 | [Kylon 参考改造](plans/v2/UI-REFRESH-20261020.md)、[同版验收](plans/v2/records/UI-REFRESH-20261020.md)、[机器记录](plans/v2/records/ui-refresh-acceptance-20261020.json) |
| v2.16 设置、内置库与本机升级 | [恢复方案](plans/v2/SETTINGS-RESTORE-20261020.md)、[验收](plans/v2/records/SETTINGS-RESTORE-20261020.md)、[机器记录](plans/v2/records/settings-restore-acceptance-20261020.json) |
| v2.17 流程图、资料与 90% 密度 | [设计](plans/v2/FLOW-PROFILE-20261020.md)、[验收](plans/v2/records/FLOW-PROFILE-20261020.md)、[机器记录](plans/v2/records/flow-profile-acceptance-20261020.json) |
| v2.18 分栏、阅读器与确认卡片 | [范围与格式边界](plans/v2/WORKSPACE-READERS-20261020.md)、[实际验收](plans/v2/records/WORKSPACE-READERS-20261020.md)、[机器记录](plans/v2/records/workspace-readers-acceptance-20261020.json) |
| v2.19 文件卡片、群头像与使用文档 | [范围](plans/v2/LIBRARY-GUIDE-20261021.md)、[验收](plans/v2/records/LIBRARY-GUIDE-20261021.md)、[机器记录](plans/v2/records/library-guide-acceptance-20261021.json) |
| v2.20 下拉菜单边框与焦点 | [修复范围](plans/v2/MENU-STYLES-20261021.md)、[验收](plans/v2/records/MENU-STYLES-20261021.md)、[机器记录](plans/v2/records/menu-styles-acceptance-20261021.json) |
| v2.21 文件返回与区域布局控制 | [范围](plans/v2/PANE-NAVIGATION-20261021.md)、[验收](plans/v2/records/PANE-NAVIGATION-20261021.md)、[机器记录](plans/v2/records/pane-navigation-acceptance-20261021.json) |
| 当前 v2.22 区域图标与选中状态 | [范围](plans/v2/PANE-ICONS-20261021.md)、[验收](plans/v2/records/PANE-ICONS-20261021.md)、[机器记录](plans/v2/records/pane-icons-acceptance-20261021.json) |
| 多 Codex 会话与真实公司业务联测 | [范围](plans/v2/COMPANY-SIMULATION-20261020.md)、[实际结果](plans/v2/records/COMPANY-SIMULATION-20261020.md)、[机器记录](plans/v2/records/company-simulation-acceptance-20261020.json) |
| 构建、部署、备份、恢复与 CI | [运维](运维/README.md)、[部署](../deploy/README.md) |
| 公开 Beta 安装、镜像与发行 | [v0.0.1-beta](releases/v0.0.1-beta.md)、[发行流程](运维/releases.md) |
| 自动化与真实环境测试 | [测试入口](../tests/README.md) |
| 早期 V2 视觉原型 | [原型](plans/v2/PREVIEW.md)、[历史 13 项原型证据](plans/v2/evidence/README.md) |
| 旧版历史 | [计划索引](plans/README.md)、[V1](plans/v1/README.md) |

代码唯一正式前端是 `web/src`。V2 原型在 `web/v2-preview`，历史原型 HTML 保留于对应计划目录；其示例人物与数据不进入生产构建。

维护规则：当前规则放在产品、界面和详细设计；计划、任务与阶段进度放在 `docs/plans/<plan>`；带日期的验收、发布和问题处理放在对应 records，截图与大型日志放 `.cache/`。修改功能应同步主要说明和自动化入口；只按实际执行范围声明通过，不把原型或 skip 算作正式证据。
