# 部署依赖来源

Judex 自有代码使用 Apache-2.0。上游组件保留各自许可证与版权声明。

## OpenSandbox

- 上游：[opensandbox-group/OpenSandbox](https://github.com/opensandbox-group/OpenSandbox)。
- 原始包：[Helm 0.2.2](https://github.com/opensandbox-group/OpenSandbox/releases/download/helm/opensandbox/0.2.2/opensandbox-0.2.2.tgz)。
- vendored 源码：`vendor/opensandbox/`；许可证见其中 `LICENSE`。
- 本地 Chart 版本：`0.2.2-judex.1`；依赖锁和打包产物一同保留，不使用 latest。
- Judex 固定镜像：Lifecycle Server v0.2.2、Controller `docker.io/opensandbox/controller:release-1.1.1`、execd v1.0.22、egress v1.1.6。父 Chart 覆盖 vendored 子 Chart 的 Controller 默认值；发行 CI 的独立 Controller 安装使用同一覆盖。Controller 的 AMD64／ARM64 索引为 `sha256:aca5cda09a0fbc5867e6028f8b6aaa60fdcd78ebee634f71a263d45d5f6c2bab`，当前本机共享 Controller 使用该版本。

本地模板修改：Controller 可以单独禁用以复用集群 Operator；Server 和 Controller 默认进入 release namespace；资源名包含 namespace 和 release；Server 使用 Recreate 并校验单副本；配置文件和 env 使用 `tpl` 注入执行 namespace / Secret，values 必须由可信部署者维护；空 global 改为对象以正确合并。

重新打包：在根目录运行 `helm --repository-config deploy/helm/repositories.yaml --repository-cache .cache/helm dependency update deploy/helm/judex`。不要直接修改打包文件。

## PostgreSQL / SeaweedFS

使用官方 `postgres:17.6-alpine` 和 `chrislusf/seaweedfs:4.47` 镜像，上游许可证随各自项目和镜像维护。

未嵌入 SeaweedFS Helm Chart，使用最小单实例 StatefulSet，以 `weed server` 启动 master、volume、filer 与 S3，数据和 filer 索引放在 PVC。本机集群运行与文件读写已有 V2 验收，实际执行范围见[运维记录](../../docs/运维/README.md)；该拓扑的容量与可用性仍需按部署环境评估。
