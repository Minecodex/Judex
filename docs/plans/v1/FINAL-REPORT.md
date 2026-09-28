# Judex V1 最终验收报告

生成：2026-09-28。基线 8edd258 → 71ac7b9（54 个任务提交，只 commit 未 push）。

## 实现范围（56 任务）

- **P0 契约与持久底座 5/5**：OpenAPI 契约源（103 路径/119 操作 + Go/TS 生成链 + 路由双向一致性测试）、M001–M010 十批迁移、pgx+固定锁序事务、幂等协议（同键重放/异载荷 409）、持久 jobs（SKIP LOCKED/租约/fencing）、outbox+项目事件序列、--mode=all|api|worker|migrate 装配。
- **P1 注册登录与项目 7/7**：Argon2id+渐进 rehash、PG 共享限流、__Host-cookie+CSRF+Origin、恢复码/停用/owner 转移运维命令、项目原子创建/归档、成员/owner 双人转移、React Router 认证路由+账户安全页。
- **P2 职责流程与材料 8/8**：职位版本/身份绑定历史/一次性 token 邀请、结构化流程发布（Mermaid 生成不可逆）、模型目录（密钥不出服务端）、S3 分片上传 exactly-once、HTML 预览隔离、议题/统一提交（clientSubmissionId 幂等）、SSE、前端三面板。
- **P3 工作审批交接 10/10**：M004 全 24 表、typed 提案+SHA256 冻结审阅+ALL 会签+原子应用、代批/修订/换人接续、首票倒计时+超时结算、分来源交接（拒收→rework/已验收保护）、验收快照+任务/计划验收/重开、Bug/仓库/发布/修复传播、统一待办、前端工作面板。
- **P4 CLI 与 Skills 7/7**：RFC8628 设备授权（scope 白名单/refresh 轮换/重放撤族）、cobra 命令树+0600 凭据、一次性人工确认意图（nonce+payload 快照+同事务执行+恰好一次）、CLI 上报/提案/交接/验收经确认、发行 Skill+install/uninstall、6 平台构建+checksums+doctor。真实垂直验证：设备登录 E2E、真实 PG+MinIO 上传。
- **P5 平台 Agent 9/10 + 1 blocked**：ModelProvider 流式适配（受控网关契约测试：拆包/无效JSON/工具累积/usage/429/5xx/取消）、OpenSandbox 适配器、受限工具注册表、分层上下文（私有层过滤/压缩保异议）、runner 模型循环+预算熔断、M005 迁移、批次/轮次表、取消/unknown 恢复语义。**blocked：真实模型验收（E11/live）——部署方未提供 baseUrl/apiKey/model 配置。**
- **P6 前端收口 4/4**：demo 工作区 lazy 分包隔离、决定门控+CSRF 时序修复、**双用户生产 E2E 5/5 通过**（A01/A04 注册→项目→隔离、A02 防枚举、F03 无 demo 残留+API 404 不回 SPA、B03/B06 提案→会签→生效、B16 退回终态）、10k 分页测量+22 条 demo E2E 回归。
- **P7 部署与发行 4/5 + 1 blocked**：Helm 全量接线（JUDEX_MODE/ORIGINS/PREVIEW/DATABASE_URL $(VAR) 组合/模型 Secret）+迁移 Job、**真实 K8s 安装闭环**（embedded PG+S3+migrate Complete+注册 201+pod 重建数据不丢）、备份/恢复 runbook、发布制品。四存储组合模板全绿；外部三组合真实安装与完整故障演练 **blocked（集群资源）**。

## 通过的证据（摘要）

- go test ./tests/...：agent 10 + backend + contract + deploy + integration 40+ 全绿
- 生产浏览器 E2E 5/5（business.config.ts，真实 PG+MinIO+生产 build）
- demo E2E 22/22 回归；web 单测 33/33
- 真实 K8s：helm install→migrate→register→restart→login 全链路
- CLI：status/doctor/项目选择/材料上传（真实 MinIO）实测

## 未验证项（blocked，恢复条件明确）

| 项 | 缺失条件 | 恢复方式 |
| --- | --- | --- |
| P5-10 真实模型中文材料验收（E01-E11） | JUDEX_MODEL_CATALOG_FILE（baseUrl/apiKeyEnv/model/maxIn/maxOut） | 提供配置后跑 tests/agent/live（研发/设计双材料、双岗位+coordinator、HTML 浏览器证据、断流/重启注入） |
| P7-04 完整故障演练（API/worker/DB/S3/model/sandbox 注入） | 集群资源 + 真实模型/沙箱配置 | 集群空闲时执行 tests/k8s 完整注入矩阵；沙箱需 JUDEX_SANDBOX_* |
| 外部 PG/S3 三组合真实安装 | 外部服务实例 | values-external 提供后 helm upgrade + 冒烟 |
| Skill 宿主实测（Codex/Claude Code） | 实际宿主环境 | judex skill install 后在各宿主跑 SKILL 流程 |

## 已知业务限制

- 单实例可恢复部署，不承诺 HA；RPO/RTO 以 docs/operations.md 实测记录为准。
- 独立 preview origin 需部署配置 JUDEX_PREVIEW_ORIGIN，未配置时不提供交互式 HTML 预览（诚实降级）。
- 讨论批次轮次/预算为工程初值（3轮/30次/100k token/30min），部署可调。
- 内置 SeaweedFS 单实例为可恢复起点。

## 演示入口

- 生产模式：`go run ./cmd/judex-server`（+PG/S3 env）托管 web/dist；开发 `npm run dev`（5173，demo fixture）。
- CLI：`bin/judex.exe --server http://... status | auth login | project list`。
- 恢复执行：提供上述任一 blocked 条件后，按对应恢复方式补跑并更新 progress.json。
