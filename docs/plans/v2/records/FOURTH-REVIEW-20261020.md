# V2 第四次完成度复核

关闭记录（2026-10-20）：R12～R14 已在 v2.14 修复，完整验收与本机升级通过，见[第四次收尾](FOURTH-FOLLOWUP-20261020.md)及[机器记录](fourth-followup-acceptance-20261020.json)。以下保留发现时点。

日期：2026-10-20。复核正式版本 `0.2.0-local.20261020.v2.13`，协议 2、schema 12。

**结论：主体重构、本机切换与 R1～R11 已有修复有效，整体仍有 R12～R14 三项需收尾。** 上一轮 154 项候选／13 项升级仍保留通过事实，但没有覆盖批量上传、CLI 复合操作的失败恢复和模型调用中跨越时间预算。不能据通过数量断言全部边界已完成。

本轮只复核、取证和更新文档，没有修改正式应用代码或正常部署。隔离运行同版源码构建的 API Server、正式 Web、发行版 Darwin ARM64 CLI、实际 PostgreSQL／S3；时间预算使用同份 peer 源码和受控 Provider 在独立数据库复现，未调用真实模型。

## R12：多文件上传部分成功后，重试整批会重复成功项

优先级：中（P2）。涉及 P4／P6。

[api.ts:15](../../../../web/src/features/workspace/api.ts#L15)在每个文件完成后立即删除其 pendingUploads 记录。后续文件失败只保留该文件的上传阶段，没有整批选择的已完成结果及恢复身份。当前页面允许一次选择多个文件，失败后没有整批重试进度或已成功文件清单。

真实浏览器选择 A、B；代理让两者实际写入 S3 和 PostgreSQL，只损坏 B 的完成响应。此时已有 A、B 两个文件。再次选择相同 A、B 重试，B 复用原 upload／key，A 却创建新 upload／key，最终为 A、B、A 三个不同文件，其中 A 多出一条附件消息。各 complete 上游都返回 200；原 B 的单项重试保持正确，R11 单文件修复仍有效。

证据：`.cache/v2-review-4/batch-upload.json` 保存前后文件 ID、完成请求 key、上游状态、B 原版本的复用及 A 新版本；`batch-upload.png` 显示文件库中的两个 A。

完成条件：建立批量操作身份及每项确认结果，失败后能继续未确认部分；保留用户明确再次上传同名内容的新意图。补 A 成功后 B 未提交失败、B 已提交但响应丢失、部分重试和整批恢复的实际浏览器回归，不能按同名／同摘要永久合并文件。

## R13：带文件的 CLI 推送丢失结果后，重新执行命令会重复推送

优先级：中（P2）。涉及 P3／P4／P6。

[files_v2.go:97](../../../../internal/cli/files_v2.go#L97)在单文件 complete 确认后删除上传回执；[v2.go:156](../../../../internal/cli/v2.go#L156)每次推送先重新执行 UploadFile，再用得到的 fileVersionIds 组装最终消息。最终推送失败时虽保留了原消息 outbox，但下一次重新上传得到不同版本 ID，消息 JSON 和 outbox 指纹随之改变，无法自动匹配原组合操作。

真实发行 CLI 执行 `push --record-only --text ... --file ...`，不额外提供 request-id。代理让首次最终 push 真正提交后仅损坏响应，CLI 报错；原封不动重新执行同一命令后成功，但数据库从一条消息变为两条，文件也从一件变为两件。两份文件 SHA256 一致，文件版本 ID、消息 ID、最终 push 的 key 均不同；首次待确认 outbox 仍存在。

本次使用 record-only 且群无岗位，实际观察为重复文件／消息，没有执行 AI，不把重复 AI 调用描述为已观察事实。单条无附件命令的 outbox 重试、手工 pending retry 及明确 request-id 属于不同路径，不能代替默认复合命令的恢复证据。

证据：`.cache/v2-review-4/cli-composite-push.json`、`cli-proof-second.log`，保存两次上游 200、不同键／附件、前后消息和剩余 pending 数量；原生 CLI 配置已在 finally 回收。

完成条件：从整次 CLI 操作层持久化请求身份及已完成的附件结果；最终消息未知时重试原始 wire body／key，不重新生成文件版本。覆盖跨进程重新执行命令、多个附件部分成功、消息已提交但响应损坏、明确新意图及 request-id；保留当前授权重查，不让恢复绕过资源权限。

## R14：技术时间预算不约束正在进行的模型调用，过期后仍可发布

优先级：中（P2）。涉及 P5。

[runtime.go:56](../../../../internal/agent/peer/runtime.go#L56)只建立可取消 Context，没有按持久 budget_started_at 设置剩余时间 deadline。[model_calls.go:65](../../../../internal/agent/peer/model_calls.go#L65)在调用之前检查 30 分钟，但 Stream 和接收循环中没有预算定时取消；无工具的最终结果直接在 [runtime.go:127](../../../../internal/agent/peer/runtime.go#L127)发布并结束，不再检查剩余时间。Provider 通用 HTTP timeout 为 10 分钟，不等于本档剩余预算。

确定性 PostgreSQL 测试把持久预算起点设为仅剩 750 毫秒，受控模型实际等待 1500 毫秒后返回终止文本。调用 Context 没有 deadline，返回已晚于本档截止时间约 825 毫秒，运行仍变为 completed，并公开发表一条 `REVIEW_LATE_BUDGET_PUBLICATION`，没有按技术预算暂停。通过预置起点模拟临界时刻，没有声称本轮真实等待了 30 分钟。

证据：`.cache/v2-review-4/technical-time-budget.json`、`time-budget.log`；复现测试及逐字复制的当前 peer 代码保留于 `.cache/v2-review-4/peer-check`，正式源码未改动。

完成条件：用持久预算起点建立执行剩余期限，约束模型、摘要和工具 I/O，耗尽时统一记录 technical_budget 并暂停；不发布过期结果，保留未知调用费用和工具副作用核对。补跨越期限、长流、无 finish、压缩中耗尽、工具中耗尽、重启不补时及人工继续的新档期限回归。

## 阶段核对

| 阶段 | 本轮结论 |
| --- | --- |
| P0／P1／P2 | 合同、账号、项目与群已有交付有效，本轮未发现新增缺口 |
| P3 | 主体完成；带附件 CLI 推送的恢复需补 R13 |
| P4 | 主体完成；批量／复合上传恢复需补 R12／R13 |
| P5 | 主体完成；执行中时间预算需补 R14 |
| P6 | 主体完成；Web／CLI 的复合动作恢复需补 R12／R13 |
| P7 | 154／13 项同版证据保留；修复后重新执行必要矩阵及本机验收再关闭 |

本轮首先遇到隔离 S3 的卷分配配置问题，只作为夹具调试，未计入产品发现；调整本轮容器容量后先做实际读写预检，再在新建群内取上述两项文件证据。没有停止正常 namespace、共享 Controller 或无关数据库，也没有读取模型 Secret。取证结束按 token 核对并清理本轮资源，正常入口保持 v2.13。
