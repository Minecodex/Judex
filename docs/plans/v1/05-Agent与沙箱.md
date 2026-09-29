# 05 Agent 与沙箱

## 1. 首版运行拓扑

主 Agent 为项目 coordinator，按 topic 保有独立 session；岗位子 Agent 按 identity＋topic 独立 session。harness 在 server worker 进程，模型调用、PG 上下文、业务工具均在服务器。四工具派发至该 run 的 OpenSandbox 容器，一 run 一 sandbox。不得在业务 Server 主机执行模型 shell，也不主动调用成员电脑。

中心协调、允许独立子调用并行和逐个返回；子 Agent 请求其他岗位意见时返回 coordinator，不自由互聊或递归委派。已有任务优先分配身份，探索讨论按节点职责选合法参与者；参加讨论不自动获得任务分配。

## 2. 真实模型适配

定义 `ModelProvider.Stream(ctx, request) -> events`，规范事件 textDelta/toolCallDelta/toolCallReady/usage/finish/error，能力元数据 toolCalling/vision/contextTokens/streaming。首版实现一个经部署选定的 OpenAI-compatible 网关适配；必须核对实际网关兼容协议，不能因为名字相似假定全部工具行为相同。具体供应商、model ID、密钥由环境提供。

Provider contract tests 用受控响应检测拆包、无效 JSON、工具调用参数累积、usage 缺失、429/5xx、取消和截断；真实中文双场景测试另列，fixture 不代替真实模型。密钥仅从 Secret 引用读取，不注入 prompt、沙箱 env 或工具结果。

模型请求失败分类 transient/quota/auth/invalid_request/context_exceeded/cancelled；重试计入 attempts/预算。缺 usage 不记零成本，标 usageKnown=false，并保守预留预算。流断后若工具结果不确定，不盲目重放工具。

## 3. 触发、轮次与去重

触发来自已 ready 的新 submission、明确手动分析及允许的正式业务事件。由服务端 eventId/submissionId 建唯一 discussion_batch，固定 maxRounds 和预算。AI 自己生成的回复、系统 heartbeat、重复 webhook/CLI 请求不构成新提交，不能循环重置限额。

一轮 = coordinator 为同一问题安排岗位分析＋消费必要结果＋形成阶段摘要。多个模型调用、工具调用和子结果回流不各算一轮。第一次启动以事务预留 round ordinal；同一 round 重试不新增 ordinal，但所有调用/工具重试仍计入独立资源预算。每轮只能一个 coordinator session lease 持有者推进。

批次状态：queued/running/waiting_human/waiting_material/limit_reached/completed/failed/cancelled。到上限停止本批自动循环，保存草稿/异议/缺口，仍允许用户输入。项目修改上限只影响以后新批次；有权限的人显式续开需记录 extension 事件与额外额度，不伪装新材料。

新提交进入正运行 topic 时：保存独立 submission/batch，串行排队；V1 不做多个来源额度混合，避免合并事件绕过限额。后续可把多个输入合为工作集，但原批次预算与来源须可追踪。

## 4. 多层预算（工程初值、可配置）

| 层 | 初值与约束 |
| --- | --- |
| 项目讨论 | 3 轮，范围 1–100 |
| 每批模型尝试 | 30 次，所有重试计入；项目可降不可超部署上限 |
| 每批总 Token | 300000 预留上限，由 coordinator 与子运行共享；实际能力不足时拆读/压缩，不能超模型窗口 |
| 子运行同时执行 | 每批 2；同 identity/topic 串行，不同 session 并行 |
| 每工具 | 60s 默认、300s 最大、1MiB 输出内联截断；完整输出对象引用 |
| 每批壁钟 | 30min，不含明确 waiting_human；沙箱 TTL 独立限制 |
| 并发模型请求 | 部署级信号量＋PG 持久预算预约；等待子调用的父 run 不占模型名额 |

