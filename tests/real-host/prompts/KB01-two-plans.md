继续原KB01/U0设计负责人。B项目已真实创建，两个流程已由U0产品UI编辑发布，三个职位全部经UI挂到两条流程的相关节点。本人CLI读身份、项目和版本，读team-context.json/workflow-context.json。

向本项目提交一次项目范围message，由真实K3协调者list_agent/query_work核对实际角色与发布流程，propose_changes生成恰两计划+五任务（七项create）。只发业务要求，不手写提案JSON，不直接建正式对象，不建新讨论。

- B-P1 首版视觉方案，owner设计负责人，workflow B-W1；B11 设计约束和需求（设计负责人参加，品牌审阅reviewer）；B12 PNG主视觉和SVG可编辑标识源（视觉设计师参加，品牌审阅reviewer）；B13 独立品牌审阅报告（品牌审阅参加，设计负责人reviewer）。目标为MiniIssue桌面问题跟踪软件的实际原创矢量设计、PNG光栅导出与独立看图，不声称照片生成模型作图。B11制定清楚的品牌名称/布局/尺寸/可读性/文件来源与许可标准；B12交付真实PNG、SVG和可复现渲染源，以当前CLI原生ReadMediaFile实看PNG，元数据不能替代看图；B13实读固定原件、逐SHA与原生看图，记录格式/可读性/来源/风险和具体改进，不抄maker的PASS。
- B-P2 适配与交付素材，owner设计负责人，workflow B-W2；B21 桌面尺寸和布局变体（视觉设计师参加，品牌审阅reviewer）；B22 透明素材与导出清单（视觉设计师参加，品牌审阅reviewer，品牌审阅独立核对）。B21基于已接受的首版固定来源，至少两种桌面尺寸/布局，实际看图与真实尺寸留证；B22交付实际透明PNG、SVG来源、SHA/尺寸/alpha清单，实查透明像素、边缘与深浅背景效果，不能以扩展名认定透明。

每任务仅一个上述participant和一个指定reviewer，fields.planId必须指对应create_plan.clientRef；workflowId/nodeId用本项目当前发布的真实流程及允许该参加职位的真实节点，字段名只能nodeId，不能workflowNodeId。目标和标准写清固定原件、独立核对、返工/修订与最终人工确认边界；不能自动发布/安排/验收。此次创建五任务requirements先空，生成真实UUID后另用第二提案补B11→B12→B13→B21→B22的开始硬前置，补齐前不开始任何工作。不凭clientRef编造task UUID。

保存submission文件与完整原CLI返回到receipts/KB01-two-plans-draft.json，经submission-analysis查询同一提交获得真实proposalId。实际CLI proposal show/review核对两计划+五任务、职位身份ID/节点/归属/标准，确认正确才proposal submit --no-wait；receipts/KB01-two-plans-submit-intent.json的intent字段保存完整原CLI JSON。失败保留，不重新发另一条消息或伪造草稿；等待U0及其他本人审批位的固定浏览器决定，不自己确认，不开始工作，不派生子代理。
