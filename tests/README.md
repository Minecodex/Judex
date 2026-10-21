# V2 测试入口

测试必须使用独立数据库、带归属标签的 Docker 依赖或 namespace。真实模型和受控网关证据分别记录，不把缺少依赖导致的 skip 视为通过。

| 范围 | 入口 | 证据 |
| --- | --- | --- |
| Go 与基础设施 | `go test ./...`、`go vet ./...` | 认证、模型流、存储、解析、沙箱、部署与合同 |
| V2 独立 PostgreSQL | `JUDEX_V2_TEST_DSN=postgres://… go test -race ./...` | 认证一致导入、同时继续、并行最后额度、多个闭环延后结算、已提交工具丢检查点恢复、公共压缩与未知副作用核对 |
| Web 单元 | `npm run test:web` | 真实输入校验、分页失败／游标循环、正文损坏／截断／错误封装、幂等重试与新意图、批次确认状态与整批／单项恢复 |
| HTTP／原生 CLI／SSE | `npm run test:e2e` | `tests/v2/local.mjs` 创建并回收独立 Docker 依赖，真实服务与原生 CLI |
| CI 桌面浏览器 | `JUDEX_V2_BROWSER=1 npm run test:e2e` | 真实注册、项目、三级群、上传并排阅读、归档、私聊、主题语言；双人仅改配置的 SSE 冲突、漏收事件的 409 恢复、提交后响应损坏与文件完成重试、查询失败、人工回复与等待收束 |
| 已部署环境 | `JUDEX_V2_URL=http://… node tests/v2/service.mjs` | 配置相同 URL 后可分别运行 cli.mjs、sse.mjs、formats.mjs |
| 完整 K8s | `node tests/v2/cluster.mjs` | 创建有归属 namespace，使用共享 Controller；HTTP／CLI／SSE／七工具／格式／Pod 重启，finally 核对 UID 并清理 |
| 受控岗位 | `node tests/v2/agents.mjs` | 十轮暂停、分支共享、暂停合并、技术预算、配置更新、七个真实工具、共享压缩 |
| 人工分支与并发 | `node tests/v2/collaboration.mjs` | 多人部分回应、原生 CLI 回复并提及、自然收束、并行分支共用额度、真正同时提交继续 |
| 第二次复核回归 | `node tests/v2/followup.mjs`、`internal/agent/peer/followup_test.go` | 缩小设备 scope 与重放、撤销任职自然收束、摘要重压缩、实际额度回退与小窗口、非额度错误不切换、逐次成本与技术预算 |
| 第四次复核恢复 | `tests/web/uploads.test.mjs`、`tests/v2/browser-batch.mjs`、`tests/v2/composite-recovery.mjs`、`internal/agent/peer/deadline_test.go` | 批次整批／子集恢复、原生 CLI 跨进程多附件恢复与新意图、执行中期限、压缩／工具耗尽与过期拒绝发布 |
| 正式界面与菜单 | `tests/v2/browser-appearance.mjs`（由 `browser.spec.mjs` 调用） | 会话搜索开关、文件库侧栏与搜索、草稿保持、固定版本阅读、知识菜单、头像语言／主题切换、90% 聊天字号与桌面溢出；9 项 |
| 内置库与项目设置 | `tests/v2/presets.mjs`、`tests/v2/browser-settings.mjs` | 服务端 16 项目录／权限／并发／幂等／去重／私人提示词检查；浏览器新增 11 项筛选预览、导入编辑、成员、版本、多岗、下载及双语主题，该阶段共享浏览器合计 54 项；已纳入 local／cluster 流程 |
| 完整权限矩阵 | `node tests/v2/permissions.mjs` | 隐藏父群、真实沙箱边界、运行中撤权、HTTP／CLI／SSE／上下文／预览失效、跨项目指定版本分享和私聊隔离 |
| 进程故障恢复 | `node tests/v2/restart.mjs`、`node tests/v2/restart-closures.mjs` | 前者验证未知调用成本和唯一发布，后者在已有 2 轮时强制重启，恢复后准确 20 条岗位发言、10 轮暂停 |
| 真实模型 | `JUDEX_V2_REAL_MODEL=1 node tests/v2/real-model.mjs` | 已授权隔离环境内 APIMart `glm-5.3-flash`，实际工具／文件／岗位协作／压缩 |
| 发行 | `node tests/release.mjs dist/release/VERSION` | 校验和、原生平台包版本、Skill 安装卸载与用户文件保持 |

