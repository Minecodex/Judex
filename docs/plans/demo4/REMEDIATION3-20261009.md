# Demo4 第三次修复与本地发布

本文保留 R7 验收和 revision 19 发布时的结果；后续[第四次复核](./REVIEW4-20261009.md)发现 R8 计划上下文留白偏差，业务修复保持，视觉收尾仍待完成。

2026-10-09，按用户“完善”关闭 [R7](./REVIEW3-20261009.md)，补齐真实分页验收，并把同一验收镜像发布到常用部署。当前 Demo4 对应范围及 R1～R7 已完成，历史复核报告保留为当时快照。

## 共用修复

TaskTimeline 使用 useTaskRecordReading，按运行模式、项目、用户、任务和查看器范围保存历史展开、已加载页数、来源筛选与原文展开。路线抽屉和讨论标签共用这一状态；切换分区、重新打开及页面刷新后，必要时补取已读页，随后恢复原位置。

useReadingPosition 的卸载保存使用最后观察到的偏移。React 替换分区内容后，缩短的容器不会再覆盖旧分区的阅读位置。UIDisclosure 增加可选的受控展开接口，其余调用继续保留原行为。

全屏进入时复制当前任务阅读状态，退出恢复进入前的分区与偏移，查看器内的筛选不回写原界面。来源定位只应用于当前任务，其他任务的记录状态独立。中央会话、发言范围、附件／文字草稿、正式权限与审批保持原能力；浏览状态不成为业务事实。

## 浏览器与真实业务

| 检查 | 结果 |
| --- | --- |
| 原 38 项＋R7 四项浏览器回归 | 42 项通过，覆盖 1120／1440／1920、中英文、深浅主题 |
| 历史与原文返回 | 抽屉和讨论右栏均保持 41 条、已展开原文及 scrollTop=900 |
| 全屏与对象隔离 | 查看器保留读过的历史，筛选不污染原界面；另一任务使用自己的状态 |
| 真实 API 多页记录 | 23 条真实 progress，上下页读取、分区返回、页面刷新、路线／全屏及原文展开通过；原记录 ID 和中央草稿保留 |
| 前端／后端 | 64 项前端逻辑、类型检查、Go 全量测试及 go vet 通过；部分 Go 结果为有效缓存 |

真实 API 证据 `.cache/e2e/judex-e2e-46328-1791513253555/`，数据库和对象存储备份恢复核对通过。首次新增用例把末页游标误限定为 null，实际接口返回空字符串；用例按已有“无下一页”契约修正，保留记录数量、ID、分页、刷新、原文和偏移的完整断言。

长记录复核的原始失败日志保留在 `.cache/demo4-review3/reading.log`；修复结果使用独立 `repair-reading/` 和 `repair-browser-results/`。原始两项期待行为继续保留，增加全屏、另一任务与原文展开检查。新证据包含 `repair-reading-final.log`、`repair-browser.log`、`repair-api-final.log`、`repair-{web,go,vet,types}.log`。

## 同一镜像隔离验收

镜像 `judex/server:collaboration-demo4-20261009023525-14db7b5f`，版本 `0.1.0-demo4.20261009023525`，源码指纹 `14db7b5f5b2ed9ac2ffb99900d0c854fcc26ac8d48d320a25764a874acc47331`，Docker 镜像摘要 `sha256:f34254ac86f645e0976015a0fef4f454cc58dca2de8dbefec883a1696a581cb0`。

`judex-ui-059d32f2` 使用该镜像，通过全部 25 项双用户真实业务，包含 R7 及此前 D4-B／R5／R6、C01、COOP-A／R2／R3、P05、T06、MAT06。随后同镜像真实 GLM-5.3 C02 通过：1 次职位运行、7 次模型调用、2 次资料读取、1 次公开任务分析，没有直接批准。

namespace 已按所属 UID／标签清理，manifest exitCode=0、cleanedUp=true；原工作负载未缩容。证据 `.cache/k8s/judex-ui-059d32f2/{manifest,real-model-proof}.json` 与独立 business-results／real-model-results；日志 `.cache/demo4-review3/repair-k8s.log`。

597 项制品输入核对无漂移，包含本轮源码和同期工作区改动；462 个代码文件均不超过 2000 行，最大 1805 行。原有无关修改保留。对照继续使用[同位置画廊](../../../.cache/demo4-review2/compare.html)，9 位置 × 12 组合，原型哈希保持一致，正式截图已随本轮回归更新。

## 常用部署与数据保留

常用 namespace／Helm release `judex`，context `docker-desktop`，revision **18 → 19**，状态 deployed、Ready=1。发布前重新核对同期资料更新 `local-materials-20261009103018-4a89bae2`，保存其数据、Secret、ConfigMap、PVC、工作负载与 Helm 快照；升级方案只更新服务及材料投影镜像，其他 Deployment、存储与运行配置保持。

PostgreSQL 备份 273918 字节，SHA-256 `f3d970c6214f95a2ce03361f1388453824f35611c949604fda433b46489b5060`。82 张现有表的业务数据保留：81 张表行数和内容摘要完全一致；model_catalog 的 3 行仅 updated_at 因启动加载刷新。已将备份中的目录恢复到独立临时数据库逐字段比较，确认 ID、模型参数、启用状态和配置引用一致，该临时容器已清理。

现有 Secret 身份及内容摘要、PVC／StatefulSet 身份与配置、ConfigMap 保持；migration=26，judex-migrate-19、judex-storage-init-19 成功。没有重置原数据库、恢复备份到原库或清理无关工作区资源。

访问 [本地应用](http://127.0.0.1:18097/)，本地转发按当前 Service 端口恢复。健康、就绪、服务版本、JS／CSS 摘要、Windows／macOS CLI 与 Skill 下载包、PDF 支持资源及四种登录语言／主题通过，pageerror=0。常用部署与隔离验收使用同一镜像。

发布证据 `.cache/deploy/demo4-20261009023525-14db7b5f/`，包含 release、acceptance、plan-verification、deployment-verification、catalog-verification、frontend-verification、前后数据摘要及备份。模型配置和备份留在本地缓存，不写入仓库。
