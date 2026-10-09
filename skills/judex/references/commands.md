# 命令表

所有命令支持 `--server`、`--profile`、`--json`；项目命令支持 `--project`。`--json` 输出 `{schemaVersion:"1",ok,data,error?,requestId?}`。

| 命令 | 用途 |
| --- | --- |
| judex version / status / auth whoami | 版本、可达性、当前账号与 scopes |
| judex auth login / auth logout | 设备授权登录（浏览器确认）/ 撤销本设备 |
| judex project list / project use ID | 本人项目 / 选择项目 |
| judex inbox list | 我的待办（对象、类型、合法下一步） |
| judex material list / material upload PATH [--purpose TEXT] [--task ID 或 --plan ID] | 资料列表 / 分片上传，登记不可变版本及可选用途关联 |
| judex submit --file submission.json | 自由文本+材料统一提交（不自动验收） |
| judex report --task ID --kind progress|delivery --file report.json | 进度/交付上报（带 expectedTaskVersion） |
| judex proposal draft --file p.json / proposal submit ID | 提案草稿 / 提交审批 |
| judex context get TASK | 当前任务、职责、前置与固定材料引用 |
| judex context get --topic TOPIC | 当前分支的公开历史与原消息来源 |
| judex task activity TASK --limit 20 --cursor CURSOR | 分页读取任务原记录、材料版本、真实来源与分析 |
| judex task-analysis show ANALYSIS / task-analysis retry ANALYSIS | 查询分析状态 / 继续原批次与预算 |
| judex discussion-suggestion list --limit 50 --cursor CURSOR | 分页查询讨论建议及处理结果 |
| judex discussion-suggestion resolve ID --mode create --title TITLE | 人确认标题与关联工作后另建专项；固定来源存在时分叉 |
| judex discussion-suggestion resolve ID --mode main / --mode link --topic TOPIC | 引用原记录在主讨论或明确指定的会话继续 |
| judex topic fork TOPIC --title TITLE --after-seq N | 固定消息处建立独立分支，经浏览器确认一次 |
| judex decision approve ID --review HASH / decision reject ID --review HASH --reason TEXT | 创建固定审阅的浏览器确认意图，不直接投票 |
| judex decision result INTENT | 读取同一浏览器决定的 resultRef |
| judex decision review ID | 读取固定审阅（变化、名单、计时） |
| judex task accept ID | 取验收快照并创建浏览器确认意图 |
| judex task skip TASK [--reason TEXT] [--waive TASK:REQUIREMENT] | 无原因时只读预览；有原因时准备一次管理员浏览器确认，豁免条件逐项列明 |
| judex task restore TASK --reason TEXT [--acknowledge-started] | 恢复原执行条件，保留后继已发生工作，经一次浏览器确认 |
| judex handoff send SOURCE --version N / handoff receive SOURCE | 交接发送/接收（经确认意图） |
| judex events watch --after N --limit 50 --timeout 10s | 从事件序号恢复 SSE，返回 items 和 nextCursor（只读） |

退出码：0 完成；2 参数；3 未认证；4 无权限；5 版本/审阅冲突；6 待确认；7 网络/依赖；8 业务条件不满足。

列表命令支持 `--limit 1..100`、`--cursor NEXT_CURSOR`；返回 `data.items` 和 `data.nextCursor`。游标仅用于相同账号、范围及筛选条件。

项目协同入口：项目卡先进入计划列表；计划卡主体进入路线，讨论按钮进入计划主讨论。`context get TASK` 返回可空 `mainTopicId` 和相对 `discussionPath`；主讨论为空时浏览器显示明确的创建入口，读取上下文或上报不会创建会话。`context get --topic TOPIC` 使用项目级深链，不猜测多关联会话的第一个所属计划。

同一讨论可以关联本项目多个计划／任务，记录只有一个 topicId；关联不改变任务所属计划或贡献。正式与通信上报、交接接收、任务验收和计划验收分别使用既有用例。冻结审阅变化时重新展示依据，由用户再次审阅；不能在确认时自动换用新快照。
