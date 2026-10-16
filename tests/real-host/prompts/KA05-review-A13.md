继续 KA05 / U4 测试乙原会话，只用 .tools/KA05.cmd 与本人 identity 846abe01-c033-40a1-8b80-63102eacf4ca。当前 A13 3309af7c-36cf-4b38-8292-a3d3b232fdf7 delivered v4，报告 3f8f990c-e39f-4cd4-86d9-407300f98672，尚未验收；你是实际 reviewer。读取 Skill、receipts/KA03-A13-first-delivery.json、固定 A11 需求与当前 task acceptanceCriteria。material download 9 个固定原件到 review/A13-original/，逐 SHA 核对，不能以可变共享 web/ 冒充验收来源。

操作员已经在 http://127.0.0.1:18481 启动本轮隔离 MiniIssue，后台与前端源码均逐字节匹配它们已上传的固定交付版本，独立 SQLite 在 Judex 的 .cache/real-host/20261012-closure/miniissue-runtime 下；只访问该本地测试地址，不能接触正常平台业务或其他浏览器标签。你可读取公开运行记录，不停止这个公共测试服务。用自己创建的无头 Edge/Chromium 临时 profile 做真实桌面交互，完成后清理自己的浏览器/临时文件；不读取或控制用户其他真实浏览器。

实际独立复核 A13 当前适用的功能条件：列表、新建、编辑、合法/非法状态动作、状态+keyword 组合、分页、服务端错误显示，全部对真实 API，不能使用 mock 或把制作人的 16 单测/7 加载检查代替本人全路径。测试数据写入仅这个隔离 MiniIssue 的测试 DB，可使用带 QA13 标记的数据，至少新建与编辑走实际页面。记录当前源码版本与实际请求/结果。对静态穿越或后端 Unicode 缺陷等其他任务问题明确标记归属，不把制作前端当作代修后端。

尊重当前正式任务范围：A13 验收标准显性把中英/深浅主题列为后续检查，因此检查到的主题刷新持久化缺口应如实保留给 A15；不能仅凭这项就把 A13 当前功能否决，也不能声称整个桌面版本已验收。若实际功能不通过，依据具体用例建议 reject；若适用条件通过，建议 accept 并保留后续范围。不能为推进而忽略失败。

保存 review/A13-review.md、独立自动化脚本/实际结果及 receipts/KA05-A13-review.json。当前不要上报 A15（它须 A12+A13 已验收才能正式开始）。可接受时经 CLI task accept --no-wait 只创建 pending 意图；不可接受则保存明确拒收理由。CLI 拒收命令正在补齐，新发行可用后再申请，不能用 accept 冒充拒收。统一回执 receipts/KA05-A13-review-intent.json 包含 intent（或 null）、snapshotBasis（taskStatus=delivered、最新 taskVersion、reviewerIdentityId、materialVersionId=主 HTML 固定版本与全部 materialVersionIds、reviewDoc、recommendation），最终由 U4 实际浏览器固定审阅确认。
