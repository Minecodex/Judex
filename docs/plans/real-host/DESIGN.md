# 联测前置能力的实现方案

更新：2026-10-12。状态：CLI 与缓存观测最小版、流程草稿、项目提交分析、候选查询、身份工具过滤和 Office 沙箱原件解析已在源码实现并完成前置验证；真实综合闭环和视觉能力仍待准入 / 验收。CLI / 缓存历史验收见[实施记录](records/CLI-CACHE-20261010.md)，最新实施见[执行记录](IMPLEMENTATION.md)。依据：[前置实测](READINESS.md)、[CLI 合同](../v1/07-CLI与Skills.md)、[Agent 与沙箱合同](../v1/05-Agent与沙箱.md)。

目标：先修复阻碍真实宿主联测的通用问题；缓存观测采用现有持久化链路的最小实现。平台继续只分析、记录和创建草稿，正式发布、安排与验收仍由有权人确认。

## 1. 推荐范围与顺序

| 顺序 | 范围 | 最小交付 | 对联测的作用 |
| --- | --- | --- | --- |
| 1 | CLI 可靠性 | 全局错误出口、正确进程退出码、单一 JSON 结果、原生二进制回归 | 宿主能准确区分成功、失败、待确认 |
| 2 | 平台缓存观测 | Provider → Runner → Journal 传递用量，复用 usage_json，固定工具顺序，SQL 导出 | 能证明缓存字段是否返回及输入复用比例 |
| 3 | 软件流程分析 | 完整候选背景、按真实运行身份过滤工具、流程草稿工具 | 能做多职位、多流程和模型建稿联测 |
| 4 | Office 原件分析 | 固定版本挂载、沙箱解析工具、带来源位置的结构化片段 | 模型能依据文件内容分析 |
| 5 | 图片分析 / 生成 | 实际视觉与生图能力、统一调用记录和固定产物版本 | 关闭图片场景的能力缺口 |

步骤 1、2 可以独立实现并做受控协议 / 原生 CLI 验证。供应商可用额度具备后，再用真实模型及隔离 namespace 验收；受控响应不能证明真实缓存命中。

## 2. CLI：一个全局错误出口

### 前置核查时的根因与修改位置

[入口](../../../cmd/judex/main.go)对任何 Execute 错误使用 TakeExitCode；[emit](../../../internal/cli/cli.go)只有业务命令进入后才设置该全局值。Cobra 必填参数、未知 flag 或未知命令错误发生在此前，加上 SilenceErrors，因此可能退出 0 且没有输出。

增加 internal/cli 的统一执行入口，main 只调用它并返回其退出码。入口负责：

1. 本次执行的模式、输出和错误状态，不沿用上一条命令的全局状态。
2. 接住解析、参数、文件、认证、网络及业务错误。
3. 按现有 CLIError 映射进程退出码；未分类错误默认非零。
4. --json 时 stdout 仅输出一份现有 schemaVersion=1 结果；成功为 ok=true，失败为 ok=false。文字模式错误写 stderr。
5. 已经输出的业务错误不重复输出。上传成功、关联失败等部分结果中的 versionId / registrationStatus 等数据必须保留。
6. 正常 help / version 保持退出 0；pending、拒绝、过期沿用已确定的确认协议，不改写成成功。

第一版保持现有 error 字段格式与已知退出码分类，不因该修复新增业务权限、自动确认或自动重试策略。若调整 error 的公共结构，应同步命令契约及 Skill，而不是只改 main。

### 必须验证

实际 native CLI 验证缺 task/file、未知命令、未知参数、错误参数类型、缺文件、无效 JSON、未登录、API 401/409/5xx，以及正常 version/help。覆盖 --json 在合法不同位置的解析错误、stdout/stderr 分离、一次输出和部分成功数据保留。

发行 Windows 包再次运行相同关键用例；Kimi 对错误命令应读取到真实失败，不得因 exit 0 把未提交工作认作已提交。认证刷新、原幂等键及待人工确认流程保留既有契约。

## 3. 缓存观测：现有 JSONB + 查询导出

### 最小持久化内容

model_calls 已有 project_id、run_id、attempt、provider_request_id、request_manifest_hash、usage_json、usage_known、started_at 和 ended_at。复用该表，第一版不用增加表或列。

