# Helm 与 Kubernetes

首个公开版本为 `v0.0.1-beta`。正式发行使用同版 Helm Chart 与 `values-beta.yaml`，Judex 自有 Server、转换器和分析沙箱以 `kakj/judex-*` 发布到 Docker Hub，支持 Linux AMD64／ARM64。公开制品记录完整提交、源码摘要、SHA-256 与镜像索引摘要；部署和发行流程见 [安装说明](../../README.md#kubernetes-安装)、[发行手册](../运维/releases.md)。既定组件边界与共享 Controller 归属保持一致。

正式部署使用 `deploy/helm/judex`：Server、PostgreSQL、S3、Office 转换器、OpenSandbox 服务；共享 OpenSandbox Controller 可复用，避免同一集群安装第二个 Controller。保留现有 Secret 引用，不把模型或基础设施凭据写入镜像、文档或日志。

模型目录使用 Secret 中的公开能力配置与 apiKeyEnv 引用，凭据通过独立 Secret 注入。岗位运行并发初始 2。群资料准备任务和运行 PVC 带所有权，挂载 `/workspace/group` 只读，不挂载项目全量资料；完成／取消后回收，暂停保留。

测试环境使用带 owner／run 标签和 namespace UID 记录的临时命名空间；清理前同时检查标记和 UID，等待主命名空间及沙箱命名空间消失。只在资源不足且已保存副本信息时暂停正常 Judex 工作负载，不操作共享 Controller 或无关服务。

本机 ARM64 镜像构建和导入必须检查节点架构；同版发行包含 Server、Web、CLI、Skill、Chart 和 OpenAPI。切换与实测证据见 [运维](../运维/README.md)及 V2 发布记录。
