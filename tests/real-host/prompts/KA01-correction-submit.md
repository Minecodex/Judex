继续 KA01。回读 receipts/KA01-plan1-correction.json 的同一 submission-analysis 最终结果，再用 decision review 核对平台实际修正草稿。当前观察候选为 008799d6-efbc-4a14-9bc1-8a57bd799d08，必须与服务端最终输出和现有正式记录交叉验证。

只允许 12 项变更：六个实际尚未执行的独立错误任务 cancel_task，以及六个 create_task，所有新 tasks 的 fields.planId 必须是既有 A-P1 c6308a0a-32c1-4f4b-9717-3755aaa81d00。逐项检查当前 expectedVersion、cancel reason、participant/reviewer、实际已发布 A-W1 nodeId、接受标准；不创建新计划，不把 dependsOn 当作归属。

核对通过后保存 receipts/KA01-plan1-correction-review.json，并仅通过 .tools/KA01.cmd proposal submit 的不等待模式请求 U0 浏览器提交确认。保存完整结果到 receipts/KA01-plan1-correction-submit-intent.json；只报告 pending / confirmUrl 后停止，不能投票或直接改正式数据。若失败，保留错误并停止，最多核对一次版本和 review，不大量猜参数。
