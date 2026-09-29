# 测试入口

| 范围 | 命令 | 覆盖 |
| --- | --- | --- |
| Go | `go test ./...`、`go vet ./...` | 领域、HTTP 契约、真实 PG、模型/沙箱受控测试；live 测试另需真实配置 |
| Web | `npm run test:web` | 40 条状态、认证及布局单测 |
| 浏览器全套 | `npm run test:e2e` | 桌面 demo 与真实生产业务两套入口 |
| 生产浏览器 | `node tests/e2e/business.setup.mjs` | 真实 PG/S3、18 条生产 E2E、CLI、全表/对象恢复校验 |
| 部署模板 | `go test ./tests/deploy` | 四种 PG/S3 配置、资源边界和无效配置 |
| K8s 探针 | `node tests/k8s/smoke.mjs` | 指定 JUDEX_TEST_IMAGE，临时 namespace 部署与生产 capability 检查 |
| K8s 矩阵 | `node tests/k8s/matrix.mjs` | 包含 mock-gateway 的 JUDEX_TEST_IMAGE，四存储组合及组件故障；真实模型使用 live.mjs |
| Skill 发现 | `node tests/skill-hosts.mjs` | 原生安装卸载、实际 Codex app-server 发现；无模型调用 |
| AI 宿主 | `node tests/skill-agent-hosts.mjs` | 需明确授权及 JUDEX_RUN_AGENT_HOST_TESTS=1；限定隔离目录会话，尚未运行 |
| 发行 | `node tests/release.mjs dist/release/VERSION` | 全包校验和、本机包解压/版本、附带 Skill 安装卸载 |

浏览器默认 Microsoft Edge，可用 PLAYWRIGHT_CHANNEL=chromium。测试使用独立端口、容器和带所有权标签的 namespace，日志/trace/备份放在忽略的 .cache 或 tests/results。不得将依赖缺失导致的 skip 算作实测通过。

K8s 测试需要已存在的兼容 Operator/CRD，不部署第二个 Controller，不停止业务服务；finally 按所有权清理。当前证据及剩余边界见 [验收报告](../docs/plans/v1/FINAL-REPORT.md)。
