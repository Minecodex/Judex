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
| judex decision review ID | 读取固定审阅（变化、名单、计时） |
| judex task accept ID | 取验收快照并创建浏览器确认意图 |
| judex handoff send SOURCE --version N / handoff receive SOURCE | 交接发送/接收（经确认意图） |
| judex events watch | 拉取项目事件（只读） |

退出码：0 完成；2 参数；3 未认证；4 无权限；5 版本/审阅冲突；6 待确认；7 网络/依赖；8 业务条件不满足。
