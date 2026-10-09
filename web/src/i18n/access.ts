export const accessZh = {
  accessDevice: "授权本地设备", accessCode: "设备码", accessReview: "查看授权请求",
  accessDeviceName: "设备", accessScopes: "请求权限", accessProjects: "允许访问的项目",
  accessApprove: "确认授权", accessDeny: "拒绝", accessDone: "操作已完成",
  accessInvite: "项目邀请", accessAccept: "接受邀请", accessMismatch: "当前账号与受邀邮箱不一致，请切换账号。",
  accessAccount: "当前账号", accessExpires: "有效期至", accessState: "状态",
  accessSelectProjects: "请选择允许此设备访问的项目。", accessUnavailable: "请求无法加载，请重试。",
  accessChanges: "变更内容", accessEvidence: "验收成果", accessReviewFirst: "先审阅固定成果和版本，再确认决定。",
  accessConfirm: "确认决定", accessCancel: "取消", accessNoChanges: "没有可审阅的变更。",
  accessReport: "成果说明", accessReload: "重新加载审阅", accessReportId: "报告", accessMaterials: "材料版本",
} as const;
export const accessEn: Record<keyof typeof accessZh, string> = {
  accessDevice: "Authorize a local device", accessCode: "Device code", accessReview: "Review request",
  accessDeviceName: "Device", accessScopes: "Requested permissions", accessProjects: "Allowed projects",
  accessApprove: "Authorize", accessDeny: "Deny", accessDone: "Action completed",
  accessInvite: "Project invitation", accessAccept: "Accept invitation", accessMismatch: "This account does not match the invited email. Switch accounts to continue.",
  accessAccount: "Current account", accessExpires: "Expires", accessState: "Status",
  accessSelectProjects: "Select the projects this device may access.", accessUnavailable: "Unable to load the request. Try again.",
  accessChanges: "Changes", accessEvidence: "Delivery under review", accessReviewFirst: "Review the fixed evidence and versions before confirming.",
  accessConfirm: "Confirm decision", accessCancel: "Cancel", accessNoChanges: "No changes available for review.",
  accessReport: "Report", accessReload: "Reload review", accessReportId: "Report", accessMaterials: "Material versions",
};
