# Judex V2 部署

Chart：`deploy/helm/judex`。组件为 Server、PostgreSQL、S3、Office 转换器、OpenSandbox 服务及可选 Controller。模型调用在可信 Go 服务内执行，沙箱不携带模型／S3 凭据。现有共享 OpenSandbox Controller 应设置 `opensandbox.opensandbox-controller.enabled=false` 复用。

```sh
helm upgrade --install judex deploy/helm/judex -n judex --create-namespace \
  -f YOUR_PRIVATE_VALUES --wait --wait-for-jobs --timeout 10m
kubectl -n judex port-forward service/judex 18097:8080
```

values 中声明 `server.allowedOrigins`、模型目录／环境 Secret 引用、沙箱镜像、预览转换器及群投影配置。生产模式必须明确允许的来源。默认模型配置使用 `apiKeyEnv` 指向 Secret，禁止明文写入仓库或构建日志。岗位并发默认 2。

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
