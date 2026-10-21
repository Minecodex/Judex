# V2 第二次完成度复核

日期：2026-10-20 10:05（Asia/Shanghai）。检查正式版本 `0.2.0-local.20261020.v2.11` 及对应工作区源码。

后续闭合：**R6～R9 已在 v2.12 修复并重新完成同版验收、正常升级和清理**，见[最终收尾](FINAL-FOLLOWUP-20261020.md)与[机器记录](final-followup-acceptance-20261020.json)。以下“未修复／仍需收尾”均为本次复核发现时点，不代表当前部署状态。

**结论：主体重构与本机切换已完成，整体计划仍需收尾。** R1～R5 的修复和此前 131 项候选／13 项升级检查继续有效；本轮在未覆盖的路径确认了 4 项缺口，撤回上一轮「P0～P7 全部完成」的整体判断。不能用套件通过数量替代逐项行为审计。

本机服务确实运行 v2.11、协议 2，五个主要工作负载均 1/1 就绪。当前源码、发行清单和上一轮验收的 sourceTreeHash 均为 `a22372fcffbd0f68318059179544254ff3220b457b281394e3b0ab1d250f5414`，因此这些发现适用于当前实现，不是部署了旧版本。

## R6：缺少 AI 请求授权的 CLI 仍可通过消息提及创建岗位运行

优先级：高。状态：**原生 CLI＋真实 HTTP／设备授权已复现，未修复**。

