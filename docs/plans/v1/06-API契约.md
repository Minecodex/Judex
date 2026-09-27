# 06 API 契约

本章是必须落实到 `api/openapi.yaml` 的接口合同。实现开始前生成规范、请求样例和客户端；此文件不是已上线接口说明。路径前缀统一 `/api/v1`，下文 `{p}`=projectId，所有业务对象仍在 SQL 校验 projectId。禁止仅凭对象 ID 命中就跳过项目鉴权。

## 1. 协议规则

JSON 字段 camelCase；UUID 字符串；时间 RFC3339 UTC；版本 positive integer。分页 `?limit=50&cursor=opaque`，范围 1–100，稳定 `(created_at,id)` 或 message.seq keyset；cursor 编码 scope/filter/hash，跨项目或变换筛选不得误用。

成功 envelope：`{data, meta:{requestId,serverTime,eventCursor?}}`；列表 data=`{items,nextCursor}`。命令还返回 `affected:[{type,id,version}]`。201 新资源、200 命令结果、202 异步接受；收到 202 仅表示登记，状态由 operation/run 查询，不能显示最终完成。

错误：`{error:{code,message,details?,retryable},requestId}`。message 只作兜底，前端/CLI 根据 code 和结构化 details 映射双语文案，不能解析句子控制流程。字段错误 details.fields=`[{path,code}]`。

所有有副作用业务命令要求 `Idempotency-Key`（客户端生成 UUID）；已有对象还要求 expectedVersion，正式决定要求 reviewId/hash。Web 用 Cookie＋CSRF，CLI 用 Bearer access grant，模型内部用受限 Principal 不向沙箱下发 Bearer。

| HTTP | code（最低集合） | 客户端行为 |
| --- | --- | --- |
| 400/422 | VALIDATION_ERROR / UNSUPPORTED_SCHEMA / INVALID_REFERENCE | 保留草稿，标字段；不自动重试 |
| 401 | UNAUTHENTICATED / SESSION_EXPIRED / GRANT_REVOKED | 清认证缓存，重新认证；不回落演示 |
| 403 | FORBIDDEN / HUMAN_CONFIRMATION_REQUIRED | 已有项目内无动作权限，展示原因/确认链接 |
| 404 | NOT_FOUND | 对非成员隐藏项目资源存在性 |
| 409 | VERSION_CONFLICT / REVIEW_STALE / DEPENDENCY_CHANGED / INVALID_TRANSITION / IDEMPOTENCY_CONFLICT / SOURCE_VERSION_CONFLICT / EVENT_CURSOR_EXPIRED | 重取差异；原批准不能自动套到新版本 |
| 412 | REQUIREMENT_UNMET | details.blockers 列明前置对象/阶段/原因 |
| 413/415 | PAYLOAD_TOO_LARGE / UNSUPPORTED_MEDIA | 提示 limit/类型 |
| 429 | RATE_LIMITED / BUDGET_EXHAUSTED | Retry-After；预算需合法续开，不无限重试 |
| 503 | DEPENDENCY_UNAVAILABLE / MODEL_UNAVAILABLE / SANDBOX_UNAVAILABLE | 保留已登记内容，retryable 明示 |

## 2. 认证、账户与 CLI 授权

| Method / path | 输入 → 输出 | 资格 |
| --- | --- | --- |
| GET /system | version/schema/公开 capability 状态 | 匿名，禁止返回密钥和私有拓扑 |
| GET /capabilities | 注册规则、上传限制、确认机制、协议版本 | 匿名公开最小值；登录后扩展可用能力 |
| POST /auth/register | displayName,email,password → user/sessionExpiry＋Cookie | 匿名＋Origin/限流 |
| POST /auth/login | email,password → user/sessionExpiry＋Cookie | 同上 |
| GET /auth/session | → user,expiresAt,csrfToken | Web 会话 |
| POST /auth/logout | → revoked=true，清 Cookie | Web，会话已过期也允许清本地 Cookie |
| POST /auth/logout-all | expectedAuthVersion → revoked counts | 本人 Web，明确全设备范围 |
| PATCH /me | expectedVersion,displayName,locale → user | 本人；改邮箱首版不开放 |
| POST /me/password | currentPassword,newPassword → rotated session | 本人 Web，撤其他会话/grants |
| POST /auth/recover | recoveryCode,newPassword → completed | 限流，一次性码；不自动登录 |
| GET /me/sessions | → 安全会话摘要 | 本人 Web，无 secret |
| DELETE /me/sessions/{id} | → revoked | 本人 |
| POST /auth/device/authorizations | deviceName,requestedScopes,projectScope → deviceCode,userCode,verificationUri,expiresIn,interval | CLI 公共客户端，限流 |
| POST /auth/device/token | deviceCode → pending/slow_down/denied/expired 或 token pair | deviceCode 持有者，禁止记录完整 code |
| POST /auth/device/confirm | userCode,approved,scopes/projectScope → state | 本人 Web＋CSRF，显示设备和权限 |
| POST /auth/token/refresh | refreshToken → rotated pair | grant 当前有效，旧 token 重放撤销族 |
| GET /me/client-grants | → devices/scopes/expiry | 本人 Web |
| DELETE /me/client-grants/{id} | → revoked | 本人 Web；CLI auth logout 撤销自己的 grant |

