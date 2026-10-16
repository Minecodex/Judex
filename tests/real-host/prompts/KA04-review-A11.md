继续 KA04 测试甲。A11 已由 U0 在产品路径开始并完成真实需求文档上传和交付上报，当前是 delivered，不是已验收。读取 work-context.json 与最新不可变交付回执 receipts/KA01-A11-delivery.json；只用 .tools/KA04.cmd 核对本人测试甲身份，再 context get 当前正式 A11 d110d23f-d64a-4fc9-925a-5fa221e356c3，不用已取消旧任务。

你是 A11 的实际 reviewer，但不是此任务的制作参与者。核对固定材料版本 781f9fbd-d40e-4bac-8c5c-7314d9061964、sha256 与当前本地 docs/requirements.md；不以可变本地文件代替固定版本来源。可以通过已授权 CLI 读取材料信息与真实 task activity / analysis；不能绕过权限去读取其他会话凭据。

审阅要求：接口/字段/状态转换、过滤与分页边界、持久化、统一错误格式、桌面中英深浅主题、真实测试和可重复启动交付条件是否足够明确供 A12/A13 实现。明确列出未决或冲突项。当前没有 API/页面实现，不能宣称软件已测试通过。只评估 A11 需求文档本身是否可接受。

保存本地 review/A11-review.md 和 receipts/KA04-A11-review.json，说明固定版本、当前任务 version、依据与建议 accept 或 reject。不要自己点击浏览器验收，不报告 A14 已完成，不把平台模型结论当作最终决定。若判断通过，可以只使用 CLI task accept 提出固定验收意图的不等待模式，保存 receipts/KA04-A11-accept-intent.json，并报告 confirmUrl 等 U3 在浏览器审阅确认；有关键缺口则报告具体退回理由并等待对应人的决定，不伪造成功。