usage_json 保留现有 totalTokens、conservative，新增计数统一放在 observation 子对象，供 24 小时到期后整体清除：

```json
{
  "totalTokens": 10500,
  "conservative": false,
  "observation": {
    "modelName": "glm-5.3",
    "inputTokens": 10000,
    "outputTokens": 500,
    "cacheReadTokens": 8000,
    "cacheWriteTokens": null
  }
}
```

- inputTokens 表示规范化后的完整输入，含已缓存部分；outputTokens 含供应商计入输出的推理消耗。
- cacheReadTokens=null 表示没有提供可确认的缓存读取信息，0 表示明确提供了 0。
- cacheWriteTokens 同样允许 null；当前 GLM 接口没有该字段时不能补成 0。
- usage_known 仍表示总用量是否可靠；它为 true 不代表缓存字段必然已知。
- 未知用量保留预算预留和 conservative=true，不能把预留量用于缓存率或账单计算。
- modelName 是本次请求模型名，在 ModelStart 固定到此 attempt；不从以后可能变化的项目配置反推。既有 provider_request_id 可随 Provider 元信息一并填入，无须新字段。
- 不保存 prompt、材料正文、私人偏好或密钥来实现该观测。

用户追加要求：观测只保留一天并自动 TTL。采用从 ended_at 起滚动 24 小时：查询到期立即排除；worker 启动时及每分钟自动批量清理 observation 和本轮填入的 provider_request_id。保留 model_calls 原行、attempt、总 token、未知用量标志和预留数据，以免恢复时重置序号或丢失预算依据。清理任务使用既有 worker 生命周期中的维护协程，不生成持续积累的定时任务行。迁移 27 仅增加到期部分索引，没有新表或列；短时停机后的过期数据由 worker 启动首轮清理。

### 六处核心接线

| 位置 | 改动 |
| --- | --- |
| internal/infrastructure/model/provider.go | 读取 GLM / OpenAI-compatible 的 prompt_tokens_details.cached_tokens；事件携带完整用量快照，缓存计数可空 |
| internal/infrastructure/model/anthropic.go | 保留分帧用量、缓存字段及其存在性，按已核实网关口径归一 |
| internal/agent/runner/runner.go | 保存完整调用用量快照；多帧累计计数更新快照，不重复求和；同次请求复用一份工具 schema |
| internal/agent/runner/journal.go | ModelEnd 从总数＋known 扩为用量快照＋known / 错误，避免跨层丢字段 |
| internal/agent/batch/journal.go | 规范化明细写 usage_json，原预留与实际预算对账保持独立 |
| internal/agent/tools/tools.go | 工具按名称固定排序；后续按真实 principal 过滤时，同一主体的工具集合仍确定性输出 |

使用一个共用的 Usage / UsageSnapshot 类型承载数值和字段存在性；具体 wire 字段仍由协议适配器解析。原来的 totalTokens / conservative 保留。恢复、断流和失败路径也应写真实 known 状态，缓存明细不会因正常结束之外的路径被误写为 0。

