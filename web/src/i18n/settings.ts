const copy = {
  accountMenu: ["账户菜单", "Account menu"],
  accountInvite: ["邀请用户", "Invite people"],
  accountSettings: ["设置", "Settings"],
  accountLogout: ["退出登录", "Sign out"],
  accountInviteScope: [
    "邀请到当前项目；仅项目管理者可操作。",
    "Invite to this project. Project managers only.",
  ],
  accountLanguage: ["语言", "Language"],
  accountTheme: ["主题", "Theme"],
  accountAppearance: ["语言与外观", "Language & appearance"],
  accountLight: ["浅色", "Light"],
  accountDark: ["深色", "Dark"],
  accountBack: ["返回应用", "Back to app"],
  accountProjects: ["返回项目列表", "Back to projects"],
  accountSearch: ["搜索设置…", "Search settings…"],
  accountPersonalGroup: ["个人", "Personal"],
  accountProjectGroup: ["当前项目", "Current project"],
  accountConnections: ["连接", "Connections"],
  accountGeneral: ["常规", "General"],
  accountGeneralHint: [
    "按你的习惯调整界面，偏好只影响当前浏览器。",
    "Make the interface yours. These preferences apply in this browser.",
  ],
  accountPreferences: ["工作偏好", "Work preferences"],
  accountProjectAI: ["项目 AI", "Project AI"],
  accountTeam: ["成员与职位", "People & roles"],
  accountFlows: ["协作流程", "Workflows"],
  accountLocal: ["本地 AI 接入", "Local AI connection"],
  accountNoSettings: ["没有找到匹配的设置", "No matching settings"],
  accountLogoutFailed: ["退出失败，请重试。", "Could not sign out. Try again."],
  accountSignedOut: ["已退出交互预览", "You left the interaction preview"],
  accountSignedOutHint: [
    "项目示例保留在当前浏览器。可以返回预览，或使用登录表单连接真实服务。",
    "Project samples remain in this browser. Return to the preview or use the sign-in form to connect to the real service.",
  ],
  accountReturnPreview: ["返回交互预览", "Return to interaction preview"],
  accountPreviewNote: [
    "体验身份只用于演示不同岗位，不代表真实登录账户。",
    "Preview identities demonstrate different roles; they are not authenticated accounts.",
  ],
  accountSaved: ["自动保存", "Saved automatically"],
} as const;
export const settingsZh = Object.fromEntries(
  Object.entries(copy).map(([k, v]) => [k, v[0]]),
) as { [K in keyof typeof copy]: string };
export const settingsEn = Object.fromEntries(
  Object.entries(copy).map(([k, v]) => [k, v[1]]),
) as { [K in keyof typeof copy]: string };
