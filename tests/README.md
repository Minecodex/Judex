# 测试入口

| 范围 | 命令 | 覆盖 |
| --- | --- | --- |
| Go | `go test ./...`、`go vet ./...` | 领域、HTTP 契约、真实 PG、模型/沙箱受控测试；live 测试另需真实配置 |
| Web | `npm run test:web` | 状态、认证、布局、工作权限、资料与模板逻辑；数量以当前输出为准 |
| 浏览器全套 | `npm run test:e2e` | 桌面 demo 与真实生产业务两套入口 |
| 生产浏览器 | `node tests/e2e/business.setup.mjs` | 真实 PG/S3、Office 转换、双用户业务、CLI、全表／对象恢复校验；可传入 spec 或 grep 限定场景 |
| 部署模板 | `go test ./tests/deploy` | 四种 PG/S3 配置、资源边界和无效配置 |
| K8s 探针 | `node tests/k8s/smoke.mjs` | 指定 JUDEX_TEST_IMAGE，临时 namespace 部署与生产 capability 检查 |
| K8s 矩阵 | `node tests/k8s/matrix.mjs` | 包含 mock-gateway 的 JUDEX_TEST_IMAGE，四存储组合及组件故障；真实模型使用 live.mjs |
| Skill 发现 | `node tests/skill-hosts.mjs` | 原生安装卸载、实际 Codex app-server 发现；无模型调用 |
| AI 宿主 | `node tests/skill-agent-hosts.mjs` | 需明确授权及 JUDEX_RUN_AGENT_HOST_TESTS=1；限定隔离目录会话，尚未运行 |
| 发行 | `node tests/release.mjs dist/release/VERSION` | 全包校验和、本机包解压/版本、附带 Skill 安装卸载 |

桌面 demo 浏览器默认 Microsoft Edge，可用 PLAYWRIGHT_CHANNEL=chromium；生产浏览器使用 Chromium。生产测试默认启动受控模型网关，验证真实 PG/S3、CLI 和业务调用，不把受控模型结果作为真实模型验收。设置 JUDEX_E2E_COLLABORATION_GATEWAY=0 可禁用网关，需只运行不依赖模型建议的场景；真实模型测试仍需显式配置和启用。

像素检查依赖 Python 3 和 Pillow，测试前在所用 Python 环境执行 `python -m pip install -r tests/e2e/requirements.txt`；可用 JUDEX_PYTHON 指定解释器。CI 使用同一份依赖清单。桌面测试显式启用正常动效；嵌套选择器选择完成后等待退出动画、DOM 移除及焦点恢复，再操作外层弹层。像素采样先核对所有目标边框颜色稳定，避免与截图结束过渡动画后的状态不一致。

测试使用独立端口、容器和带所有权标签的 namespace，日志/trace/备份放在忽略的 .cache 或 tests/results。不得将依赖缺失导致的 skip 算作实测通过。

生产浏览器入口默认构建并启动独立 Office 转换容器，按源文件摘要复用构建镜像；可用 JUDEX_TEST_CONVERTER_IMAGE 指定已有镜像，或用 JUDEX_MATERIAL_CONVERTER_URL 指定测试转换服务。数据库 SQL 夹具在 Docker 和 Kubernetes 两种入口都先校验本次运行的所有权，测试结束只清理所属容器／namespace。

K8s 测试需要已存在的兼容 Operator/CRD，不部署第二个 Controller，不停止业务服务；finally 按所有权清理。当前证据及剩余边界见 [验收报告](../docs/plans/v1/FINAL-REPORT.md)。
