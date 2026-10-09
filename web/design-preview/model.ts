export type Text = { zh: string; en: string };
export type Project = {
  id: string; title: Text; description: Text; role: 'owner' | 'admin' | 'member';
  color: 'green' | 'violet' | 'sand'; members: number; discussionCount: number; pending: number; fresh?: boolean;
};
export type Topic = { id: string; projectId: string; title: Text; subtitle: Text; seeded?: boolean };
export type Message = { id: string; topicId: string; text: string; attachments: string[]; time: string };
export const people = [
  { initials: 'LX', name: { zh: '林晓', en: 'Xiaolin' }, color: 'green' },
  { initials: 'CA', name: { zh: '陈安', en: 'Chen' }, color: 'violet' },
  { initials: 'ZN', name: { zh: '周宁', en: 'Ning' }, color: 'sand' },
] as const;
export const seedProjects: Project[] = [
  { id: 'judex', title: { zh: 'Judex 产品体验', en: 'Judex experience' },
    description: { zh: '把讨论、决定和本地工作连成一个清晰的协作体验。', en: 'Connect discussion, decisions and local work into a clear team experience.' },
    role: 'owner', color: 'green', members: 6, discussionCount: 8, pending: 2 },
  { id: 'brand', title: { zh: '品牌与设计系统', en: 'Brand & design system' },
    description: { zh: '让每个界面说同一种语言，建立团队共同的设计准则。', en: 'A shared visual language and design principles for every interface.' },
    role: 'member', color: 'violet', members: 4, discussionCount: 5, pending: 0 },
  { id: 'platform', title: { zh: '基础设施升级', en: 'Infrastructure renewal' },
    description: { zh: '围绕可靠性、部署体验与日常维护，一起改善基础设施。', en: 'Improve reliability, deployment and day-to-day infrastructure maintenance.' },
    role: 'admin', color: 'sand', members: 8, discussionCount: 6, pending: 1 },
];
export const seedTopics: Topic[] = [
  { id: 'experience', projectId: 'judex', title: { zh: 'V1 产品体验优化', en: 'V1 experience improvements' },
    subtitle: { zh: '先看方案，再确认实施方向', en: 'Review the proposal before implementation' }, seeded: true },
  { id: 'scope', projectId: 'judex', title: { zh: '项目目标与范围', en: 'Project goals & scope' },
    subtitle: { zh: '对齐目标、边界与验收要求', en: 'Align goals, boundaries and acceptance criteria' } },
  { id: 'delivery', projectId: 'judex', title: { zh: '周五交付与验收', en: 'Friday delivery & review' },
    subtitle: { zh: '整理交付资料与验收安排', en: 'Prepare materials and plan the review' } },
  { id: 'tokens', projectId: 'brand', title: { zh: '设计语言与组件规范', en: 'Design language & components' },
    subtitle: { zh: '形成统一的设计语言', en: 'Build one shared design language' } },
  { id: 'reliability', projectId: 'platform', title: { zh: '可靠性改进计划', en: 'Reliability improvements' },
    subtitle: { zh: '整理现状与下一步计划', en: 'Review the current state and next steps' } },
];
