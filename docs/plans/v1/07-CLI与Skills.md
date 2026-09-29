# 07 CLI 与 Skills

## 1. 最新确认与实现边界

用户本轮明确：说的是 **CLI**，沿用 `judex`。正式审批由 CLI／Skill 发起，**浏览器审阅并确认一次，CLI 接收结果**。该答复覆盖旧 D10 中“不重复 Web 确认”的未解决实现条件；不是用户先在 CLI 批一次再去 Web 批第二次。

CLI 是公共业务 API 的客户端，不另开绕权限接口；Skill 是调用规范，不是持权服务。成员手动调用本地 AI/Skill 拉取工作和上报，平台不向电脑推可执行指令，不安装常驻远控 Agent。

## 2. 工程与发布目录

```text
cmd/judex/main.go             小入口、信号、exit code
internal/cli/                root/auth/project/context/material/report/decision 等命令
pkg/client/                  公共 API client、分页、错误、上传和事件协议
skills/judex/SKILL.md         发行用主技能，不复用 grilling
skills/judex/references/      命令表、审批流程、报告格式和错误恢复
skills/judex/agents/          宿主元数据（仅在该宿主支持时提供）
tests/cli/                   实际二进制与真实 API 集成
tests/skills/                场景、命令契约、越权与误操作检查
```

CLI 可用 Cobra 实现命令树（工程选择，在 P0 锁版本）；每命令小文件，禁止 main.go 数千行。支持 Windows amd64/arm64、Linux amd64/arm64、macOS amd64/arm64 的构建；未实际测过的平台标“已构建未运行验证”，不能列作实测通过。

## 3. 登录和凭据

`judex auth login --server URL` 发 device authorization，显示目标站点、设备名、短码和 verification URL，打开浏览器登录/确认授权；无法自动打开时用户复制 URL。CLI 按 interval 轮询，处理 pending/slow_down/denied/expired；码只用一次。初次登录授权设备不等于批准今后所有业务决定。

access token 默认 15min、refresh 30d，可配置；refresh rotation，旧 refresh 重放撤销 family。服务端每次调用核对 grant、user active/authVersion、实际项目成员和当前 identity binding。移除项目权限立即收缩，不等 Token 过期。

scope 最低集合：projects:read、context:read、materials:read/write、submissions:write、reports:write、proposals:draft、agent:request、events:read、intents:create。设备登录默认请求读取＋上报，不包含隐式全项目权限；项目范围为空时只能列本人项目并请求后续授权。创建新项目可单独授权 projects:create，但实际创建仍要求浏览器确认意图。没有 decisions:approve 或 task:accept 的长期 grant。服务端按 scope 与业务资格的交集执行，scope 不替代本人当前任职。

优先 OS 安全凭据存储；非 secret 配置记录 server/profile/当前项目。没有可用 keyring 时提示使用一次性 env token或显式受限文件存储，创建 0600／Windows 当前用户 ACL，明示风险；不要静默退到世界可读明文。`.judex/project.json` 可进仓库但只含 server 别名/projectId/默认 identity，严禁 token、口令和 Secret。

HTTPS 默认必需；本机 loopback 开发例外。私有 CA 用 `--ca-file`；不默认提供跳过证书校验的安装示例。server URL 改变必须重新确认，禁止把当前 token 自动发给另一个 origin。日志/`--debug` 脱敏 Authorization、Cookie、deviceCode、refreshToken。

## 4. 命令合同

所有命令支持 `--server`、`--profile`、`--json`；项目命令支持 `--project`。写命令接受 `--request-id` 作幂等键；首次未提供则生成并持久到本地 outbox，以便网络恢复后同键重试。