device/token 使用标准设备流程的 pending/slow_down 语义；若 endpoint envelope 不完全符合 RFC，不宣传成可接所有 OAuth SDK 的完整 OAuth Server。生成客户端准确描述此差异。

## 3. 项目、成员、职责和流程

| Method / path | 输入／效果 | 资格 |
| --- | --- | --- |
| GET /projects | 本人的项目分页，包含 role、summary | 已登录／projects:read |
| POST /projects | title,description,kind,初始配置 | 已登录真人 Web；CLI 创建需 07 意图确认 |
| GET /projects/{p} | 公开项目配置＋本人 capabilities | active member |
| GET /projects/{p}/bootstrap | 基础概览、当前人身份、首屏摘要、eventCursor | member，不能返回所有消息和私有偏好 |
| PATCH /projects/{p} | expectedVersion,title/description/AI limits/defaultModel | manager；owner 专属项另命令 |
| POST /projects/{p}/archive /restore | expectedVersion,reason | owner；两个独立 endpoint |
| POST /projects/{p}/owner-transfer | targetUserId,expectedVersion → ownershipTransfer | owner；目标本人确认后生效 |
| GET /projects/{p}/owner-transfers/{id} | from/to/expiry/state，当前可操作项 | 转出或转入本人 |
| POST /projects/{p}/owner-transfers/{id}/decisions | expectedVersion,accept/decline；接受时锁项目并重新校验双方资格 | 转入本人 Web 或 CLI 确认意图 |
| GET /projects/{p}/members | 成员和公开职责分页 | member |
| PATCH /projects/{p}/members/{u} | expectedVersion,role=manager/member | owner，不移除最后 owner |
| POST /projects/{p}/members/{u}/remove | expectedVersion,reason → affected responsibilities | manager，不移除 owner，不静默遗留职责 |
| POST /projects/{p}/leave | expectedVersion | 本人，先处理活动职责 |
| GET/POST /projects/{p}/invitations | 分页／targetEmail,positionIds | manager |
| POST /projects/{p}/invitations/{id}/revoke | expectedVersion | manager |
| GET /me/invitations | 本人邮箱匹配待接受邀请 | 登录 |
| GET /invitations/resolve?token=... | 精简邀请预览；token 不日志 | 登录且匹配受邀邮箱 |
| POST /invitations/{id}/accept /decline | token?,expectedVersion | 受邀本人；两个独立命令 |
| GET/POST /projects/{p}/positions | 列表／职位草稿（名称、职责、节点绑定） | member 读、manager 写 |
| PATCH /projects/{p}/positions/{id} | expectedVersion,fields → new positionVersion | manager |
| GET /projects/{p}/identities | 稳定身份、当前公开绑定和职责 | member |
| POST /projects/{p}/identities | positionId,userId → 新任职 | manager，成员必须有效 |
| POST /projects/{p}/identities/{id}/replace | expectedBindingVersion,newUserId,reason | manager，事务生成新绑定 |
| GET/PUT /projects/{p}/me/preferences | 本人 prompt／expectedRevision,prompt | 本人，CLI 仅自己的 scope |
| GET /models | 公开模型目录与能力，无 credentials | 已登录 |
| GET/POST /projects/{p}/workflows | 列表／新 draft | member 读、manager 写 |
| GET /projects/{p}/workflows/{id}/versions | 已发布版本与公开差异 | member |
| PUT /projects/{p}/workflows/{id}/draft | expectedVersion,结构化规则 | manager，AI 只能另建草稿 |
| POST /projects/{p}/workflows/{id}/publish | expectedVersion,draftHash | manager 人工确认 |
| GET/POST/PATCH /projects/{p}/repositories[/{id}] | 元数据、expectedVersion；不操纵 Git | member 读、manager 写；规范中展开为独立路由 |

