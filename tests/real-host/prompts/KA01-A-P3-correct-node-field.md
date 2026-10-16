继续KA01/U0产品负责人。上一份真实模型A-P3草稿 dcec678c-c3f0-496d-b352-3912c9b49dd5 没有被浏览器确认或批准：操作员严格字段检查发现三个create_task用了fields.workflowNodeId，而公开合同是fields.nodeId。你上一轮“全部核对通过”仅核对值、没有核对字段名，结论需要保留并纠正。原draft和原pending/过期意图保留，不冒称已正式创建A-P3，不使用原意图。

当前发行应为.16，以CLI version实测为准。草稿创建/修订/提交已共用正式字段白名单，未知字段会立即VALIDATION；平台propose_changes同样受约束，不会自动映射workflowNodeId。用本人CLI query实际任务，A-P1/A-P2与A24状态以当前事实为准；只修A-P3模型草稿，不改既有正式任务。

通过一个明确“字段合同纠正”的新项目message，请平台真实GLM重新生成1个A-P3计划3任务（四项create），完整业务要求沿用submission-ap3.json/原真实draft-review（100000条自有基准SQLite、目标低占比状态+Unicode过滤、p95至少改善20%、控制中位数恶化不超过10%、噪声不可分不通过；A31/A32/A33职责和A24前置全部不放松）。本地提交只写业务目标和错误说明，不能将修好的changes JSON当作平台模型结果。

关键：create_plan clientRef；每个create_task.fields.planId指该clientRef，fields.workflowId=A-W3 a56f66a0-ef11-5797-b244-34a18d43c322，fields.nodeId分别aw3-base/aw3-opt/aw3-compare（绝不是workflowNodeId）。owner产品2a2888fd-4c5b-457b-a622-33b358ca640c；A31参加QA甲af739ffe-a283-4caf-b0ab-67e903e55704/reviewer产品，唯一硬start task_acceptance A24 8b355144-610b-4349-8031-662cb3174ee0；A32后端4b02a3ee-4c71-488a-a985-827d641558f7/reviewerQA乙846abe01-c033-40a1-8b80-63102eacf4ca；A33 QA乙/reviewer产品。后继依赖待实际UUID生成后另提两项set_requirements，本轮无虚假UUID、不开始任务。

回读同一新analysis的真实终态，保存receipts/KA01-A-P3-correction-draft.json（sourceId/analysisId/full proposalId/原问题与旧草稿ID），完整实际decision review写review/A-P3-corrected-draft-review.json。核对每项真实字段名/值、归属、人数、标准后proposal submit --no-wait，只写receipts/KA01-A-P3-correction-submit-intent.json，停止待实际浏览器确认；不能代人批准，不reset旧analysis预算，不派生子代理。
