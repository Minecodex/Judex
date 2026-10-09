# Kubernetes 部署骨架

2026-10-09 本地 `judex` 已更新到 Helm revision 21，包含 Demo4 正式交互、运行例外及共享资料体验修复，访问 `http://127.0.0.1:18097/`。当前源码、镜像、同镜像隔离业务与真实模型证据见 [Demo4 当前部署复核](../docs/plans/demo4/REVIEW5-20261009.md)和[共享资料体验收尾](../docs/共享资料体验收尾-20261009.md)。发布保留既有数据、Secret、PVC、模型及沙箱配置；此前 revision 14／15／18／19／20 的记录继续作为历史证据保留。

Go 单体服务托管生产前端。Helm 同时声明 PostgreSQL、SeaweedFS、OpenSandbox，也可独立选择外部 PG / S3。需要可用 K8s、动态 PVC StorageClass、Helm 3.16+、安装 Operator / CRD 的权限及集群可拉取的 Judex 镜像。

```sh
npm run build
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

业务服务实际连接 PostgreSQL、S3 和已配置的模型 / OpenSandbox；Chart 创建带数据库 advisory lock 的迁移 Job 与内置对象桶初始化 Job。发布使用 `--wait --wait-for-jobs` 等待服务及初始化完成。`/readyz` 检查服务排空状态与数据库连接，不替代对象存储、模型或沙箱的业务验证。

## OpenSandbox

依赖与补丁见 [UPSTREAM](helm/UPSTREAM.md)。Lifecycle Server 在平台 namespace，执行 Pod 在 `<namespace>-sandboxes`。Server 使用单副本 Recreate。每个 namespace 只安装一个 Judex，平台 namespace 不超过 53 字符。

集群只保留一个管理这些工作负载的兼容 Operator。已有 Operator 或安装第二套时加 `--set opensandbox.opensandbox-controller.enabled=false`。不要因开发 / 生产 values 不同而重复安装集群控制器。

自备 API Secret 设置 `global.sandboxAPIKeySecret=<名称>`、`global.sandboxAPIKeyManaged=false`，键 `api-key`。API 和沙箱使用同一配置。上游子 Chart 的 server 名称 / namespace 不应单独覆盖，否则会破坏连接约定。

服务端 Agent harness（模型调用在 server 进程）、OpenSandbox 客户端、项目资料只读挂载、派生提交和网络隔离按 [V1 计划](../docs/plans/v1/README.md) P5/P7 实现；平台不部署任何持密钥的沙箱 Runner。Chart 尚未为 OpenSandbox 的本地状态配置持久化，不能承诺执行恢复。

## 验证边界

运行 `go test ./tests/deploy` 与 Helm lint 验证模板。K8s 冒烟入口为 `tests/k8s/smoke.mjs`，需要已有兼容 Operator / CRD 和可拉取镜像。

2026-09-29 已在 Docker Desktop 集群验证四种内置/外部 PG/S3 组合、组件故障和资源回收。证据与镜像范围见 docs/plans/v1/CONTINUATION.md。

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

## 更新已有本地部署（PowerShell）

本机 Docker 与 Kubernetes 节点 containerd 是不同镜像存储。每次构建使用独立 tag，先导入节点，再用 Helm 更新；仅重启旧 tag 不能保证使用新构建。本地未推送 registry 的镜像使用 `IfNotPresent`。

以下命令针对已有的 `docker-desktop` 集群、`desktop-control-plane` 节点容器和 `judex` Release / Namespace，在仓库根目录顺序执行，任何命令失败都应停止后续步骤。导入脚本直接传递镜像归档的二进制字节，避免 PowerShell 文本管道损坏归档：

```powershell
$imageVersion = 'local-' + (Get-Date -Format yyyyMMdd-HHmmss)
$env:JUDEX_RELEASE_VERSION = $imageVersion
npm run build
docker build -f deploy/docker/server.Dockerfile --build-arg VERSION=$imageVersion -t "judex/server:$imageVersion" .
node scripts/deploy/load-local-image.mjs "judex/server:$imageVersion"

