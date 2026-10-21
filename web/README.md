# Judex V2 Web

React 19、HeroUI v3、Tailwind、TanStack Query、Zustand。正式工程在 `src/`，真实 API 默认同源 `/api/v1`，开发 Vite 代理到 Go；不存在生产模拟数据模式。`v2-preview/` 仅保留早期交互原型。

```sh
npm ci
npm run dev
npm run build
npm run test:web
```

默认中文，模块 i18n 在 `src/i18n/index.ts` 注册，偏好 `argus.locale`。桌面三栏工作区、项目卡片和设置共用统一 token 与 HeroUI 组件；深浅主题持久化。设计见 [界面规范](../docs/界面/README.md)。

服务端状态由 TanStack Query 与 SSE 更新；草稿按人和会话隔离，失败保留。消息结构化提及、固定文件版本、分享、只读预览、岗位讨论控制全部使用真实 API。自动化入口 `tests/v2/browser.spec.mjs`，隔离依赖与浏览器入口见 [测试文档](../tests/README.md)。当前正式范围仅桌面 Web。
