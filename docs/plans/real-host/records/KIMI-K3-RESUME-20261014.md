# Kimi K3 恢复与流程缺陷修复（2026-10-14）

本轮由用户“继续完善，现在有流量了”恢复。07:42（Asia/Shanghai）单次准入HTTP200，返回k3、142 token。原namespace UID、服务与正式工作先核对，18182自有转发恢复；没有切换GLM。08:20两制作会话再次403后，单次诊断确认供方五小时额度上限，未给准确重置时刻。整体业务验收仍未完成。

## 实际推进

- 原KB01产生B12←B11、B13←B12、B21←B13、B22←B21四项真实UUID开始硬前置，U0/U2/U3本人固定页面审批。原KC01只retry原source/analysis/batch，未新发同一计划message或重置预算；K3产生两计划五任务，U0/U4/U2实际审批；另补四项开始硬前置并审批。合计8个正式计划、26个业务任务。
- KB03原会话独立职责/来源核对，实际CLI以本人grant访问C项目被拒（exit4，project outside grant scope），保存的B选择不变；未读取C材料。
- B11首版固定要求含“1600×900各边80px等于宽高5%”矛盾。原KB03发现但初荐接受并以新裁定补口径，操作员独立复核指出固定原件仍不满足“逐项写清”，未确认原accept，要求真实拒收。U3实际固定页reject→rework v4；原KB01新固定要求明确80px、横5%/纵约8.89%、两适配尺寸64/64及96/96，并将4.5:1明确为硬门槛。原KB03独立下载/SHA/逐项验算后，U3接受v6。旧接受推荐、拒收、旧原件保留，未替制作人补写成品。
- B11返工首次上报发生真实version conflict：制作会话开始时读取delivered v3，实际拒收随后生效；旧版本上报被拒，重读rework v4后新delivery成功。这是操作员时序与回执封装问题所引发的真实失败，不能改述为预登记故障注入。
- B12首次开始按钮在B11正式接受前被禁用，未开始；B11实际接受后U2正式开始B12 working v3。原KB02准备矢量制作/字体轮廓脚本时额度中断，未产生PNG/SVG交付或验收。
- U4开始C11 working v2。原KC02通过本人CLI下载C五件固定来源，独立SHA、安全解包与raw-rounds统计复算（来源为线性插值百分位）完成；生成66,988字节初步XLSX及生成源。当时inspect的“下降率”负号与报告正值对照存在偏差，须在原会话修正/复核；未完成业务LibreOffice重算、逐页看图、上传、delivery或验收，不将初稿计为通过。

## 全局硬条件修复

真实C12/C13/C21/C22 context除已批准的任务前置，还继承发布流程中无targetId的material_ready硬条件，被投影为全零UUID且永不满足。这是流程输入校验与编辑能力缺口，未采用任务waive、SQL改状态或删除实际前置。

- 流程Create/UpdateDraft/Publish共用结构校验；targetId必填、非零UUID且引用本项目已存在的对应任务/接收来源/资料版本。Publish重新校验已存旧草稿，失败事务不改head或旧发布版本。
- Agent draft_workflow Schema与说明、OpenAPI/生成类型及V1设计文档同步；未来对象条件留在节点职责，生成实际对象后通过正式工作提案设置requirements。
- HeroUI共用表单编辑器增加硬条件查看/编辑/移除，文案中英文、样式沿用统一token与深浅主题；无目标保存被禁用。
- 65项Web测试、typecheck、针对Agent与PG回归通过；`go test ./internal/... ./tests/cli ./tests/agent ./tests/integration`通过，完整integration132.124秒。发布构建56项checksum通过。
- `.20`部署至同一自有namespace，Helm revision18，实际system/model目录验证K3配置保留；三个本地目录发行CLI/Skill同步。源码fingerprint `5153b2b75140cbf53225a167bab97233716f0e7239fed7c7a1e7ca1aa2e115df`。正常环境未发布，未commit/push。
- U0实际编辑两条C流程，只移除8条无目标占位条件，保存/发布rev2；旧发布版本、其他节点/角色/连线/正文保持，四项真实task_acceptance/start/hard前置逐UUID核对保持且未waive。C-W1新版本 `91c208ba-1b8d-4660-9a8d-538aaf8d4ffd`；C-W2 `2a45cb76-cc97-45f1-a68a-77bdd6248bf2`。首次UI发布成功后测试脚本误等编辑器关闭，已按实际published head恢复记录，没有重复发布。
- 实际桌面zh-CN/en×light/dark四组合测试和截图检查通过，没有额外保存测试草稿。新Schema的真实K3调用仍待额度恢复，不能由受控测试替代。