helm upgrade judex deploy/helm/judex --kube-context docker-desktop -n judex --reuse-values --set-string image.repository=judex/server --set-string image.tag=$imageVersion --atomic --wait --timeout 10m
kubectl --context docker-desktop -n judex rollout status deployment/judex --timeout=180s
kubectl --context docker-desktop -n judex get pods
```

前端构建会生成同版本的 Windows/macOS CLI 和 Skills ZIP；`JUDEX_RELEASE_VERSION` 必须与 Docker 的 `VERSION` 相同，镜像构建会检查这一点。`--reuse-values` 保留现有模型、凭据、沙箱和数据服务配置，`--atomic` 在失败时回滚。升级前检查模板变化；若 Chart 引入必需的新配置，先审查并显式合并该配置，再升级。导入的归档保存在 `.cache/deploy`，作为本机部署证据。

发布后重新启动本地转发（原转发随旧 Pod 退出会断开），并检查 `/healthz`、`/readyz`、`/api/v1/system` 返回的版本是否为本次 tag：

```powershell
kubectl --context docker-desktop -n judex port-forward service/judex 18097:8080
```

部署验证可在临时 Namespace 运行 `tests/k8s/smoke.mjs`；浏览器业务 E2E 通过 `JUDEX_E2E_BASE_URL` 指向临时部署，必须把该访问 Origin 加入 `server.allowedOrigins`。测试资源按运行标签核对归属后删除。


2026-10-07 12:05 的本机发布为 Helm revision 6，镜像 `judex/server:local-20261007120038-ea51afc0`，服务端与 Windows/macOS CLI、Skills 下载清单版本均为 `0.1.0-ui.20261007120038`。更新前通过隔离 namespace 的 11 条真实浏览器测试；更新后核对健康检查、生产网页校验值、三个实际下载 ZIP 和业务数据/Secret/PVC 保持。证据见 `.cache/deploy/ui-downloads-20261007120038-ea51afc0/manifest.json`，完整界面验收说明见 [桌面界面统一改造](../docs/界面优化预览.md)。


2026-10-07 14:10 的职位场景模板发布为 Helm revision 7，镜像 `judex/server:local-20261007140030-9b42bc04`，应用和客户端下载版本 `0.1.0-presets.20261007140030`。Migration 19 增加可空来源标识及活跃预设唯一索引，既有业务记录、Secret/PVC 与运行配置已核对保持。当前镜像通过隔离 namespace 的 13 条业务浏览器验收，详情和证据见 [职位场景模板](../docs/职位场景模板.md)。


2026-10-07 15:15 的边框、卡片按钮与职位优先分配更新为 Helm revision 8，镜像 `judex/server:local-20261007151045-4ad93210`，应用/客户端下载版本 `0.1.0-ui.20261007151045`。部署前的当前镜像通过隔离 namespace 的 14 条业务浏览器验收，原有 9 个职位及其他业务数据、Secret/PVC、运行配置已核对保持；完整范围见 [职位场景模板](../docs/职位场景模板.md)。


2026-10-07 16:24 的标签边框与功能列表悬停更新为 Helm revision 9，镜像 `judex/server:local-20261007161448-b7fad684`，应用/客户端下载版本 `0.1.0-ui.20261007161448`。当前镜像通过隔离 namespace 的 14 条真实业务浏览器测试，原有数据、Secret/PVC 与运行配置核对保持；记录见 [统一界面组件规范](../docs/统一界面组件规范.md)。

2026-10-07 16:49 的固定返回项目列表更新为 Helm revision 10，镜像 `judex/server:local-20261007164302-00199b76`，应用/客户端下载版本 `0.1.0-ui.20261007164302`。通过隔离 namespace 的 15 条真实业务浏览器测试，包含项目间往返、草稿/附件恢复与实际提交；测试资源已按所属标识清理。生产资源、三个下载包、现有业务数据、Secret/PVC 与运行配置已核对。端口转发 `http://127.0.0.1:18097/` 已恢复；证据见 `.cache/deploy/project-return-20261007164302-00199b76/manifest.json`。
