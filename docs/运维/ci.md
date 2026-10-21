# V2 CI 门槛

源代码 CI 的 backend／web／business 必须全部成功。backend 设置独立 PostgreSQL 管理 DSN，执行 vet、race 测试和 cmd 构建，包含认证导入、预算及岗位恢复。web 执行现行单元测试与正式构建。business 建立有归属的临时 Docker PG／S3，运行真实 HTTP、原生设备 CLI、SSE 和桌面浏览器。

发行按固定源树构建同版组件和校验和，六种原生 CLI 平台解压并验证版本与 Skill 安装卸载。独立临时 Kubernetes 集群部署候选，复用该集群自有 Controller，运行群权限、文件格式、七工具、闭环、公共压缩与 Pod 重启；finally 清理测试 namespace 与集群。

真实 APIMart 联测需要明确启用并注入 Secret，在授权环境单独记录；不能把受控网关输出或 CI 未来配置视为本轮真实模型证据。当前执行记录在 [V2 状态](../plans/v2/IMPLEMENTATION.md)。
