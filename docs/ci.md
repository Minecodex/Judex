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

Go vet、race 单元与 HTTP/部署契约测试、CLI/server 编译；安装 Helm，避免部署测试因缺少 Helm 而跳过。前端状态测试、TypeScript/生产构建以及既有 Playwright 桌面端全套 E2E，使用 Chromium 并保留失败证据。生产业务测试使用独立 PostgreSQL/S3 与受控模型网关；真实模型调用另行显式验收。

工作流也支持主分支 push 和手动运行。手动运行使用 Actions 页面的 Run workflow，选择待检查的分支；功能分支首次引入新工作流时，先创建 PR 触发检查。

此工作流验证组织仓库实际提交的代码。现有 K8s smoke 需要兼容 OpenSandbox Operator/CRD 和可拉取镜像，应在独立 namespace 按 `tests/README.md` 执行；本工作流不把该集群验收或未提交的本地改动计为通过。

CI 失败会阻止合并。修复失败后在同一个功能分支继续提交，重新运行检查；不要通过删除必需检查或设置管理员绕过来把失败当作通过。

锁文件在无 node_modules 的干净目录从已固定的 workspace 依赖生成，保留已有版本并包含全部平台原生可选包，避免 Windows 生成的锁文件遗漏 Linux TypeScript / 构建工具依赖。

E2E 共用 `openTool` 助手在右栏“＋”打开的菜单内选择入口，选择器限定于当前菜单，避免与中央讨论中的同名动作混淆。
