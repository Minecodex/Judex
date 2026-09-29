export const lifecycleZh = {
 lcLoadMore:"加载更多", lcProgress: "上报进度",
 lcPlanAccept: "整体验收计划", lcReopen: "重开工作", lcChange: "提出工作变更", lcGraph: "执行关系", lcStage: "阶段 {n}",
 lcCreateHandoff: "新建交接", lcSource: "来源任务", lcTarget: "目标任务", lcSender: "发送职责", lcReceiver: "接收职责",
 lcSend: "确认发送该成果", lcSummary: "交接说明", lcReceiptHint: "接收交接不会自动验收来源任务。", lcScope: "修改范围", lcAssign: "调整职责", lcRequirements: "调整前置条件", lcCancel: "取消工作", lcReference: "引用任务成果",
 lcRequirementKind: "前置条件类型", lcPhase: "生效阶段", lcStart: "开始前", lcAccept: "验收前", lcBoth: "开始与验收前", lcHard: "必须满足", lcAdd: "添加条件", lcRemove: "移除",
 lcTaskAcceptance: "任务验收", lcHandoffReceipt: "来源交接接收", lcMaterialReady: "材料版本就绪", lcSubmitChange: "创建变更草稿", lcEvidence: "验收依据", lcRoleRequired: "此操作由当前职责绑定人办理。",
 lcNewPlan: "新建计划", lcNewTask: "新建任务", lcActivate: "工作正式生效", lcLinkMaterial: "关联材料", lcLinkTopic: "关联讨论",
 lcView: "查看", lcRemind: "提醒接收人", lcSuccess: "操作已完成", lcDraft: "草稿", lcReady: "待开始", lcWorking: "进行中", lcDelivered: "待验收", lcAccepted: "已验收", lcRework: "返工", lcCancelled: "已取消",
} as const;
export const lifecycleEn: Record<keyof typeof lifecycleZh, string> = {
 lcLoadMore:"Load more", lcProgress: "Report progress",
 lcPlanAccept: "Accept the whole plan", lcReopen: "Reopen work", lcChange: "Propose a work change", lcGraph: "Execution dependencies", lcStage: "Stage {n}",
 lcCreateHandoff: "Create handoff", lcSource: "Source task", lcTarget: "Target task", lcSender: "Sender responsibility", lcReceiver: "Receiver responsibility",
 lcSend: "Confirm sending this report", lcSummary: "Handoff summary", lcReceiptHint: "Receiving a handoff does not accept the source task.", lcScope: "Change scope", lcAssign: "Change assignments", lcRequirements: "Change prerequisites", lcCancel: "Cancel work", lcReference: "Reference task evidence",
 lcRequirementKind: "Prerequisite type", lcPhase: "Applies at", lcStart: "Start", lcAccept: "Acceptance", lcBoth: "Start and acceptance", lcHard: "Required", lcAdd: "Add condition", lcRemove: "Remove",
 lcTaskAcceptance: "Task acceptance", lcHandoffReceipt: "Handoff source receipt", lcMaterialReady: "Ready material version", lcSubmitChange: "Create change draft", lcEvidence: "Acceptance evidence", lcRoleRequired: "The current responsibility holder performs this action.",
 lcNewPlan: "Create plan", lcNewTask: "Create task", lcActivate: "Activate work", lcLinkMaterial: "Link material", lcLinkTopic: "Link discussion",
 lcView: "View", lcRemind: "Remind receiver", lcSuccess: "Action completed", lcDraft: "Draft", lcReady: "Ready", lcWorking: "Working", lcDelivered: "Delivered", lcAccepted: "Accepted", lcRework: "Rework", lcCancelled: "Cancelled",
};
