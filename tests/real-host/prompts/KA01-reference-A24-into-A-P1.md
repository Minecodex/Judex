继续KA01/U0产品负责人，仅在A-P2已实际整体验收后执行。本人CLI plan show核对A-P1 c6308a0a-32c1-4f4b-9717-3755aaa81d00仍active、完整当前version与owner；A24 8b355144-610b-4349-8031-662cb3174ee0已accepted且仍归A-P2 1c27d436-d46b-492f-90d1-7e87181ffedd，A-P2已accepted。旧六任务及原失败历史保留。

通过新的项目message请平台真实GLM query_work后propose_changes生成恰好一项reference_task，targetType=plan，targetId=A-P1实际UUID，expectedVersion=当前真实版本，fields={taskId:A24完整UUID}。目的：A-P1的整体功能由A24已接受v0.1.1补丁提供明确修补证据；不复制任务、不改A24所属计划、旧任务workflow/node/status不动、不将原v0.1失败抹掉。A-P1原goal/criteria不放松，整体后续另作人工验收。

新submission只写业务意图和实际关联，不本地造changes充当平台模型。保存receipts/KA01-A-P1-reference-draft.json（sourceId/analysisId/full proposalId/真实终态）；完整decision review保存review/A-P1-reference-draft-review.json。核对仅一项、完整IDs/expectedVersion/归属/当前接受状态正确后proposal submit --no-wait，写receipts/KA01-A-P1-reference-submit-intent.json（原CLI JSON），停止待实际U0及各有权账号浏览器确认。不能直接验收A-P1、替人审批或派生子代理。
