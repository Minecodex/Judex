# PR 与 CI 合并规则

本仓库默认主分支为 `main`。日常开发从最新主分支创建功能分支，将功能分支推送到组织仓库，再通过 PR 合并。

```sh
git fetch origin
git switch -c feature/your-change origin/main
git push -u origin feature/your-change
```

主分支由仓库规则集保护：必须通过 PR，必须通过 `CI` 检查，合并前必须同步最新主分支，禁止强制推送和删除。规则没有管理员绕过名单；目前不额外要求他人批准，PR 上的审查讨论必须解决。

`CI` 是固定名称的汇总检查。它在所有 PR 上运行，任一必需任务失败、取消或意外跳过都会失败，避免工作流名称或矩阵版本变化导致保护规则失效。检查来源限定为 GitHub Actions。

## 自动检查范围

每次版本标签执行 release.yml 完整发布验收：复用同一提交的 PR 基础检查，构建一批候选包，在 Linux/Windows/macOS 的 amd64/arm64 六个原生 Runner 上实际执行打包的 CLI 与 Skill 安装卸载；Linux 两种架构在独立 Minikube 集群中验证实际候选服务镜像、四种 PG/S3 组合及故障恢复。Release CI 要求每项均成功，取消、失败、意外跳过都失败。没有定时任务。手动运行或给 PR 加 ci:release 标签可在创建版本前检查完整候选；工作流只保存内部制品，不创建公开发行。

集群服务镜像直接使用候选包的 server/web 字节；受控模型网关是独立测试夹具镜像，不能将包含 mock-gateway 的测试服务镜像当作发布候选。矩阵入口必须分别提供 JUDEX_TEST_IMAGE 和 JUDEX_TEST_GATEWAY_IMAGE。原生包、集群与基础检查各自保留其真实覆盖范围；受控模型不构成真实提供商质量或宿主模型验收。

`backend` 检查 Go vet、race 单元与 HTTP/部署契约测试、CLI/server 编译；安装 Helm，避免部署测试因缺少 Helm 而跳过。`web` 检查前端状态测试、TypeScript/生产构建以及 Playwright 桌面全套回归，使用 Chromium 并保留失败证据。`business` 独立执行真实业务回归、CLI 与恢复校验，使用 PostgreSQL、与 Helm 默认一致的 SeaweedFS 4.47 S3、Office 转换容器和受控模型网关；真实模型调用另行显式验收。三项都属于 `CI` 汇总的必需任务。

业务 job 在测试前显式拉取固定存储镜像，避免本地镜像缓存掩盖公共仓库授权或可用性问题。原 MinIO 镜像匿名拉取返回拒绝，测试改用项目既有 SeaweedFS；应用继续通过同一 S3 API 读写、分片与恢复，不改变业务存储接口。

业务脚本直接用 Node 启动时，也从 Windows 的 Node 目录或 Linux/macOS 的 `lib/node_modules` 定位 npm CLI，不依赖 npm 父进程注入 `npm_execpath`。

每次运行的凭据目录权限为 0700，S3 认证文件只读挂载并允许镜像内非 root 用户读取；失败时保存所属容器的状态和脱敏启动日志，再按运行标签清理。CI 仅上传这些诊断及浏览器结果，不上传认证配置或数据库备份。

CLI `--no-wait` 确认测试仅点击一次浏览器确认，然后轮询只读回执直到 `committed`，继续核对结果引用；点击返回不等于服务端确认请求已提交。

跨多任务前置、四次任务验收、计划验收与重开的完整界面旅程使用 120 秒整项预算；每次状态核对仍限 10 秒，普通界面用例默认 45 秒，业务断言不变。

工作流也支持主分支 push 和手动运行。手动运行使用 Actions 页面的 Run workflow，选择待检查的分支；功能分支首次引入新工作流时，先创建 PR 触发检查。

此工作流验证组织仓库实际提交的代码。现有 K8s smoke 需要兼容 OpenSandbox Operator/CRD 和可拉取镜像，应在独立 namespace 按 `tests/README.md` 执行；本工作流不把该集群验收或未提交的本地改动计为通过。

CI 失败会阻止合并。修复失败后在同一个功能分支继续提交，重新运行检查；不要通过删除必需检查或设置管理员绕过来把失败当作通过。

锁文件在无 node_modules 的干净目录从已固定的 workspace 依赖生成，保留已有版本并包含全部平台原生可选包，避免 Windows 生成的锁文件遗漏 Linux TypeScript / 构建工具依赖。

Web job 用 Python 3.12 和 `tests/e2e/requirements.txt` 安装像素检查所需的 Pillow。桌面浏览器显式使用正常动效，选择器及账户弹层通过 DOM 移除和焦点恢复确认关闭，避免 Linux 退出动画期间的嵌套 Escape 和点击拦截。像素采样等待全部目标边框颜色稳定，保留原采样覆盖与阈值。

E2E 共用 `openTool` 助手在右栏“＋”打开的菜单内选择入口，选择器限定于当前菜单，避免与中央讨论中的同名动作混淆。
