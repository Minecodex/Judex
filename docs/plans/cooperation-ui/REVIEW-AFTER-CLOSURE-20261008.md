# 收尾后的独立复核

日期：2026-10-08。对照八阶段 44 项、页面与数据契约、验收矩阵、当前源码及上一轮收尾制品重新检查。结论：**主体界面重构已经落地，但尚未达到整个实施计划全部完成的门槛。** 上一轮把 44 项全部标记 completed 的判断过早。本报告作为最新状态；原实施、复核和收尾记录保留为历史证据。

后续状态：本报告的五处发现已按用户要求修复，重开的 8 项补真实验证后关闭；最新状态见[修复与验证](REMEDIATION2-20261008.md)。以下保留当时的发现及失败证据，不覆盖原结果。

## 已落实的主体

| 范围 | 当前实现 |
| --- | --- |
| 页面层级 | 项目列表 → 协同四类标签 → 计划路线／计划讨论 → 任务详情／任务讨论；邀请、设置独立操作 |
| Chatbox | 当前计划或任务的平级会话列表；旧交接信箱、跨计划专区、底部账户入口、全项目待办／工作地图常驻页已退出新主路径 |
| 任务详情 | 统一 TaskRecords，任务记录／流程参考、上报、问题、原记录、启动、交接、验收、重开 |
| 会话能力 | 按需任务主讨论、多对象关联、完整中央／右侧标签、固定点分叉、来源定位、草稿、阅读位置、拖拽与最大化 |
| 设置与账户 | 右上角账户；项目设置复用完整职位、任职、邀请、流程、AI、私有偏好及 CLI／Skill 页面 |
| 正式与 demo | 共用项目协同、路线、聊天、详情和设置组件；正式数据由 API 提供，5196 是明确 demo 数据预览 |

这些内容已不是独立原型页面。保留的旧组件文件、项目历史讨论及交接记录访问不能单凭文件名认定为仍使用旧常驻主界面。右侧标签保留上次选中对象符合多标签保存约定；复核中直接任务 Chatbox 深链恢复旧计划标签的观察不计作缺陷。

## 必须补齐的问题

### R2-01：AI 建议追加关联未推进 linksVersion，旧窗口可覆盖新关联

正式 `collaboration.Resolve` 的 main/link 分支直接向 topic_work_links 插入任务、计划关联，未递增 topics.links_version，也未走 discussion.changeLinks 的统一版本与关联事件路径。同类直接追加还存在于安排变更的 link_topic 分支。

真实 PostgreSQL 复现：一个初始无关联的已有话题 linksVersion=1；确认建议后新增两条任务／计划关联，版本仍为 1。随后旧编辑窗口携带 expectedLinksVersion=1 完整替换为空，服务返回成功，两条新关联都被删除。未发生本应要求重新核对的版本冲突。它会影响跨范围可见性和用户明确确认的关联，不能以 PUT /links 自身的并发测试通过证明全部写入路径正确。

位置：`internal/collaboration/suggestions.go` 的 Resolve；`internal/work/change_policy.go` 的 link_topic；`internal/discussion/links.go`。

证据：`.cache/cooperation-ui/review-after-closure-20261008/association-reproduction-verified.log`、`projection_test.go`。这里测试返回 exit 1 是复现缺陷，不是验收通过。影响 P1-04、P5-03、P5-07，验收 TOPIC-03／AI-01。

### R2-02：过期上报草稿没有明确重新核对与继续提交的入口

报告草稿保存 expectedVersion，恢复基准固定为 task.id；任务版本改变后仍恢复旧版本。正式命令会以 VERSION_CONFLICT 拒绝旧版本，但弹窗只保留输入和提交按钮，未提供获取最新任务、展示变化并明确重新确认的动作。

浏览器复现：进展草稿记录版本 1；任务仍 working，但版本变为 2；刷新、提交失败、关闭重开再提交，草稿预期版本仍为 1，记录数未增加。正文保留正确，重试交互不完整。正式后端每次 progress 也会推进 task.version，所以另一参与者上报进展即可造成该条件，不只发生于任务范围修改。绕道调整任务选择不应成为恢复版本的隐含操作。

位置：`web/src/features/chat/CollaborationDialogs.tsx` 的 TaskActivityDialog、`web/src/features/work/apiCollaborationActions.ts` 的 recordTaskActivity、`internal/work/reports.go`。

证据：browser-results.json 的 stale-report-draft 及对应截图；本次浏览器复现使用 demo 状态模拟版本更新，正式拒绝逻辑和版本推进已沿源码核对，未将其冒称真实双用户 API 浏览器验收。影响 P4-04、P7-06，验收 TASK-03／VIEW-02。

### R2-03：多职责任职者不能通过当前“开始任务”选择身份

actingIdentity 要求本人恰好持有一个参与身份，否则要求明确选择。TaskRecords 的开始按钮和 taskAction.start 都没有身份选择参数，只有正式报告弹窗增加了选择器。

复现：同一 userId 绑定两个 active 任务参与身份；正式 action executor 在读取任务后报 422 VALIDATION_ERROR / choose a current task responsibility，未发送 start 请求。按钮本身可用，界面没有相应职责选择。后端有能力接收明确 identityId，缺口位于前端操作契约与交互。

