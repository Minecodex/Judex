# Judex

Judex V2 是让人和岗位 AI 在群里一起工作的协作平台。登录后从项目卡片进入工作空间：左侧群聊／平台私聊，中间真实消息与文件，右侧并排阅读固定版本内容。成员在本地工作，通过 CLI 把进展送回群。

项目保留成员、职位、参考工作流与个人职位提示词；群成员独立、子群最多三层。普通消息不调用 AI，明确提及岗位 AI 才唤醒目标；本地推送触发本人全部本群岗位。各岗位共享公共知识，讨论到达闭环上限后由人继续或终止。工作是否完成由人决定，旧计划、任务、交接、审批和验收领域已移除。

正式工程：Go/Gin 模块化单体、PostgreSQL、S3、数据库持久队列和 OpenSandbox；桌面 Web 使用 React／HeroUI／Tailwind／TanStack Query／Zustand，支持中文、英文、深浅主题。API 前缀 `/api/v1`，协议版本 **2**。

## 开发与验证

需要 Go 1.26+、Node 22.12+、npm；发行下载包需要 Python 3，部署需要 Docker／Kubernetes／Helm。

```sh
npm ci
go mod download
# 程序不自动读取 .env，先导出 .env.example 中的进程配置
go run ./cmd/judex-server
# 另一个终端启动真实 API 前端
npm run dev
```

```sh
npm run gen:api
go test ./...
go vet ./...
npm run test:web
npm run build
# 独立 Docker 数据依赖、真实服务、原生 CLI 和 SSE，完成后回收
npm run test:e2e
```

设置 `JUDEX_V2_TEST_DSN` 后，Go 测试会建立独立数据库，验证账号一致导入、岗位闭环、预算、公共压缩与未知副作用恢复。完整桌面浏览器、七工具、格式、K8s 重启与真实模型检查见 [测试入口](tests/README.md)。

## 文档与交付

- [产品](docs/产品/README.md) · [界面](docs/界面/README.md) · [架构](docs/详细设计/README.md)
- [V2 实施与验收状态](docs/plans/v2/IMPLEMENTATION.md) · [领域／权限／API／执行合同](docs/plans/v2/CONTRACTS.md)
- [部署与切换](deploy/README.md) · [运维](docs/运维/README.md) · [文档索引](docs/README.md)
- [前端](web/README.md) · [CLI／Skill](skills/judex/SKILL.md)

发行：`node scripts/release/build.mjs VERSION` 生成同版 Server／Web／六平台 CLI／Skill／Chart／OpenAPI 与校验和。`scripts/deploy/load-local-image.mjs` 按实际 Kubernetes 节点架构导入 Docker Desktop 镜像。

V2 计划及 R1～R14 已完成，本机正式版本 `0.2.0-local.20261020.v2.15` 运行于 [Judex](http://127.0.0.1:18097/)。正式界面已按 Kylon 的聊天、菜单和文件侧栏调整，支持双语深浅主题。本轮 95 项同版服务／浏览器检查与 24 项前端单元测试通过，升级保持认证数据和基础设施 Secret；见[界面改造验收](docs/plans/v2/records/UI-REFRESH-20261020.md)及[实施状态](docs/plans/v2/IMPLEMENTATION.md)。v2.14 的 170 项完整候选／13 项升级证据保留于[第四次收尾记录](docs/plans/v2/records/FOURTH-FOLLOWUP-20261020.md)，不计为本轮重新执行。

历史原型 HTML 和原型 13 项证据留在各计划目录；正式页面不使用示例内容。仓库 [Minecodex/Judex](https://github.com/Minecodex/Judex)，许可证 [Apache-2.0](LICENSE)。
