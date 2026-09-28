# Kubernetes 部署骨架

Go 单体服务托管生产前端。Helm 同时声明 PostgreSQL、SeaweedFS、OpenSandbox，也可独立选择外部 PG / S3。需要可用 K8s、动态 PVC StorageClass、Helm 3.16+、安装 Operator / CRD 的权限及集群可拉取的 Judex 镜像。

```sh
docker build -f deploy/docker/server.Dockerfile -t judex/server:0.1.0-dev .
helm lint deploy/helm/judex -f deploy/helm/judex/values-dev.yaml
helm upgrade --install judex deploy/helm/judex -n judex-dev --create-namespace -f deploy/helm/judex/values-dev.yaml --wait --timeout 10m
kubectl -n judex-dev port-forward service/judex 8080:8080
```

本地构建后需要自行载入开发集群或推送镜像。生产安装用 `values-prod.yaml`，覆盖实际 image.repository / image.tag。内置数据服务为单实例，生产 values 不代表 HA、备份恢复或生产业务已验收。

## 存储

`postgresql.embedded.enabled` 与 `objectStorage.embedded.enabled` 可独立选择。外部配置见 `helm/judex/values-external.example.yaml`，四种组合都有模板测试。

- 外部 PG：host、port、sslMode、username、database、existingSecret；Secret 键 `password`。
- 外部 S3：endpoint、region、bucket、existingSecret；Secret 键 `access-key` / `secret-key`。
- 内置 PG：单实例 StatefulSet + PVC；生成密码 Secret。已初始化数据库换密码时须同步数据库，单独改 Secret 不会修改数据库。
- 内置 S3：SeaweedFS 的 master / volume / filer / S3 同一 StatefulSet，数据和索引写 PVC。应用桶创建待存储模块实现。
- Secret 使用 lookup 保留现有凭据；PVC、生成的 Secret 和执行 namespace 卸载时保留，清理由运维按归属处理。

当前 Go 壳层只读取 HTTP / Web 配置。Chart 注入的 PG / S3 / OpenSandbox 变量是预留连接契约，尚未连接或运行迁移。`/readyz` 只检查 HTTP 服务，不检查基础设施健康。

## OpenSandbox

依赖与补丁见 [UPSTREAM](helm/UPSTREAM.md)。Lifecycle Server 在平台 namespace，执行 Pod 在 `<namespace>-sandboxes`。Server 使用单副本 Recreate。每个 namespace 只安装一个 Judex，平台 namespace 不超过 53 字符。

集群只保留一个管理这些工作负载的兼容 Operator。已有 Operator 或安装第二套时加 `--set opensandbox.opensandbox-controller.enabled=false`。不要因开发 / 生产 values 不同而重复安装集群控制器。

自备 API Secret 设置 `global.sandboxAPIKeySecret=<名称>`、`global.sandboxAPIKeyManaged=false`，键 `api-key`。API 和沙箱使用同一配置。上游子 Chart 的 server 名称 / namespace 不应单独覆盖，否则会破坏连接约定。

服务端 Agent harness（模型调用在 server 进程）、OpenSandbox 客户端、项目资料只读挂载、派生提交和网络隔离按 [V1 计划](../docs/plans/v1/README.md) P5/P7 实现；平台不部署任何持密钥的沙箱 Runner。Chart 尚未为 OpenSandbox 的本地状态配置持久化，不能承诺执行恢复。

## 验证边界

运行 `go test ./tests/deploy` 与 Helm lint 验证模板。K8s 冒烟入口为 `tests/k8s/smoke.mjs`，需要已有兼容 Operator / CRD 和可拉取镜像。

本轮机器 Docker daemon 和 Kubernetes API 不可用，尚未完成真实镜像构建或安装验收；模板渲染不等于部署成功。

## 本地全内置部署（Docker Desktop，2026-09-28 实测）

所有依赖（embedded PostgreSQL + SeaweedFS + OpenSandbox 控制器与 Server）随同一 Helm Release 部署到 `judex` 命名空间，不依赖其他项目的共享实例：

```bash
# 1) 前端生产构建 + 服务端镜像（镜像名须与 values.image 一致）
cd web && npm run build && cd ..
docker build -f deploy/docker/server.Dockerfile -t judex/server:0.1.0-dev .

# 2) 导入节点 containerd（desktop-control-plane 容器内的 k8s.io 命名空间；
#    pullPolicy=IfNotPresent，本地镜像不推 registry 时必须此步）
docker save judex/server:0.1.0-dev |
  docker exec -i desktop-control-plane ctr --namespace k8s.io images import -

# 3) 命名空间与 Secret（密钥不进 values/仓库）
kubectl create ns judex
kubectl -n judex create secret generic judex-opensandbox-auth --from-literal=api-key="$(openssl rand -hex 24)"
kubectl -n judex create secret generic judex-model-catalog --from-file=models.yaml=deploy/models.yaml.example
kubectl -n judex create secret generic judex-model-keys   --from-literal=JUDEX_MODEL_KEY_GLM53=<真实key>   # 键名与 models.yaml 的 apiKeyEnv 一致

# 4) 安装（模型网关经 modelEnvSecret 引用密钥；目录同步入库供选择）
helm install judex deploy/helm/judex -n judex   --set environment=development   --set server.allowedOrigins="{http://localhost:18097,http://127.0.0.1:18097}"   --set server.modelCatalogSecret=judex-model-catalog   --set server.modelEnvSecret=judex-model-keys   --set server.modelGateway.protocol=openai-compatible   --set server.modelGateway.baseUrl=https://open.bigmodel.cn/api/coding/paas/v4   --set server.modelGateway.model=glm-5.3   --set server.modelGateway.apiKeyKey=JUDEX_MODEL_KEY_GLM53   --set global.sandboxAPIKeySecret=judex-opensandbox-auth   --set global.sandboxAPIKeyManaged=false

# 5) 本地访问
kubectl -n judex port-forward svc/judex 18097:8080   # http://127.0.0.1:18097
```

注意：port-forward 目标 Pod 重建（rollout/崩溃）会断流，需重开；生产模式用 Ingress（`ingress.enabled`）。`/api/v1/system` 的 `capabilities` 按实际接线如实上报（identity/projects/persistence/agentExecution）。
