# 项目协同界面本地发布记录

2026-10-08，按用户“本地重新构建部署升级下”的明确请求，已将当前工作区代码重新构建并升级现有 Docker Desktop Kubernetes 部署。此发布独立于此前的实现与隔离验收，未重置工作区或业务数据。

## 当前运行版本

- context：`docker-desktop`；namespace／Helm release：`judex`。
- Helm revision：13 → **14**，状态 `deployed`；服务 Ready=1，新 Pod restartCount=0。
- 镜像：`judex/server:local-cooperation-20261008215701-4b10e3a2`。
- 服务及 CLI／Skill 发行版本：`0.1.0-cooperation.20261008215701`。
- 源码指纹：`4b10e3a2f32f25f0401a4e9b241176a60a100a157f3b81350b6b8edd978fb63f`。
- 镜像 ID：`sha256:c46142442146345d16e6f365876714b75ee7775cbd18ef127a61e922f2764c9e`。
- 访问：[本地服务](http://127.0.0.1:18097/)。已重新建立该服务的隐藏 port-forward；5196 仍为原同组件树 demo 预览。

实际滚动升级使用 `--reuse-values --atomic --wait --wait-for-jobs --timeout 10m`，仅更新服务镜像与对应材料投影镜像。升级前比对了 Helm 模板及实时工作负载配置；空环境变量在 Kubernetes 省略 `value` 与模板 `value: ""` 的表示差异按等价处理，没有修改配置值。

## 本轮验证

- `go test ./...` 全套通过，部分结果为已有缓存；`npm run test:web` 61 项通过。正式发行构建及 Helm lint 通过。
- 新镜像以相同镜像 ID 的验收标签部署在独立 `judex-ui-4bf930f3`，完成 **20 项**真实 API／CLI／双用户／受控模型浏览器验收。包括任务主讨论并发复用、多对象关联、版本冲突、分页、分叉、草稿与附件隔离、交付接收、任务与计划验收、真实 CLI 浏览器确认、完整职位与流程设置。
- 使用现有 GLM-5.3 配置完成另一个真实模型最小分析：**8 次模型调用、1 次岗位运行、1 次公开任务分析保存**，任务状态未由聊天自动推进为验收完成。密钥只在内存读取并复制到测试命名空间的独立 Secret。
- 临时 namespace 经 UID／所属标识校验后删除；manifest `exitCode=0`、`cleanedUp=true`。原部署与其他项目的命名空间保留。
- 正式部署 `/healthz`、`/readyz`、`/api/v1/system` 均为 200，服务版本与此次发行一致。生产入口 HTML／JS／CSS 摘要与构建输出一致。
- 三个平台 CLI／Skill ZIP 下载版本、大小、摘要和下载响应头通过；不存在的下载返回 JSON 404。
- 正式地址的 1440×1000 浏览器检查通过：中文浅色、英文深色，登录控件、统一字号／颜色、页面宽度正常，无 pageerror。已认证业务流程使用上面的独立双用户验收，不在现有项目写入测试记录。

## 数据与配置保留

- 发布前已保存 PostgreSQL custom-format 备份，250082 字节，SHA-256 为 `e0c6b3aa13618c80513bcd47549905fbfd5c26044184e38f94562432467852fb`；未在原库执行恢复。
- 40 张业务表的行数及内容摘要升级前后相同。比较时仅排除新增的 `main_topic_id`、`links_version` 字段，保留全部原业务字段。
- 当前用户 8、项目 6、议题 7、消息 9；计划／任务／材料均为 0，与发布前相同。
- 5 个非 Helm Secret 的 UID 与内容摘要保持一致；两个存储 PVC 的 UID／卷绑定及 PostgreSQL、SeaweedFS StatefulSet 身份／镜像保持一致。
- 服务环境、ConfigMap、模型配置、OpenSandbox 配置保留；材料投影镜像随本次服务镜像更新。
- goose migration 22 → **23**；`judex-migrate-14` 和 `judex-storage-init-14` 均成功。
- 未提交改动未清理、未提交；未停止其他项目服务或删除其数据。

## 证据与边界

本轮发布证据目录：`.cache/deploy/cooperation-20261008215701-4b10e3a2/`。其中 `release.json`、`plan-verification.json`、`helm-upgrade.log`、`deployment-verification.json`、`frontend-verification.json`、`acceptance.json` 与前后快照登记构建、发布和保留检查。数据库备份、含配置的 Helm 快照留在本地缓存，不写入仓库。

隔离业务及真实模型证据：`.cache/k8s/judex-ui-4bf930f3/manifest.json`、`controlled-browser/`、`real-model-proof.json`、`real-model-analysis.json`。此前完整 58 项桌面和 96 场景视觉证据仍见 REMEDIATION3 与 REVIEW4，本轮没有把它们改记为重新执行。

通用多流程自动路由的既有门槛仍按原报告单独跟踪；本次发布不将该缺口记为已解决。
