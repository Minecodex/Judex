# 命令表

所有命令支持 `--server`、`--profile`、`--json`；项目命令支持 `--project`。`--json` 输出 `{schemaVersion:"1",ok,data,error?,requestId?}`。

设备登录会请求 CLI 工作所需权限，包含 `projects:create` 与 `intents:create` 以发起建项确认。权限由本人在浏览器逐项选择，正式建项仍需另一次固定内容确认；只有读取项目的旧授权不能创建项目，可重新登录审阅新请求。

| 命令 | 用途 |
| --- | --- |
| judex version / status / auth whoami | 版本、可达性、当前账号与 scopes |
| judex auth login / auth logout | 设备授权登录（浏览器确认）/ 撤销本设备 |
| judex project list / project use ID | 本人项目 / 选择项目 |
| judex project create --file project.json [--no-wait] | 准备建项确认，由本人在浏览器审阅后创建；不直接建立项目 |
| judex inbox list | 我的待办（对象、类型、合法下一步） |
| judex material list / material upload PATH [--purpose TEXT] [--task ID 或 --plan ID] | 资料列表 / 分片上传，登记不可变版本及可选用途关联 |
| judex material download --version VERSION --output FILE [--entry PATH] | 下载固定原件或明确条目，核对SHA；FILE为本次新文件路径 |
| judex identity list / identity use ID | 本人实际任职 / 选择本次职责，不以职位名称代替identity UUID |
| judex submit --file submission.json | 自由文本+材料统一提交（不自动验收） |
| judex report --task ID --kind progress|delivery --file report.json | 进度/交付上报（带 expectedTaskVersion） |
| judex proposal draft --file p.json / proposal submit ID | 提案草稿 / 提交审批 |
| judex context get TASK | 当前任务、职责、前置与固定材料引用 |
| judex context get --topic TOPIC | 当前分支的公开历史与原消息来源 |
| judex task activity TASK --limit 20 --cursor CURSOR | 分页读取任务原记录、材料版本、真实来源与分析 |
| judex task acceptances TASK --limit 20 --cursor CURSOR | 只读任务验收历史、退回理由、reviewer与固定审阅来源 |
| judex task-analysis show ANALYSIS / task-analysis retry ANALYSIS | 查询分析状态 / 继续原批次与预算 |
| judex submission-analysis SUBMISSION | 查询项目或任务提交对应的公开分析、依据和草稿引用 |
| judex discussion-suggestion list --limit 50 --cursor CURSOR | 分页查询讨论建议及处理结果 |
| judex discussion-suggestion resolve ID --mode create --title TITLE | 人确认标题与关联工作后另建专项；固定来源存在时分叉 |
| judex discussion-suggestion resolve ID --mode main / --mode link --topic TOPIC | 引用原记录在主讨论或明确指定的会话继续 |
| judex topic fork TOPIC --title TITLE --after-seq N | 固定消息处建立独立分支，经浏览器确认一次 |
| judex decision approve ID --review HASH / decision reject ID --review HASH --reason TEXT | 创建固定审阅的浏览器确认意图，不直接投票 |
| judex decision result INTENT | 读取同一浏览器决定的 resultRef |
| judex decision review ID | 读取固定审阅（变化、名单、计时） |
| judex task accept ID / task reject ID --reason TEXT | 取同一固定验收快照，提出接收或带具体理由退回的浏览器确认意图；pending 不代表决定生效 |
| judex plan show ID / plan review ID | 只读正式计划和固定验收快照（任务version/acceptanceId/阻塞条件）；复核整体目标用原件与实际测试 |
| judex plan acceptances PLAN --limit 20 --cursor CURSOR | 只读计划验收历史与整体退回理由 |
| judex plan accept ID --no-wait | 计划owner在任务分别接受后提出独立整体验收意图；仍由固定浏览器审阅确认 |
| judex task skip TASK [--reason TEXT] [--waive TASK:REQUIREMENT] | 无原因时只读预览；有原因时准备一次管理员浏览器确认，豁免条件逐项列明 |
| judex task restore TASK --reason TEXT [--acknowledge-started] | 恢复原执行条件，保留后继已发生工作，经一次浏览器确认 |
| judex handoff send SOURCE --version N --summary TEXT / handoff receive SOURCE | 来源发送/接收，N为当前来源版本（未发送时用1）；当前版本与说明进入固定审阅，确认后生成下一版；均经浏览器确认意图 |
| judex handoff list --limit 100 --cursor CURSOR | 只读来源清单、版本、摘要和接收状态；按 nextCursor 补读 |
| judex events watch --after N --limit 50 --timeout 10s | 从事件序号恢复 SSE，返回 items 和 nextCursor（只读） |

退出码：0 完成；2 参数；3 未认证；4 无权限；5 版本/审阅冲突；6 待确认；7 网络/依赖；8 业务条件不满足。

建项 JSON 必填 `title`（1～200 字）；可选 `description`、`kind`（software 或 design）、`maxDiscussionRounds`（1～100）、`approvalTimeoutSeconds`（至少 60）和 `defaultModelId`（当前启用模型的 UUID）。不填默认模型时使用平台配置。`--no-wait` 返回固定意图和确认链接，浏览器确认后查询同一结果；不能把 pending 当作已创建项目。

列表命令支持 `--limit 1..100`、`--cursor NEXT_CURSOR`；返回 `data.items` 和 `data.nextCursor`。游标仅用于相同账号、范围及筛选条件。

项目协同入口：项目卡先进入计划列表；计划卡主体进入路线，讨论按钮进入计划主讨论。`context get TASK` 返回可空 `mainTopicId` 和相对 `discussionPath`；主讨论为空时浏览器显示明确的创建入口，读取上下文或上报不会创建会话。`context get --topic TOPIC` 使用项目级深链，不猜测多关联会话的第一个所属计划。

同一讨论可以关联本项目多个计划／任务，记录只有一个 topicId；关联不改变任务所属计划或贡献。正式与通信上报、交接接收、任务验收和计划验收分别使用既有用例。冻结审阅变化时重新展示依据，由用户再次审阅；不能在确认时自动换用新快照。

create_plan/create_task的fields使用公开合同字段。任务流程节点为nodeId；未知字段在草稿创建/修订/提交即返回VALIDATION，不自动改名。模型草稿生成成功仍需逐字段核对，不能把合理的字段值当作正确字段名。
## 固定流程版本读取

`judex workflow get WORKFLOW_ID --version VERSION_ID --json` 读取当前项目指定已发布流程的完整正文、`bodySha256` 和 `isCurrent`。流程定义 UUID、流程版本 UUID 与材料版本 UUID 是不同对象，不能用 `material download` 或材料解析工具读取流程。历史版本可以回读，但 `isCurrent=false` 时不能作为当前流程依据；正文读取不构成发布或任何正式决定。
