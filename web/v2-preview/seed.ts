import { c, type DemoState, type Person, type Group, type Project } from './model';

export const people: Person[] = [
  { id: 'me', name: c('林舟', 'Lin Zhou'), subtitle: c('产品设计 · 你', 'Product design · You'), initials: 'LZ', tone: 'olive', online: true },
  { id: 'chen', name: c('陈墨', 'Mo Chen'), subtitle: c('工程开发', 'Engineering'), initials: 'MC', tone: 'sand', online: true },
  { id: 'xu', name: c('许知夏', 'Zia Xu'), subtitle: c('产品策略', 'Product strategy'), initials: 'ZX', tone: 'rose', online: true },
  { id: 'tang', name: c('唐可', 'Ke Tang'), subtitle: c('视觉设计', 'Visual design'), initials: 'KT', tone: 'blue', online: false },
  { id: 'lu', name: c('陆遥', 'Yao Lu'), subtitle: c('质量体验', 'Quality & experience'), initials: 'YL', tone: 'lilac', online: true },
  { id: 'he', name: c('何雨', 'Yu He'), subtitle: c('品牌与内容', 'Brand & content'), initials: 'YH', tone: 'peach', online: false },
];
const roles = [
  { id: 'design', name: c('产品设计师', 'Product designer'), prompt: c('从用户目标、信息层级与阅读节奏出发给出设计建议。需要工程判断时明确 @ 工程师 AI；需要人做取舍时 @ 关联人。', 'Design around user goals, information hierarchy and reading rhythm. Explicitly mention the engineer AI for technical input, or its human for a decision.') },
  { id: 'product', name: c('产品负责人', 'Product lead'), prompt: c('澄清目标、使用场景与取舍。工作流是参考建议，具体决策交给群内的人。', 'Clarify goals, scenarios and trade-offs. Treat workflows as guidance and leave final decisions to people.') },
  { id: 'engineer', name: c('工程师', 'Engineer'), prompt: c('关注结构、实现成本和边界。引用群内文件与工具结果说明依据；仅在需要协作时明确 @。', 'Consider architecture, implementation cost and boundaries. Cite group files and tool results, and mention peers only when collaboration is needed.') },
  { id: 'quality', name: c('体验研究员', 'Experience researcher'), prompt: c('通过具体用户路径检查可用性、可访问性与边界场景，提出可验证的问题。', 'Review usability, accessibility and edge cases through concrete user journeys.') },
];
const workflows = [
  { id: 'experience', name: c('体验共创', 'Experience co-design'), description: c('理解目标 → 讨论方案 → 补充证据 → 由人决定。可邀请设计、工程与体验研究参与。', 'Understand the goal → explore ideas → add evidence → let people decide. Invite design, engineering and research as useful.') },
  { id: 'review', name: c('方案评审', 'Design review'), description: c('检查可用性与实现边界，保留不同意见，向相关成员提出建议。', 'Review usability and implementation boundaries. Preserve differing views and offer suggestions to the relevant people.') },
  { id: 'research', name: c('开放探索', 'Open exploration'), description: c('从当前资料提出假设、验证方法与下一步讨论方向，不强制交付。', 'Use available material to propose hypotheses, ways to test them and next directions for discussion.') },
];
const projects: Project[] = [
  { id: 'judex', name: c('Judex 2.0', 'Judex 2.0'), description: c('让人与 AI 的协作，自然发生在对话里。', 'A more natural way for people and AI to work together.'), icon: 'flower', tone: 'olive', members: ['me', 'chen', 'xu', 'tang', 'lu', 'he'], roles, workflows, limit: 10 },
  { id: 'brand', name: c('品牌与内容', 'Brand & stories'), description: c('一起塑造产品的声音，讲有温度的故事。', 'Shape our voice. Tell stories that feel human.'), icon: 'sparkle', tone: 'peach', members: ['me', 'he', 'tang'], roles, workflows, limit: 10 },
  { id: 'lab', name: c('开放实验室', 'Open laboratory'), description: c('给新想法留点空间，从一个小问题开始。', 'Make room for new ideas. Start with a small question.'), icon: 'orbit', tone: 'lilac', members: ['me', 'chen', 'lu'], roles, workflows, limit: 10 },
];
const group = (g: Partial<Group> & Pick<Group, 'id' | 'name' | 'purpose'>): Group => ({ projectId: 'judex', icon: 'hash', tone: 'olive', members: { me: ['design', 'product'], chen: ['engineer'], xu: ['product'], tang: ['design'], lu: ['quality'] }, workflows: ['experience'], archived: false, unread: 0, preview: c('开始一段新的讨论', 'Start a new conversation'), time: '10:42', ...g });
const groups: Group[] = [
  group({ id: 'workspace', name: c('工作区体验', 'Workspace experience'), purpose: c('让讨论、文件与 AI 协作，在同一个空间自然发生。', 'Bring conversations, files and AI collaboration into one natural space.'), icon: 'layout', unread: 3, preview: c('陈墨 · 这个留白和阅读节奏很舒服', 'Mo · The spacing and reading rhythm feel right') }),
  group({ id: 'chat', parentId: 'workspace', name: c('聊天与文件', 'Chat & files'), purpose: c('讨论聊天布局、文件阅读和分享体验。', 'Explore chat layout, file reading and sharing.'), icon: 'message', preview: c('林舟 · 侧边阅读，不打断对话', 'Lin · Read alongside the conversation'), time: '10:28' }),
  group({ id: 'versions', parentId: 'chat', name: c('文件版本与分享', 'Versions & sharing'), purpose: c('梳理文件版本、历史引用和跨群分享。', 'Explore file versions, historical references and explicit sharing.'), icon: 'file', preview: c('许知夏 · 保留每一次引用的原貌', 'Zia · Preserve each referenced version'), time: '09:56' }),
  group({ id: 'agents', parentId: 'workspace', name: c('岗位 AI 协作', 'Role AI collaboration'), purpose: c('梳理明确 @、共享上下文和讨论循环保护。', 'Explore explicit mentions, shared context and loop protection.'), icon: 'sparkle', tone: 'lilac', unread: 1, preview: c('讨论已达 10 轮，等你来定方向', '10 loops reached. Your direction is needed.'), time: '10:16' }),
  group({ id: 'lounge', name: c('灵感茶水间', 'The idea room'), purpose: c('随手分享灵感，也聊聊工作之外的事。', 'A space for small ideas and things beyond work.'), icon: 'coffee', tone: 'peach', workflows: [], preview: c('唐可 · 分享一个很喜欢的配色 ☕', 'Ke · A color palette I love ☕'), time: '昨天' }),
  group({ id: 'archive', name: c('第一次体验探索', 'First explorations'), purpose: c('最早的想法与讨论，还可以随时回来继续。', 'Early ideas and conversations. Always open to a new thought.'), icon: 'leaf', archived: true, preview: c('最初的想法，都留在这里', 'Where the first ideas live'), time: '10/12' }),
  group({ id: 'brand-chat', projectId: 'brand', name: c('新的品牌语言', 'A new brand language'), purpose: c('让每一段文案都更接近真实的人。', 'Make every word sound a little more human.'), members: { me: ['design'], he: ['product'], tang: ['design'] }, icon: 'sparkle', tone: 'peach', preview: c('何雨 · 从一句欢迎语开始吧', 'Yu · Let’s start with a welcome'), time: '09:32' }),
  group({ id: 'lab-chat', projectId: 'lab', name: c('下一件小事', 'The next small thing'), purpose: c('先问一个好问题，再开始动手。', 'Ask a good question, then start making.'), members: { me: ['product'], chen: ['engineer'], lu: ['quality'] }, icon: 'orbit', tone: 'lilac', workflows: ['research'], preview: c('陈墨 · 我有一个小实验', 'Mo · I have a small experiment'), time: '周五' }),
];

