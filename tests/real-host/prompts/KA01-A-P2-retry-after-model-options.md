继续原KA01，保留A-P2最初失败回执。平台现为 .15；token/round上限不变，原模型glm-5.3与glm-5.3-flash仍用Coding Plan。两款真实API均验证thinking.type=disabled可调用工具（不是免费降级），该非秘密选项现已通过目录同步/数据库/Resolver/Provider接线。另修正常SSE choices.finish_reason层级和length截断失败，避免空返回冒充完成；没有重置旧预算、造新提交或SQL改业务。

当前只处理原业务提交82afdcad-a000-4ea4-b8c7-3f61d04d586c /分析a98c7a60-9b7a-453f-af1f-3d18f937811c，先读取其实际状态/剩余额度和旧收据。通过实际task-analysis retry合法续跑同一批次；平台若拒绝次数/预算则保留原因，不造第二份相同提交。继续同一个submission-analysis退避等终态，实际失败仍如实报告。你不能自己写changes替代真实模型。

目标保持上个业务要求：A-P2一个计划、A21~A24四任务，owner产品，正式A-W2；每个任务fields.planId明确归同一新计划；角色/reviewer/node aw2-repro/fix/reg/patch准确。A21开始硬依赖已经accepted的A16；A22~A24的实际UUID生成后另提三项正式开始依赖。A-P1未整体验收，极大合法页码500违反旧标准；补丁通过后另以reference_task正式引用A24，不放松原条件，不移动归属，不覆盖旧失败。

若真实模型产出可复核草稿，以服务端实际proposalId核对恰好五项create（one plan/four tasks），不得抄旧不存在的proposalId或inbox A-P1计划提醒。核对输出/条件/分工/来源正确后，仅proposal submit --no-wait创建pending意图，写新receipts/KA01-A-P2-retry-result.json与receipts/KA01-A-P2-submit-intent.json；原failed回执不覆盖。到此停止，正式提交和各人审批由真实测试账号浏览器承担。回执用实际CLI版本与系统时间，不派生子代理。
