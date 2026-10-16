# 真实模型、Kimi Code 与 CLI / Skill 联测前置核查

核查日期：2026-10-09。范围：用户要求的软件开发、图片设计、Office 文件场景，以及 prefix cache 观测。结论：**基础环境和 Kimi 最小接入已可用，完整联测的前置条件尚未全部成熟。**

2026-10-10 后续：CLI 与平台 24 小时缓存观测最小版已在当前源码落地并进行隔离验收，见[实施记录](records/CLI-CACHE-20261010.md)。本文下面的缺口与数字保留 2026-10-09 核查时点，不将源码修复误读为正常部署已升级。

本轮只执行前置探针，不将条件未满足的场景列为可以开始正式验收。尚未展开各场景的完整执行脚本；下文给出各项关闭条件，满足后再设计并运行场景流程。

## 1. 证据与核查范围

- 脱敏摘要：[readiness-20261009.json](evidence/readiness-20261009.json)。
- 本机原始证据：[本轮目录](../../../.cache/real-host-readiness-20261009-4ccd77a9/)。
- 已核查 Docker Desktop K8s、现有 Helm release、公开健康与能力接口、发布版 Windows CLI、当前源码 CLI、Kimi 实际 Skill / 命令执行、Kimi 本轮调用用量、真实 Office 转换、平台工具序列和相关源码。
- GLM 标准 API 探针使用**现有部署 Secret 中的凭据**，只在进程内存中处理；用户本次贴出的 key 未写入配置，也未被此探针消费。不要把本次错误归到尚未测试的新凭据。
- 本轮未创建业务账号、项目、任务、报告或正式决定，未创建 K8s 测试 namespace，未停止或升级已有服务。原件转换只使用仓库内合成 fixture，临时 port-forward 已关闭。
- 本机仅创建本轮所属的临时文件与一个 Kimi CLI 预检会话，保留其日志供复核；没有在 Kimi 桌面内创建业务测试项目，也未验证桌面会话自动发现该 Skill。

## 2. 准入判定

| 条件 | 状态 | 当前证据及边界 |
| --- | --- | --- |
| K8s、平台、PG、对象存储、转换器 | 通过基础检查 | docker-desktop 节点 Ready；Judex revision 21、六个 Pod Ready；readyz / system / capabilities 为 200 |
| OpenSandbox 基础组件 | 通过基础检查 | server/controller 运行，三个相关 CRD Established；尚未创建新沙箱或验证本轮固定版本挂载 |
| 发布版 CLI 获取与协议 | 通过基础检查 | Windows ZIP 真实下载、SHA256 一致；native amd64 输出发布版本 0.1.0-materials.20261009121137，协议 1 |
| CLI 失败语义 | 未通过 | 发布版与当前源码均在 judex --json report 缺少必填参数时退出 0，stdout / stderr 均为空 |
| Kimi 实际调用 Skill / CLI | 通过限定预检 | 0.36.1、默认 kimi-code/k3；真实加载 judex，执行版本与帮助，读取三份 references，并说明人工边界 |
| 多用户设备授权及桌面场景配置 | 待准备 | 本轮 CLI doctor 为 authenticated=false；未进行测试成员设备授权、桌面工作区关联或多身份上报 |
| 平台使用的标准 GLM 服务 | 未通过当前配置 | 默认仍为 Coding Plan 端点；现有凭据在标准 GLM-5.3 API 返回 HTTP 429、code=1113 |
| Kimi prefix cache 数据 | 通过可观测性预检 | 从本轮所属 wire.jsonl 获取五条 usage.record，区分首调与后续调用并去掉重复投影 |
| 平台 prefix cache 数据 | 未具备 | Provider 不保留缓存明细；journal 主要保存 totalTokens；没有可用于验收的真实平台命中率 |
| 请求工具列表稳定性 | 未通过 | 同一个 Registry 连续序列化 30 次，得到 12 种不同工具顺序；实际缓存影响仍需模型对照 |
| 平台计划 / 任务草稿 | 有实现，待真实模型联测 | propose_changes 支持 create_plan / create_task；本轮未对平台执行正向模型调用 |
| 平台自动创建流程定义 | 尚缺模型工具入口 | 现有工作变更操作不含 create_workflow；流程 API / 编辑器存在不等于 Agent 已能生成并保存该草稿 |
| 未绑定任务的候选流程 | 仍有源码缺口 | 任务级查询只读取 tasks.workflow_id；任务未绑定时无法取得计划 / 职位的完整候选 |
| 岗位分析工具权限 | 仍有源码缺口 | 岗位使用 RegisterDefaults，含 record_task_analysis；持久保存仅允许 coordinator |
| 图片预览 | 有已实现路径 | 原图 / 缩略图读取与验证存在；本轮不把已有预览能力当作模型看图或生图证明 |
| 图片语义分析 / 生成 | 未具备已验证路径 | 三个目录模型均 vision=false，GLM-5.3 为文本模型；当前 Provider Message 为文本，未验证有效图像工具链 |
| DOCX / XLSX / PPTX 转换 | 通过限定预检 | 真实转换均 200；PDF 为 1 / 2 / 2 页，正文及第二表、第二页的独立内容标记保留 |
| Office 原件理解 / 公式 / 生成质量 | 待验证 | binary read_material 只返回需沙箱挂载的提示；转换结果不代表平台已读懂原件、公式或图表 |

