# 命令表

所有命令支持 `--server`、`--profile`、`--json`；项目命令支持 `--project`。`--json` 输出 `{schemaVersion:"1",ok,data,error?,requestId?}`。

| 命令 | 用途 |
| --- | --- |
| judex version / status / auth whoami | 版本、可达性、当前账号与 scopes |
| judex auth login / auth logout | 设备授权登录（浏览器确认）/ 撤销本设备 |
| judex project list / project use ID | 本人项目 / 选择项目 |
| judex inbox list | 我的待办（对象、类型、合法下一步） |
| judex material list / material upload PATH | 资料列表 / 分片上传并登记不可变版本 |
| judex submit --file submission.json | 自由文本+材料统一提交（不自动验收） |
| judex report --task ID --kind progress|delivery --file report.json | 进度/交付上报（带 expectedTaskVersion） |
| judex proposal draft --file p.json / proposal submit ID | 提案草稿 / 提交审批 |
| judex context get TASK | 当前任务、职责、前置与固定材料引用 |
| judex decision approve ID --review HASH / decision reject ID --review HASH --reason TEXT | 创建固定审阅的浏览器确认意图，不直接投票 |
| judex decision result INTENT | 读取同一浏览器决定的 resultRef |
| judex decision review ID | 读取固定审阅（变化、名单、计时） |
| judex task accept ID | 取验收快照并创建浏览器确认意图 |
| judex handoff send SOURCE --version N / handoff receive SOURCE | 交接发送/接收（经确认意图） |
| judex events watch --after N --limit 50 --timeout 10s | 从事件序号恢复 SSE，返回 items 和 nextCursor（只读） |

退出码：0 完成；2 参数；3 未认证；4 无权限；5 版本/审阅冲突；6 待确认；7 网络/依赖；8 业务条件不满足。

列表命令支持 `--limit 1..100`、`--cursor NEXT_CURSOR`；返回 `data.items` 和 `data.nextCursor`。游标仅用于相同账号、范围及筛选条件。
