# 运维手册（P7-03）

## 备份
- 范围：PG 全库（pg_dump -Fc）+ S3 对象清单；SeaweedFS 元数据随 StatefulSet PVC 一并快照。
- 频率与保留由部署方决定；每次备份生成 `manifest.json`（cutoff 一致性时间戳）。
- `scripts/operations/backup.sh <ns> <release> <dir>`

## 恢复
1. `scripts/operations/restore.sh <ns> <release> <dir>/<cutoff>`（pg_restore --clean + 计数校验）。
2. S3 对象回放（mc mirror 或 seaweed volume 恢复）后核对 material_versions.sha256。
3. 记录实际 RPO/RTO 与限制（11 §6：不承诺未实测的可用性等级）。

## 账号运维
- 恢复码：`judex-server account recovery-code --email ... --reason ...`（15 分钟一次性）。
- 停用：`judex-server account disable --email ...`；恢复最后负责人：`judex-server project transfer-owner --id ... --to-user ... --reason ...`。
- 所有运维操作写审计（actor_type=operator），不授予业务权。

## 已知限制
- 内置 SeaweedFS 单实例为可恢复起点，非 HA。
- Helm rollback 不回滚数据库迁移；升级失败走前滚修复或经验证的恢复流程。