上表用 `GET/POST` 表示同一路径两种 method；`/archive /restore` 等必须在 OpenAPI 分列，不生成包含空格的路径。

## 4. 讨论、材料和事件

| Method / path | 合同 |
| --- | --- |
| GET/POST /projects/{p}/topics | 分页／title,initialMessage?,links?；显式用户新建，AI 走建议 |
| GET /projects/{p}/topics/{t} | 标题、关联、本人可操作项 |
| GET /projects/{p}/topics/{t}/messages | before/afterSeq,limit；提交记录和正式决定卡引用 |
| POST /projects/{p}/submissions | clientSubmissionId,purpose,text,materialVersionIds,topicId?,taskId?,identityId? → submission＋message/report refs |
| POST /projects/{p}/topics/{t}/links | targetRefs,expectedVersion；用户明确链接，AI 用 proposal |
| GET/POST /projects/{p}/uploads | 本人未完成会话／file manifest → upload plan |
| PUT /projects/{p}/uploads/{id}/parts/{n} | 二进制流、分片摘要；同编号不同摘要冲突 |
| POST /projects/{p}/uploads/{id}/complete | parts/checksum/sourceVersion? → immutable materialVersion |
| DELETE /projects/{p}/uploads/{id} | 取消未完成会话，不删正式材料 |
| GET /projects/{p}/materials | metadata、query、kind、关联过滤、分页 |
| GET /projects/{p}/materials/{m}/versions | immutable 版本列表 |
| GET /projects/{p}/materials/{m}/versions/{v}/content | 附件下载，Range；HTML 不直接同源执行 |
| POST /projects/{p}/materials/{m}/versions/{v}/preview-session | → 隔离 origin 的短时预览能力 |
| GET /projects/{p}/events | SSE，after/Last-Event-ID；见 04 |
| GET /me/actions | 项目过滤、类型过滤、分页；服务器计算 capabilities |
| GET /me/notifications | 本人站内提醒分页 |
| POST /me/notifications/read | ids/cursor → read 标记，无业务状态变化 |

## 5. 工作与决定

| Method / path | 合同／权限 |
| --- | --- |
| GET/POST /projects/{p}/plans | 列表／draft，含 ownerIdentityId、目标、验收标准、workflowId |
| GET /projects/{p}/plans/{id} | 正式状态、任务统计、历史验收/重开、capabilities |
| GET/POST /projects/{p}/tasks | 分页／draft；planId?,parentId?,participants,reviewer,node,criteria,requirements |
| GET /projects/{p}/tasks/{id} | 详情、当前报告、硬前置、可执行动作 |
| GET /projects/{p}/execution-map | planId? → 有界任务图＋外部依赖摘要，不拉全部消息 |
| POST /projects/{p}/tasks/{id}/start | expectedVersion；participant |
| POST /projects/{p}/tasks/{id}/reports | submissionId,kind,expectedVersion；幂等链接既有 ready 提交 |
| GET /projects/{p}/tasks/{id}/acceptance-review | → reviewId/hash、materials、版本、blockers；reviewer |
| POST /projects/{p}/tasks/{id}/acceptances | reviewId/hash,expectedVersion,decision=accept/reject,reason?；human |
| POST /projects/{p}/tasks/{id}/reopens | expectedVersion,acceptanceId,reason；reviewer human |
| GET /projects/{p}/plans/{id}/acceptance-review | → 计划快照含 taskAcceptanceIds；owner |
| POST /projects/{p}/plans/{id}/acceptances /reopens | 快照／原因；plan owner human |
| GET/POST /projects/{p}/proposals | 列表／draft typed payload |
| PUT /projects/{p}/proposals/{id}/draft | expectedVersion,changes,报告；只改 draft |
| POST /projects/{p}/proposals/{id}/submit | expectedVersion,draftHash → frozen review；sender human |
| GET /projects/{p}/proposals/{id}/review | fixed payload/manifest/slots/votes/diff/actions |
| POST /projects/{p}/proposals/{id}/decisions | reviewId/hash,expectedVersion,decision,slotIds,actingBindingVersions,reason?；当前职责办理人 |
| POST /projects/{p}/proposals/{id}/delegate-decisions | 同上＋required reason；明确代批资格 |
| POST /projects/{p}/proposals/{id}/revisions | previousReviewId,changes,reason → 新 draft，无旧票 |
| GET/POST /projects/{p}/handoffs | 列表／draft sources、receiver、targetTask、kind |
| POST /projects/{p}/handoffs/{h}/sources/{s}/revisions | 新 report/evidence/summary；sender |
| POST /projects/{p}/handoffs/{h}/sources/{s}/send | sourceVersion,reviewHash；sender human |
| POST /projects/{p}/handoffs/{h}/sources/{s}/decisions | sourceVersion,reviewHash,accept/reject,reason?；receiver human |
| POST /projects/{p}/handoffs/{h}/reminders | expectedVersion；合法成员，冷却限制 |
| GET/POST /projects/{p}/release-reports | 真实环境与版本上报、报告引用；成员及当前职责 |
| POST /projects/{p}/tasks/{id}/fix-propagations | 用户选 targetReleaseRefs → 目标工作提案草稿 |
| GET /projects/{p}/audit | 可见业务审计分页，无 private prompt |

