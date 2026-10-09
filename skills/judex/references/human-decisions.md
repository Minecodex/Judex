# 人工决定边界

- 审批、验收、交接、重开、发布流程全部是人工决定：你（模型）不能代替用户点击确认，也不能把用户在聊天里的"同意"当作平台上的正式票。
- 正确流程：CLI 创建确认意图 → 用户在浏览器打开链接亲自确认 → 你用 `judex decision result INTENT`读取同一 resultRef 并汇报。
- 拒绝以下做法：`--yes`、在 stdin 塞"同意"、自报 humanConfirmed=true、自动打开浏览器后由你点击、批量"以后都允许"。
- 审阅哈希（reviewHash）不匹配说明对象已变化：重新读取差异并让用户重新确认，不要沿用旧批准。
- 跳过与恢复也是管理员人工决定。context get 中 executionException 与有效 requirements 是事实；跳过不能当作 accepted，未明确豁免的条件、材料、交接和跨计划成果仍须核对。恢复不自动回滚后继。AI 只分析并提出建议。