额度 reservations 在开始外部调用前入库，结束 reconcile；未知用量暂扣到核对完成，不能因断流退还全部。超限状态显示原因和合法续开入口，不自动批准业务或不断重试。

## 5. 上下文与隐私

每次模型调用按 manifest 构建：系统工具权限 → 当前已发布项目流程与节点 → 岗位职责 → 本人项目偏好（岗位 run 私有）→ 工作/决定事实 → 未决异议/缺料/子结果 → 历史摘要与近消息 → 新材料。

固定本调用 bindingVersion、workflowVersion、positionVersion、preferenceRevision、review/material/report refs、模型配置和消费游标。下一调用重新验证 current membership/binding；替换人后停止旧 run，不用旧私有上下文给继任者续接。

共享页面只展示岗位的对外分析、摘要、工具公开证据与结果；不公开原始私有系统 prompt、完整模型上下文或私密调试轨迹。coordinator 不读取子 Agent 私人偏好原文；子返回包区分 summary/evidenceRefs/disagreements/missing/partial/artifactRefs。

压缩只处理探索历史，保留消息序号范围、原文引用、正式决定、反对理由、待决、未返回子任务和下一步。不能只存摘要删除原文；不能把“摘要没写反对”当同意。若硬上下文无法放入窗口，标 context_blocked，不静默截断。

## 6. 工具注册与作用边界

每个工具声明 name/schemaVersion/inputSchema/outputSchema/effectClass/allowedPrincipal/limits。工具执行前服务端校验权限和 JSON Schema，工具结果是数据而非高权限指令。

| 工具 | 执行位置 | 必须约束 |
| --- | --- | --- |
| read/write/edit/bash | sandbox，经适配器调用 | 只访问当前 sandbox；project mount 只读；edit 校验 expected hash；命令无平台凭据 |
| list_agent | server | 仅本项目可发现 identity 和公开职责，无个人 prompt |
| call_agent | server | 只有 coordinator，合法绑定和节点；idempotency key＋已分配身份优先；无递归自由互调 |
| query_work/read_material | server | 按项目过滤，分页/范围读取，固定材料 version；不把全部项目复制进上下文 |
| propose_changes/propose_links | server | 只能创建草稿和差异，不提交人工 vote、不验收 |
| publish | server | 从当前 sandbox 提取文件，声明 source versions、target relative path、说明；S3+DB 新版本，来源落后冲突 |
| record_analysis | server | 公开结果经字段白名单，不夹带个人 prompt／凭据／私有工具日志 |

read/write/edit/bash 是分析工具，不授权平台执行真实生产部署、git push 或成员本地命令。沙箱即使运行测试脚本，也只能作为材料分析证据，不能替代用户定义的实际测试责任和人工验收。

## 7. OpenSandbox 适配与项目挂载

当前仓库锁定的 OpenSandbox chart/镜像见 `deploy/helm/UPSTREAM.md`。实施时复核其真实 Lifecycle/execd API；Go 适配优先正式 Go SDK（若锁定版本没有则由上游 OpenAPI 生成客户端＋契约测试），不捏造方法签名或加一个持密钥的沙箱 Runner。

Sandbox 接口至少 Create/Connect/Exec/Read/Write/Stat/List/Pause/Resume/Kill/GetStatus；每种不支持能力要显式返回，不能空实现成功。项目 sandbox image 从部署 allowlist 选择，固定 digest；默认通用 code-agent 镜像版本由 P0/PoC 锁定。

`/workspace/project` 只读挂本项目共享资料，其余 `/workspace` 为本 run 可写。可信基础设施负责项目 PVC/挂载，不把存储管理凭据暴露给 Agent 容器；serviceAccount 不可调用 K8s API，关闭 token automount，默认 deny egress（含元数据端点、PG/S3/控制面）。模型调用发生在 server，不需为沙箱开放模型密钥网络。

