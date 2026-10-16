继续KA01/U0产品，提出A-P4隔离故障演练的真实模型工作草稿。仅创建计划/任务，不注入或恢复故障。本人CLI实读当前A-P3及A33 f118d0a6-cb9c-42fa-8a78-60947a1bf739状态。通过项目message让平台真实GLM query_work、propose_changes生成恰1个A-P4计划+3任务（四项create）。

计划A-P4 MiniIssue测试服务不可用演练，owner产品2a2888fd-4c5b-457b-a622-33b358ca640c，workflow A-W4 4b1c2af6-6945-59ce-a04f-43c852313146。目标仅为本次测试的独立fault实例：127.0.0.1:18482、A工作区fault/子目录、操作员创建且有明确UID/PID/源SHA清单；正常Judex、18182平台入口、18481共享测试实例以及无关软件不属演练目标。
- A41 故障诊断：运维b861062d-ef8c-48ea-a591-0445000bb32d参加，产品reviewer，fields.nodeId=aw4-diag。开始硬前置A33已接受，requirements kind=task_acceptance phase=start hard=true targetId=A33真实完整UUID。操作员在该前置成立后以已接受软件固定源创建演练实例，播种少量合成数据并验证健康/持久化，再以明确受控输入使DB路径配置错误、服务无法启动。仅依照fault/scenario.json、真实日志/进程/HTTP结果定位，不把已知注入说明当作独立诊断，不修服务；交付诊断依据、影响范围、候选根因、恢复建议。
- A42 恢复与结果：后端4b02a3ee-4c71-488a-a985-827d641558f7参加，测试乙846abe01-c033-40a1-8b80-63102eacf4ca reviewer，fields.nodeId=aw4-restore。依据A41诊断，在明确自身拥有的fault实例恢复正确配置/启动，保留数据库与源文件；不删除业务记录换取成功，不操作正常服务。保存实际动作前后、进程身份/正确路径/原始HTTP健康、数据/过滤/极大页码控制和恢复结果，新固定证据。A41实际UUID生成后另补开始硬前置，之前不执行。
- A43 恢复验证和复盘：测试甲af739ffe-a283-4caf-b0ab-67e903e55704参加，产品reviewer，fields.nodeId=aw4-review。独立检查该fault实例真实健康、原数据/SHA或语义摘要保持、API关键行为、重复启动/日志与恢复来源；复盘区分真实故障原因和受控注入，不虚构生产事故；报告未覆盖和改进建议。A42实际UUID生成后另补开始硬前置。

所有任务fields.planId指同次create_plan.clientRef，fields.workflowId为A-W4，nodeId必须是真实发布节点（若上述节点不存在，先核对发布定义并报告差异，不能编造）。计划标准为三任务分别接受、隔离服务真实恢复且原数据不丢失、原因/恢复/独立验证可追溯；该计划不替代生产恢复能力或平台整体停机验收。旧事实不抹去，不另创占位任务。

保存receipts/KA01-A-P4-draft.json与review/A-P4-draft-review.json，实读核对字段名、owner/participants/reviewer/节点/前置及严格隔离范围正确后proposal submit --no-wait，保存receipts/KA01-A-P4-submit-intent.json（intent字段存原CLI JSON）。停止待真实浏览器确认，不代人审批、不开始工作、不派生子代理。
