export const agentsZh = {
  projectRobots: "项目机器人",
  personalPrompts: "我的提示词",
  robotsIntro:
    "给每位伙伴一份清楚的职责。机器人承接工作与记录，对应的人作出决定。",
  createRobot: "创建机器人",
  robotName: "伙伴叫什么",
  robotRole: "负责什么岗位",
  robotBinding: "绑定的决策人",
  robotNamePlaceholder: "例如：小验",
  robotRolePlaceholder: "例如：质量测试伙伴",
  robotPrompt: "项目职责提示词",
  robotPromptHint:
    "写清楚负责什么、需要哪些依据、何时找人决策。此处的要求只用于当前项目。",
  robotCreateHint:
    "创建后可在新议题中选择它作为负责伙伴或审批岗位；已有议题的参与人与审批席位保持原样。",
  robotRequired: "请填写名称、岗位和职责提示词。",
  robotCreated: "机器人已加入当前项目",
  editRobotPrompt: "编辑职责",
  saveRobotPrompt: "保存职责提示词",
  robotPromptSaved: "项目职责提示词已保存",
  promptPreview: "查看组合提示词",
  promptPreviewTitle: "这位伙伴如何理解工作",
  promptPreviewHint:
    "这是当前配置的组合预览，也用于本地工作简报。Demo 使用固定示例回复，尚未连接真实 AI。",
  promptBoundary:
    "提示词影响工作方法与表达，不能修改审批权、资料权限或已经确认的工作安排。",
  promptPlatform: "平台边界",
  promptWorkflow: "项目协作约定",
  promptRole: "机器人职责",
  promptPersonal: "本人项目偏好",
  promptTask: "当前议题与交付条件",
  promptDefault: "默认职责 · 尚未自定义",
  promptVersion: "职责 v{version}",
  promptUpdated: "由 {name} 更新",
  promptSaveConflict:
    "保存失败：权限或版本已变化。请关闭后重新打开，核对最新配置。",
  personalIntro:
    "让这个项目里的伙伴更懂你的工作习惯。它属于你，不随机器人交接。",
  personalLabel: "我希望在这个项目中这样协作",
  personalPlaceholder:
    "例如：先给结论和待我决定的内容，再给依据。技术建议附验证方式，不确定的地方明确标注。",
  personalHint:
    "仅适用于你在当前项目所选职位的机器人。每个职位和项目独立设置；前任的私人配置不会展示给继任者。",
  personalPrivate: "本人可见",
  personalSave: "保存我的提示词",
  personalSaved: "你的职位提示词已保存",
  personalClearHint: "留空保存即可清除个人偏好，继续按项目职责工作。",
  personalAppliesTo: "在此项目中为你工作",
  personalNoRobot: "你暂未承担本项目的职位，分配职位后可以设置对应的提示词。",
  personalExample: "填入一个示例",
  personalExampleText:
    "先给结论和需要我决定的事项，再列依据。方案尽量给出一个推荐选择及其取舍；技术结论附验证方法。未验证的内容明确标注，不擅自扩大工作范围。",
  personalNoAccess: "当前身份没有参与此项目，不能设置项目提示词。",
  robotAccessHint:
    "项目创建人或管理员可以创建机器人和编辑职责；成员配置自己的项目工作提示词。",
  robotKeep: "随机器人保留",
  personKeep: "随本人保留",
  roleOwnership: "职责、项目经验、业务记录",
  personalOwnership: "表达偏好、个人工作习惯",
  robotAppearance: "选择伙伴的颜色",
  toneLilac: "丁香紫",
  toneBlue: "雾蓝",
  tonePeach: "暖杏",
  toneMint: "鼠尾草绿",
  sampleTitle: "一个软件想法，四位伙伴一起推进。",
  sampleDescription:
    "轻笺 · Go + React 团队任务工具。体验需求确认、本地开发、测试回传和部署记录。",
  sampleOpen: "体验软件研发项目",
  sampleReopen: "继续软件研发示例",
  sampleAdded: "软件研发示例已准备好",
  sampleBadge: "可操作的虚构示例",
  sampleNotice:
    "所有代码、测试和部署记录均为示例数据。这里演示协作流程，没有实际仓库、运行中的软件或发布操作。",
  sampleGuide: "从这里开始",
  sampleGuideRobots: "1 · 看四位伙伴的职责",
  sampleGuidePersonal: "2 · 设置我的协作偏好",
  sampleGuideReview: "3 · 验收一次修复",
  projectPromptLinks: "各项目的工作要求",
  projectSetupHint:
    "项目创建后，可在「项目机器人」中新增伙伴，在「我的提示词」中设置个人偏好。",
};
export const agentsEn: Record<keyof typeof agentsZh, string> = {
  projectRobots: "Project robots",
  personalPrompts: "My prompts",
  robotsIntro:
    "Give each companion a clear responsibility. Robots hold work and records; the people bound to them make decisions.",
  createRobot: "Create robot",
  robotName: "Companion name",
  robotRole: "Role",
  robotBinding: "Bound decision maker",
  robotNamePlaceholder: "e.g. Vera",
  robotRolePlaceholder: "e.g. QA companion",
  robotPrompt: "Project responsibility prompt",
  robotPromptHint:
    "Describe responsibilities, required evidence, and when to involve a person. These instructions apply only to this project.",
  robotCreateHint:
    "Choose this robot as an owner or reviewer in new topics. Existing participants and approval slots stay unchanged.",
  robotRequired: "Enter a name, role, and responsibility prompt.",
  robotCreated: "Robot added to this project",
  editRobotPrompt: "Edit responsibilities",
  saveRobotPrompt: "Save responsibility prompt",
  robotPromptSaved: "Project responsibility prompt saved",
  promptPreview: "Preview combined prompt",
  promptPreviewTitle: "How this companion understands work",
  promptPreviewHint:
    "This previews the current configuration and feeds the local work brief. Demo replies use fixed examples; no live AI is connected.",
  promptBoundary:
    "Prompts guide methods and expression. They cannot change approval authority, data access, or confirmed work commitments.",
  promptPlatform: "Platform boundaries",
  promptWorkflow: "Project agreement",
  promptRole: "Robot responsibilities",
  promptPersonal: "My project preferences",
  promptTask: "Current topic and delivery criteria",
  promptDefault: "Default responsibilities · not customized",
  promptVersion: "Responsibilities v{version}",
  promptUpdated: "Updated by {name}",
  promptSaveConflict:
    "Not saved: permission or version changed. Close and reopen to review the latest configuration.",
  personalIntro:
    "Help companions understand how you like to work on this project. Preferences belong to you and do not transfer with a robot.",
  personalLabel: "How I want to collaborate on this project",
  personalPlaceholder:
    "e.g. Lead with the conclusion and decisions I need to make, then evidence. Include verification steps for technical advice and label uncertainty.",
  personalHint:
    "Applies only to your robots for the selected position in this project. Each position and project has separate settings; successors never see a predecessor's private configuration.",
  personalPrivate: "Visible to me",
  personalSave: "Save my prompt",
  personalSaved: "Your position prompt is saved",
  personalClearHint:
    "Save an empty prompt to clear preferences and follow project responsibilities.",
  personalAppliesTo: "Working with you here",
  personalNoRobot:
    "You have no positions in this project yet. Set a prompt after a position is assigned.",
  personalExample: "Use an example",
  personalExampleText:
    "Start with the conclusion and decisions I need to make, then evidence. Recommend one option and explain its tradeoffs. Include verification steps for technical claims. Label unverified information and never expand scope without approval.",
  personalNoAccess:
    "This identity is not a participant in the project and cannot set project preferences.",
  robotAccessHint:
    "Project creators and managers create robots and edit responsibilities. Members configure their own project work prompts.",
  robotKeep: "Stays with the robot",
  personKeep: "Stays with the person",
  roleOwnership: "Responsibilities, project knowledge, and business records",
  personalOwnership: "Expression preferences and personal work habits",
  robotAppearance: "Choose a companion color",
  toneLilac: "Lilac",
  toneBlue: "Mist blue",
  tonePeach: "Peach",
  toneMint: "Sage green",
  sampleTitle: "One software idea. Four companions to move it forward.",
  sampleDescription:
    "Leaf · a Go + React task app. Explore scope approval, local development, QA reports, and deployment records.",
  sampleOpen: "Explore the software project",
  sampleReopen: "Continue the software example",
  sampleAdded: "Software example is ready",
  sampleBadge: "Interactive fictional example",
  sampleNotice:
    "All code, test, and deployment records are sample data. This demonstrates collaboration; there is no real repository, running app, or release operation.",
  sampleGuide: "Start here",
  sampleGuideRobots: "1 · Meet the four companions",
  sampleGuidePersonal: "2 · Set my preferences",
  sampleGuideReview: "3 · Review a fix",
  projectPromptLinks: "Project-specific instructions",
  projectSetupHint:
    "After creation, add companions under Project robots and set your preferences under My prompts.",
};
