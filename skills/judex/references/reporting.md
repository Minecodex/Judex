# 报告格式

report.json（progress/delivery 通用）：

```json
{
  "text": "完成列表筛选；附件包含本地验证记录。",
  "expectedTaskVersion": 4,
  "materialVersionIds": ["<material upload 返回的 versionId>"],
  "codeRefs": [{"repositoryId":"<uuid>","branch":"task/filter","commit":"<full-sha>","verification":"reported"}]
}
```

- `verification` 只能写 `reported`（客户端声明）；平台固定显示"reported"，不允许自称 verified。
- 交付引用的材料必须 ready：先 `material upload`，把返回的 versionId 放入 materialVersionIds。纯文字交付是否满足验收条件仍由任务约定与有权人核对。
- 批量交付先用不带 `--purpose/--task/--plan` 的 `material upload PATH` 上传固定原件，再用一份 report 集中引用全部版本。`uploadStatus=ready`、`registrationStatus=not_requested` 表示原件可用、未另行提交用途，不表示上传失败。源码可打成排除数据库/凭据/运行时文件的 ZIP，按一个文件上传；upload 不要求每个源文件单独提交。
- 进度是提示不是验收；不要用 progress 代替交付。

本地文件只传明确选择的文件。上报引用固定 `materialVersionIds`，所属计划由任务确定；上传人、时间、职位和实际报告来源由服务端记录，无需把整份计划／任务内容塞入文件元数据。

需要上游本轮成果时，先读取来源任务当前状态与 `latestAcceptanceId`，再从 `task acceptances` 找 `acceptanceId` 匹配的 accept 记录，核对其报告和固定 materialVersionId/SHA。历史记录的 `id` 不等于 acceptanceId。协议示例、同名包和旧 producer 的状态不能代替当前接受快照；明确指定的历史输入仍按原固定UUID/SHA使用。引用报告中记录实际来源任务、接受快照、材料版本与SHA，不能把旧测量换标签当新测量。

来源移交用 `handoff send SOURCE --version N --summary TEXT` 说明本次固定材料、适用范围和缺口；说明与原报告一起进入固定审阅。CLI 仍只准备意图，接收人独立核对后通过浏览器确认 receive/reject；来源接收与任务最终验收分开，不因移交退回直接撤销已成立的验收。

独立提交一份资料供分析时可运行 `judex material upload ./evidence.md --purpose "用于核对登录测试" --task <任务 uuid>`，或使用互斥的 `--plan <计划 uuid>`。这些关联选项会另行生成资料提交并触发相应范围的自动分析，不宜用于交付包每个文件的机械重复提交。没有工作对象时资料仍属于项目共享资料；用途提交不生成进度报告，真实进展继续用 report。

上传成功而用途关联提交失败时，CLI 返回原 `versionId`、`uploadStatus=ready` 与 `registrationStatus=failed`，退出非零。同一源文件和上下文重试复用上传版本及原提交键；成功结果为 `registrationStatus=committed`。分享已有材料只引用版本，不复制原件；删除资料库条目后，历史交付与审阅仍保留原版本。

问题与讨论回复使用通信提交，正式 report kind 仍只有 progress/delivery：

```json
{
  "clientSubmissionId": "<稳定且可重试的 uuid>",
  "purpose": "message",
  "discussionIntent": "question",
  "taskId": "<任务 uuid>",
  "text": "目前发现重复回调，需要开发与核对岗位持续协商证据。"
}
```

运行 `judex submit --file submission.json`。回复时用 discussionIntent=reply，并添加人明确选定的 topicId；通信进入任务时间线及对应讨论，不构成交付贡献。连续上报不会自动创建会话，AI 建议保留为待处理状态；人选择专项、主讨论继续或已有讨论后，用 discussion-suggestion resolve 建立同一个处理结果。

跨计划核对来源或请求变更草稿时，使用项目范围 submit，不附 taskId/planId，并明确目标对象 ID。任务或计划范围的 query_work 只返回当前范围及已引用对象，空结果不能当作整个项目中不存在。平台模型的 propose_changes 可显式 kind=work_change 修正既有工作；省略 kind 保留 work_arrangement。两者均仅生成草稿，仍须发起人实际提交和有权人固定审阅。

重新获取任务记录用 task activity TASK 的 nextCursor，避免把首个分页当成完整历史。模型未配置/失败/预算耗尽保留原记录，查询 task-analysis show 后决定是否重试；重试不重置预算或重复工具效果。

任务被退回时，先用 `judex task acceptances TASK --limit 100` 按决定时间读取最近正式反馈，核对 decision、reason、targetVersion 与来源；必要时沿 nextCursor 继续读取。`latestAcceptanceId` 仅指接受快照，退回后可为空，不能用它定位最近退回。计划整体反馈用 `judex plan acceptances PLAN`。重新打开时还须核对用户明确给出的重开理由与当前状态，不能把历史接受理由当作重开理由。旧退回若 provenance=legacy_audit，仅理由来自原审计，未知版本和快照保持 null。验收记录与报告活动分别读取，保留旧决定和固定原件，在新修订中处理具体问题；读取反馈不产生新意图或决定。
