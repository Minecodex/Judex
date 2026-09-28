# Judex 协作技能

## 何时使用

用户要求你与 Judex 平台交互时：查看任务/待办、上传工作成果、提交报告、处理提案或交接。本技能不适用于 Judex 平台本身的开发。

## 前置条件

1. 先运行 `judex auth whoami` 确认已登录且未过期；未登录则提示用户执行 `judex auth login`（浏览器确认一次，不要代替用户点击）。
2. `judex project list` → `judex project use ID` 选择项目。项目上下文存于 `.judex/project.json`（非机密，可入库）。
3. 一切正式决定（审批/验收/交接/重开）都是**人工动作**：CLI 只能创建确认意图，由用户本人在浏览器确认一次；你读取同一结果，绝不重复提交一票。

## 常用流程

- 查看我的任务：`judex inbox list` → `judex decision review <ID>`（提案详情）
- 提交本地成果：核对任务 → 明确列出要上传的文件清单并让用户确认 → `judex material upload <每个文件>` → `judex report --task <ID> --kind delivery --file report.json`
- 同意提案：`judex decision review <ID>` → 向用户复述变化清单与审批人 → `judex proposal submit`/确认意图 → 用户浏览器确认 → 你查询结果并汇报
- 交接：`judex handoff send <SOURCE> --version N`（经确认意图）

详细命令表、报告格式与错误恢复见 references/。

## 保密边界

只上传用户明确选择的文件；禁止上传整个工作目录、.env、SSH key、Git credential、浏览器配置。意外遇到敏感文件时停下来让用户选择，不要自行判断"应该没关系"。

## 冲突处理

- 409/版本冲突：重新读取对象，向用户展示差异；不自动改用新版本重试审批。
- 网络失败：读操作退避重试；写操作仅在用户指示下用原幂等键重试。
- `HUMAN_CONFIRMATION_REQUIRED`：提示用户打开确认链接，不要尝试绕过。
