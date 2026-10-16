继续KA01/U0产品负责人，本轮仅提出A-P3正式工作草稿，不执行性能工作。通过本人CLI提交项目message给平台真实GLM，以query_work查询实际工作、propose_changes生成模型草稿；不本地写changes代替真实模型。

A-P3 MiniIssue列表查询性能改进：owner产品2a2888fd-4c5b-457b-a622-33b358ca640c；workflow A-W3 a56f66a0-ef11-5797-b244-34a18d43c322。恰好1计划3任务，每项fields.planId引用create_plan.clientRef，dependsOn仅创建顺序。
- A31 固定基准与性能目标：测试甲af739ffe-a283-4caf-b0ab-67e903e55704参加，产品reviewer，node aw3-base。开始时必须满足硬前置A24补丁包8b355144-610b-4349-8031-662cb3174ee0已accepted（kind task_acceptance、phase start、hard true）；当前A24若仍working只创建该依赖，不提前开始。之后从固定A24原件核对SHA，在自己的临时SQLite/HTTP实例生成固定种子100000条合成缺陷数据，遵守原合法状态；选择低占比状态+Unicode关键词过滤的真实列表查询为优化目标，包含无过滤/普通分页/极大空页控制。固定数据摘要、机器/版本、预热与重复次数、逐次原始延迟、p50/p95、正确结果摘要和EXPLAIN定位；正式预定目标为同环境同数据目标查询p95至少下降20%，正常控制中位数不得恶化超过10%，噪声无法区分目标则不通过。交付可复跑脚本、原始结果、源版本与目标合同。合成100000条可对自己的基准SQLite按相同schema事务批量播种，不能逐条制造平台业务事实；不写入18481共享运行库或平台PG。
- A32 列表性能改进：后端4b02a3ee-4c71-488a-a985-827d641558f7参加，测试乙846abe01-c033-40a1-8b80-63102eacf4ca reviewer，node aw3-opt。从接受的A31基准与A24固定后端定位瓶颈，做可解释优化，保留A11全部行为、Unicode字面子串/%/_/总数/排序/极大页码，38 API和20 UI原门槛；不得改基准数据/目标/原脚本换取通过。交付新固定后端ZIP、实际测试与对比。A31实际UUID产生后另行set_requirements补硬前置，之前不开始。
- A33 独立性能比较：测试乙846abe01参加，产品reviewer，node aw3-compare。独立下载旧新固定原件及A31数据/脚本，SHA核对，同机器交错运行重复测量，核对全部结果等价和目标/控制阈值，保留逐次原始数据、统计与异常，不能仅相信开发者PASS。A32实际UUID产生后补硬前置，之前不开始。

计划整体验收：三项均人工接受；独立A33证明预先固定目标与正确性条件满足；旧版本、原始基准与失败保留。未测负载/并发不得声称通过，不改变旧任务正式流程，不创建多余占位任务。

回读同一submission-analysis到真实完成或失败，保存receipts/KA01-A-P3-draft.json（sourceId/analysisId/proposalId），decision review完整保存review/A-P3-draft-review.json。核对实际恰好四项create、归属/owner/participant/reviewer/node/A31前置和阈值无误后，仅proposal submit --no-wait创建pending，写receipts/KA01-A-P3-submit-intent.json，停止等待U0实际浏览器确认；不替他人审批，不开始A31，不派生子代理。