正式改变 active 工作的范围、职责、依赖只能通过提案，不能通过 draft PUT 路由跨状态写入。

工作取消同样通过 work_change 的 cancel_plan/cancel_task 操作；draft 的 discard 使用 `POST /projects/{p}/plans/{id}/discard` 或 tasks 对应路径，必须作者、expectedVersion 且没有正式引用。取消不满足后继依赖。`POST /projects/{p}/tasks/{id}/reports` 与带任务目的的 submissions 共用同一 Report 用例，返回既有报告而不双建，见 04。

## 6. Agent 与人工意图

| Method / path | 合同 |
| --- | --- |
| POST /projects/{p}/topics/{t}/runs | sourceSubmissionId,expectedContextVersion,reason? → batch/run refs；member/agent:request，重复源不续额度 |
| GET /projects/{p}/runs/{id} | 公开运行状态、budget、待材料、子结果、artifact refs |
| GET /projects/{p}/runs/{id}/events | 脱敏公开增量，按 run seq 恢复 |
| POST /projects/{p}/runs/{id}/cancel | expectedVersion,reason；发起者/manager，无业务撤销 |
| POST /projects/{p}/batches/{id}/extensions | extraRounds,expectedVersion,reason；manager human，部署上限 |
| POST /projects/{p}/confirmation-intents | typed operation＋canonical payload/review refs → intentId,confirmUrl,expiresAt；CLI 仅准备 |
| GET /projects/{p}/confirmation-intents/{id} | 本人/grant → pending/committed/rejected/expired/stale＋resultRef |
| POST /projects/{p}/confirmation-intents/{id}/confirm | intentHash,csrf,approve/reject → 事务调用原领域命令；仅 browser session |

Intent confirm 不接受任意 URL 转发或 SQL patch，operation 为白名单。确认时服务端执行的就是原审阅内容；CLI 轮询只取结果，不再次提交一票。

## 7. 必备 JSON 样例

工作提交：

```json
{
  "clientSubmissionId": "client-generated-uuid",
  "purpose": "delivery",
  "topicId": "topic-uuid",
  "taskId": "task-uuid",
  "expectedTaskVersion": 4,
  "identityId": "identity-uuid",
  "text": "已完成列表筛选；附件包含本地验证记录。",
  "materialVersionIds": ["version-uuid"],
  "codeRefs": [{"repositoryId":"repo-uuid","branch":"task/filter","commit":"full-sha","verification":"reported"}]
}
```

`verification` 是客户端上报声明，服务端固定为 reported，不允许客户端传 verified 获取可信状态。提交作者、时间和实际 channel 不从 body 取信。

审批决定：

```json
{
  "reviewId": "review-uuid",
  "reviewHash": "sha256:...",
  "expectedVersion": 7,
  "decision": "approve",
  "slotIds": ["slot-uuid"],
  "actingBindingVersions": [{"identityId":"identity-uuid","bindingVersion":2}],
  "reason": ""
}
```

冲突响应：

```json
{
  "error": {
    "code": "REVIEW_STALE",
    "message": "The reviewed evidence changed.",
    "retryable": false,
    "details": {"reviewId":"review-uuid","changed":[{"type":"report","id":"report-uuid","expectedVersion":2,"currentVersion":3}]}
  },
  "requestId":"request-uuid"
}
```

## 8. 契约交付要求

每个 operation 有稳定 operationId、完整 schema、enum、必填/nullable、security、错误和示例；日期/ID 示例在真正 OpenAPI fixture 中换成合法值。生成 Go DTO 与 TS client，CLI 复用客户端，检查生成后工作树无额外 diff。

接口集成测试验证未知字段拒绝、边界长度、外项目引用、权限、版本、幂等。错误 schema 不能由不同模块各造一套。任何目录中“未来待实现”的生产路由不能返回 200 空数组掩盖 501；阶段未完明确能力关闭，最终 V1 gate 必须全部所需能力可用。
