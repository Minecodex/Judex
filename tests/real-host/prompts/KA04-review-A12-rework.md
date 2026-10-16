继续原 KA04 / U3 测试甲，使用 .tools/KA04.cmd 与本人 reviewer 身份。A12 Unicode 修订已完成，新报告 332f38c3-585c-4310-b0a8-c323f51cb9fa，当前应 delivered v6；原 v4 失败与 v5 正式退回已保留。读取 receipts/KA02-A12-rework-delivery.json，回读当前任务和最新报告，不能重复验收旧 v4。

通过 material download 取得新固定源码 ZIP 71b001d9-3035-48ca-aa43-361ab41477b1 和摘要 bb06019b-5ff6-4791-947f-86426794e26f，核对 SHA。解压到新 review/A12-rework-original/，检查 ZIP 名称/路径边界，不覆盖 v4 原件或 QA 原始失败输出。核对关键修改与固定 A11 §4：完整 Unicode casefold、参数化查询、instr 字面匹配、组合过滤和分页；源码不得悄悄裁剪 A11。

从新固定原件真实执行 36 项开发者测试，并把本人此前 review/a12_independent_check.py 的逐字节原样副本放在新原件旁运行，实际重测全部 62 项与 F1。不要采用制作人 62/62 自述代替自己的结果，不改 QA 脚本来使其通过。清理自己的临时端口/SQLite，不能停止 18481 公共测试服务或修改制作人源码。

结果写 review/A12-rework-review.md、新的独立结果输出与 receipts/KA04-A12-rework-review.json。若已满足当前适用标准，经 CLI task accept --no-wait 仅提出 pending 意图；否则 task reject --reason 使用具体新失败。保存 receipts/KA04-A12-rework-review-intent.json 的完整 intent+snapshotBasis，含 taskStatus delivered、实际 taskVersion、reviewerIdentityId、两份 materialVersionIds、主版本、reviewDoc、recommendation。正式决定仍由 U3 浏览器确认；不开始/上报 A14。