[commands.go:32](../../../../internal/transport/http/commands.go#L32) 仅让 `pushGroup` 检查 `agent:request`，`postMessage` 只检查 `messages:write`。[messages.go:184](../../../../internal/collab/messages.go#L184)收到岗位提及后直接投递，没有核验 CLI 是否拥有 AI 请求能力。

复现采用真实设备登录，批准的 scope 只有 `projects:read`、`context:read`、`messages:write`，并收窄到一个合成项目。直接 `push` 得到 HTTP 403；普通消息没有创建讨论；随后使用 `message send --mention-ai ROLE_ID` 却成功创建 active Discussion 和 pending RoleRun。本轮服务为 API-only，未执行该岗位或消耗真实模型额度；已经证明调用可越过授权进入执行队列。

影响：设备授权中取消 AI 请求能力，不能真正阻止该设备触发模型。需要在服务端按消息的实际 AI 副作用统一鉴权，覆盖普通发言、结构化提及、本地推送、只记录和幂等重放，不能仅依赖 CLI 界面或一个路由。

完成条件：缺少 `agent:request` 时，岗位提及和会触发 AI 的本地推送均被拒绝且不留下投递；普通文字、@人、明确回复仍遵守各自授权和零触发规则；增加缩小 scope 的原生 CLI 与 HTTP 回归。

证据：`.cache/v2-review-2/http-scope.json`、`http-scope.log`、`server.log`；复现入口 `.cache/v2-review-2/http-scope.mjs`。

## R7：缩小上下文窗口后，只有旧摘要的重压缩发生越界 panic

优先级：高。状态：**真实隔离 PostgreSQL 已复现，未修复**。

[context.go:121](../../../../internal/collab/context.go#L121) 在 `entries` 为空时仍强制 `keepStart=1`，随后执行 `entries[:1]`。该状态合法：先前压缩可以覆盖全部原文，仅保留公共摘要；模型或指导配置变化可缩小公共预算，而配置变化本身不新增群知识。

复现先通过正式 Post／Snapshot 路径生成 3 条较长消息及已覆盖全部历史的公共摘要，确认 `Entries=0`、`CoveredThrough=SourceSeq`。再经项目配置接口将有效窗口字段切换为 20000，未追加知识，调用 Snapshot。捕获到 `runtime error: slice bounds out of range [:1] with capacity 0`，没有进入正常重压缩／明确暂停分支。

影响：已有执行或并发岗位读取同一群、配置变化后重新读取公共快照时可能遇到这一状态。岗位运行从 [runtime.go:98](../../../../internal/agent/peer/runtime.go#L98) 调用 Snapshot，队列 [job.go:316](../../../../internal/job/job.go#L316) 没有 panic 恢复；**据此判断存在服务进程退出风险**。本轮在测试中捕获 panic，没有让正式进程崩溃，也未将此风险描述为已观察到的正式故障。

完成条件：摘要重压缩必须支持零条未覆盖原文，保留覆盖范围和所有原文引用；缩小模型窗口及增大提示词预算后应重新压缩或明确暂停，不能 panic；补多岗位、配置变化及小窗口回归。

证据：`.cache/v2-review-2/database-reproductions.log` 中 `TestV2SecondReviewCompressionAfterSmallerModel`。

## R8：保留群成员但撤销其岗位后，等待讨论没有自然结束

优先级：中。状态：**真实队列／岗位执行＋隔离 PostgreSQL 已复现，未修复**。

[groups.go:185](../../../../internal/collab/groups.go#L185) 将被取消任职的岗位设为 inactive，再由 [cancelInactiveRoles](../../../../internal/collab/groups.go#L235)取消其投递和未完成运行，但这一分支没有调用 Settle。移除整位群成员走的 revokeRoles 包含 Settle；两条路径行为不一致。

复现让受控岗位通过实际 send_message 请求一个人回应，使用队列执行完成该岗位，进入 waiting_human。随后仅撤销发问者的岗位、保留其群成员资格。结果：相关投递均已取消、未完成投递数为 0、queued／running 后台任务数也为 0，Discussion 仍是 active。完成过的后台任务不会自动再次领取，因此没有后续任务替它收束。

影响：界面可能一直显示岗位正在工作；后续显式触发会并入已经没有活动分支的旧讨论，沿用旧闭环记录／额度，违反「全部分支结束后自然收束，之后的新触发新建讨论」。

完成条件：统一成员和任职撤销后的讨论收束，保留其他仍有效的分支；撤销最后一个活动／等待岗位后自然结束，下一次触发创建新讨论；覆盖等待人工、运行中与待执行状态。

证据：`.cache/v2-review-2/database-reproductions.log` 中 `TestV2SecondReviewUnassignedWaitingRole`。该用例实际经过队列 Claim／Execute／Finish，确认没有未处理后台任务。

## R9：现有模型额度回退配置未接入岗位执行器

优先级：中。状态：**受控 Provider＋真实队列／隔离 PostgreSQL 已复现，未修复**。

模型目录仍接收 `quotaFallbackIds`，[resolver.go](../../../../internal/agent/resolver.go#L58)也构造 WithQuotaFallbacks。但是 [quota_fallback.go:15](../../../../internal/infrastructure/model/quota_fallback.go#L15) 的 Stream 只调用首个 Provider，需要执行器读取 QuotaTargets；当前 peer 执行器在 [runtime.go:137](../../../../internal/agent/peer/runtime.go#L137)直接调用 Stream，没有消费回退目标或处理明确额度错误。旧模型适配能力只剩配置与目标封装。

复现构造包含主、备两个目标的同一适配器，主目标返回明确的 `model.ErrorQuota`，备用目标可正常完成。经真实岗位 ExecuteJob／队列 Finish 后，主目标调用 1 次、备用目标 0 次，讨论被直接暂停为 `model_failed`。不是把普通限流 429 误认成额度耗尽的测试。

影响：项目保留的备用模型配置不生效，主模型额度耗尽时无法按已有配置继续。对应原计划保留模型适配基础设施和执行恢复的范围，不应以接受配置代替实际执行。

完成条件：在对等执行器中恢复明确额度错误的回退选择，普通限流／认证错误不随意换模型；保留每次真实调用、预算和未知成本记录，不重置闭环；处理备用模型窗口，并补目录解析到执行的受控联测。

证据：`.cache/v2-review-2/quota-fallback.log` 中 `TestV2SecondReviewConfiguredQuotaFallback`。

## 阶段与剩余完成条件

| 阶段 | 本轮结论 |
| --- | --- |
| P0／P1 | 合同基线、干净 V2 数据、认证保持及既有迁移证据有效 |
| P2 | 项目／群主体已落地；任职撤销后的收束联动需补 R8 |
| P3 | 消息、回复、私聊和 SSE 已落地；CLI AI 副作用授权需补 R6 |
| P4 | 文件版本、授权和预览证据有效；动态公共压缩需补 R7 |
| P5 | 多岗位主流程已落地；R7、R8、R9 尚未完成 |
| P6 | 正式 UI 与 CLI 主体已落地；需随后端修复补授权／状态联测 |
| P7 | 本机切换和上一轮清理已完成；修复四项后须再做最终同版验收和清理 |

建议收尾次序：R7 防止进程异常退出 → R6 统一授权 → R8 统一撤销／收束 → R9 恢复模型回退 → 对最终同版候选补完整回归。完成前保持「整体需收尾」，R1～R5 不因新增缺口而重新标成未修复。

## 证据、范围与清理

本轮是源码审计与定向隔离复现，不冒充重新执行全部 Kubernetes／浏览器／真实模型验收。3 个 PostgreSQL／队列用例和 1 个真实 HTTP／原生 CLI 用例确认缺口；测试中的 PASS 表示成功复现已知问题，**不表示满足产品合同**。

API-only 复现服务使用当前源码另行构建，运行于独立 PostgreSQL 和随机 loopback 端口；岗位不执行。数据库用例使用现有 setup 为每个测试独立创建／删除数据库，Provider 为受控夹具。未调用真实模型、未使用生产凭据。

临时 Go 测试源完整副本保存在 `.cache/v2-review-2/audit_test.go.txt`，执行后已从正式源码移除。两个带 `judex.owner=v2-review-2` 和本轮 token 标签的 PostgreSQL 容器核对归属后均已删除，临时 HTTP 服务及原生 CLI 配置也已回收，见 `.cache/v2-review-2/cleanup.json`。本轮没有修改或升级正式业务代码、没有操作真实账号或项目，正式部署仍为 v2.11。
