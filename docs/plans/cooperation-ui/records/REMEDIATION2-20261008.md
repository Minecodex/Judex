# 独立复核缺口的修复与验证

日期：2026-10-08。对应 REVIEW-AFTER-CLOSURE-20261008.md 的 R2-01～R2-05 及重开的 8 项任务。五处缺口修复及当前镜像的临时 Kubernetes、真实模型检查已完成，8 项任务重新关闭；44 项状态与各阶段原证据在 progress.json 登记。通用多流程自动路由仍单独列为既有门槛，运行部署未升级。

后续状态以[第三次复核](REVIEW3-20261008.md)为准：本记录的五处修复与通过证据保留有效，新补测发现另外三处组合场景，相关 5 项任务重新打开，不能再以本轮 44 项全关表示完整交互验收结束。

## 修复内容

| 发现 | 正式改动 | 回归 |
| --- | --- | --- |
| R2-01 关联版本被绕过 | collaboration.ChangeTopicLinks 统一已有会话的关联写入；普通追加／完整替换、AI main/link 建议、已批准的 link_topic 安排均在同一事务锁会话、验证对象、推进版本并发事件。旧编辑窗口拒绝覆盖新关联 | 真实 PostgreSQL 的建议追加、过期替换、幂等重放、安排追加和无变化追加；COOP-R2-B 双用户实际关联冲突 |
| R2-02 报告草稿过期无法直接恢复 | 单独 TaskActivityDialog 保存正文、类型、身份及版本快照；版本变化后禁用旧提交，读取真实最新安排，显示完成条件和职责，由人确认后保留正文继续上报；读取失败可重试 | COOP-D15；COOP-R2-A 另一用户提交进展后恢复草稿，含断网读取失败及重试 |
| R2-03 多职责启动不能选择 | 任务启动和报告复用 TaskResponsibilityPicker；一个当前职责直接执行，多个当前职责明确选择。命令传 identityId，API 和 demo 都验证本人当前职责 | COOP-D16；COOP-R2-A 验证真实 start／report 请求的身份及两个正式报告 |
| R2-04 个人交接筛选误归类 | waiting／revise 按本人每条来源的当前版本状态过滤，再聚合成同一交接卡；原 incoming／outgoing 和接收／任务验收边界保持。交接摘要取实际来源版本正文 | 真实 PostgreSQL 的 accepted 本人来源＋他人 rejected/pending；COOP-D17；COOP-R2-C 两个真实发送者与独立接收／拒收 |
| R2-05 已有讨论只能选当前已加载对象 | DiscussionPicker 在明确打开选择器时独立读取项目讨论，搜索先于分页；可加载更多、显示已选对象并处理读取失败。新建专项同时确认标题和多对象关联，保留必需来源任务及计划 | COOP-D18；COOP-R2-B 55 条另一计划讨论，选取后续页对象、服务端精确搜索、当前范围可见及原草稿保留 |

新建会话的初始关系与版本 1 同事务建立。对已有会话完整替换会推进版本；重复追加已存在关系不改变版本。AI 建议重复处理仍返回同一结果，不再插入关系、引用或触发第二次业务处理。

同时补齐了关联编辑深链接的加载边界：指定 editTopicId 时先取得真实话题再初始化表单，避免空标题或新建状态。并发冲突的操作改为“核对最新关联／保存关联”；网络读取失败保留已选内容。报告、问题、关联与启动仍复用既有后端权限和状态用例，没有改动本地工作、人工正式决定或通用流程路由边界。

## 当前证据

证据目录：`.cache/cooperation-ui/remediation2-20261008/`。

- Go 全套通过，真实 PostgreSQL 集成包重新运行约 92 秒；vet 通过。新增持久回归在 tests/integration/cooperation_remediation_test.go。
- 前端逻辑 59 项通过，类型检查、正式发行构建、demo 构建和真实 CLI／Skill 下载验证通过。
- 55 项桌面回归通过；关联表单的最后修改后，相关 12 项桌面场景再次通过。新增 tests/e2e/cooperation-remediation.spec.ts。
- 新增报告核对、启动身份、已有讨论、新建建议关联四类弹窗，覆盖 1120／1440／1920、zh-CN／en、light／dark、显示比例 1／1.25，共 96 场景，无 pageerror 和越界。显示比例是 deviceScaleFactor，不冒称浏览器菜单缩放。未改变基础布局 token，上一轮主要页面对照证据保留。
- 真实 API：COOP-R2-A 和 COOP-R2-C 在 api-attempt3 通过，COOP-R2-B 在 api-attempt4 通过。两用户、实际报告、55 条分页、查询响应、旧编辑冲突、建议重放和 PostgreSQL／对象存储恢复校验均留有证据。
- 当前镜像 `judex/server:collaboration-cooperation-remediation2-final-20261008` 在 namespace `judex-ui-e90921fa` 集中通过 18 项真实 API／CLI／双用户／受控模型检查，包含三项新增缺口场景与原 15 项。另用现有 GLM-5.3 配置通过真实最小分析：8 次 model_calls、1 次角色运行、1 次 record_task_analysis 成功保存；与受控结果分列。
- namespace 所属核验后已删除，manifest exitCode=0、cleanedUp=true，当前无 judex-ui-* namespace。原 judex Deployment 镜像仍为 local-preferences-20261007-07cae2f1，Ready=1。
- completion.json 从实际日志提取检查计数，登记 414 个源码／契约文件、发行 JS/CSS 校验值、镜像 ID 与环境状态；380 个 Go／TS／TSX／CSS 文件均不超过 2000 行，最大 1806 行。原未提交文件的校验变化仅落在本轮相关范围，其他基线文件保留。

## 失败记录与环境

api-attempt1 保留旧记录：测试中拒收后立即导航会取消未完成请求；后续等待真实 rejected 状态再导航。已有讨论补测的早期版本依赖弹出列表后的快速 Escape，并曾使用与实际文案不一致的重试定位；最终通过实际分页／搜索响应和正常明确选择验证，保留原业务断言。关联编辑增加真实 GET 延迟检查加载顺序。

首次新增视觉运行时 5196 服务已退出，记录 ERR_CONNECTION_REFUSED；恢复同一地址的明确 demo 预览后 96 场景通过。用户已有浏览器数据不清空。

首次 Kubernetes 导入因镜像标签未符合 runner 的 collaboration- 前缀而在部署前终止；所属 namespace judex-ui-3e674fbb 已清理。使用同一镜像 ID 的合规标签重新运行，未放宽镜像导入或 namespace 所属检查。

现有运行部署不在发布范围内。当前正式代码、5196 的同组件树 demo 预览、通过证据及独立路由门槛分别记录；本轮没有升级现有部署或清理其业务数据。
