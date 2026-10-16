继续 KA01 / U0 的既有项目创建场景。现在通过 Kimi CLI 续接同一会话，桌面操控已经结束。不要派生子代理，也不要操作 Judex 开发仓库。

发行 CLI/Skill 和隔离服务已更新，以当前 test-connection.json 的 version 为准。读取当前项目的 test-connection.json、Judex Skill 及 references/commands.md；始终只用 .tools/KA01.cmd，核对版本、status 和 auth whoami。已有 U0 授权包含 projects:create 和 intents:create；服务已恢复。之前空对象产生的 pending 探查意图不要确认或重用。

项目目标：可运行的 MiniIssue 缺陷跟踪小站，含后端 API、桌面 Web、状态/关键字过滤、分页、测试和发布包；后续覆盖缺陷修复、隔离性能实验、测试服务恢复与综合版本交付。

现在生成 project-create.json：title 为「MiniIssue A 软件闭环」，kind=software；描述覆盖上述目标；maxDiscussionRounds=3；defaultModelId 取当前 test-connection.json 的 models.primary。按已更新 Skill 的字段合同生成，不再以空对象探查。

通过 CLI 的 project create 提出确认意图（不等待浏览器的模式），把完整 JSON 结果保存为 receipts/KA01-project-intent.json。必须检查 ok、进程退出码、state 和 confirmUrl；pending 只能说等待确认。此步做到提出意图即可停止，把链接和待确认内容告诉测试负责人。不要自行点浏览器确认，不通过 HTTP/SQL 绕过创建，也不要开始代码实现。
