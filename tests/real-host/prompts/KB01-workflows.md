继续KB01/U0设计负责人，团队三个职位/三任职已通过实际产品UI配置并邀请成员接受。本人的设备权限已通过实际浏览器补充新项目范围。读team-context.json、receipts/KB01-project-created.json和test-connection.json，实读本人CLI身份/项目/版本。

将以下目标作为无任务、无讨论的项目范围message经CLI submit发送，由平台真实Kimi K3协调者调用draft_workflow创建两份平台流程草稿，并record_task_analysis保存结果。你只提交业务要求，不给节点JSON，不直接HTTP/SQL建流程，不发布或审批。

1. “B-W1 视觉设计”：要求确认→方案/PNG主视觉和SVG可编辑标识→品牌审阅→修改→资源交付→负责人核对。设计负责人、视觉设计师、品牌审阅三类职位按实际职责约束节点；返工是人工退回，不建循环执行依赖。
2. “B-W2 素材适配”：固定来源确认→桌面尺寸/布局适配→透明素材导出→格式/内容审阅→资源交付→负责人核对。两个流程都要覆盖三职位；同一职位后续挂多个流程与节点。

要求先list_agent核对本项目template/positionId和identityId，节点allowedPositionIds用职位模板ID，不能混用任职ID；至少五个可读节点/流程，清楚责任、条件与验收边界，无delegationUserIds代理授权。流程正文使用不随draft/published状态失效的职责语言，草稿状态由平台状态字段表达。不能创建正式计划/任务或自动发布，不假定PNG技术元数据已完成看图，也不将SVG/PNG交付说成照片模型生成。

保存submission-workflows.json及原提交/分析/草稿引用到receipts/KB01-workflows.json；通过submission-analysis查询同一原提交，适当退避约5分钟。未生成恰好两份草稿时保留真实结果/错误，不制造第二次提交或伪造完成；由U0实际UI审阅编辑发布后再安排工作。
