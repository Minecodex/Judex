# Web 工程

React / TypeScript / Vite，HeroUI 是唯一基础 UI 库，Tailwind 负责工具样式。业务样式位于 `src/styles`，自定义类名使用 `.judex-*`，样式值来自 token。

- `src/app`：应用入口、真实 API 壳层、登录 / 注册表单。
- `src/components/ui`：HeroUI 适配组件；Button、Modal、Input、TextArea、Select、Checkbox、Card、Chip、Alert、Disclosure 统一来自 HeroUI，Tab / Popover 等直接组合 HeroUI。
- `src/features/chat`：当前聊天主界面、工作提案与逐人确认、执行顺序图、按需工作区。
- `src/features/work`：共用的项目/职位/流程配置、交接与验收组件及演示状态规则。旧工作室导航和层级图已移除，尚未逐页改为服务端读写。
- `src/lib/api`：统一请求、错误模型、数据源边界。
- `src/stores`：Zustand 保存语言 / 主题等客户端偏好。
- `src/i18n`：中文 / 英文模块清单。延续 `argus.locale` 偏好键。
- `src/styles`：HeroUI / Tailwind 入口、design tokens、业务样式。

目标架构由 TanStack Query 管理服务器状态，Zustand 保存客户端偏好；当前完整业务仍运行在演示状态模型，API 接入尚未完成。预览数据通过独立 Query key 读取，以 `judex.web.preview.v1` 保存在浏览器；这不是正式后端数据协议。生产数据源不能使用预览 mutation。

从根目录安装依赖并运行 `npm run dev`。默认 5173，代理 API 到 8080，可用 `JUDEX_API_PROXY` 调整。只有显式 `VITE_DATA_MODE=demo` 才允许进入预览；`.env.development`、`.env.e2e`、`.env.preview` 已明确声明，普通生产构建默认为 API。

任务图按执行先后排列，父子拆分不表示依赖。聊天可以整理提案、固定人员与版本、逐人决定并生成正式演示工作；交接接收不替代任务和计划最终验收。不代表生产权限、并发或数据库事务已经实现，后续按业务模块迁移 API，保留预览与真实接口的分界。

纯前端浏览器回归：在根目录执行 `npm exec -- playwright test --config tests/e2e/chat.config.ts`，仅启动隔离的前端服务，不启动后端。覆盖聊天、提案、依赖图、项目配置及原有交接/验收路径。最新产品边界见 [聊天协作前端重构](../docs/聊天协作前端重构.md)。

独立 `docs/demo` 已删除，不再维护第二套原型。原生 DOM 仅用于语义结构、图形和浏览器能力；`FilePicker.tsx` 保留隐藏的 `input[type=file]` 获取 FileList，可见入口使用 HeroUI Button。表单共享适配见 `FormControls.tsx`，统一样式见 `styles/controls.css`。

生产前端仍需补齐认证会话、工作区入口、服务器读写、实时事件及真实材料上传。具体证据见 [前端生产就绪审计](../docs/前端生产就绪审计.md)。
