import {
  words as W,
  type WorkState,
  type Task,
  type Position,
  type Evidence,
} from "./types.ts";
export const WORK_KEY = "judex.web.preview.v1";
export const uid = () => crypto.randomUUID();
export function seedWork(): WorkState {
  const now = Date.now();
  const file = (
    id: string,
    name: string,
    text: string,
    author: string,
  ): Evidence => ({ id, name, text, author, at: now - 3600000 });
  const api = file(
    "f-api",
    "创建与列表 · 本地验证.md",
    "# 虚构软件示例\n来源：顾言的本地 AI 主动上报\n仓库/分支：leaf-api / guyan/task-list\n完成：任务创建、列表分页和空状态。\n本地验证：12 个示例用例通过。\n局限：尚未验证真实生产数据与高并发。",
    "顾言 · 本地 AI",
  );
  const guide = file(
    "f-guide",
    "第一次使用轻笺.md",
    "# 虚构说明文档\n来源：夏禾主动上传。\n创建清单 → 输入标题 → 添加任务 → 标记进度。\n已提供操作文字，当前尚缺空状态截图，供体验核对。",
    "夏禾",
  );
  const base = (
    id: string,
    title: ReturnType<typeof W>,
    owner: string,
    status: Task["status"],
    planId: string | null = "leaf-first",
  ): Task => ({
    revision: 1,
    id,
    projectId: "leaf",
    planId,
    title,
    expected: title,
    criteria: [
      W("交付与确认的范围一致", "Delivery matches the agreed scope"),
      W(
        "说明验证结果与未解决的问题",
        "Include verification and remaining gaps",
      ),
    ],
    seatIds: [owner],
    reviewerSeatId: "lead",
    flowId: "delivery",
    nodeId: "make",
    status,
    requirements: [],
    files: [],
  });
  const positions: Position[] = [
    {
      id: "lead-role",
      projectId: "leaf",
      name: W("产品统筹", "Product steward"),
      prompt: W(
        "对照阶段目标整理取舍、影响和验收依据。正式分工与结论交给有权的人确认。",
        "Assess scope, tradeoffs, impact and acceptance evidence. Authorized people confirm formal commitments.",
      ),
      tone: "mint",
      bindings: [{ flowId: "delivery", nodeId: "accept" }],
    },
    {
      id: "build-role",
      projectId: "leaf",
      name: W("实现伙伴", "Maker"),
      prompt: W(
        "整理本地实现与验证回报。明确代码引用、已完成项和缺口，不代替成员在本地改代码。",
        "Organize local implementation and test reports. Identify code references and gaps; do not operate local tools.",
      ),
      tone: "blue",
      bindings: [{ flowId: "delivery", nodeId: "make" }],
    },
    {
      id: "experience-role",
      projectId: "leaf",
      name: W("体验伙伴", "Experience partner"),
      prompt: W(
        "核对交付是否足以继续工作。说明拒收原因、补充要求和证据，不把资料收到当作验收完成。",
        "Check whether received work is usable. Explain rejection and missing evidence. Receipt is not final acceptance.",
      ),
      tone: "lilac",
      bindings: [{ flowId: "delivery", nodeId: "receive" }],
    },
    {
      id: "content-role",
      projectId: "leaf",
      name: W("内容伙伴", "Content partner"),
      prompt: W(
        "整理使用说明与信息结构，保留资料来源，并明确尚未验证的描述。",
        "Organize guides and information structure. Preserve sources and label unverified claims.",
      ),
      tone: "peach",
      bindings: [{ flowId: "delivery", nodeId: "make" }],
    },
    {
      id: "brand-role",
      projectId: "wild",
      name: W("品牌策划", "Brand partner"),
      prompt: W(
        "围绕品牌目标整理视觉和文案建议。",
        "Prepare visual and copy suggestions around the brand goal.",
      ),
      tone: "peach",
      bindings: [{ flowId: "brand-flow", nodeId: "make" }],
    },
  ];
  const tasks = [
    {
      ...base(
        "scope",
        W("明确首版的交付边界", "Agree on the first release scope"),
        "lead",
        "accepted",
      ),
      acceptedAt: now - 86400000,
      files: [
        file(
          "f-scope",
          "首版范围.md",
          "虚构示例：创建、列表、基础说明；暂不包含自动分类。",
          "林然",
        ),
      ],
    },
    {
      ...base(
        "build",
        W("让任务创建和列表顺畅可用", "Make task creation and lists work well"),
        "maker",
        "delivered",
      ),
      files: [api],
      branch: "leaf-api / guyan/task-list",
    },
    {
      ...base(
        "guide",
        W("写好第一次使用的说明", "Write the getting-started guide"),
        "writer",
        "delivered",
      ),
      files: [guide],
    },
    {
      ...base(
        "package",
        W("整理首版体验与交付包", "Prepare the first delivery package"),
        "receiver",
        "ready",
      ),
      requirements: [
        {
          id: "r-receipt",
          kind: "receipt" as const,
          ref: "first-review",
          hard: true,
          label: W(
            "明确接收功能与说明两份交付",
            "Explicitly receive both the feature and guide",
          ),
        },
        {
          id: "r-evidence",
          at: "accept" as const,
          kind: "evidence" as const,
          ref: "",
          hard: true,
          label: W(
            "验收前提交交付说明或验证依据",
            "Provide delivery notes or evidence before final acceptance",
          ),
        },
      ],
    },
    {
      ...base(
        "empty-state",
        W(
          "补充空状态的可访问性说明",
          "Add accessibility notes for empty states",
        ),
        "maker2",
        "working",
      ),
      parentId: "build",
    },
    base(
      "research",
      W("收集三位朋友的使用反馈", "Gather feedback from three friends"),
      "receiver",
      "working",
      null,
    ),
    {
      ...base(
        "brand-copy",
        W("确定咖啡包装上的一句话", "Choose the line for the coffee packaging"),
        "brand-seat",
        "working",
        "wild-launch",
      ),
      projectId: "wild",
      reviewerSeatId: "brand-seat",
      flowId: "brand-flow",
    },
  ];
  return {
    schema: 4,
    currentUser: "周宁",
    projects: [
      {
        id: "leaf",
        title: W("轻笺", "Leaf"),
        description: W(
          "给小团队，一个轻一点的任务工具。",
          "A lighter place for a small team to get things done.",
        ),
        kind: "software",
        members: [
          { name: "林然", role: "owner" },
          { name: "沈言", role: "manager" },
          { name: "顾言", role: "member" },
          { name: "周宁", role: "member" },
          { name: "夏禾", role: "member" },
          { name: "江澄", role: "member" },
        ],
      },
      {
        id: "wild",
        title: W("野间咖啡", "Wild Coffee"),
        description: W(
          "让每天的第一杯，有自己的样子。",
          "Give the first cup of the day its own character.",
        ),
        kind: "design",
        members: [
          { name: "周宁", role: "owner" },
          { name: "林然", role: "manager" },
        ],
      },
    ],
    positions,
    seats: [
      {
        id: "lead",
        positionId: "lead-role",
        person: "林然",
        notes: W(
          "首版保持轻量，优先清晰与可靠。",
          "Keep the first release simple, clear and reliable.",
        ),
      },
      {
        id: "maker",
        positionId: "build-role",
        person: "顾言",
        notes: W("关注创建与列表。", "Owns creation and lists."),
      },
      {
        id: "maker2",
        positionId: "build-role",
        person: "江澄",
        notes: W(
          "与顾言同岗，独立负责可访问性资料。",
          "Shares a position with Gu Yan, with a separate work identity.",
        ),
      },
      {
        id: "receiver",
        positionId: "experience-role",
        person: "周宁",
        notes: W(
          "先核对真实使用路径，再整理交付包。",
          "Check real user journeys before preparing the package.",
        ),
      },
      {
        id: "writer",
        positionId: "content-role",
        person: "夏禾",
        notes: W(
          "以第一次使用者的视角组织说明。",
          "Write from a first-time user's perspective.",
        ),
      },
      {
        id: "brand-seat",
        positionId: "brand-role",
        person: "周宁",
        notes: W(
          "自然、留白、小批量印刷。",
          "Natural tones, space and small print runs.",
        ),
      },
    ],
    plans: [
      {
        id: "leaf-first",
        projectId: "leaf",
        title: W(
          "把轻笺的第一版交到大家手里",
          "Bring the first Leaf release to the team",
        ),
        goal: W(
          "完成一条从创建到查看任务的完整体验，附清楚的使用说明与验证依据。",
          "Deliver a complete create-to-list experience, with a clear guide and verification.",
        ),
        criteria: [
          W(
            "创建、列表和空状态形成可用体验",
            "Creation, lists and empty states form a usable experience",
          ),
          W(
            "所有约定成果均有可追溯依据",
            "Every agreed outcome has traceable evidence",
          ),
        ],
        ownerSeatId: "lead",
        flowId: "delivery",
        status: "active",
        referenceTaskIds: [],
      },
      {
        id: "leaf-next",
        projectId: "leaf",
        title: W("下一步，让清单更懂你", "Next, make lists feel more personal"),
        goal: W(
          "探索标签与筛选，先确认价值再安排投入。",
          "Explore labels and filtering before committing resources.",
        ),
        criteria: [W("范围与投入得到明确确认", "Agree on scope and effort")],
        ownerSeatId: "lead",
        flowId: "delivery",
        status: "draft",
        referenceTaskIds: ["build"],
      },
      {
        id: "wild-launch",
        projectId: "wild",
        title: W("一杯咖啡的新开始", "A new beginning, one cup at a time"),
        goal: W(
          "准备包装文案与可小批量印刷的品牌资料。",
          "Prepare packaging copy and brand materials for a small print run.",
        ),
        criteria: [
          W(
            "风格一致并说明印刷约束",
            "Consistent style with print constraints documented",
          ),
        ],
        ownerSeatId: "brand-seat",
        flowId: "brand-flow",
        status: "active",
        referenceTaskIds: [],
      },
    ],
    tasks,
    handoffs: [
      {
        id: "first-review",
        projectId: "leaf",
        title: W(
          "首版的两份交付，等你看一眼",
          "Two first-release contributions, ready for your review",
        ),
        taskId: "package",
        receiverSeatId: "receiver",
        flowId: "delivery",
        flowVersion: 1,
        stale: false,
        kind: "dependency",
        history: [],
        sources: [
          {
            id: "source-api",
            taskId: "build",
            senderSeatId: "maker",
            revision: 1,
            summary: W(
              "创建与列表已完成本地验证，附代码引用和 12 项检查记录。",
              "Creation and lists verified locally, with code references and 12 checks.",
            ),
            files: [api],
            status: "pending",
            sentBy: "顾言",
            sentAt: now - 7200000,
          },
          {
            id: "source-guide",
            taskId: "guide",
            senderSeatId: "writer",
            revision: 1,
            summary: W(
              "第一次使用说明已整理，想请你核对步骤与缺少的截图。",
              "The first-time guide is ready. Please check the steps and missing screenshots.",
            ),
            files: [guide],
            status: "pending",
            sentBy: "夏禾",
            sentAt: now - 3600000,
          },
        ],
      },
    ],
    flows: [
      {
        id: "delivery",
        projectId: "leaf",
        name: W("从想法到交付", "From idea to delivery"),
        version: 1,
        instructions: W(
          "成员本地完成工作并回报。AI 整理下一步建议，各来源本人确认发送，接收人明确接受或说明拒收原因。符合约定后由有权人验收；计划负责人整体验收。普通箭头是协作参考，显式硬条件单独检查。",
          "Members work locally and report back. AI suggests next steps; each contributor confirms sending. Recipients explicitly accept or reject with a reason. Authorized people accept outcomes; the plan owner accepts the whole plan. Arrows guide collaboration; explicit hard conditions are checked separately.",
        ),
        nodes: [
          { id: "make", label: W("准备与上报", "Prepare & report") },
          { id: "receive", label: W("交接与完善", "Receive & refine") },
          { id: "accept", label: W("成果验收", "Accept outcomes") },
        ],
        edges: [
          ["make", "receive"],
          ["receive", "accept"],
        ],
        history: [],
      },
      {
        id: "brand-flow",
        projectId: "wild",
        name: W("品牌共创", "Brand collaboration"),
        version: 1,
        instructions: W(
          "围绕目标提出方案，确认后本地制作，再回传成果和依据。",
          "Propose a direction, confirm it, work locally, and return results with evidence.",
        ),
        nodes: [
          { id: "make", label: W("形成方案", "Prepare direction") },
          { id: "receive", label: W("协同复核", "Review together") },
          { id: "accept", label: W("接受成果", "Accept outcome") },
        ],
        edges: [
          ["make", "receive"],
          ["receive", "accept"],
        ],
        history: [],
      },
    ],
    topics: [
      {
        id: "labels",
        projectId: "leaf",
        title: W("首版需要标签吗？", "Do we need labels in the first release?"),
        planIds: ["leaf-first", "leaf-next"],
        taskIds: ["build"],
        messages: [
          {
            id: "message-1",
            actor: "林然",
            kind: "person",
            text: W(
              "先把最基本的使用体验做好，标签能否留到下一步？",
              "Could labels wait until the basic experience is ready?",
            ),
            at: now - 10800000,
          },
          {
            id: "message-2",
            actor: "Judex",
            kind: "ai",
            text: W(
              "我整理了相关范围。这次可以只保留讨论结论；若要调整正式任务，需要另行确认。",
              "This can remain a discussion conclusion. Changes to formal work require a separate decision.",
            ),
            at: now - 10000000,
          },
        ],
      },
      {
        id: "brand-discussion",
        projectId: "wild",
        title: W(
          "怎样表达自然的松弛感？",
          "How should the brand express a slower pace?",
        ),
        planIds: ["wild-launch"],
        taskIds: ["brand-copy"],
        messages: [],
      },
    ],
    invites: [
      {
        id: "invite-zhao",
        projectId: "leaf",
        person: "赵可",
        positionIds: ["build-role", "content-role"],
        status: "pending",
        sender: "林然",
      },
    ],
    preferences: [],
    events: [],
  };
}
