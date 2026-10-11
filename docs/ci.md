# PR 与 CI 合并规则

默认分支要求 PR、固定 GitHub Actions `CI`、同步主分支及解决审查讨论；失败、取消、意外跳过均不能通过。禁止强推、删除或管理员绕过。

默认主分支为 `main`。
| 阶段 | 范围 |
| --- | --- |
| PR / main push | Go vet/race/契约/编译，Web 单测/类型/生产构建；Chat、标签、协同、工作、讨论规则和设置六组核心桌面规格 |
| 每个 PR 的真实业务 | 隔离 PostgreSQL、SeaweedFS、Office 转换容器和受控模型；真实 API、双用户、CLI 和业务恢复 |
| v* / 手动 release | 同一提交基础与完整 21 组桌面规格；同一批六平台 CLI/Skill 候选，双架构临时 K8s 四存储组合及故障矩阵 |
| 独立环境 | 真实模型质量、真实 Agent/Skill 宿主及其专用账户/凭据 |

通过 `JUDEX_E2E_PROFILE=pr|release` 选择桌面范围，未知值直接失败。本地默认 release 全套，`CI full` 与发布 source-ci 显式使用完整范围。源代码、存储与实际生产业务的原断言不减少。

发布仅由版本标签或手动候选触发，`ci:release` 标签不再让每次 PR 提交重跑完整矩阵。固定 `Release CI` 汇总同一候选，失败、取消、意外跳过都失败；只保存内部制品，不创建公开版本。

服务镜像使用实际候选字节，受控网关保持独立测试镜像；CI 只在本次自建集群安装 Operator/CRD，并按所有权清理。Actions 不要求额外专用环境配置。受控模型不证明真实提供商质量，独立宿主检查保持待验收。入口见 [tests/README.md](../tests/README.md)。没有定时任务。
