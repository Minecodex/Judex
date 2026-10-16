继续 KA01。上次 proposal submit 的服务端字段错误根因已经定位并修复：确认意图的 OpenAPI 枚举漏了 proposal.submit，并非 reviewHash 应改成 UUID。当前 CLI / Skill / 服务都以 test-connection.json 最新版本为准（.8）。

不要重新提交分析，不新造提案，不改变业务安排。使用同一 proposal 0fe51b39-5fae-4f9d-8d27-d910d127977e，先 decision review 核对当前仍为 draft、version=1、1 计划+6 任务和固定 reviewHash。--review 必须使用实际 64 字符 reviewHash，不用 reviewId、不要拼接或猜格式。

通过 .tools/KA01.cmd proposal submit 创建不等待浏览器的提交确认意图，将旧错误回执另存保留，再将新完整 JSON 结果写到 receipts/KA01-plan1-submit-intent.json。只执行一次合法提交尝试；若仍失败，最多核对一次版本/当前 review 后报告，不再检索二进制或大量试参。成功也只能报告 pending 和 confirmUrl，然后等 U0 浏览器确认；不要批准或开始代码工作，不通过 HTTP/SQL 绕过产品。