内置 SeaweedFS 与外部 S3 两条适配分别验收。历史 D15 只证明单节点 PoC；还需验证实际 S3 PUT→DB 登记→CSI 可读路径映射、桶 collection 容量、多节点与回收。动态新版本受目录缓存 TTL 影响：publish 后 material ready 不等于每个 sandbox mount ready；run 维护 preparing/available/failed，Stat 并核验固定 path/hash 后才交给模型。

一运行一沙箱不使用不兼容的 per-sandbox volume＋poolRef 组合。目录越界、symlink 和原件写入由挂载/权限强制；仅 prompt 写“不要写”不算隔离。

## 8. 生命周期、恢复和未知结果

run：queued→provisioning→running→waiting_children/waiting_human/waiting_material→succeeded/failed/cancelled。业务任务不跟随此状态自动完成。

等子结果保持 sandbox TTL 心跳，释放模型并发名额；长等人决定尝试 pause，有能力限制则先保存允许恢复的工作产物清单再回收，下次重建。临时目录丢失要说明，不宣称所有 shell 进程可原样恢复。

执行前持久 tool_call(prepared) 和 fencingToken；调用外部后持久结果。server 重启发现 running 无最终结果时标 unknown，先查沙箱任务状态/产物摘要；可证明未执行再重试。发布类工具用业务幂等恢复同 version，bash 的未知副作用不自动重跑。

取消显式传播到 provider、工具和所属子调用；已提交材料/人工决定保留。lease 接管者不能和旧 worker 同时续接 session。cleanup jobs 对 sandbox Kill 幂等，删除 sandbox 不删除项目 PVC；扫孤儿只处理带本部署 ownership 标签的对象。

## 9. 首版记忆

先实现 PG 存储的结构化摘要、关键词/标题/对象引用查询、knowledge candidates，中文以实际材料测试。未确认经验标 unverified，输出附来源和适用范围。跨项目自动抽取、向量服务和长期自学习不作为 V1 必装组件。

## 10. 验收证据

P5 的真实集成可先用本地/隔离 namespace 的 Judex API＋PG/S3 fixture，连接临时 OpenSandbox 和真实模型；P5-02 要交付该 fixture 启动方式，不依赖尚未完成的 P7 全量 Helm 发布。这保证真实 Agent 验收不会与最终部署验收形成循环依赖。P7 再用正式 chart 重做整套联调。

研发与设计两套中文材料，至少双岗位＋coordinator；真实工具读取附件、浏览器渲染 HTML 并引用截图；部分子失败保留其他结果；压缩后异议不丢；轮次/资源超限；模型断流；server/sandbox 重启；成员撤回；项目隔离；发布源版本冲突。所有结果关联 runId、model request ID（可取得时）、toolCallId 与 artifactVersionId；日志脱敏后留证。

## 2026-09-29 实现校准

为真实多岗位中文材料保留足够输入预约，批次 token 初值调整为 300000，并将独立子调用并行数收紧至 2。调用循环、PG 预约和显式续开共用 runner 常量；父运行最多 30 分钟、子运行最多 5 分钟。当前这些资源初值是服务端常量，尚未开放部署配置，不能声称已有全部可配置项。

探索工具的大结果替换为持久原文引用和 SHA256，通过 read_context 在同项目/session/binding 下分段回读；人类输入、岗位意见及硬条件保持原文。检查点不保存私有系统消息。恢复只续接已完成的服务端工具记录；未知副作用、未返回子运行和丢失的沙箱工作状态均要求人工核对。更一般的长对话语义摘要恢复仍属验收缺口。

write/edit 只写本 run 的 /workspace 且排除项目只读目录；edit 要求旧内容 SHA256。publish 由服务端核验固定来源版本及租约，写入对象存储后同事务登记材料、版本、来源、审计和事件；幂等键限于当前 run，同键异内容拒绝。来源变更返回冲突，不自动覆盖。
