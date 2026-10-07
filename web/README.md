# Web 工程

React / TypeScript / Vite，HeroUI 是唯一基础 UI 库，Tailwind 负责工具样式。业务样式位于 `src/styles`，自定义类名使用 `.judex-*`，样式值来自 token。

- `src/app`：应用入口、真实 API 壳层、登录 / 注册表单。
- `src/components/ui`：HeroUI 适配组件；Button、Modal、Input、TextArea、Select、Checkbox、Card、Chip、Alert、Disclosure 统一来自 HeroUI，Tab / Popover 等直接组合 HeroUI。
- `src/features/chat`：当前聊天主界面、工作提案与逐人确认、执行顺序图、按需工作区。
- `src/features/work`：共用的项目/职位/流程配置、交接与验收组件；demo 状态规则（seed/actions）与 api 数据源（`apiModel.ts` 映射、`apiStore.ts`、`apiActions.ts` 真实 REST 动作）在此装配。旧工作室导航、层级图及 WorkPanel 等旧真实 API 面板已移除。
- `src/lib/api`：统一请求、错误模型、数据源边界。
- `src/stores`：Zustand 保存语言 / 主题等客户端偏好。
- `src/i18n`：中文 / 英文模块清单。延续 `argus.locale` 偏好键。
- `src/styles`：HeroUI / Tailwind 入口、design tokens、业务样式。

目标架构由 TanStack Query 管理服务器状态，Zustand 保存客户端偏好；demo（`VITE_DATA_MODE=demo`）继续运行在演示状态模型，api 模式已由统一 ChatWorkspace 树经 `WorkProvider` 分发到真实数据源（`useApiWorkbench`）。预览数据通过独立 Query key 读取，以 `judex.web.preview.v1` 保存在浏览器；这不是正式后端数据协议，生产数据源不使用预览 mutation。

从根目录安装依赖并运行 `npm run dev`。默认 5173，代理 API 到 8080，可用 `JUDEX_API_PROXY` 调整。只有显式 `VITE_DATA_MODE=demo` 才允许进入预览；`.env.development`、`.env.e2e`、`.env.preview` 已明确声明，普通生产构建默认为 API。

任务图按执行先后排列，父子拆分不表示依赖。聊天可以整理提案、固定人员与版本、逐人决定并生成正式工作；交接接收不替代任务和计划最终验收。api 模式的正式决定以服务端 review 与冻结引用为准，业务动作全部经 `apiActions.ts` 走真实 REST（幂等键 + 分片上传）；预览与真实接口的分界由 `WorkProvider` 按模式分发。

纯前端浏览器回归：在根目录执行 `npm exec -- playwright test --config tests/e2e/chat.config.ts`，仅启动隔离的前端服务，不启动后端。覆盖聊天、提案、依赖图、项目配置及原有交接/验收路径。最新产品边界见 [聊天协作前端重构](../docs/聊天协作前端重构.md)。

独立 `docs/demo` 已删除，不再维护第二套原型。原生 DOM 仅用于语义结构、图形和浏览器能力；`FilePicker.tsx` 保留隐藏的 `input[type=file]` 获取 FileList，可见入口使用 HeroUI Button。表单共享适配见 `FormControls.tsx`，统一样式见 `styles/controls.css`。

生产模式的认证会话、工作区入口、服务器读写、实时事件与真实材料上传均已接入（统一前端重构，2026-10）；历史缺口与逐条关闭状态见 [前端生产就绪审计](../docs/前端生产就绪审计.md) 的缺口复核。
