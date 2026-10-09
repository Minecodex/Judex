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
- 进度是提示不是验收；不要用 progress 代替交付。

本地文件只传明确选择的文件。上报引用固定 `materialVersionIds`，所属计划由任务确定；上传人、时间、职位和实际报告来源由服务端记录，无需把整份计划／任务内容塞入文件元数据。

单独登记资料可以运行 `judex material upload ./evidence.md --purpose "用于核对登录测试" --task <任务 uuid>`，或使用互斥的 `--plan <计划 uuid>`。没有工作对象时资料仍属于项目共享资料。这个命令登记用途，不生成进度报告；真实进展继续使用 `report --task ... --file report.json`。

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

重新获取任务记录用 task activity TASK 的 nextCursor，避免把首个分页当成完整历史。模型未配置/失败/预算耗尽保留原记录，查询 task-analysis show 后决定是否重试；重试不重置预算或重复工具效果。
