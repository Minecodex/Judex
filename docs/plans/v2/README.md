# Judex V2 群聊协作重构

更新：2026-10-21。**P0～P7 与 R1～R14 已完成**。当前本机 `0.2.0-local.20261021.v2.22`，协议 2／schema 15。本轮统一区域语义图标与选中态，81 项桌面回归、13 项文件与使用文档检查通过；[验收与升级](records/PANE-ICONS-20261021.md)、[机器记录](records/pane-icons-acceptance-20261021.json)列明范围。历史完整矩阵保留原版本，不改标为本轮证据。

## 当前有效入口

- [设置与内置库恢复](SETTINGS-RESTORE-20261020.md)、[v2.16 验收与升级](records/SETTINGS-RESTORE-20261020.md)、[机器记录](records/settings-restore-acceptance-20261020.json)：完整设置、内置职位／工作流、成员与任职、个人提示词及本机部署。

- [正式界面改造](UI-REFRESH-20261020.md)、[v2.15 验收与升级](records/UI-REFRESH-20261020.md)、[机器记录](records/ui-refresh-acceptance-20261020.json)：Kylon 参考、统一字号与配色、会话菜单、文件侧栏、双语主题及本机部署。
- [第四次收尾验收](records/FOURTH-FOLLOWUP-20261020.md)、[历史机器记录](records/fourth-followup-acceptance-20261020.json)：R12～R14 修复、170／13 项验收、升级和清理依据。

- [第四次完成度复核](records/FOURTH-REVIEW-20261020.md)：保留三项缺口的发现、复现和完成条件；已在 v2.14 关闭。
- [第三次收尾验收](records/THIRD-FOLLOWUP-20261020.md)、[历史机器记录](records/third-followup-acceptance-20261020.json)：保留 v2.13 的 R10／R11 修复、154／13 项回归、本机升级与清理事实；保留第四次复核重开的历史时点，当前关闭依据见第四次收尾。
- [第三次完成度复核](records/THIRD-REVIEW-20261020.md)：保留 v2.12 的 R10／R11 发现、影响和完成条件，已在 v2.13 收尾。
- [实施进度与实际检查](IMPLEMENTATION.md)：P0～P7 当前状态与本机切换门槛。
- [最终收尾验收](records/FINAL-FOLLOWUP-20261020.md)、[同版机器记录](records/final-followup-acceptance-20261020.json)：保留 v2.12 对 R6～R9 的修复及完整回归／升级／清理事实，保留第三次复核重开的历史时点，当前关闭证据见第四次收尾。
- [第二次完成度复核](records/SECOND-REVIEW-20261020.md)：保留 v2.11 发现时点，四项缺口已在最终收尾闭合。
- [已有收尾验收](records/FOLLOWUP-20261020.md)、[同版机器记录](records/followup-acceptance-20261020.json)：保留 v2.11 对 R1～R5 的修复和逐项通过事实，后续整体状态以最终收尾为准。
- [完成度复核](records/REVIEW-20261020.md)：保留 v2.10 的原始发现；问题均在收尾记录闭合。
- [首次发布与本机切换记录](records/RELEASE-20261020.md)、[历史验收汇总](records/acceptance-20261020.json)。
- [完整领域、权限、API／事件／CLI 与执行合同](CONTRACTS.md)。
- [工程任务与验收入口](TASKS.md)。
- [产品](../../产品/README.md)、[架构](../../详细设计/README.md)、[界面](../../界面/README.md)、[运维](../../运维/README.md)。

登录后从项目卡片进入群工作区。项目保留描述、职位、参考工作流、成员和运行配置；成员直接添加。群成员独立，允许多个同级子群，最多三层。只展示本人加入的群，不泄漏未加入祖先名字或 ID；归档级联且仍可讨论。平台私聊跨项目复用、纯人类参与。

消息与文件属于会话；文件版本不可变，显式分享只授予指定内容／版本。岗位按群／人／职位稳定标识，共享公共知识和统一压缩摘要，每次调用重组指导提示词；普通消息零触发，本地推送触发本人全部岗位，其他岗位只在结构化提及时触发。讨论共享闭环额度，暂停合并新内容，人工继续／终止，技术预算与副作用恢复分别记录。

不兼容旧计划、任务、交接、审批和验收领域。按用户后续完整计划保留账号、密码、有效 Web session 与恢复记录及基础设施 Secret，新建干净 V2 数据库和对象前缀，CLI 重新授权。最新实施合同覆盖早期“只讨论／Demo、不替换服务”的时点范围。

## 历史讨论与原型

- [分栏、文件阅读与确认卡片讨论（2026-10-20）](WORKSPACE-READER-CARDS-DISCUSSION-20261020.md)：后续已确认实施，见[正式实现范围](WORKSPACE-READERS-20261020.md)。
- [Q1～Q55 完整讨论](DISCUSSION.md)、[已确认答复](DECISIONS.md)、[方案收口](REVIEW.md)。
- [设计讨论快照](DESIGN-DRAFT.md)、[Agent 讨论快照](AGENT-DRAFT.md)。
- [桌面原型](demo.html)、[原型运行范围](PREVIEW.md)、[历史 13 项原型证据](evidence/README.md)。

历史原型含本地示例与模拟 AI，仅供视觉对照，不是正式服务或模型验收。正式前端在 `web/src/features/workspace`，原型独立在 `web/v2-preview`。完整实施不再依赖旧业务对象。
