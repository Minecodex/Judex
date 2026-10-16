继续KX01/U1后端新会话，仅 .tools/KX01.cmd，身份与KA02相同。A22已由U4实际浏览器accepted v5，当前task context回读为准。本次是已批准prefix cache专项对照，不做重复交付、不造正式工作或改偏好/流程。

通过公开CLI顺序提交两份明确的缓存对照message（purpose=message/discussionIntent=question/taskId=A22/materialVersionIds=[92be4fd4-3fc3-4a7d-888c-daa33d287cbc]），正文完全相同，仅clientSubmissionId各自新UUID：
“缓存专项对照：只核对A22当前正式工作流与节点、A21开始条件及固定短摘要的来源。以query_work/read_material必要片段保存公开record_task_analysis，明确摘要是制作人声明，正式接受来自当前人工事实；不重新测试、不委派岗位、不提工作变更、不创建讨论或验收。未覆盖保持明确。”

每份submit成功后查询同一个submission-analysis直到completed或真实终态，再开始下一份；保存各sourceId/analysisId/batchId和实际version状态。两次之间回读A22，不变更工作状态；若当前事实改变则记录对照不纯，不强行算同前缀。不要重试相同message骗预算，本次两份是用户明确批准的独立cache测试刺激，失败和成本全部保留。

写receipts/KX01-cache-pair.json，scope明确“真实GLM平台同身份/同任务/同正文两次调用对照”，不是全局冷/热缓存承诺。Kimi stream-json没有usage，cache仍未知；不能从LLM文本猜GLM命中，用量与命中字段由操作员在model_calls按这两个batch实际查询，明细仍24h TTL、永久仅汇总。不要上传raw日志或请求ID，不派生子代理。
