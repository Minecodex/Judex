继续 KA01。U0 已在实际产品固定审阅中退回修正提案 008799d6，原因是所有 cancel_task 均遗漏 fields.reason。工作安排的顶层 reason/分析说明不能代替每个取消操作的必填 fields.reason；公开 schema 的 fields 为 additionalProperties=true，并未禁止 reason。不要沿用上一轮「schema不含reason」的错误判断。

原六个独立错误任务仍 ready、无报告；既有 A-P1 仍 active，没有重复计划。请用当前 CLI 回读同一退回提案和正式记录，保存拒绝事实。随后只提交新的项目范围修正 message 给真实 GLM，不提供本地 changes JSON、不直接改数据库或 HTTP formal work。

新草稿必须保持 12 项：6 个 cancel_task，实际 targetId + 当前 expectedVersion=1，每项 fields.reason 明确「未执行任务缺所属计划，保留失败记录并重建到既有 A-P1」；6 个 create_task，fields.planId=c6308a0a-32c1-4f4b-9717-3755aaa81d00、对应实际身份和已发布 A-W1 节点。不得再创建计划。说明当前拒绝错误与正确定义，使用平台 propose_changes 生成新的可执行草稿并记录原退回提案作为依据。

回读最终分析和实际 draft review，严格核对每项 reason、planId、身份、版本、6+6 数量；保存到 receipts/KA01-cancel-reason-correction.json。核对成功后可以仅用 CLI proposal submit 不等待模式提出 U0 的提交确认意图，保存完整结果到 receipts/KA01-cancel-reason-submit-intent.json，然后停止等 U0 浏览器确认与各审批位决定。不得自行投票、审批、执行代码或大量试参。
