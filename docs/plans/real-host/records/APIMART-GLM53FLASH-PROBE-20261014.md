# APIMart GLM 5.3 Flash 准入探测 2026-10-14

用户提供APIMart官方模型/文档链接及本次Key，明确要求先探测能否用于后续联测。本轮只使用合成输入验证接口、共享适配器与临时Kimi CLI；没有更改已部署模型目录/Secret、原业务会话、任职、计划、任务或正式决定。

结论：可以作为后续测试的候选。实际请求模型为`glm-5.3-flash`，网关返回`ZHIPU/GLM-5.3-Flash`。文本、工具往返、SSE、受控图片输入、Kimi CLI原生工具及Judex共用Go适配器小样本通过。关闭思考不兼容，缓存写入仍有未报告项；完整业务恢复验收未执行。

## 官方契约与实际身份

- [APIMart概览](https://docs.apimart.ai/cn)指定OpenAI兼容Base URL `https://api.apimart.ai/v1`。
- [模型列表契约](https://docs.apimart.ai/cn/api-reference/texts/models/list)支持GET `/v1/models?expand=parameters&category=chat`。实际HTTP200，Key可见190模型，精确`glm-5.3-flash`存在，`owned_by=custom`、endpoint=openai、category=chat、标签Text/File；没有返回该模型的参数Schema或上下文上限。
- [余额契约](https://docs.apimart.ai/cn/api-reference/account/token-balance)实际HTTP200、success=true、余额非零。文档说明单位取决于站点配置，本次未据余额字段推算美元或可用次数。
- 用户第一个链接是带Embedding过滤的chat目录，目标按其具体GLM链接识别为chat模型，本轮没有执行Embedding测试。
- HTTP/SSE实际响应模型字段均为`ZHIPU/GLM-5.3-Flash`，与请求别名区分记录；这是中转报告的名称，不是对其底层权重的独立认证。

## 实际探测结果

| 项目 | 实际结果 | 范围 |
| --- | --- | --- |
| 文本 | HTTP200，准确返回约定测试标记，usage有完整prompt/completion/total | 普通非流式chat |
| 工具调用 | HTTP200，真实function调用、正确JSON参数、非空调用ID | OpenAI嵌套function结构 |
| 工具回传 | HTTP200，取得工具执行时新生成的随机标记并准确返回 | 先调用、再返回，不能提前猜答案 |
| 当前客户端推理字段 | `thinking.type=enabled`被接受；assistant工具回放不带reasoning_content仍正确完成 | 匹配Judex当前序列化方式 |
| SSE | HTTP200，text/event-stream、145帧、finish和DONE、末帧usage齐全 | 未把chunk累计重复计数 |
| 受控图片 | HTTP200，逐字OCR `JK3-F92A00`、左红方形/右蓝圆形全部正确 | 直接image_url/data URI；接口样本效果，不代表全部品牌审阅/OCR |
| Kimi CLI0.36.1 | 临时独立目录实际Read与ReadMediaFile，文件随机标记及图上文字/颜色/形状全部正确，exit0、30.54s | 不续接或改写13个原业务/专项会话，不加载业务profile，不装新依赖 |
| Judex Go OpenAIProvider | 两次真实流式调用，工具参数及回传标记正确，finish/usage正常；输入201/245、输出48/70、缓存读取0/0 | 当前共用生产适配器直接调用，不是假响应；未经过完整worker/数据库业务链路 |

合成图片SHA为`72b90e235e56e3b7983ca1e7d6d72b9167d9e1e9c17cf0b704a20896edc13c83`。正式PNG/SVG/Office品牌和版面判断、长上下文、并发限额及异常恢复仍需在真正续跑中验证。

## 思考与缓存

- 关闭思考的单次请求HTTP400、code1210：供方明确该模型始终思考，不支持关闭，提示low/high/max。失败原件保留，不归quota/auth，不触发其他模型。
- 改为`thinking.type=enabled`和`reasoning_effort=low`后，两次同前缀请求均HTTP200。每次prompt_tokens=2681；第一请求cached_tokens=0，第二2624，第二读取率约97.87%。仅两例、同输入集合，不能称供应商全局冷缓存、持续保证或费用节省。
- 非流式还返回`input_tokens=0/output_tokens=0`等其他协议字段；本次按OpenAI的prompt_tokens/completion_tokens取数，实际Go适配结果正确。不得选择其他协议的0覆盖有效计数。
- Go流式缓存写入为null，保留未报告状态；不能从非流式的其他命名字段推填流式或原Kimi CLI未知计数。原生CLI stream-json仍未提供供方usage，与直接API数据分开。
- 当前Judex RequestOptions只有thinkingType；显式reasoning_effort=low尚未配置到平台。但当前Go adapter以enabled、无effort覆写的真实调用已通过，不是接入阻塞。

## 保留及后续接入

Key只从当前线程用户消息载入进程，发往指定HTTPS端点；未写入脚本、命令行、报告、全局Kimi配置或原namespace Secret。探测回执及无秘密脚本在忽略目录`.cache/real-host/20261014-apimart-probe`。临时CLI会话`session_2a196fc2-10ee-4a81-a19a-18cc7c1ffc96`是此次新授权的前置探测，不是原业务身份接岗或重复交付。

03:06:08UTC实际只读system/model API确认原`.21`与协调者/职位Kimi K3仍保留；没有自动续跑或新增正式意图。原17/30任务接受、4/9计划验收及未完成B/C/A-P5仍按[原业务检查点](KIMI-K3-CONTINUE-20261014.md)继续。

如后续按用户指令切换，应仅更新原测试namespace的两个模型目标及新Key Secret引用，保持原source/analysis/batch/预算与会话ID。公开Kimi CLI采用process-local OpenAI provider与本次Base URL/model；已有会话的模型改变需单独记录。本轮只证明准入小样本，不把候选探测当完整业务验收。
