继续 KA01。平台已形成 A-P1 的实际安排草稿，不要重新提交业务需求或新造提案。回读 receipts/KA01-plan1-draft.json 中同一 submission-analysis 的最终结果，再用 decision review 查询真实 proposalId；当前已观测候选为 0fe51b39-5fae-4f9d-8d27-d910d127977e，应以服务端实际结果交叉核对。

严格核对一个 A-P1 计划、六个 A11～A16 任务，计划 owner 和每项 participant/reviewer 实际身份、当前已发布 A-W1 nodeId、预期输出和可复核接受条件。保存完整草稿审阅到 receipts/KA01-plan1-review.json。若数量、职责或来源不符，报告并停止，不代人修订成已批准。

草稿符合要求时，仅通过 .tools/KA01.cmd proposal submit 创建提交确认意图（不等待浏览器的模式），把完整 JSON 保存到 receipts/KA01-plan1-submit-intent.json；报告固定审阅 hash 与 confirmUrl 后停止。此步只是请 U0 在浏览器把草稿提交审批，不能自行投票或批准，不开始任何代码工作。后续每个有权人仍需在固定审阅上确认自己的审批位置。