export function seed(): DemoState {
  return structuredClone({ projects, groups,
    files: [
      { id: 'experience', roomId: 'workspace', name: c('工作区体验提案', 'Workspace experience proposal'), kind: 'brief', versions: [
        { number: 1, authorId: 'me', date: '10/18 · 16:20', note: c('第一版 · 信息架构与聊天布局', 'First draft · information architecture & chat layout') },
        { number: 2, authorId: 'me', date: '10/19 · 10:34', note: c('补充文件阅读、三级群聊与提及规则', 'Added file reading, nested groups & mention rules') },
      ] },
      { id: 'decisions', roomId: 'workspace', name: c('界面共创记录', 'Co-design notes'), kind: 'table', versions: [{ number: 1, authorId: 'xu', date: '10/19 · 10:12', note: c('把大家的想法放在一起', 'Bringing everyone’s ideas together') }] },
      { id: 'tokens', roomId: 'chat', name: c('design-tokens.css', 'design-tokens.css'), kind: 'code', versions: [{ number: 1, authorId: 'chen', date: '10/19 · 09:58', note: c('统一的色彩、间距与圆角', 'Shared color, spacing & radius tokens'), content: ':root {\n  /* A little warmth, a little room to breathe. */\n  --surface: #ffffff;\n  --canvas: #f5f4ef;\n  --ink: #292b25;\n  --olive: #737d50;\n  --lime-soft: #eff7e4;\n\n  --space-unit: 4px;\n  --radius-panel: 24px;\n  --radius-message: 18px;\n  --reading-width: 680px;\n}' }] },
      { id: 'dm-file', roomId: 'dm-chen', name: c('一点设计灵感.md', 'A little inspiration.md'), kind: 'text', versions: [{ number: 1, authorId: 'chen', date: '10/19 · 09:20', note: c('私聊文件 · 仅双方可见', 'Private file · only visible to both people'), content: 'Keep the interface quiet, so the conversation can be heard.\n\n留白让信息更清楚，好的交互让下一步更自然。' }] },
    ],
    messages: [
      { id: 'w1', roomId: 'workspace', authorId: 'xu', text: c('新的一周，从一个更舒服的工作空间开始。🌿\n我把这次体验重构的讨论都放在这里了，大家随时补充。', 'A fresh week, a better space to work. 🌿\nI’ve brought our workspace conversations here. Add your ideas anytime.'), time: '10:18' },
      { id: 'w2', roomId: 'workspace', authorId: 'me', kind: 'local', text: c('刚把本地整理的体验提案推上来了。想让聊天和成果放在一起，点开文件就能边看边聊。', 'Just pushed the experience proposal from my local workspace. Conversations and outcomes should live together, so we can read and discuss side by side.'), time: '10:34', files: [{ fileId: 'experience', version: 2 }] },
      { id: 'w3', roomId: 'workspace', authorId: 'me', roleId: 'design', text: c('我看了这版提案。保留三件事就很清楚：左边找到讨论，中间一起思考，右边阅读成果。\n@陈墨·工程师 AI 文件阅读区可以按需展开，让对话平时有更多空间吗？', 'I’ve reviewed the proposal. Three clear places: find conversations on the left, think together in the middle, read outcomes on the right.\n@Mo·Engineer AI Could the reader open on demand so conversations have more room?'), time: '10:35', tool: c('read · 工作区体验提案 v2，已加入群公共知识', 'read · Workspace proposal v2, added to shared group knowledge') },
      { id: 'w4', roomId: 'workspace', authorId: 'chen', roleId: 'engineer', text: c('可以。文件阅读区默认收起，点击附件后展开；消息始终保留引用的版本。已经把这个建议补进共创记录了。', 'Yes. The reader stays closed until a file is opened, and each message keeps its exact file version. I’ve added this suggestion to our co-design notes.'), time: '10:37', files: [{ fileId: 'decisions', version: 1 }], tool: c('write · 界面共创记录 v1，群内岗位共享此结果', 'write · Co-design notes v1, available to all roles in this group') },
      { id: 'w5', roomId: 'workspace', authorId: 'chen', text: c('这个留白和阅读节奏很舒服。文件不用来回跳转，讨论也不会被打断。', 'The spacing and reading rhythm feel right. No tab hopping to read a file, and the conversation stays in view.'), time: '10:42' },
      { id: 'c1', roomId: 'chat', authorId: 'me', text: c('文件应该是对话的一部分。右侧只在阅读的时候出现，关掉以后，聊天区自然变宽。', 'Files should feel like part of the conversation. The reader appears when needed, and closing it gives the chat its space back.'), time: '10:20' },
      { id: 'c2', roomId: 'chat', authorId: 'chen', text: c('把设计 token 整理好了。颜色、字号、间距和圆角走同一套规范。', 'I’ve put the design tokens together. Colors, type, spacing and radii follow one shared system.'), time: '10:28', files: [{ fileId: 'tokens', version: 1 }] },
      { id: 'v1', roomId: 'versions', authorId: 'xu', text: c('旧消息引用 v1，就应该一直打开 v1。文件库默认最新版，需要更新时明确新增版本。\n跨群分享也固定具体版本，让上下文始终准确。', 'A message referencing v1 should always open v1. The library defaults to the latest version, with explicit updates.\nSharing across groups should also pin a specific version.'), time: '09:56' },
      { id: 'a1', roomId: 'agents', authorId: 'me', text: c('@陈墨·工程师 AI 和设计一起讨论一下三级群聊的导航方式。', '@Mo·Engineer AI Please explore navigation for nested groups with the designer.'), time: '10:02' },
      { id: 'a2', roomId: 'agents', authorId: 'chen', roleId: 'engineer', text: c('@林舟·产品设计师 AI 我倾向于展开树，但只有加入的群才会显示。', '@Lin·Product designer AI I’d use an expanded tree, showing only joined groups.'), time: '10:04' },
      { id: 'a3', roomId: 'agents', authorId: 'me', roleId: 'design', text: c('我们已经围绕层级、可读性和导航讨论了 10 个闭环。仍有两个方向：始终展开，或默认只展开当前分支。需要大家给一点方向。', 'We’ve completed 10 loops discussing hierarchy, readability and navigation. Two options remain: expand everything, or only the current branch. We could use your direction.'), time: '10:16' },
      { id: 'l1', roomId: 'lounge', authorId: 'tang', text: c('分享一个很喜欢的配色 ☕\n暖白、鼠尾草绿，再加一点杏色。像午后在纸上写东西的感觉。', 'A color palette I love ☕\nWarm white, sage green and a touch of apricot. Like writing on paper in the afternoon.'), time: '昨天' },
      { id: 'ar1', roomId: 'archive', authorId: 'me', text: c('这段探索先放进归档，想法都留着。以后有新的发现，我们还可以接着聊。', 'Let’s keep this exploration in the archive. Our ideas stay here, and we can pick it up whenever inspiration returns.'), time: '10/12' },
      { id: 'b1', roomId: 'brand-chat', authorId: 'he', text: c('从一句欢迎语开始吧。希望产品有温度，也尊重每个人的工作节奏。', 'Let’s start with a welcome. A product should feel warm and respect everyone’s rhythm.'), time: '09:32' },
      { id: 'lab1', roomId: 'lab-chat', authorId: 'chen', text: c('我有一个小实验：能不能把每天的灵感更自然地留在群里？', 'A small experiment: could we keep our everyday ideas in the conversation more naturally?'), time: '周五' },
      { id: 'd1', roomId: 'dm-chen', authorId: 'chen', text: c('林舟，下午有空一起看下新的聊天布局吗？', 'Lin, shall we look at the new chat layout this afternoon?'), time: '09:18' },
      { id: 'd2', roomId: 'dm-chen', authorId: 'chen', text: c('先分享一点灵感。我很喜欢“界面安静一点，让对话被听见”这个方向。', 'A little inspiration first. I like the idea of a quiet interface that lets the conversation be heard.'), files: [{ fileId: 'dm-file', version: 1 }], time: '09:20' },
      { id: 'd3', roomId: 'dm-chen', authorId: 'me', text: c('好，下午两点见。我先把交互细节再梳理一下。', 'Sounds good. See you at two. I’ll refine the interactions first.'), time: '09:26' },
      { id: 'd4', roomId: 'dm-xu', authorId: 'xu', text: c('欢迎来到新的协作空间，有想法随时聊。', 'Welcome to our new space. Always happy to hear your ideas.'), time: '昨天' },
    ],
    dms: [ { id: 'dm-chen', personId: 'chen', unread: 1, preview: c('好，下午两点见。', 'See you at two.'), time: '09:26' }, { id: 'dm-xu', personId: 'xu', unread: 0, preview: c('有想法随时聊', 'Always happy to hear your ideas'), time: '昨天' } ],
    discussions: [{ id: 'loop-demo', roomId: 'agents', loops: 10, limit: 10, status: 'paused', feedback: '', people: ['me', 'chen'] }],
    notices: [{ id: 'n1', roomId: 'agents', text: c('岗位 AI 协作的讨论已达 10 轮。已通知林舟与陈墨，请给出方向或终止本次讨论。', 'Role AI collaboration reached 10 loops. Lin and Mo have been notified to continue or stop this discussion.'), read: false }],
    personalPrompts: { 'judex:me:design': '建议尽量具体，先考虑桌面端的阅读与输入体验。' }, guests: [], hiddenFiles: {},
  });
}
