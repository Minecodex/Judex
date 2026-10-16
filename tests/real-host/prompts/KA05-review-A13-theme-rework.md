继续原 KA05 / U4 测试乙会话，身份 846abe01-c033-40a1-8b80-63102eacf4ca，仅用 .tools/KA05.cmd。读取 Judex Skill、当前 task context 和 receipts/KA03-A13-theme-rework-delivery.json。A13 3309af7c-36cf-4b38-8292-a3d3b232fdf7 已正式 reopen 后修订，当前 delivered v7，报告 4740cde6-7369-4d0b-b730-d79a80e42389。不能沿用旧 accepted v5 或自由模型的错误压缩摘要。

先从实际平台用 material download 下载固定 ZIP 3783470a-0d17-4be8-8b1d-c151f885287d（SHA256 811f1f8315807bdd584f4d0307220ade7f8c074b4abbf3376fc8169b295d119d）及摘要 71306532-742a-4702-9752-387f133929d4（SHA256 57fd852aa2674d37e69801cab06c42830160bdd0b15ec20de9bb053e8a447307）到新的 review/A13-theme-original/。安全解包，不执行未经复核的材料代码。对照 A11 固定需求、A15 F-1 旧失败与本次修订源文件，不更改制作人的交付、旧 QA 脚本/结果、旧失败证据。

操作员已用 hash 匹配的固定 ZIP 更新 http://127.0.0.1:18481 隔离实例：后端仍是已验收 A12 修订 ZIP 71b001d9，前端现在是上述新固定 ZIP；数据库保留本轮隔离测试数据。运行记录在 D:/workspace/go/Judex/.cache/real-host/20261012-closure/miniissue-runtime/process.json。核对运行源码与本人下载的新固定原件 SHA，不能把共享可变 web/ 当作原件。不停止公共实例，不接触正常部署、用户真实浏览器或其他凭据。

用本人新建的独立自动化脚本（可复制本人 qa15-cdp-test.js 到新路径，保留原脚本与原结果）对实际 Edge 临时 profile / 真实 API 做回归。最少保留原 19 用例判定：CRUD、状态、组合过滤/分页、真实 400/409、Unicode、zh/en × light/dark、语言与主题刷新持久化；新增非法保存值安全回退。原 F-1 必须实际切换 dark→保存 miniissue.theme=dark→刷新仍 dark，不能降门槛或只信制作人测试。数据带新标记，输出和截图写新 review/qa13-theme-output/，记录请求、返回、错误和未覆盖项。只清理本人临时浏览器。

若独立结果符合当前适用条件，task accept --no-wait 创建 pending 意图；若失败，task reject --reason 具体失败 --no-wait。都不能自行点击浏览器。保存 receipts/KA05-A13-theme-review.json 与 receipts/KA05-A13-theme-review-intent.json；后者包含 intent（原 CLI JSON）、snapshotBasis {taskStatus:"delivered", taskVersion:7, reviewerIdentityId, materialVersionIds:[上述2版本], reviewDoc:"review/A13-theme-review.md", recommendation:"accept"或"reject"}。新回归证据可 bare upload 一个 ZIP 和短摘要，CLI report progress（如需）前核对 taskVersion，避免改变待验收版本造成意图过时。最终由 U4 实际固定审阅浏览器确认。旧 A15 报告是带失败的真实测试贡献，本次修订复核不能改写它为通过。

任务聚焦，完成下载、实际回归和 pending 意图即可，不派生子代理，不批量重做其他任务，不能以 exit 0 或自述代替实际结果。