`JUDEX_V2_CLI` 指定同版原生 CLI；`JUDEX_V2_EVIDENCE` 指定本轮证据目录，默认 `.cache/v2-k8s`，`JUDEX_V2_ENV` 可显式指定 environment.json。cluster 必须用 `JUDEX_TEST_IMAGE` 指定本地已导入的同版 server 镜像，`JUDEX_TEST_CHART` 可指定对应发行 Chart；网关／沙箱／转换器镜像可通过 JUDEX_TEST_GATEWAY_IMAGE、JUDEX_SANDBOX_IMAGE、JUDEX_CONVERTER_IMAGE 指定。重启测试放在其他 HTTP／浏览器／真实模型检查之后执行，普通端口转发会随 Pod 终止，需要重新连接。

浏览器测试通过仅监听 loopback 的 `browser-proxy.mjs` 注入指定 GET 查询失败、屏蔽配置事件及破坏已真实提交的写响应，不在正式产品增加故障开关。localhost 与 127.0.0.1 使用独立 Cookie，双人修改均通过真实表单，测试夹具在浏览器之外建立合成项目；本机由 CUA 调用与 CI 共享的操作流程，不启动额外浏览器驱动。

文件格式使用 `tests/fixtures/materials` 的真实 PDF、图片、Office 和 ZIP；模型夹具 `tests/fixtures/v2-gateway` 只用于确定性断言，不冒充真实模型。原型的 13 项记录仍在 `tests/v2-preview`，与正式验收分开。GitHub CI 运行 backend、web、business；发行另运行同版双架构制品和独立 Kubernetes 回归。

流程图与账号资料：`tests/v2/workflow-profile.mjs` 提供 14 项真实 API 检查；`browser-flow-profile.mjs` 提供 9 项按钮/密度/AI 草稿/历史/昵称头像/双语主题检查，已接入 CI 浏览器流程。`tests/web/workflow-graph.test.mjs` 检查中文标签、分支、反馈和语法注入边界。local/cluster 服务回归已加入新用例，K8s 受控网关更新为 r6。真实模型自然语言生成与受控输出分别记录。本轮实际浏览器执行范围见对应 records，未执行的旧矩阵不计入本轮。

分栏／阅读器／确认卡片：构建 `deploy/material-converter` 镜像后，在已有隔离环境设置 `JUDEX_V2_EVIDENCE`、`JUDEX_V2_URL`、`JUDEX_V2_CLI`、`JUDEX_CONVERTER_IMAGE`，运行 `node tests/v2/readers.mjs`；`JUDEX_V2_BROWSER=1` 加跑 1600×1000 桌面浏览器。完整 `cluster.mjs` 可通过 `JUDEX_V2_READERS=1` 纳入该流程。测试创建真实旧 Office、OpenDocument、工作簿、编码 CSV、图像、音视频与归档样本，验证内容摘要／授权／受控岗位及确认分支，再检查实际 UI 操作；不把扩展名数量当成实测样本数量。


文件卡片／组合头像／使用文档：在完整隔离 V2 部署上运行 `JUDEX_V2_URL=http://127.0.0.1:18100 JUDEX_V2_EVIDENCE=.cache/v2-library npx playwright test --config tests/v2/library-guide.config.mjs`。测试自行注册合成用户，覆盖不可变版本、提交人、指定版本分享边界、成员撤销、SSE 头像刷新、六章节示例和下载，输出截图与 JSON 记录。仅在测试 namespace 使用，不在正常项目注入示例数据。


完整桌面流程包含 `tests/v2/browser-menus.mjs`：验证布局、设置 Select、文件版本菜单的指针悬停无外扩描边、键盘内侧焦点、选项留白及 Escape／Enter 行为，覆盖深浅主题；截图输出到 `JUDEX_V2_EVIDENCE`。
