# 运维手册（P7-03）

## 备份

- `scripts/operations/backup.sh <namespace> <release> <directory>` 导出 PG 全库、全部 S3 对象字节与每对象清单、SHA256SUMS。清单本身不能恢复文件，因此必须保留 `objects.tar`。
- 数据库先快照，对象随后导出；材料是不可变版本，备份期间暂停对象 GC。精确的全表前后摘要比对还需暂停 API 写流量及 worker。
- 默认使用内置 PostgreSQL Pod；外部数据库需要部署方用对应 pg_dump/pg_restore 连接信息执行。脚本不声称外部数据库已实测。
- 频率和保留策略由部署方设定；原始归档按敏感业务数据保护。

## 恢复

1. 创建独立 namespace/空数据库/空对象桶，以 `server.mode=api` 启动、不开放用户流量。API 模式连接 PG 但不启动 worker。
2. 运行 `scripts/operations/restore.sh <new-namespace> <release> <backup-directory>`。脚本拒绝覆盖含用户的数据库或非空桶；先校验归档，恢复对象和数据库。
3. `judex-server verify-recovery` 校验所有 ready 材料条目的字节长度及 SHA256，并输出全表行数和摘要。脚本保存为 `restore-verification.json`。与已静默的来源摘要比较，可检查审批/验收/截止时间/运行记录完全保留。
4. 检查未知工具记录和 pending 审批，恢复 worker 后观察一次到期结算；未知副作用不得重新执行以推测结果。验证后再开放流量。
5. 本轮真实 E2E 执行了新库/新桶恢复并比对所有表、5 个材料条目。完整 K8s 故障矩阵和 RPO/RTO 尚未验收，不能由该样本推出可用性等级。

## 账号运维
- 恢复码：`judex-server account recovery-code --email ... --reason ...`（15 分钟一次性）。
- 停用：`judex-server account disable --email ...`；恢复最后负责人：`judex-server project transfer-owner --id ... --to-user ... --reason ...`。
- 所有运维操作写审计（actor_type=operator），不授予业务权。

## 已知限制
- 内置 SeaweedFS 单实例为可恢复起点，非 HA。
- Helm rollback 不回滚数据库迁移；升级失败走前滚修复或经验证的恢复流程。
