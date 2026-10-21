# V2 API、事件与 CLI

[OpenAPI](../../api/openapi.yaml)是公共可执行合同：69 路径、77 操作，`/api/v1` 前缀、协议能力 2。服务端启动验证全部操作注册，Web／Server／CLI 同版发布。旧计划、任务、审批等路径移除。

写操作使用 UUID Idempotency-Key，配置／文件版本／讨论控制使用 expectedVersion；命令重放前重新授权。消息、附件、通知、知识与投递同事务。消息和上下文使用序号分页，SSE 使用会话／个人各自独立的持久游标，支持断线恢复；长期流每次重新验证主体和成员权限。

原生 CLI 保留设备登录授权，改为项目／群选择、消息、上下文、文件、push、run list、inbox 和 watch；Skill 与 CLI 同版。普通 push 触发本人全部岗位，record-only 只存内容；用户不自行选择作者或单一岗位身份。

message send／push 支持 `--reply-to`，与结构化提及分别发送。Discussion 的 waitingFor 只返回尚未解决的消息／人／岗位关联；对应人明确回复后解除该等待，不自动唤醒 AI。非空 CLI projectScope 在项目列表 SQL 查询层过滤，不能通过发现接口暴露范围外元数据。

实际命令和授权范围见 [V2 合同](../plans/v2/CONTRACTS.md)及 `judex help`。设备 grant 不迁移，旧客户端必须重新登录。

岗位提及是 AI 副作用：消息写权限不包含模型请求权限，需同时持有 `agent:request`。领域 Post 与幂等缓存读取前使用同一效果鉴权；相同用户换用较窄设备授权时，旧成功响应仍须重新核验。push 包括 record-only 保持自身接口 scope，record-only 只影响是否投递，不能作为普通消息提及的授权绕过字段。

项目配置、项目成员及模板修订通过 project.changed 个人事件独立刷新；个人提示词通过仅本人的 personal-prompt.changed 刷新。载荷只含必要标识，和写入同事务，补收仍检查当前项目权限，不借用任何未加入群的资料。配置事件不触发 AI。

CLI 组合推送由 `cli/operation.go` 管理整次恢复身份，附件上传复用 `files_v2.go`，单条普通写操作继续使用公共 client outbox。恢复文件原子替换并限制为本人可读写，锁随进程退出自动释放；完成的附件不会在最终消息未知时重新生成。最终请求始终访问服务端，保持当前 grant／成员资格／效果权限重查。

## 流程图与资料补充（2026-10-20）

当前 OpenAPI 为 74 路径/82 操作，协议仍为 2。新增人类管理者私有 workflow-drafts 创建/读取、已认证头像读取；PATCH /me 支持昵称和 avatarData。profile.changed 个人事件驱动显示刷新，不触发岗位。工作流详情、修订与保存合同含 graph。

## 确认卡片补充（schema 15）

复用消息创建和消息 SSE：confirmation.title 指定卡片标题，结构化人类 requiresResponse 指定接收人；decision 保存回应选择和固定版本 1。CLI message send 新增 --confirmation、--decision，并沿用 --reply-to／--mention-human／--mention-ai。岗位 send_message 工具同步可选 confirmation，未增加审批端点。