## 观测与证据修正

- 并行会话启动清理同一过期观测文件，Windows一次EPERM；实查目标已被另一清理移除，24h过期残留0。清理只在ENOENT/EPERM且目标确已不存在时容忍竞争，不忽略真实权限失败。
- 首次Kimi prompt失败未输出resume_hint。只从CLI自己的session_index读取精确本工作区、唯一新增session并核对创建时间（与首调用差约0.6秒），恢复KB02 `session_8e1da1b9-afe4-4591-b495-31a0c1f47a4b`、KC02 `session_a19d67a9-b20e-4ab3-b8a9-79ac41056a8f`；没有创建替代会话或读取其他会话提示词。后续helper补同类恢复与未跟踪旧会话禁止重开保护。
- 旧检查点中“原KB01未生成续期意图”不准确。本轮补齐实际CLI返回并由API核实 `6553f1ac-7d84-4f56-81bd-fc0b3f02b01f` 曾持久化，只是quota前未完成回执；它未确认，已expired。实际审批仍是操作员另行续期dfe97028的同一原草稿。
- Native把B未采用草稿decd01a0称“空”，独立API实读有4项；按未采用草稿保留，未删除或据此伪称schema绕过。C未采用草稿同样不计正式业务任务。
- 回执解析集中兼容原CLI JSON或带exitCode/stdoutJson的原执行封装，不改CLI原件。转发器新增供应商返回model与脱敏error聚合，6个受控透传测试通过；历史调用不回填新增未知字段。
- 自然TTL扩展样本：331条model_call已到期。首次有1条刚到期残留，随后正常周期清理为0，全部331条计费总账保留；未改时钟或人工清理数据库。这关闭该样本，不代表停机积压/持续负载矩阵已完成。

## 最新保留与继续入口

当前17/30任务接受、4/9计划整体验收；8/9计划和26/30任务已创建，12/14实际Kimi会话。剩余13业务任务未接受，A-P5四任务尚未创建；KC03/KX02会话尚未建立。

恢复先单次准入，再用上面实际sessionId续原KB02/KC02；B先完成真实PNG/SVG和独立看图审阅/必要返工，C先纠正公式符号/对照并实际重算与渲染，不能重复上传或伪造delivery。再完成B/C后继任务、原件跨项目交付至A、A-P5/KX02及剩余专项矩阵。`.20`相关规范与实际workflow-context已刷新，不使用旧C占位条件。

原18482后端45332在暂停期间已退出，退出时刻未知；DB字节SHA保持。操作员在本轮检查原PID不存在后经同一控制器（验证namespace/source30SHA/端口/配置/原DB）重启，控制器24068/后端30912，00:35:13UTC，health200、原四行DB SHA仍 `b748ae116865505f95242e4073a76fcee82befa09b1d377e4972d89a60c1b1e4`。这是保留现场恢复，不是新Kimi恢复测试，不声称服务全程存活；18481和正常namespace未停止。

namespace UID仍 `79725dd1-d0de-4810-999c-4e1d6224a099`；平台queued/running=0、Kimi lock=0；18182自有转发35816、`.20`namespace/原件/部分制作源保留。未自动恢复、未安排提醒；正常发布和最终清理等待完整验收。证据在原`.cache/real-host/20261012-closure`、`.cache/real-host/20261013-kimi-resume`及独立工作区，逐调用观测维持24h范围。
