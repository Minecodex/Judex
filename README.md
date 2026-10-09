# Judex

Judex 是面向团队的项目协同平台。成员在本地完成实际工作，平台负责记录成果、共享资料、分析和提醒；正式安排、交接和验收由有权人员确认。

当前桌面 Web 入口按“项目列表 → 项目协同与计划 → 任务路线／计划或任务讨论”组织。讨论区保留会话、分叉和附件草稿，右栏通过标签查看计划、任务概览、记录、流程参考和共享资料；项目设置使用独立页面。正式模式与示例预览共用组件和主题，支持中英文、深浅配色。

仓库：[Minecodex/Judex](https://github.com/Minecodex/Judex)。主分支 `main` 使用保护规则，修改应提交到工作分支并通过 PR 合并。

## 当前能力

- 项目、计划和任务：依赖图、路线全屏与只读预览，多参与职责、交接、人工验收和重开。
- 工作安排：版本化草稿编辑和丢弃，正式变更提案及逐人确认；管理员可审阅影响后跳过／恢复任务，保留原阶段和依据。
- 讨论与记录：对象范围会话、共享历史分叉、原始报告与独立 AI 分析、来源定位及阅读位置恢复。
- 共享资料：上传、固定版本、内容预览、用途关联和历史；支持 PDF、图片、文本／源码、目录和压缩包等场景，权限由真实 API 校验。
- 配置与本地接入：职位与流程场景模板、人工发布、个人职位工作偏好、CLI／Skill 上报及需要人工决定的确认入口。

对应实现与验收见 [Demo4 逐项复核](docs/plans/demo4/REVIEW5-20261009.md)、[共享资料体验收尾](docs/共享资料体验收尾-20261009.md)及[文档索引](docs/README.md)。各记录说明自身范围与制品，整套 V1 的运行门槛另见实施计划。

## 正式工程入口

工程组成：`cmd/` 放 server 与 CLI 入口（Agent harness 位于 `internal/agent`，随 server 进程运行）；`internal/` 放 Go + Gin 模块化单体；`web/` 放 React + HeroUI + Tailwind + Zustand + TanStack Query；`tests/` 集中测试；`deploy/` 放 Docker / Helm；根目录提供 Makefile 和 Apache-2.0 LICENSE / NOTICE。

需要 Go **1.26+**、Node.js **22.12+**、npm；正式前端构建还需要 Python 3（可通过 `JUDEX_PYTHON` 指定），用于 CLI／Skill 下载包生成。

```sh
git clone https://github.com/Minecodex/Judex.git
cd Judex
go mod download
npm ci
```

启动前按 [.env.example](.env.example) 设置进程环境变量。程序不会自动读取 `.env`：业务服务需要 PostgreSQL，资料功能需要 S3 兼容对象存储，平台 Agent 需要模型目录或网关；沙箱能力使用 OpenSandbox。依赖部署与 Secret 配置见 [部署说明](deploy/README.md)。在两个终端分别运行：

```sh
# API http://127.0.0.1:8080
go run ./cmd/judex-server
```

```sh
# 真实 API 开发前端 http://127.0.0.1:5173
npm run dev
```

生产构建执行 `npm run build`，由 Go 服务托管，使用真实 API。开发模式也默认 API；演示需显式执行 `npm run dev:demo --workspace @judex/web`。启动业务服务前配置 PostgreSQL；材料功能需 S3，Agent 需模型目录或网关，沙箱需 OpenSandbox。当前实现和验证边界见 [修复记录](docs/plans/v1/REMEDIATION.md) 与 [验证报告](docs/plans/v1/FINAL-REPORT.md)。

```sh
go test ./...
go vet ./...
npm run test:web
npm run build
npm run test:e2e
go run ./cmd/judex --server http://127.0.0.1:8080 status
```

安装 GNU Make 后可使用 `make deps`、`make api`、`make web`、`make check`、`make image`。API 配置见 [.env.example](.env.example)，程序不自动加载 .env。

- [前端工程](web/README.md) · [测试说明](tests/README.md)
- [Helm 部署](deploy/README.md) · [工程实现状态](docs/详细设计/13-工程骨架与实现状态.md)
- [Apache-2.0](LICENSE) · [NOTICE](NOTICE) · [部署依赖](deploy/helm/UPSTREAM.md)

## 完整业务实施计划

后端、CLI／Skill、注册登录和正式前端联调的实施合同见 [V1 开发计划](docs/plans/v1/README.md)，包括 8 阶段、56 项任务及真实验收门槛。计划是实现与验收合同；当前证据和剩余项见上述修复记录，不能按历史任务完成数判断整套 V1 已验收。

## 设计与实现边界

正式前端工程统一在 `web/`。历史 `docs/demo` 已删除；交互决策原型保存在 [项目协同](docs/plans/cooperation-ui/demo.html)、[共享资料 Demo3](docs/plans/cooperation-ui/demo3.html)和[计划任务 Demo4](docs/plans/cooperation-ui/demo4.html)，其示例人物和模拟数据只用于对照。

原型源代码位于 `web/cooperation-preview`、`web/material-preview` 和 `web/task-route-preview`。例如执行 `node web/task-route-preview/build.mjs` 生成本地 `demo4.html` 后，运行 `node tests/task-route-preview/server.mjs`，可在 `http://127.0.0.1:5396/demo4.html` 查看交互原型。

- [文档索引](docs/README.md)
- [详细设计总览与技术基线](docs/详细设计/README.md)
- [工作室 2.0 卡片树与闭环](docs/工作室2.0卡片树与闭环.md)
- [工作室交互闭环](docs/工作室交互闭环.md)
- [前端生产就绪审计](docs/前端生产就绪审计.md)
- [产品主线](docs/产品主线.md)
- [设计讨论与决策记录](docs/重新设计讨论.md)
- [Agent 协同架构](docs/Agent协同架构讨论.md)
