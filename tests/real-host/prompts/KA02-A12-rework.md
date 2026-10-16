继续原 KA02 / U1 后端身份，当前恢复用原生 Kimi 模型。A12 f3309487-a97d-4880-95ab-6eb1d290e69c 已由 U3 测试账号在浏览器正式 reject，当前应 rework v5；真实理由见 review/A12-review.md、receipts/KA04-A12-review.json。制作人 34 项测试虽然通过，独立复核 61/62 找到 Unicode 过滤缺陷；不能沿用此前“全部通过”或降低 A11 要求。

只使用 .tools/KA02.cmd，读取当前 context 和真实退回理由；本人后端 identity 4b02a3ee，不混用运维。不改 A11，按原要求实现完整 Unicode 不区分大小写 substring 匹配（如注册 SQLite str.casefold 函数），保持 SQL 注入/百分号下划线字面匹配、组合过滤与分页。补独立非 ASCII 大小写回归用例，真实重跑全部 HTTP+SQLite unittest，并用 QA 留下的复現核对修订；保留旧版原件和失败报告。只改本人的 backend/、tests/api/、docs/API.md 与回执；别改前端或 QA 原始检查脚本/结果。

清理此前仅 ASCII 的限制说明，记录改动、实际命令/退出码/用例数/未覆盖项；不能自验收。请把本人的完整源文件和测试/说明打成一个可复核 ZIP（排除数据库、凭据、运行时缓存、他人前端/QA文件）；再另存简短修订与测试摘要文件。

重要：此次上传使用 bare material upload PATH，不带 --task、--plan、--purpose；这些关联选项会逐文件生成独立自动分析，我们要由一份交付报告统一引用本次包和摘要。bare 上传返回 uploadStatus=ready、registrationStatus=not_requested 表示原件已 ready、未另行登记用途，不是失败。读取固定 versionId/SHA，将这两份原件放进一份 delivery report 的 materialVersionIds，回读最新 expectedTaskVersion 后正式 report --task A12。报告本身形成任务与材料关联并触发一次真实平台分析。不要把 ZIP 误当作 upload 不支持，也不要再逐文件提交资料用途。

保存 receipts/KA02-A12-rework-delivery.json，记录旧报告、退回依据、新材料版本/SHA、真实测试和新报告以及待验收状态；不覆盖原 receipts/KA02-A12-delivery.json 或伪造已通过最终验收。
