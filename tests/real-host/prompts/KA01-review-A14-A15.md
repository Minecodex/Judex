继续原 KA01 / U0 产品负责人。A11/A12/A13 的实际适用任务验收已完成；A14 和 A15 是两名 QA 的独立测试报告，当前均 delivered v4。你是这两个测试报告任务的实际 reviewer，但不替代测试人员出具贡献。

只用 .tools/KA01.cmd，读取 receipts/KA04-A14-delivery.json 与 receipts/KA05-A15-delivery.json，context get 回读各任务的最新 version、报告与验收口径。经 material download 获取各自固定证据包并校验 SHA（A14 e4965fd7 前缀的实际完整 UUID 以回执为准；A15 26d7f3b9-8a68-4128-b7aa-734b9d46ebfb）。安全读取报告、脚本与实际输出；两份独立来源分别核对，不用制作人概括替代原件。需要截图时只看包内已选择的实际截图，不读取其他会话私有文件。

分别判断测试报告这个交付成果是否满足任务口径。A14 本次 36 开发者复跑+62独立复核+4真实500路径均通过；A15 真实桌面/四套语言主题组合18/19，主题跨刷新失败按原要求显性记录。测试工作报告真实完整可以接受，但不能把 A15 报告 accepted 写成产品测试全部通过；主题未修前不接受完整软件/计划。保留未覆盖与失败来源，不能降低标准。

写 review/A14-owner-review.md、review/A15-owner-review.md 与 receipts/KA01-A14-A15-review.json。每项通过其报告标准时仅经 task accept --no-wait 提出各自 pending 意图；不通过则带具体 reason 的 task reject。各自保存 receipts/KA01-A14-review-intent.json 和 receipts/KA01-A15-review-intent.json（完整 intent、snapshotBasis taskStatus delivered、实际 taskVersion、U0 产品 reviewerIdentityId、对应 materialVersionIds、reviewDoc、recommendation）。正式决定由 U0 实际浏览器审阅确认。不要开始 A16 或验收计划。

最后给出主题缺口处理建议：可由 A13 实际 reviewer U4 走正式 task reopen 后，原前端会话实现 localStorage 持久化，上传新固定版本并重测；你不能自己替 U4 的职位或修改前端。整体软件仍未闭环这一事实必须保留。
