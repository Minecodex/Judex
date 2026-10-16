继续 KA04 / U3 测试甲。Kimi 原生额度预计已重置，现在用原会话核对 A12，不创建新会话或任务。只使用 .tools/KA04.cmd 和本人 QA 甲 identity af739ffe-a283-4caf-b0ab-67e903e55704；你是 A12 reviewer。任务 f3309487-a97d-4880-95ab-6eb1d290e69c 已上报 delivery，当前应为 delivered v4，报告 6b8985e1-ef41-4fb1-9e58-369084616522，尚未验收。

读取发行 Skill、receipts/KA02-A12-delivery.json、固定 A11 需求、A12 正式接受标准。通过 context get 回读当前状态与 latestReportId，使用 material download 核对 11 个固定原件的 SHA，按回执中的 file 路径重建到 review/A12-original/；验收依据为不可变上传原件，不以共享可变 backend/ 代替。只允许写 review/ 与本人的 receipts/；不能改制作人的源文件或 tests/api，也不能开始/上报 A14（A14 是依赖 A12 accepted 的另外正式任务）。

实际运行原件里的开发者测试，并独立选取真实 HTTP 复核用例：字段/异常 JSON、错误格式、全部状态转换、过滤的大小写/Unicode 内容/组合语义、分页、SQLite 重启持久化、静态路径边界。使用临时端口、独立临时 SQLite，从固定来源启动，真实请求，不能 mock 或沿用开发者 34 passed 当作自己通过。对照需求和说明的冲突、已知限制分别判断，不能把制作人自述“限制”自动当作已获授权的需求裁剪。检查和清理只涉及你自己创建的进程/数据库。

把实际命令、退出码、测试及复现数据写 review/A12-review.md 与 receipts/KA04-A12-review.json；未通过则说明具体要求、实际结果与所需修订，未覆盖显性保留。不要为推进流程忽略失败。

根据真实核对结论，可经 CLI task accept 或 task reject（拒绝必须具体 reason），仅 --no-wait 创建 pending 验收意图，不点击浏览器、不自己声称决定已经生效。把完整 intent 和 snapshotBasis 保存 receipts/KA04-A12-review-intent.json；snapshotBasis 包含 taskStatus=delivered、最新 taskVersion、reviewerIdentityId、materialVersionId（API 说明主版本）及全部 materialVersionIds、reviewDoc、recommendation（accept/reject）。最终由 U3 测试账号在实际浏览器固定审阅页确认。
