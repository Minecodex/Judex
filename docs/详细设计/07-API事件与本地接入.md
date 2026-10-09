# 07 API、事件与本地接入

2026-09-27 具体实施合同、命令与生成要求已细化于 [V1 API](../plans/v1/06-API契约.md) 和 [CLI／Skill](../plans/v1/07-CLI与Skills.md)。下列为早期契约草案，与新计划冲突时以最新确认和计划为准，均不代表已上线。业务权限和决定语义沿用已确认规则，不以端点存在证明能力已实现。

## 1. 通用约定

- 建议前缀 `/api/v1`，JSON 请求 / 响应；文件原件走独立上传会话或签名地址。
- 资源请求明确 project_id，并与授权主体的访问范围交叉校验。
- 写入命令携带 expected_version / review_version 及 Idempotency-Key。
- 返回 request_id、服务端时间和可程序识别的错误码；客户端不解析自然语言错误来决定重试。
- 私有资源无权限时按统一策略返回不存在或拒绝，避免无意暴露项目存在性。
- Q074 的项目共享业务文件统一按项目访问资格读取，不再增加逐文件审批 / 白名单；私人配置和私有执行数据走独立权限路径。
- 普通用户、本地授权客户端、沙箱执行身份使用可区分的认证来源和能力集合。

## 2. 接口分组草案

| 分组 | 代表端点 | 说明 |
| --- | --- | --- |
| 注册登录 | POST /auth/register、/auth/login、/auth/logout | 内网邮箱＋密码自由注册，首版不依赖邮件验证（2026-09-27 确认） |
| 项目 | GET/POST /projects | 返回可见项目；创建当前人的项目成员关系 |
| 团队 | /projects/{id}/members、/positions、/identities、/invitations | 管理动作受项目权限限制 |
| 我的偏好 | /projects/{id}/me/preferences | GET 返回本人当前任职的独立提示词列表；PUT 指定 positionId/expectedRevision，只修改该职位，不作为可公开团队字段 |
| 工作安排 | /projects/{id}/plans、/tasks、/proposals | 创建 / 修改正式安排使用草稿与决定路径 |
| 任务执行 | POST /tasks/{id}/reports、/acceptances、/reopens | 区分本人上报、最终验收与重开 |
| 计划验收 | POST /plans/{id}/acceptances | 计划负责人按快照整体验收 |
| 讨论 | /projects/{id}/topics、/topics/{id}/messages | 普通消息不直接触发人工批准 |
| 审批 | GET /me/actions、POST /proposals/{id}/decisions | 索引与讨论内卡片共用同一对象 |
| 交接 | /handoffs、/handoffs/{id}/sources/{sourceId}/send、/decisions | 各来源独立确认与修订 |
| 文件 | /uploads、/uploads/{id}/complete、/materials/{id}/versions | 原件登记、校验和访问 |
| 材料发现 | /projects/{id}/materials、/materials/{id}/versions/{versionId}/content | 候选查询 / 分段读取契约；授权目录、分页、固定版本与读取范围不能省略 |
| 流程 | /projects/{id}/workflows、/workflows/{id}/publish | 规则、图、环节说明同版本 |
| Agent | /topics/{id}/runs、/runs/{id}/cancel | 显式触发 / 取消；线上事件也可调度 |
| 实时 | GET /projects/{id}/events | SSE，按当前权限过滤 |

服务端可选择更细的 URI 结构；不能把所有对象都暴露为无需领域校验的通用 PATCH。

## 3. 正式决定请求示意

```json
{
  "reviewVersion": "review-version-id",
  "expectedObjectVersion": 7,
  "decision": "approve",
  "slotIds": ["slot-id"],
  "reason": "",
  "source": "web"
}
```

`source` 是展示 / 审计输入，最终可信来源由认证链路推导。请求中的 user_id、role、human_confirmed 等自报字段不能证明授权。

代批请求明确覆盖哪些审批位，服务端核验代批资格。退回 reason 必填。授权与依据过期返回冲突，不能自动批准服务端最新版本。

建议错误码：UNAUTHENTICATED、FORBIDDEN、NOT_FOUND、VERSION_CONFLICT、DEPENDENCY_CHANGED、REQUIREMENT_UNMET、INVALID_TRANSITION、IDEMPOTENCY_CONFLICT、UPLOAD_INCOMPLETE。具体 HTTP 状态和结构在 OpenAPI 中固定。

## 4. SSE 与恢复

建议事件信封包括 event_id、project_id、object_type/id、object_version、event_type、occurred_at 及最小 payload。

事件包括 message.delta、message.committed、run.progress、run.finished、proposal.changed、handoff.changed、task.changed、plan.changed。名称属于协议方案，正式变更须版本化。

客户端使用事件 ID 恢复并按 ID 去重。事件保留窗口之外，明确返回需要重新拉取快照，不能默默遗漏。重连时重新检查项目资格及私有数据权限，不继续复用失效订阅范围；共享材料事件不逐文件授权过滤。

模型流式片段与最终消息区分：最终记录以 committed 事件和查询结果为准。普通状态变更可使相关 TanStack Query 缓存失效，不在多个前端 Store 中各自猜测新状态。

## 5. CLI + Skills

本地用户主动调用 Skill 获取事项，平台不推送命令去启动其电脑。CLI 与 Web 共用业务命令服务和身份边界，不拥有隐藏的越权接口。

候选能力：认证、列出本人项目 / 事项、读取上下文、上传原始材料、上报本人工作、提交提案、确认发送、接收 / 退回，以及本人明确同意具体事项后的审批代提交。

命令名、安装包、Token 流程和模型工具适配尚未实现，不将示例命令写成已可运行说明。

“取得待办”与“授权审批”分别处理。正式代提交必须绑定事项、版本、操作者与本轮明确意图；长期客户端凭据不能单独证明本轮有人亲自审阅。D10 已于 2026-09-27 由用户明确：CLI 准备正式命令后，浏览器审阅并确认一次，CLI 查询同一决定结果。采用绑定事项、版本、payload 与操作者的一次性确认意图；不以长期授权或一个布尔字段代替人类确认。

## 6. 文件自由表达与最小上下文

不强制所有领域填写同一报告模板。上报允许文本、文件及结构化补充信息，但仍需最小可信元数据：所属项目、认证主体、代表身份、时间、来源、幂等提交 ID 和可选投递对象。

不指定任务或议题时先登记原件，再由 AI 提出关联 / 新议题草稿供人确认。解析失败不抹掉原件，不把上传成功视为已经理解或接受。

同一次提交的文字与文件包有共同 submission_id；完成上传与材料登记后发布材料可用事件。Agent 的工作区临时写入不自动广播；发布新共享草稿的唯一入口是 publish 业务工具（声明来源版本、产物路径与说明，服务端执行登记）。来源版本落后于当前最新版时返回冲突错误，要求取最新版重新修改后再发布；同一发布的幂等重试返回同一新版本，不能写回覆盖原件。

## 7. 沙箱执行与 OpenSandbox 集成

原“沙箱内 Runner 回传事件”链路已随 2026-09-25 执行架构修订取消。harness 位于服务端，经 OpenSandbox SDK 对本次运行的沙箱执行命令与文件读写；沙箱不回调业务接口、不持有任何平台凭据，命令结果与退出状态由 SDK 同步返回并写入 run_events / tool_calls。运行有效性、绑定与访问范围的核验全部在服务端完成。

沙箱内不产生需要去重的异步回传；命令结果不确定时（超时、连接中断）按“结果未知”处理，先检查产物与幂等记录，不盲目重跑。正式业务变更仍经独立命令与人的决定。
