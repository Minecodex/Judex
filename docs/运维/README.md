# V2 运维

公开发行入口：[v0.0.1-beta](../releases/v0.0.1-beta.md)、[构建与 Docker Hub 发布](releases.md)。公开 SemVer 与下方内部 V2 阶段编号分别管理，首次安装使用 Release 中的 Helm Chart 与配置示例。

部署、认证导入和本机切换见 [部署说明](../../deploy/README.md)。当前实现与已执行验收见 [V2 状态](../plans/v2/IMPLEMENTATION.md)。历史 [2026-10-19 本机 V1 启动](本机启动-20261019.md)仅记录切换前基线。

[2026-10-20 V2 发布记录](../plans/v2/records/RELEASE-20261020.md)记录已完成的账号／会话导入、真实模型验证、旧库删除、Secret 保持与临时资源回收。[后续收尾升级](../plans/v2/records/FOLLOWUP-20261020.md)完成 v2.11／schema 12、最终同版重启、逐行凭据保持和临时资源清理。正常入口仍为 `http://127.0.0.1:18097/`。

[运维操作](operations.md)说明组件、恢复和资源归属；[CI](ci.md)说明自动化门槛。模型、数据库、S3 和沙箱 Secret 保持私密，不写入文档、镜像或测试输出。演示与正式制品的证据分开。

[最终收尾升级](../plans/v2/records/FINAL-FOLLOWUP-20261020.md)已完成 v2.11→v2.12，保持 schema 12 和全部当前 V2 数据。升级前后核对完整认证行及五个 Secret 的 UID／内容，真实 CLI／模型检查通过；测试账号停用，原 1 位真实用户及 1 个有效会话保留。两个临时 namespace、所属存储与测试依赖已清理，正常五个工作负载各 1/1，共享 Controller 和无关服务保持运行。

[第三次收尾升级](../plans/v2/records/THIRD-FOLLOWUP-20261020.md)已完成 v2.12→v2.13，保持 schema 12 和当前 V2 数据。R10／R11 已修复；154 项候选／13 项升级、两种重启、真实模型、完整认证行与五个 Secret 一致核对、测试账号停用及归属资源清理全部通过。正常五个工作负载各 1/1，共享 Controller 和无关服务保持运行，入口不变。

[第四次复核](../plans/v2/records/FOURTH-REVIEW-20261020.md)又确认 R12～R14，当时整体计划重新进入收尾。上文 154／13 项升级与资源清理保持原执行事实。该次复核只运行带归属标签的临时复核依赖，未修改正常部署或模型 Secret；复核资源已回收，当时正常版本为 v2.13。

[第四次收尾升级](../plans/v2/records/FOURTH-FOLLOWUP-20261020.md)完成 v2.13→v2.14，关闭 R12～R14，保持 schema 12、全部 V2 数据及五个 Secret。170 项候选／13 项升级检查通过；完整认证行对照一致，合成账号停用，原真实账号与会话保持。两临时 namespace、PV、开发数据库、沙箱投影与代理端口回收；正常五个工作负载各 1/1，正式入口保持可用。当前总体完成状态以[第四次同版记录](../plans/v2/records/fourth-followup-acceptance-20261020.json)为准。

[正式界面升级](../plans/v2/records/UI-REFRESH-20261020.md)完成 v2.14→v2.15。95 项同版服务／浏览器检查与 24 项前端单元通过；升级前保存私有备份，完整认证行及五个 Secret 的 UID／内容一致。schema 12、数据库和现有内容保持，原 Chrome 登录会话直接进入新版项目空间。最新制品与资源回收见[机器记录](../plans/v2/records/ui-refresh-acceptance-20261020.json)；历史真实模型证据不改标为新版本。

[设置与内置库升级](../plans/v2/records/SETTINGS-RESTORE-20261020.md)完成 v2.15→v2.16／schema 13，新增模板来源标识而不清空现有内容。122 项同版检查及 24 项 Web 单元通过；升级前保存私有完整备份，认证全行和五个 Secret 保持一致。原 Chrome 会话可直接打开现有项目与内置库；原标签页未提交表单保留，使用新标签验证。两个临时 namespace、所属 PV 和测试端口已回收，正常五个工作负载各 1/1，入口不变。最新状态见[机器记录](../plans/v2/records/settings-restore-acceptance-20261020.json)。

[流程图与资料升级](../plans/v2/records/FLOW-PROFILE-20261020.md)完成 v2.16→v2.17/schema 14，124 项同版场景、25 项 Web 单元及 8 项正常冒烟通过。备份和全部原有认证字段保持校验通过，五项 Secret 保持；现有内置原文对应流程恢复图形。两临时 namespace、PV、集群角色和测试端口清理，正常五个工作负载各 1/1。最新[机器记录](../plans/v2/records/flow-profile-acceptance-20261020.json)明确本轮实际执行与未重跑范围。

[分栏、阅读与确认升级](../plans/v2/records/WORKSPACE-READERS-20261020.md)完成 v2.17→v2.18/schema 15，256 项同版场景、27 项 Web 单元和 7 项正常冒烟通过。完整认证字段和五个 Secret 保持；新转换器包含 LibreOffice、ffmpeg、libarchive。临时 namespace、存储和集群资源清理，正常入口和工作负载保持就绪。详见[机器记录](../plans/v2/records/workspace-readers-acceptance-20261020.json)。


[文件卡片、群头像与使用文档升级](../plans/v2/records/LIBRARY-GUIDE-20261021.md)完成 v2.18→v2.19，schema 保持 15。13 项专项、63 项完整桌面流程、14 项服务、13 项原生 CLI、3 项发行包 Skill 与 6 项正常冒烟检查通过；27 项 Web 单元、Go 全测试／vet 和 TypeScript 通过。升级前保存私有备份，17 用户／17 密码凭据／91 Web 会话的完整行及五项 Secret UID／内容保持一致。最新资源清理与实际范围以[机器记录](../plans/v2/records/library-guide-acceptance-20261021.json)为准。


[下拉菜单样式升级](../plans/v2/records/MENU-STYLES-20261021.md)完成 v2.19→v2.20，协议 2／schema 15 保持。70 项实际桌面场景通过（含 7 项菜单指针／键盘专项）；构建、TypeScript、同版校验和及源码行数检查通过。本轮仅修改前端 CSS 与自动化、文档，后端／模型测试未重跑。认证完整行和五个 Secret 保持一致，测试资源回收及正常入口检查见[机器记录](../plans/v2/records/menu-styles-acceptance-20261021.json)。
