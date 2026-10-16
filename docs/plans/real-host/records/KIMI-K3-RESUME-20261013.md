# Kimi K3 恢复与真实 CLI 复核

2026-10-13，用户明确恢复联测，提供 Kimi Code 凭据，并说明 GLM 已无可用额度。沿用原有场景、身份及 14 会话安排，不启用 GLM 降级。

## 接入与验证

- 实际模型目录返回 `k3`，最小调用 HTTP 200、响应模型为 `k3`。配置使用[官方模型 ID 与接口](https://www.kimi.com/code/docs/kimi-code/models.html)；思考保持开启，避免关闭思考时路由到其他模型。
- Kimi Code CLI 0.36.1 通过进程内临时模型配置续接 KX01，实际执行 Judex 版本、doctor 和 inbox 查询，退出 0。11 次 K3 请求均 HTTP 200；真实读缓存为 468224 / 532563 输入 token，属于这一调用组的观察，不代表全账号或全局冷热缓存。
- 独立真实工具往返调用成功；传回工具结果后 HTTP 200。强制指定工具与开启思考的负例返回 400，保留于本轮证据。平台使用正常自动工具选择。
- 凭据只注入 CLI 进程和自有 namespace 的 Secret；源码、发行包、回执与报告不含密钥。

## 平台与第一轮失败

自有 namespace 仍为 `judex-ui-realhost-20261012105211-5d4b9c`，UID `79725dd1-d0de-4810-999c-4e1d6224a099`。`.18` 发行原件与源码指纹匹配，56 项发行校验通过；部署 revision 16，实际服务端与三工作目录 CLI / Skill 均为 `.18`。协调者和职位目录均改为 K3，保持原 token 上限，取消本轮 GLM fallback。

KX01 经真实 CLI 提交 A22 单次只读问题，source `3c8f2178-e363-43d8-8adb-a088443c8865`、analysis `5d0f8cda-73e3-46cb-a96c-14c24070057c`、batch `c588518c-84ef-42b5-9f6d-aa82743c7568`。分析真实失败：供应商严格校验发现零参数工具的 `properties:null`，返回 HTTP 400；尚未调用岗位 Agent。该失败不计为模型业务通过，usage / cache 未知不填零。A22 状态、版本、report 和 acceptance 前后不变。

共享 `objectSchema` 已修复：空参数 properties 为 `{}`，无必填字段省略 required。新增测试检查所有注册工具的 wire 类型，以及空对象通过、额外字段仍被拒绝。Agent、CLI、完整 internal / PostgreSQL integration 回归通过（integration 164.897s）；`.19` 发行构建及 56 项发行校验通过。`.19` 已部署（revision 17），实际接口和本机 CLI / Skill 同为 `.19`。

第一轮手工 retry 被 KX01 设备 grant 拒绝，缺少 `agent:request`，rc4；该负例保留。随后通过公开 CLI 登录与实际 U1 设备授权页面添加这一项必要 scope，未更改用户角色、节点或预算。原 analysis/source/batch 经一次实际 retry 完成，协调者 3 次、后端职位 2 次 K3 调用，实际 `call_agent`、`list_agent`、`read_material` 和 `record_task_analysis` 成功。原失败 model_call 仍存在，未新建 source 或重置批次预算；A22 version/report/acceptance 前后完全相同。

后端公开意见保留具体条件和异议，未以岗位意见替代验收。已知私人测试标识在公开 summary 与 call_agent 结果中均为 0；偏好 hash/revision 可见，原始 system context 按设计不落 checkpoint，因此不由 system 字段为空推断未注入私人偏好，也不声称拦截所有改写。

独立 A42 交付的自动分析也 completed：6 次 K3 协调者调用，实际原件读取/分析后保存公开结论。6 次中只有 4 次返回可用于 cache 比例的已知字段，缺字段保持 unknown。

自然 24 小时 TTL 实际观察到 8 条已到期 model_call：临时 observation 与供应商 request ID 均已清理（到期残留各为 0），8 条总 token/accounting 原记录保留。未调整时钟或到期日期；这关闭对应样本的自然 TTL 条件，不代表整个角色、流程与跨项目缓存矩阵完成。

## A41 当前闭环

原 KA01 使用 K3，实际读取 A41 delivered v3 与 A33 已满足前置，核对固定原件并再次运行操作员提供的只读探针，10/10 通过。复核文档明确辅助脚本的来源，保留旧失败与旧探针输出；未修配置或启停故障实例。

U0 经实际固定浏览器确认同一接受意图，A41 accepted v4。A42 的原 KA02 / K3 完成配置恢复，两轮各 9 项实际 HTTP 检查；保留四行库与 30 份原程序 SHA 不变。原 KA05 / K3 又独立核对固定原件、配置/进程/数据并执行 20/20，只读检查后由 U4 固定浏览器确认，A42 accepted v5。

当前接受任务 15 个；A-P4 仍 active。A43 已由 U3 实际 UI 开始，原 KA04 / K3 下载并核对四件固定材料，确认配置、源码、原库、实例和现场健康一致；尚未执行独立重启、上报或验收。已关闭计划仍为 3 个，完整 9 计划、30 任务验收未完成。

## 本轮额度检查点

2026-10-13 20:16（Asia/Shanghai，12:16 UTC）核查：KA04 最后一轮被实际 HTTP 403 中断，供方明确表示已达到 5 小时用量上限，当前窗口结束后恢复，未提供具体重置时间。这不是 key 无效、普通限流或平台预算失败；没有改用 GLM、伪造 A43 交付或继续失败探针。

所有本轮 Kimi CLI 已退出，模型运行 queued/running 为 0，会话 lock 为 0。保留 `.19` 自有 namespace、原始文件、已有正式事实、18182 转发与受控服务。18482 PID 7944 仍健康，原四行 DB SHA `b748ae116865505f95242e4073a76fcee82befa09b1d377e4972d89a60c1b1e4` 不变；18481 未停止。没有自动恢复或提醒。

恢复时先做小请求确认 K3 配额，再续接原 KA04，复用 `review/A43-original` 同 SHA 原件，完成尚未进行的独立 HTTP/重启验证和报告；再由原 KA01 审阅及 U0 固定浏览器决定，另作 A-P4 计划验收。B/C 负责人 CLI 设备授权已准备，但项目、流程、计划和业务会话尚未创建；继续保持软件优先，设计 / Office 随准入展开。

本轮证据位于 `.cache/real-host/20261013-kimi-resume/`，公开 Kimi 会话及业务原件仍在原场景目录。临时逐调用观测维持 24 小时范围，永久只保留必要结果和脱敏统计。

## 23:20 用户再次恢复

2026-10-13 23:20（Asia/Shanghai，15:20 UTC）单次文本准入实际 HTTP200，返回模型 `k3`，输入92、输出25、合计117 token。先前明确五小时额度403仍保留；没有改用GLM或将等待时间视为准入。

自有 namespace UID/运行标签、`.19` 三个 Deployment 和五个 Pod 均实读核对正常。18182没有监听，已在同一 namespace 新建隐藏的局部端口转发并记录所属PID/创建时间；未停止任何原有进程。正式快照仍为15个接受任务、3个关闭计划，A43 working v3、A-P4 active v1。18482实例仍健康、原四行DB SHA不变。原KA04会话已续接，只复用固定原件并完成尚未进行的独立检查、重启和交付；是否成功以新实测回执为准。

原KA04随后完成独立重启前后各11/11实际HTTP检查；控制器核验后仅停止7944，启动新后端45332（父26876）。原四行DB字节SHA与30份源码SHA不变。固定报告/30项证据ZIP经bare upload后一次delivery，A43 delivered v4；原KA01下载固定原件并独立10/10只读核对，U0实际固定浏览器接受为v5。当前接受任务16个，A-P4另行整体审阅中；尚未关闭综合场景。

A43正式报告源 `631d92ef-6110-4a21-a330-9537640560a7` 对应分析 `705dc0ae-aa28-4c37-9162-84f8709db5b9` completed，6次协调者K3真实调用成功，包含原件解析/读取及结构化记录；缓存4次已知、2次未知。同一已知子集输入89136、读取76288，不能将未知补零。原KA04本轮公开CLI由回环观测到24次HTTP200、usage/cache均已知，输入2495411、输出20074、读取2319872；与平台统计分列。KA01任务复核中的一次普通429保留，未更换模型。

图片准入用不含预期答案的随机文字/红方块/蓝圆形合成PNG，K3直接API HTTP200，三项识别断言通过；原生Kimi CLI图片读取另由计划内KB01实际验证，不使用旧GLM MCP。[原生图片工具说明](https://moonshotai.github.io/kimi-cli/en/customization/agents.html)只说明接口，不替代本机调用证据。

Office以独立合成XLSX作重算挑战，输入2、5，公式SUM/AVERAGE的缓存故意设999。同一现有LibreOffice7.4.7.2镜像在禁止拉取/网络下默认转存保留999，首次检查失败；自有临时profile设 `OOXMLRecalcMode=0` 后实际重算为7、3.5，公式/输入保留，没有安装UNO或其他新依赖。该设置对应[官方加载重算说明](https://help.libreoffice.org/latest/en-US/text/shared/optionen/01060900.html)。原生Excel许可只读限制仍在，C业务工作簿和完整Office验收尚未开始。