位置：`web/src/features/chat/TaskRecords.tsx`、`web/src/features/work/storeTypes.ts` 的 taskAction、`web/src/features/work/apiActions.ts` 的 start 与 actingIdentity。

证据：browser-results.json 的 multi-responsibility-start；正式 executor 的任务读取采用拦截响应，属于组件／命令复现，不冒称真实 API 验收。影响 P4-04、P7-06，验收 TASK-03／BIZ-01。

### R2-04：个人交接筛选把其他发送者的未完成来源算给本人

deliveries 投影先按整个 handoff 聚合 status，再将“至少一条来源由本人发送”的 outgoing 与整体 status 相与。未要求需要补交或等待的来源本身属于当前用户。demo 查询使用相同聚合逻辑，也有该问题。

真实 PostgreSQL 复现：A 的来源 accepted，B 的来源 rejected，A 仍在 filter=revise 中看到该交接；B 改为 pending，A 仍在 filter=waiting 中看到它。期望按本人来源的当前状态过滤，同一交接仍可作为卡片聚合展示。该复现与单来源的拒收→补交→接收通过并不矛盾。

位置：`internal/work/cooperation.go` 的 ListDeliveries、`web/src/features/cooperation/queries.ts` 的 useDeliveryCards。

证据：postgres-reproduction.log 两个失败断言，以及 browser-results.json 的 delivery-filter-other-sender。影响 P1-05、P3-03、P7-06，验收 HUB-03／BIZ-02。

### R2-05：AI“关联已有讨论”仍依赖当前范围已加载的话题

源码确认：SuggestionDialog 仅枚举 state.topics；正式 apiStore 的列表按当前 plan/task 范围分页，另外只补当前和已保存标签中的话题。对话框没有项目范围的独立检索、分页与失败重试。未在当前范围、未打开为标签或处于后续页的已有讨论，无法通过这条交互完整选择；后端 Resolve(link) 支持同项目已有话题，不要求事先属于当前范围。

这也使 demo 中全项目话题都存在时的选择通过，不能证明正式分页数据下同样可用。需要在用户明确选择时查询整个项目的合法讨论，而非恢复全项目常驻列表。新建建议的关联对象确认也应与统一多对象选择器核对。

位置：`web/src/features/chat/CollaborationDialogs.tsx` 的 SuggestionDialog、`web/src/features/work/apiStore.ts` 的 topics 查询。

证据层次：正式数据装配与组件源代码核查；本轮没有伪造一份真实 API 浏览器通过记录。影响 P5-07、P7-06，验收 AI-01／HUB-05。

## 检查与证据边界

- 本轮新跑 TypeScript 类型检查通过；新浏览器诊断保存 4 个场景、0 个 pageerror，其中 3 个复现交互／命令问题，1 个是保留标签的观察。
- 新建独立 Docker PostgreSQL 数据库，调用当前正式应用查询与建议处理用例。个人交接筛选的两个断言、关联版本与覆盖检查复现失败。测试数据库、容器由该 fixture 清理，正常部署数据未触及。
- 上一轮实际日志仍确认 59 项前端逻辑、51 项桌面回归、20 组完整页面视觉加 1 项身份偏好、15 项 Kubernetes 受控模型业务检查，以及 1 项真实模型检查通过；真实模型记录 10 次调用。这些是已覆盖场景的有效证据，不能替代上述新发现的出口。
- 当前 18 个 cooperation 源文件及 100 个发行 JS/CSS 制品与上一轮 completion.json 的 SHA-256 一致。该 manifest 未给全部依赖的 chat/work 源文件生成指纹；统计中一些计数由脚本写死，已另核验实际日志及场景文件，未仅依据 status=complete 下结论。
- 原型对照 192 对主页面和 216 个弹窗场景的证据保留。deviceScaleFactor 是显示比例，不能当浏览器菜单缩放。未重跑全部旧测试或真实模型；本轮新测试反映新增发现，不抹去上一轮通过记录。
- 当前 `judex-ui-*` namespace 为空，原 judex Deployment Ready=1，镜像仍为 local-preferences-20261007-07cae2f1。运行部署未升级，旧部署页面不能用于判断本轮源码是否已发布。

本轮证据目录：`.cache/cooperation-ui/review-after-closure-20261008/`，包括 browser-results.json、截图、evidence-audit.json、类型检查及两个 PostgreSQL 复现日志。最初关联复现脚本的字段名编译错误日志单独保留；后续 verified 日志为上述真实查询与用例结果。

## 状态校准与后续出口

计划改回 in_progress：重开 P1-04、P1-05、P3-03、P4-04、P5-03、P5-07、P7-06、P7-07 共 8 项，另外 36 项保留原状态与证据。8 项是受影响的功能和验收任务，不代表 8 个独立缺陷，也不是界面完成度百分比。

后续优先统一全部追加关联的事务用例和版本事件；再补任务操作身份选择、报告过期核对、按本人来源过滤、AI 已有讨论完整选择。将上述复现转换为正式回归，并补真实多用户浏览器验证后才能再次关闭相关任务。通用多流程自动路由仍是独立旧门槛；现有部署升级仍属于后续发布，不因这次复核自动执行。
