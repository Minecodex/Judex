# Judex V2 部署

## 公开 Beta 首次安装

当前公开版本为 `v0.0.1-beta`，首次安装按 [README](../README.md#kubernetes-安装)下载 Release 中的 `judex-0.0.1-beta.tgz`、`values-beta.yaml` 和 `models.example.yaml`。无需部署用 ctl 或本机编译；镜像从 Docker Hub 拉取。三个自有镜像均支持 Linux AMD64／ARM64，公开版本与内部 V2 阶段编号分别管理。

模型示例中的地址和名称需替换为实际提供方的配置。创建 `judex-model-catalog`（键 `models.yaml`）与 `judex-model-keys`（示例键 `JUDEX_MODEL_KEY_DEFAULT`），然后用同版 values 安装。数据库、S3 和 OpenSandbox 的随机凭据由 Chart 生成并在升级时保留；用户模型密钥只通过单独的 Secret 注入。

Chart 默认安装 OpenSandbox Controller 和 CRD。已有兼容 Controller 时设置 `opensandbox.opensandbox-controller.enabled=false`；同一集群只能有一个 CRD／Controller 归属。默认配置为内置 PostgreSQL／SeaweedFS，并启用文件转换、分析沙箱和群资料投影。

## 自定义部署

Chart：`deploy/helm/judex`。组件为 Server、PostgreSQL、S3、Office 转换器、OpenSandbox 服务及可选 Controller。模型调用在可信 Go 服务内执行，沙箱不携带模型／S3 凭据。现有共享 OpenSandbox Controller 应设置 `opensandbox.opensandbox-controller.enabled=false` 复用。

```sh
helm upgrade --install judex deploy/helm/judex -n judex --create-namespace \
  -f YOUR_PRIVATE_VALUES --wait --wait-for-jobs --timeout 10m
kubectl -n judex port-forward service/judex 18097:8080
```

values 中声明 `server.allowedOrigins`、模型目录／环境 Secret 引用、沙箱镜像、预览转换器及群投影配置。生产模式必须明确允许的来源。默认模型配置使用 `apiKeyEnv` 指向 Secret，禁止明文写入仓库或构建日志。岗位并发默认 2。

外部存储从 [values-external.example.yaml](helm/judex/values-external.example.yaml) 开始：PostgreSQL Secret 使用 `password` 键，S3 Secret 使用 `access-key`／`secret-key`；分别关闭内置依赖并指定外部地址、库名／bucket 与现有 Secret。数据库账号须具备初始化和迁移对应数据库的权限。

启用 `ingress.enabled` 时配置 `ingress.className`、`ingress.host` 和 `ingress.tlsSecret`。`server.allowedOrigins` 必须包含浏览器实际来源（协议、域名与端口），不能只修改 Ingress。交互式 HTML 使用单独的 `server.previewOrigin` 并提供可访问的独立入口；其他文件阅读不要求开启交互式 HTML。

## 卸载与资源保留

先结束或终止正在执行的讨论，再执行 `helm uninstall judex -n judex`。默认 StatefulSet PVC、基础设施 Secret、`judex-sandboxes` namespace 和 CRD 按各自保留策略留下；外部数据库／S3 不由卸载命令删除。检查两个 namespace 内的沙箱、Job 与 PVC，确认是否需要保留备份或供下次安装复用。共享 Controller／CRD 可能服务其他应用，不能随 Judex 清理。

内置或外部 PostgreSQL／S3 仍由 Chart 统一配置。V2 使用独立数据库，绝不能把新镜像直接指向旧业务库；迁移器检测旧 users 无 V2 标记时拒绝启动。S3 原件使用 `v2/` 前缀，固定版本不会被后续上传覆盖。

沙箱文件以当前群公共快照准备到运行专属 PVC，挂载 `/workspace/group` 只读，不再挂载项目全文件。准备任务与 PVC 带所有权标记，终止／完成后回收，暂停期间保留。HTML 预览能力受当前会话和群权限约束。

## V1 本机切换

1. 在带 owner／run／UID 记录的临时 namespace 完成同版 ARM64 制品、受控和真实模型、权限、文件及浏览器验收。
2. 停止旧服务写入，保持 PostgreSQL、S3、OpenSandbox 与共享 Controller 运行；按需保存私有回退备份。
3. 在现有 PostgreSQL 建立 `judex_v2`，先运行 V2 migrations。用 `cmd/judex-account-import` 和环境 `JUDEX_IMPORT_SOURCE_URL`／`JUDEX_IMPORT_TARGET_URL` 导入认证记录；工具要求空 V2 目标，逐表完整行摘要相同才提交。
4. 导入 users、password_credentials、有效 Web user_sessions 和 recovery_tokens，保留 UUID、摘要、会话字段及 CSRF；旧 CLI grant 和业务数据不导入。模型、存储、沙箱 Secret 引用不变。
5. Helm 切换同版镜像与新数据库，入口保持 `http://127.0.0.1:18097/`。验证旧密码和旧有效 Cookie、新群聊、重新授权 CLI 推送及真实模型响应。
6. 验证通过后清理旧业务数据库、旧对象前缀和旧项目投影。按 UID／标签清理本轮临时 namespace，恢复本轮曾暂停的正常副本。

## 构建与隔离验收

已完成 V1 切换后，V2 小版本升级继续使用现有 `judex_v2`，不要重新创建数据库或再次运行要求空目标的认证导入。先保存私有数据库备份、认证完整行摘要与 Secret UID／内容摘要，再用同版发行 Chart 升级 Server／Web／CLI／投影镜像并执行增量 migration，前后核对认证数据与 Secret。schema 12 增加延期闭环记录，协议能力仍为 2。Pod 更换会结束旧 port-forward，需要重新连接原端口并验证实际版本。

`node scripts/release/build.mjs VERSION` 固定源文件摘要并校验构建期间未变化，产出同版组件及 SHA256。ARM64 镜像使用对应 server 二进制；Docker Desktop 导入工具检查真实节点架构。候选测试：`tests/v2/cluster.mjs`，不在已有集群安装第二个 Controller，不停止无关服务。

备份／恢复脚本位于 `scripts/operations`；原件归档命令 `judex-server objects export|import`。实际本机切换与清理结果见 [V2 实施状态](../docs/plans/v2/IMPLEMENTATION.md)和相应发布记录。
