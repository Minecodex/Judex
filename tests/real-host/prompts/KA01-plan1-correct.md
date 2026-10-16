继续 KA01。上一份提案 0fe51b39-5fae-4f9d-8d27-d910d127977e 已经由 U0～U4 在产品固定审阅中各自批准，平台确实建立了一个计划和六个 ready 任务。但实际正式结果验收发现语义缺陷：六个任务的 fields.planId 全部缺失，顶层 dependsOn 仅控制同一变更集的创建顺序，不建立 owning plan，也不建立正式任务的运行前置关系。

这是真实失败样本，尚未执行任何任务，也没有工作报告。不要删除记录、直接修改数据库或创建相同任务后冒充正常闭环。使用 .tools/KA01.cmd 回读当前 A-P1 与六个独立任务，保留错误原提案和 formal IDs，核对 task.status=ready / version。

通过项目范围 message 把修正业务要求提交给平台真实模型，目标是创建一个新的修正提案草稿：撤销上述六个尚未执行的错误独立任务（cancel_task，实际 targetId 和当前 expectedVersion），并在已有 A-P1 计划中重新创建六项对应 A11～A16 的正确任务。不能再次创建计划。每个 create_task 的 fields.planId 必须是当前已存在、active 的 A-P1 实际 UUID；participant/reviewer、已发布 A-W1 nodeId、预期输出和验收标准保留原业务要求。创建顺序不代替计划归属或硬前置条件。

若需要执行前置关系，本次不要把 clientRef 塞进 UUID targetId；先完成正确定义和归属，实际任务前置另以受审批的 set_requirements 后续绑定真实 task UUID。制作和验收身份仍分别明确：A11 U0/测试甲，A12 后端/测试甲，A13 前端/测试乙，A14 测试甲/U0，A15 测试乙/U0，A16 运维/U0。

你只能提交业务修正要求，让真实 GLM 用 propose_changes 形成草稿，不在本地写 changes JSON 替代模型，不通过 HTTP/SQL 直接改正式数据，不批准或实现代码。核对修正提案至少六个 cancel_task 和六个 create_task，每个 new task 必须有实际 planId；保存 submission、analysis、proposalId 和核对依据到 receipts/KA01-plan1-correction.json。完成后停止等 U0 审阅。若存在非法操作或来源不明，保存错误并报告，不自行绕过。
