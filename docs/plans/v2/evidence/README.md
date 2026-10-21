# V2 交互 Demo 验收记录

日期：2026-10-19。范围仅为 [V2 桌面 Demo](../PREVIEW.md)，不代表 V2 后端或真实 Agent 已实现。

## 实际执行

- TypeScript：`node node_modules/typescript/bin/tsc -p web/v2-preview/tsconfig.json --noEmit --noUnusedLocals --noUnusedParameters`，通过。
- 单文件构建：`npm run build:v2` 对应的 Vite 构建入口，通过，输出约 1076 KiB 的 [demo.html](../demo.html)。
- 浏览器：macOS 上的 Codex in-app browser，默认桌面视口 1280 × 720；无横向页面溢出。
- 自动化：通过 CUA Playwright 接口执行仓库 [scenarios.mjs](../../../../tests/v2-preview/scenarios.mjs) 的同一套可复用场景，共 13 项通过，检查结果见 [browser-checks.json](browser-checks.json)。操作使用实际按钮、输入框、文件选择器与阅读区，不注入应用状态。
- 浏览器 error 级日志：验收时为空。

## 场景覆盖

1. 项目卡片进入工作区，文件区默认关闭，点击后展开。
2. 指定版本分享；目标群不能切到未分享版本；从列表移除后历史引用仍能打开。
3. 上传显式更新到 v3，历史消息仍打开 v2；同名正常上传产生独立文件。
4. 自由建群、可选目的与零工作流；普通网页消息和 @ 人不唤醒 AI。
5. 明确 @ 指定岗位 AI，仅该身份产生模拟回复。
6. 本地只记录不唤醒；普通推送唤醒本人全部本群岗位。
7. 同群公共背景一致，工具结果可追溯，切换岗位不展示他人的私有提示词。
8. 多同级子群、三层上限、独立成员选择。
9. 父群及全部子孙群级联归档，归档后仍能发送消息。
10. 讨论暂停后带意见继续、空意见继续、终止；新 @ 仍可开启讨论。
11. 纯人私聊、附件阅读、跨项目复用同一对话。
12. 中英文、浅色 / 深色主题切换与桌面布局。
13. 项目循环上限、职位与个人提示词保存，示例邀请成员可用于建群。

标准 Playwright 入口也已提供，可在安装了 Chromium 的开发机执行：

```sh
npm run test:v2
```

该命令会重新构建 Demo，使用本地 5199 预览服务执行同一场景集。本轮实际通过 CUA 执行；未额外启动 Playwright 的独立 Chromium 进程。测试样例只上传仓库自带的公开示例 [review-v3.md](../../../../tests/v2-preview/fixtures/review-v3.md)。

## 视觉记录与未测边界

截图输出到忽略目录 `.cache/v2-preview/evidence/`，避免把大型图像纳入计划源文档：项目卡片、浅色工作区与文件、深色英文工作区、讨论暂停卡片。预览服务、浏览器状态和截图均为本机演示制品。

未执行真实登录、后端鉴权、网络故障、多人并发、真实模型 / 四工具 / 压缩、实际 CLI、运行取消、消息事件投递与 K8s E2E。本轮没有清理数据或替换既有部署。