[GLM 官方文档](https://docs.bigmodel.cn/cn/guide/capabilities/cache)说明其缓存为自动识别，返回 usage.prompt_tokens_details.cached_tokens。当前 GLM OpenAI-compatible 请求的 prompt_tokens 是完整输入，缓存读取是其子集，不再次叠加到输入总量。

[Anthropic 官方口径](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)把 input_tokens、cache_read_input_tokens、cache_creation_input_tokens 分别计数，完整输入为三者之和。GLM 的 Anthropic-compatible 网关不能仅凭协议名字套用该语义：先核对实际分帧和计数；尚未核实的缓存 / 输入口径标未知并保持保守预算，不报告看似精确的命中率。

第一版优先关闭当前 GLM OpenAI-compatible 路径的观测，不引入手动 cache_control、缓存保活请求、提前预热任务或上下文重排工程。工具排序之外的前缀优化，基于真实观测再决定；继续每次验证最新权限、任职与已发布流程。

### 查询和展示

附[只读 SQL 样例](cache-observation.sql)，按项目、run 和本次请求模型分组。第一版由测试驱动查询并导出 JSON / CSV；明细时长复用现有时间戳。

核心报表：

- 调用次数。
- 缓存可判定调用数与未知调用数。
- 同一可判定集合的完整输入和缓存读取 token。
- token 加权命中率：Σ缓存读取 / Σ完整输入。
- 可区分 state 的明细与调用时长。

只统计 usage_known=true 且输入 / 缓存计数有效的记录，0 输入时比率为空。字段未知的调用不混入分母，不默认为未命中；报表同时显示未知调用数，防止选择性统计造成误读。成功和失败尝试按实际可确认用量统计，重试不会重置此前消耗。

缓存读取降低的输入成本还取决于供应商规则，不能把 80% 输入命中率直接称作 80% 总费用节省。缓存输入仍占上下文窗口与完整输入预算。

查询足以满足这轮联测。后续需要常驻产品展示时，增加受项目权限约束的只读查询 API，在既有运行详情中展示；不要向普通用户开放直接 SQL 权限。

### 最小验收

受控协议与真实 PG 验证：

1. 输入 1000、缓存读取 800、输出 50，落库保持原计数，总 token 为 1050。
2. 缓存字段缺失 → null；明确 0 → 0，两个状态能区分。
3. 多个累计 usage 帧按最终快照记一次；分帧缺字段不覆盖已知字段，明确 0 不被当作缺失。
4. 断流、429、取消及缺 usage 的预算预留不被退成 0，不用估计值计算缓存率。
5. 协议计数口径确认后的 Anthropic 缓存输入不会漏计或重复计入。
6. 同一 principal / 工具集合重复序列化得到相同顺序；checkpoint 和请求重用相同 schema。
7. 用不同输入长度的合成行检查加权率，不计算每次百分比的简单平均。

真实模型部分在隔离环境用相同工具和足够长的稳定前缀，改变尾部用户问题并保留首调 / 后续调用原始 usage；再改变流程或职位版本、跨会话和跨项目作对照。输出是否缺字段、是否明确 0、是否出现缓存读取分别报告。首次请求未必是供应商全局冷缓存，不预设每次后续请求都必须命中，也不在实测前规定任意 90% / 95% 门槛。

## 4. 流程与职位：补上下文和权限

### 已有任务的候选解析

抽出共用 WorkflowCandidates 查询 / 上下文构建，协调者、岗位和 CLI context 使用同一业务解析结果。区分正式绑定和候选来源：

- 任务已有关联：task.workflow_id / node_id 为正式依据；其他相关背景不会覆盖它。
- 任务未绑定：加入所属 plan.workflow_id、当前参与 / 任职职位的 position_node_bindings。
- 仍没有足够候选：查询本项目已发布流程，按页 / 范围提供摘要或读取工具。
- 不使用未发布草稿或他项目流程；按 workflowId / versionId / nodeId 去重和确定排序，不能只以 nodeId 判唯一。
- 候选多于上下文预算时分段查询，不能静默截断硬约束。

每次模型边界读取最新版本和任职。模型输出分 currentNodeCandidates、relatedEvidenceNodes、suggestedNextNodes、needsClarification 和公开依据；可先扩充已有 task_analyses.output_json 和 TaskAnalysisOutput，再同步公开 API / CLI / UI。输出属于建议，解析和引用需验证，不直接改任务绑定或推进正式节点。

含糊或与已有绑定冲突时保存分歧和最少必要澄清。不要以未经真实样本验证的固定 confidence 阈值代替业务校验。

### 工具主体与可见性

当前 executeTool 把全部运行的 principalKind 固定为 agent，Schemas 不按身份过滤。不能只在 record_task_analysis 加 coordinator 判断，否则可能连协调者也无法调用。

由服务端已校验的 agent identity kind 填入运行 / 调用上下文，工具目录和 Execute 使用同一 coordinator / position principal：

- coordinator 看到 call_agent、record_task_analysis 等协调能力。
- position 返回分析结果及原引用；不看到协调者专用保存或递归委派能力。
- 所有主体都按获准能力确定工具顺序，执行时继续复核 run、lease、项目、任职及业务权限。
- RecordTaskOutput 的 coordinator 限制继续保留，不能靠放宽数据库保存权限“消除报错”。

### 模型生成流程草稿

增加受约束的 draft_workflow 工具，接现有 workflow.Body / Validate / domain transaction；先支持创建新草稿。已存在草稿的覆盖另需明确目标、expectedVersion 和权限，不按同名自动覆盖。

输入为节点、职位约束、依赖、硬规则及说明，校验项目归属和引用。服务端从结构生成 Mermaid，模型不以 Mermaid 文本替代业务定义。返回 workflowId / draftVersionId / draftHash，可在既有流程编辑器读取编辑；publishedVersionId 仍为空，正式发布仍由管理员确认。

现有 insertDefinition 的审计固定为用户 / Web，接入 Agent 时应抽出显式 ActorContext / 来源参数，保留请求成员用于权限验证，记录真实运行身份和 run，不把模型建稿伪装成用户 Web 创建。新增工具必须使用现有工具幂等 / lease 效果边界，重复调用返回同一草稿；恢复遇到未知效果先核对结果，不能重复创建。

计划 / 任务继续走 propose_changes 和既有确认安排路径，保持与流程定义草稿分开。此扩展符合“AI 可创建草稿、人工发布”的已定边界，不授予 Agent 发布、投票或验收权。

### 无任务 / 无讨论的自由提交

属于单独的项目范围分析入口；不能给 report 松掉 task 必填校验来实现。自由 submit 保存 ready 后，可创建项目范围内部分析，读取候选并输出目标工作 / 新安排的草稿建议。不要为承载分析自动建立正式任务或可见讨论。

需要确认现有 agent_sessions 的 topic / task 约束并完善 project-scope session / batch 生命周期、幂等触发和公开结果查询，再验收自由提交闭环。先关闭已有 task 的候选缺口，不把后者说成全部自由提交路由已完成。

## 5. 文件分析：复用固定版本与现有沙箱

### Office 最小版

read_material 保持原文 / manifest 读取职责；新增明确的 analyze_material 读取分析能力：

1. 以 materialVersionId 读取本项目 ready 原件和 manifest，验证固定 hash。
2. 通过已有 project volume / OpenSandbox 放入只读 versions/<versionId>/content。
3. 在锁定的沙箱镜像提供版本化解析器及依赖，模型不用临时联网安装。
4. 工具在该运行的沙箱解析原件，返回可按页 / 工作表 / 范围读取的结构化片段。
5. 返回原版本、hash、解析器版本、覆盖范围、截断 / 无法解析项，模型引用这些事实。
6. 派生文件保留在本次工作区，需要共享时用既有 publish 声明来源版本，不覆盖原件。

| 格式 | 首版结构输出 | 明确保留的限制 |
| --- | --- | --- |
| DOCX | 段落、标题、表格与位置 | 复杂版式 / 图片不由文本提取证明 |
| PPTX | 每页标题、文字、表格与页码 | 图形关系、图表及视觉布局需要额外检查 |
| XLSX | 工作表、单元格位置、值、公式、已有缓存结果 | 公式文本 / 缓存值不等于本次重算结果 |
| PDF | 每页文本和页码 | 扫描页或无文字页标明需 OCR / 视觉分析 |

统一的解析模块与 analyze_material 工具适配放在独立文件，不在每个场景各写临时解析逻辑。首版按需解析、不新增后台提取队列；将来需要复用结果时以固定 versionId + hash + parserVersion 作派生缓存键。文件解析缓存与供应商 prefix cache 分开统计，原件状态也不由解析失败倒退。

语义与排版验证分开：文本 / 表数据用于 GLM 分析，现有 PDF 预览供人审阅；需要自动视觉核对时走真实视觉能力。公式重算可复用 LibreOffice 环境，但必须读取并核对实际重算结果，不以 PDF 转换 200 宣称计算通过。

新增解析器遵守既有“业务 Server 不执行模型 shell”的边界：继续在每运行沙箱内处理，不让 Agent 直接操作转换器、PG 或对象存储凭据。不是新增一个平台远控执行服务。

### 图片

GLM-5.3 主模型为文本模型。图片分析需要一个实际可用的图像输入模型 / 视觉工具；若以独立工具接入，复用模型适配、逐调用用量和预算记录，避免视觉调用游离于 model_calls 之外。[官方模态说明](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3)

选择直接多模态 Provider 时，需要结构化 image content、真实能力校验和受权固定图像版本传输；把 vision=true 或 base64 字符串塞进普通 text 不会建立有效视觉能力。看图与生图分别验收，生图输出仍上传 / 发布为固定材料版本。

软件编码、设计原图和 Office 原件的实际制作继续由成员本地 Kimi 完成。平台沙箱用于资料解析、核对和分析派生产物，不能因新增文件工具变成代执行本地工作或代替人工验收。

2026-10-12 实测补充：带用途/任务关联的逐文件上传会各自生成资料提交及分析，交付包改为 bare upload 后由一份 report 集中引用；源码 ZIP 是可用原件，不需要逐文件机械提交。任务内 list_agent 和 call_agent 共用同一身份授权集合，只暴露在任且可委派的岗位。真实模型曾对源码全包重复读取、重跑成员测试，11 次已知调用使用 207516 token 后因下一次预算预留超限失败；保持现有预算，明确分析与本地成员完整测试的边界，优先报告/固定版本/关键来源与具体岗位意见，缺覆盖必须公开保留。原失败批次不改为通过，新来源实际复测另记。

2026-10-13 实测补充：A14 分析对 ZIP 执行 head，含 NUL 的输出导致 JSONB 22P05；read/bash 现在对非 UTF-8 或 NUL 输出显式使用有 SHA 的二进制表示，小输出可逆保存，大输出注明未内联，不能作为原文理解。固定原件不变，仍用 analyze_material 解析。A13 新报告分析已知使用 187550 token，但下一次完整探索上下文的保守预留超过剩余预算；在现有压缩机制中加入解析工具输出，并在请求前以岗位自身和共享批次的剩余预算收紧探索上下文，原始结果与正式请求/异议/岗位意见保持可回读。模型收到临近预算的公开提示；原子预留和 300000/100000 上限不改变，不能因此声称实际新分析已通过。

2026-10-13 后续实测：.14新请求的协调者已保存结构化分析，但岗位在58k左右实际用量后仍不能为完整上下文预留下一次调用。改为按当前上下文的保守预留在最后可保证的调用中收尾：岗位返回有明确缺口的公开意见，协调者只开放 record_task_analysis；不再执行探测工具。成功持久保存且位于本轮最后的结构化分析直接作为运行公开结果，不再额外调用模型复述；记录失败/未知效果或必需岗位失败仍保留，分析不授予业务验收。固定原件提示按MIME选择小页解析并给出实际content路径，减少二进制探测和重复目录遍历。预算、任职、lease和人工决策边界不变；真实新请求复测另记。

2026-10-13 建稿复测另发现：GLM-5.3两次均返回8192输出用量、空公开文字和无工具，不能作为完成。网关结束原因位于choices.finish_reason，修正既有错误层级；length响应标为输出截断，保留最终已知用量，禁止执行未完成工具，不触发quota降级。两款Coding Plan模型实测thinking.type=disabled均HTTP200并调用ping（166输入、11输出、无思考字段），只作为协议准入。部署目录新增显式requestOptions.thinkingType，经目录同步/独立request_options存储/主模型及fallback解析/Provider传输；真实请求hash含非秘密选项。当前隔离分析使用disabled，仍保持8192输出和全部批次上限，不硬编码所有GLM或其他供应商的默认模式。

## 6. 实施验证与边界

- CLI / 缓存阶段：原生进程、受控 SSE 和真实 PG 验证字段与预算；真实模型额度恢复后验证 raw usage 与落库一致。
- 流程阶段：真实模型 + 三流程多节点 + 多职位，覆盖已绑定、仅计划、仅职位、重叠、冲突、无适用候选和自由提交。当前 / 历史 / 建议节点按预先登记的标准分别评估。
- 文件阶段：开启真实 Sandbox 与材料投影的所属隔离 namespace，实际原件解析、来源 / 分页、公式与视觉限制、上报、退回和最终验收分别留证。
- 发布 / 部署按同一可追溯镜像、CLI、Skill 及解析器制品验收；测试命名空间按所有权清理，保留正常部署数据。

当前 SQL 已按 observation 子对象和滚动 24 小时过滤读取。此前仅验证查询逻辑时，不构成平台已记录真实缓存的证明；当前源码的实际接线与 TTL 验证按实施记录分别说明。流程和文件分析不因缓存阶段完成而视为已关闭。

本轮已在真实 PostgreSQL 只读执行该 SQL，现有模型调用表返回 0 行；另以 CTE 合成四条调用验证逻辑：两条已知、两条未知，已知完整输入 1100、缓存读取 900，加权率 81.82%。其中明确缓存 0 参与统计，null 和 usage_known=false 排除；合成行没有写入业务表。此检查不代表平台已记录真实缓存数据。