服务能力接口中的 agentExecution=true 仅证明接线声明。本轮标准 GLM 正向调用没有成功，因此不把该声明或历史隔离实测当作当前配置的完整 Agent 验收。

## 3. GLM 配置准入

现有部署默认模型为 glm-5.3，OpenAI-compatible Base URL 是 https://open.bigmodel.cn/api/coding/paas/v4。模型目录另含 Anthropic-compatible 编程端点及一个标准 API 的 glm-4-flash 条目；这不构成已验证可用的标准 GLM-5.3 配置。

针对现有默认凭据，向 https://open.bigmodel.cn/api/paas/v4/chat/completions 发出一次最小 GLM-5.3 请求，使用 thinking=enabled、reasoning_effort=low、max_tokens=128。返回：

- HTTP 429。
- error.code=1113。
- 提示余额不足或无可用资源包；没有成功返回模型输出或 usage。

此结果限定于实际测试的现有部署凭据，不代表所有账号或本次聊天中未消费的新 key。

[官方 Coding Plan FAQ](https://docs.bigmodel.cn/cn/coding-plan/faq)要求自建应用使用标准 API。后续准入需要一个适用于 Judex 的标准 GLM 配置和可用额度，并在隔离环境通过真实流式输出、工具调用、工具结果回传和 usage 检查。额度属于供应商账号条件，修改客户端代码不能消除这个错误。

## 4. Kimi 与缓存实证

本轮在独立目录安装仓库的 Judex Skill，用 --skills-dir 限定加载范围，通过 Kimi 提示模式调用真实默认模型。

实际工具轨迹包含 Skill、Glob、Read 和两条 Bash 命令：

1. ./judex.exe --json version。
2. ./judex.exe --help。

轨迹中的工具结果对应真实 CLI 输出。未在宿主内访问平台、登录、上传、审批、写业务文件或委派其他代理。此结果关闭“宿主能否加载技能、读取规范、执行本地 CLI”这一前置问题；完整 D03 / D04 / D06 业务闭环仍未执行。

Kimi 标准输出未携带用量；本轮从该会话自己的 agents/main/wire.jsonl 中读取 usage.record，每次请求只计一条。context.append_loop_event 中重复的用量投影不另计。

| 调用 | 未缓存输入 | 缓存读取输入 | 输出 |
| --- | ---: | ---: | ---: |
| 1 | 21905 | 0 | 59 |
| 2 | 1009 | 21760 | 122 |
| 3 | 472 | 22528 | 93 |
| 4 | 981 | 22784 | 182 |
| 5 | 2167 | 23552 | 447 |

本组缓存写入输入均为 0。按缓存读取 / 总输入计算：

- 五次整体：90624 / 117158 = 77.35%。
- 排除首调的四次：90624 / 95253 = 95.14%。

这是一个 Skill / 版本 / 帮助会话的样本，只证明 Kimi 本地侧可观测并出现真实缓存复用，不能外推到其他项目、模型、长业务会话或平台 GLM。

平台侧应补充每次调用的输入、缓存读取 / 写入、输出、usageKnown、cacheUsageKnown、供应商请求 ID、模型、协议、调用关联和延迟；采用协议各自的实际计数口径，避免把缓存输入重复叠加。缺字段为未知，不能记 0。上下文预算与账单 / 套餐积分独立，缓存命中仍占上下文窗口。[官方缓存说明](https://docs.bigmodel.cn/cn/guide/capabilities/cache)提供 prompt_tokens_details.cached_tokens；当前[模型适配](../../../internal/infrastructure/model/provider.go)与[持久用量](../../../internal/agent/batch/journal.go)尚未保留它。

工具数组应确定顺序，工具 schema 和上下文各层应有稳定版本 / 指纹；只保留受权的当前上下文，不为提高命中率跨项目混用事实或省略权限刷新。30 次工具顺序探针只证明请求不稳定，不能据此计算供应商缓存损失百分比。

## 5. 软件场景的附加准入

需要先补齐 CLI 全局解析 / 必填参数错误的非零退出和机器可读错误，不能只改一个 report 分支。发布包应与源码复验一致。

需要对齐岗位可见工具和真实持久化权限，并为未绑定任务提供计划、职位和项目已发布流程候选。保留现有正式关联；将当前节点、历史依据和后续建议分别表达。

若要求平台模型创建项目流程定义，应提供受权限和 schema 校验的草稿入口及人工发布步骤，并能从 UI / API 重新读取节点、依赖、职位约束和固定版本。现有计划 / 任务提案路径应单独测试，不能把任务执行图当作流程定义自动建稿。

独立测试成员、grant、CLI profile / 配置目录、项目 / 职位 / 任务和本地工作目录的映射，属于测试准备项。本轮没有复制真实用户凭据。正式确认边界保持 [CLI 与 Skills 设计](../v1/07-CLI与Skills.md)中的一次浏览器确认；测试驱动的合成角色决定须与真实用户确认分开留证。

相关依据：[多流程实测](../../功能/职位多流程关联实测.md)、[候选上下文](../../../internal/agent/batch/context.go)、[工具注册](../../../internal/agent/tools/task_analysis.go)、[协调者保存限制](../../../internal/collaboration/analysis.go)。

## 6. 图片与 Office 场景的附加准入

图片场景需要明确三条实际能力：本地制作 / 修改图像、平台读取固定图像版本的内容、平台评审并推进工作。生图和看图分别确认工具 / 模型及实际调用。设置 vision=true 或展示图片不构成该能力。[GLM-5.3 官方模态说明](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3)

本地 Office 工具方面，已发现 bundled Python 中的 python-docx、openpyxl、python-pptx、pypdf、Pillow，不需以默认系统 Python 的缺包推断环境不可用。转换器真实结果与 PDF 提取标记已验证，尚未进行本轮排版视觉 QA、公式重算、图表一致性、平台原件内容分析或完整交付。

当前 read_material 遇到非 UTF-8 二进制只返回 binary=true 和需沙箱挂载的提示。Office 原件理解需要验证固定版本投影、真实沙箱中的解析工具、解析结果与来源引用，再验证模型依据和缺口。不能仅把原件的用途说明或本地上报摘要当作读过文件。

现有 tests/k8s/ui.mjs 默认关闭 sandbox 和 materialProjection；需要采用或扩展确实开启这些能力的隔离入口，不能沿用该默认配置宣称 Office 原件分析 / 平台派生产物已验收。

## 7. 后续准入顺序与最终交付

1. 先确认标准 GLM 服务的合法配置、凭据和额度。
2. 关闭 CLI 错误语义、平台缓存明细与工具序列稳定性三个通用缺口。
3. 补齐软件场景所需的候选流程、岗位工具权限和流程定义草稿入口。
4. 准备测试成员、设备授权、桌面工作区 / Skill 发现和可复核的确认参与方式。
5. 软件共同门槛通过后，再设计实际项目、职位会话、正常 / 退回 / 异常路径及断言。
6. 图片和 Office 在各自工具链准入后，补充对应场景流程；逐轮保留失败、修复和复测结果。

隔离执行沿用项目要求：独立 namespace / 数据库 / 对象范围和所有权标签，同一可追溯镜像 / CLI / Skill 制品，按 manifest 清理并核对原部署。当前节点通过代理 stats 可读取约 6.94 GiB 可用内存的单次样本，不必为本次前置检查停止正常服务；真正创建测试环境时重新量测资源。Metrics API 缺失没有被算作业务阻碍。

成熟后的交付应分别报告业务闭环、真实成果质量、权限与恢复、模型调用与缓存指标。所有“未验证”项保持未验证，不用模拟模型、工具发现或状态接口代替真实执行。

## 8. 预检自身的修正

首个宿主探针从空 fixture 目录调用 installer，未能找到 Skill 来源；同一探针把提示模式与 --yolo 同时使用，Kimi 0.36.1 拒绝该组合。已改为从仓库根目录安装到本轮独立目标，再用提示模式单独执行，实际宿主检查通过。

Office 标记检查初次使用了与 fixture 不一致的预期，并遇到 Windows stdout 编码和标题换行。已按 fixture 源标记、UTF-8 和空白归一修正；PDF 原件及页数未重生成。上述均是测试探针问题，不计为产品缺陷。初始错误与修正保存在本机证据目录。