| 命令 | 行为与关键参数 |
| --- | --- |
| `judex version` / `status` / `doctor` | 协议版本、服务可达性、凭据/网络诊断；无秘密内容 |
| `auth login` / `whoami` / `logout` | 设备授权、当前账号和 scopes、撤本设备 grant |
| `project list` / `project use ID` / `project show ID` | 仅本人可见项目，选择保存非 secret context |
| `project create --file project.json` | 准备建项目意图→浏览器确认→结果 |
| `identity list` / `identity use ID` | 仅本项目本人有效任职；不凭姓名自动切其他人 |
| `inbox list --kind ...` | 我的待办分页，带 object/review/version、合法下一步 |
| `context get --task ID` / `--topic ID` | 正式上下文 JSON、材料 manifest、来源版本、约束、未决与实际下一步 |
| `material list` / `material download --version ID --output PATH` | 版本固定，项目隔离，下载校验摘要 |
| `material upload PATH` / `bundle upload DIR --entry index.html` | 流式／分片上传，返回材料版本；目录先生成可审阅 manifest |
| `submit --file submission.json` | 自由文本＋材料＋投递位置，可不指定任务；不自动验收 |
| `report progress --task ID --file report.json` | 本人职责进度，上报可触发分析，不当成最终完成 |
| `report delivery --task ID --file report.json` | 交付报告＋准确材料/代码引用，状态 delivered，不自动 merge／验收 |
| `bug create --file bug.json` | Bug 草稿及证据/原任务/环境引用，正式安排再确认 |
| `release report --file release.json` | 本地实际部署结果、环境/URL/版本引用；不是部署命令 |
| `proposal draft --file proposal.json` | 生成 typed 草稿，返回差异与服务端建议必需审批名单 |
| `proposal submit ID --review HASH` | 发送人确认意图，不直接批准接收方 |
| `decision review ID` | 获取最新不可变审阅版本、变化、名单、计时和权限 |
| `decision approve ID --review HASH` / `reject ... --reason-file FILE` | 创建浏览器确认意图并等待结果，`--no-wait` 只返回 pending |
| `handoff send SOURCE --version N` / `receive ...` / `reject ...` | 各来源正式动作均通过确认意图；拒收理由必填 |
| `task accept ID` / `task reopen ID` / `plan accept ID` | 先取得验收 snapshot，再浏览器确认；不能只用任务标题匹配 |
| `run show ID` / `run watch ID` / `run cancel ID` | 公开运行状态；取消 run 不撤业务批准 |
| `events watch` | 只拉事件，断线游标恢复，不执行服务端返回的 shell 文本 |
| `pending list` / `pending retry ID` | 本地中断提交恢复，用原请求键；冲突需人工更新，不自动换新版本 |
| `skill install --target codex|claude-code|path --path ...` | 安装发行技能和支持的宿主元数据，不含凭据 |

CLI 没有 `git push/merge`、`kubectl deploy` 的平台代执行快捷命令。Skill 可以提示成员在其本地 AI 既有工具中做实际工作，但 Judex API 只收报告。

## 5. 一次正式确认的完整协议

1. CLI 读取 review，打印报告、变化、审批名单、哈希和将要执行的动作；仅代表准备。
2. POST confirmation-intents，服务器绑定 userId、grantId、operation、objectId、reviewHash、payloadHash、scope、expiresAt（默认 10min）、随机 nonce。返回 confirmUrl，不是批准 token。
3. 浏览器必须登录同一 user；显示账号、项目、内容/版本、变化清单、接收名单及“由设备 X 请求”。错账号可切换，不静默用当前浏览器其他人批准。
4. 本人在页面点击一次同意/退回；服务器在同一事务中复核 intent 未消费、membership/binding/review/权限/期限，执行原领域命令、记录 human_cli（经 browser）、消费 intent、记录 resultRef。浏览器 Cookie＋CSRF 不能由普通 CLI grant 换取。
5. CLI GET intent 轮询或等事件取得同一 resultRef，输出 committed/rejected/stale/expired；**不再另发 decision**。浏览器关闭/CLI 中断不回滚已经成立决定；重连查询原 intent。

不能自动打开 URL 后由 Skill 控制浏览器点击确认；SKILL.md 明确要求由用户操作。拒绝 `--yes`、stdin “同意”、聊天文字、自报 humanConfirmed 当作审批证明。生产 grant 可以读/上传/报进度/建草稿，但不直接调用人工命令。所有人工命令经同一能力门控，不能只保护 proposal votes 而漏掉 handoff/acceptance/workflow publish。

批量审阅必须列明每项 intent/review/hash，各项独立结果；不做“以后类似事项全部允许”。超时策略是项目明示规则，不是 CLI 自动点击。

## 6. JSON、错误与退出码

stdout 在 `--json` 下只输出一份机器可读结果：`{schemaVersion:"1",ok,data,error?,requestId?,nextActions?}`；进度/log 去 stderr。不得把 ANSI 动画混入 JSON。`nextActions` 只为结构化提示，不给 Skill 未校验 shell 脚本。

