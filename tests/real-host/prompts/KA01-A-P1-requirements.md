继续 KA01 / U0 产品负责人。A11 需求固定版本已经由 U3 在实际浏览器验收，taskId d110d23f-d64a-4fc9-925a-5fa221e356c3，status accepted，version 4；不是仅 delivered。其余当前 A-P1 任务见 work-context.json，原六个取消任务不可再使用。

先经本人 .tools/KA01.cmd context get 核对实际 A11 和其余任务；不实现他人的代码。需要补齐既有计划的正式依赖：顶层 dependsOn 仅为提案变更顺序，不能替代 task.requirements。请把以下业务要求通过项目范围 submit message 提交给平台真实协调者，让模型 query_work 后通过 propose_changes 产生恰好五项 set_requirements 变更草稿；不要直接生成本地 changes JSON，不新增计划任务，不改 workflow/node/assignment/owning plan，不自行批准。

- A12 API：开始前硬条件 A11 task_acceptance。
- A13 页面：开始前硬条件 A11 task_acceptance。
- A14 接口测试：开始前硬条件 A12 task_acceptance。
- A15 桌面测试：开始前硬条件 A12 和 A13 各一项 task_acceptance。
- A16 发布包：开始前硬条件 A14 和 A15 各一项 task_acceptance。

requirements 每项使用 kind=task_acceptance、phase=start、hard=true、targetId=本项目当前任务的实际 UUID（不是 A11 等代码、不是 clientRef）；label 说明具体成果。fields.requirements 是完整目标列表。五项分别 targetType=task、targetId=对应正式任务 UUID、expectedVersion=读取的实际最新 version；不改变已验收的 A11。模型只建草稿，正式采纳仍由列明的人类测试账号在产品页面审阅决定。

保存本次 sourceId、analysisId、真实 proposalId 与每项的目标/条件核对到 receipts/KA01-A-P1-requirements.json。用 submission-analysis 持续退避查询同一提交，最多等约 10 分钟。不能因模型工具失败另造同名任务；失败需原样记录。草稿完成后用 proposal review CLI 读取同一固定提案；若五项及 targetId/requirements 正确，可用 proposal submit --no-wait 仅创建待确认意图，保存到 receipts/KA01-A-P1-requirements-intent.json，留给 U0 浏览器确认，不能声称审批已完成。
