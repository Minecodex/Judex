# Judex

当前前端是以项目与聊天为中心的协作工作区：中央讨论与人工决定、右侧任务执行图和设置。运行与边界见 [聊天协作前端重构](docs/聊天协作前端重构.md)。旧独立原型 `docs/demo` 已删除，前端统一在 `web/` 维护。

## 正式工程入口（2026-09-25）

已生成工程骨架：`cmd/` 放 server 与 CLI 入口（Agent harness 位于 `internal/agent`，随 server 进程运行）；`internal/` 放 Go + Gin 模块化单体；`web/` 放 React + HeroUI + Tailwind + Zustand + TanStack Query；`tests/` 集中测试；`deploy/` 放 Docker / Helm；根目录提供 Makefile 和 Apache-2.0 LICENSE / NOTICE。

需要 Go 1.25+、Node.js 22.12+、npm。在根目录执行 `go mod download`、`npm ci`，然后分别在两个终端运行：

```sh
# API http://127.0.0.1:8080
go run ./cmd/judex-server
```

```sh
# 完整交互预览 http://127.0.0.1:5173
npm run dev
```

生产构建执行 `npm run build`，由 Go 服务在 8080 托管，默认使用真实 API 模式。开发模式显式使用浏览器预览数据，保留完整工作室交互；生产模式不会自动回退到假数据。当前前端尚未完成真实登录后的工作区入口及业务命令接入；演示通过不代表生产联调完成。前端证据与缺口见 [前端生产就绪审计](docs/前端生产就绪审计.md)，本轮未审查后端实现。

```sh
go test ./...
go vet ./...
npm run test:web
npm run build
npm run test:e2e
go run ./cmd/judex -server http://127.0.0.1:8080 status
```

安装 GNU Make 后可使用 `make deps`、`make api`、`make web`、`make check`、`make image`。API 配置见 [.env.example](.env.example)，程序不自动加载 .env。

- [前端工程](web/README.md) · [测试说明](tests/README.md)
- [Helm 部署](deploy/README.md) · [工程实现状态](docs/详细设计/13-工程骨架与实现状态.md)
- [Apache-2.0](LICENSE) · [NOTICE](NOTICE) · [部署依赖](deploy/helm/UPSTREAM.md)

## 设计与实现边界

独立原型及其 4173 预览入口已移除。当前唯一前端工程为 `web/`，历史设计文档仅供理解决策背景。

- [文档索引](docs/README.md)
- [详细设计总览与技术基线](docs/详细设计/README.md)
- [工作室 2.0 卡片树与闭环](docs/工作室2.0卡片树与闭环.md)
- [工作室交互闭环](docs/工作室交互闭环.md)
- [前端生产就绪审计](docs/前端生产就绪审计.md)
- [产品主线](docs/产品主线.md)
- [设计讨论与决策记录](docs/重新设计讨论.md)
- [Agent 协同架构](docs/Agent协同架构讨论.md)
