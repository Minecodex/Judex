# Demo4 第四次修复与发布

2026-10-09，按用户“完善”关闭 [R8](./REVIEW4-20261009.md)，完成桌面对照、长列表检查及常用部署升级。当前 Demo4 对应范围内 R1～R8 已关闭。

## 实现与对照

共享右栏 body 对直接承载 PlanContext 的情况使用零外层内边距，保留计划内容自身的 24px token。内边距与溢出规则分开：任务详情继续管理内部滚动，计划上下文保留父容器 overflow:auto 和既有阅读位置管理。没有添加新组件、改动业务接口或审批规则。

| 桌面宽度 | 右栏宽度 | Demo4 内容宽度 | 改造前 | 改造后 |
| --- | --- | --- | --- | --- |
| 1120 | 384 | 335 | 296 | 336 |
| 1440 | 440 | 391 | 352 | 392 |
| 1920 | 440 | 391 | 352 | 392 |

原型与正式界面的 1px 差来自右栏边框定义；两者自身内边距均为 24px，外层为 0。1120／1440／1920 × 中英文 × 深浅主题的 12 组测量全部对齐。

同样 12 组检查使用 36 项任务验证长列表：容器正常滚动，切换到任务详情再返回保持 scrollTop=600，中央讨论草稿保留。正式组件树原有 42 项回归、64 项前端逻辑通过；发行构建包含类型检查。

几何及长列表最终验证在本轮冻结源码的独立预览中执行。初次长列表检查因预览初始化脚本在刷新时重置数据，独立预览又缺少现有 E2E 环境标志，均属于验证准备问题；修正准备后保留完整滚动／草稿断言，最终 12 组通过。中间日志保留，完成结论只使用最终结果。

证据 `.cache/demo4-remediation4/{geometry-summary,geometry-*,scroll-*}.json`，同位置截图、geometry-scroll-isolated.log、browser.log、web.log、build-image.log。历史 R8 测量保留在 `.cache/demo4-review4/`，原型不变。

## 制品范围与同期修改

镜像 `judex/server:collaboration-demo4-20261009035057-bf5c85ad`，版本 `0.1.0-demo4.20261009035057`，源码指纹 `bf5c85ad16c85dc50fc097f2f3ad3231ab9294b82f9a92a462801edc660a7750`，镜像摘要 `sha256:daade9196d9032f6e4b75957b7f2d76719c53b60549915740b69836e361f2364`。

该镜像相对常用 revision 19 的已验收源码，仅 `web/src/styles/task-details.css` 变化。构建结束后，工作区同期资料功能的 6 个文件继续修改，发布校验检测到漂移。因此保留本轮已构建制品，按 597 项 SHA-256 从精确版本快照恢复出独立 accepted-source；校验以该冻结源码和镜像身份为准，没有把后续进行中的资料修改混入发布，也没有回写或重置这些工作区文件。

保留的同期路径为 materials 的 api.ts、MaterialDialog.tsx、PdfReader.tsx、UploadMaterialDialog.tsx，以及 i18n/materials.ts、styles/materials.css。范围与排除证据 source-scope.json、accepted-source 和 release.json 保存在本轮缓存。不能把本轮制品指纹称为后续整个工作区的指纹。

## 同镜像业务验收

临时 namespace `judex-ui-8b4f31b6` 使用上述镜像通过 25 项真实双用户业务，覆盖 D4-B／R5／R6／R7、C01、COOP-A／R2／R3、P05、T06、MAT06。随后实际 GLM-5.3 C02 通过：1 次职位运行、9 次模型调用、2 次资料读取、1 次公开分析保存，未直接批准。

namespace 经所属 UID／标签核对清理，exitCode=0、cleanedUp=true；原服务没有缩容。证据 `.cache/k8s/judex-ui-8b4f31b6/{manifest,real-model-proof}.json`，独立业务与模型结果目录；本轮 k8s.log 保存命令过程。既有 Go 全量／vet 结果来自同一后端源码的第三次修复，本轮没有将其重记为再次执行。

## 常用部署

Docker Desktop Kubernetes 的 judex release **19 → 20**，deployed、Ready=1；发布与隔离验收使用同一镜像。升级方案核对旧部署身份、运行环境、其他 Deployment、存储、Secret、ConfigMap 和 Service，保留已有配置，仅更新服务及材料投影镜像。

PostgreSQL 备份 273918 字节，SHA-256 `dc14adb0bb5f3559a1628c93751428978fc20e4a46fd3532ad7cd707d10b672f`。82 张现有表业务数据保留：81 张行数与内容摘要一致；模型目录 3 行仅启动更新时间刷新，备份恢复到独立临时数据库逐字段确认模型配置、ID、启用状态与引用不变，该验证容器已清理。

Secret 身份／摘要、PVC／StatefulSet 身份与配置、ConfigMap 均保留。migration=26；judex-migrate-20、judex-storage-init-20 成功。没有重置原数据或向原库恢复备份。

[本地应用](http://127.0.0.1:18097/)已恢复转发。健康、就绪、版本、JS／CSS 摘要、Windows／macOS CLI 与 Skill 下载、PDF 支持资源及四组语言／主题浏览检查通过，pageerror=0。

发布证据 `.cache/deploy/demo4-20261009035057-bf5c85ad/`，包括 release、acceptance、plan-verification、deployment-verification、catalog-verification、frontend-verification、前后数据摘要及备份；本轮脚本与汇总在 `.cache/demo4-remediation4/`。私有配置和备份留在本地缓存。