退出码：0 已完成读/写或明确 `--no-wait` 已创建请求；2 参数错误；3 未认证；4 无权限；5 版本/审阅冲突；6 待本人确认/运行未完成（仅同步等待超时）；7 临时网络/依赖错误；8 业务条件不满足；1 其他内部失败。`ok` 与 operationStatus 区分“请求创建成功”和“业务已完成”。

读请求可退避重试；写请求只用原幂等键重试。review stale 不能自动拉最新后再次批准；需输出新差异并产生新的人类确认。

## 7. Skill 的具体内容与安装行为

主 Skill 必须包含：何时调用、先 auth/context、项目/身份解析、材料处理、报告规则、正式确认边界、冲突/网络恢复、保密边界及命令示例。至少三套 references：`commands.md`、`reporting.md`、`human-decisions.md`；技能读取 CLI `--help --json` 或版本化命令说明，拒绝和不兼容 CLI 混用。

安装仅写用户指定/当前宿主规定的 skill 目录。安装前展示目标和变更清单；存在同名用户自改技能时提示差异，不直接覆盖。提供 version/checksum 与 `skill uninstall`，只移除安装 manifest 所属文件。Codex、Claude Code 的实际安装约定在实施时查各自官方文档并实测，不能猜测 @mention 语法都一样。

Skill 流程：

```text
用户：查看我的任务
  → auth whoami → inbox list → context get → 展示任务、前置、材料、验收标准
用户：把本地结果提交到任务 T
  → 核对 T/project/identity → 选择明确文件并生成 manifest → upload → report delivery
  → 返回 submissionId、reportId、版本、平台链接和待验收状态
用户：同意该提案
  → decision review → 确认所指对象无歧义 → decision approve --review HASH
  → 用户浏览器审阅 → CLI 查询同一结果 → 报告实际决定
```

不得上传整个工作目录、.env、SSH key、Git credential、浏览器配置或未选文件。只上传明确提交清单；意外敏感文件触发待选择，不由模型猜“应该没关系”。文本和图片/HTML 包是一份 submission，缺附件不能回报完整交付。

## 8. 验收与发行

真实 CLI 二进制对接 API 完成设备授权、上下文读取、上传、断网重试、Browser-confirm 一次提交、撤销 grant。测试错账户、改 review、重复确认、过期和旧绑定全部拒绝。用实际 Codex/Claude Code 中至少一种验证技能端到端，另一种需明确适配和证据状态，不用文档截图代替实测。

发行 assets：各平台二进制、sha256sums、版本化 skill zip、安装说明、协议兼容范围、升级/卸载说明。server、web、CLI 和 skill 同版本 manifest，不能只发布 server 镜像。

参考：[RFC 8628 设备授权流程](https://www.rfc-editor.org/rfc/rfc8628) 提供 CLI 授权的码、轮询与过期机制依据；它只解决客户端授权，本节每项业务确认是 Judex 独立事务协议，不能混为同一次长期授权。

## 2026-09-29 实现与发行

空 projectIds 的 grant 只允许发现项目和明确授权的首次建项目流程，不允许读写任意项目。首次项目使用用户级确认意图，浏览器确认不会自动扩展已有 grant。正式决定默认等待同一结果至两分钟，--no-wait 只返回待确认意图；decision result --global 可查询首次建项目结果。

凭据包含独立 access/refresh；刷新用操作系统跨进程文件锁及原子写入，设备码只兑换一次。pending list/retry 使用原请求正文、服务器和幂等键。material bundle 提供清单预览与确定性 ZIP，上传复用本地收据，固定版本下载核验字节。events 使用 SSE 游标。

Codex 默认目录 ~/.agents/skills/judex，Claude Code 为 ~/.claude/skills/judex；SKILL.md 包含 name/description frontmatter。安装收据只拥有发行清单内五个文件，拒绝覆盖修改文件，卸载保留用户额外文件。实际 Codex app-server 发现/卸载已验证，额外 AI 宿主会话尚未运行。

npm run release:build -- VERSION 生成六平台 CLI、双架构 Linux server、web、Chart、OpenAPI 和 Skill。构建记录源树指纹，制品/Skill 版本一致，全文件校验和可由 node tests/release.mjs dist/release/VERSION 复核。Windows amd64 包已实际运行，其余架构仅构建。
