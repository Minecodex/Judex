# Kimi K3 继续制作与验收 2026-10-14

用户再次要求“继续，有流量了”。09:42（Asia/Shanghai）单次准入 HTTP200，供应商实际返回 k3 与 K3_READY，135 token；没有切换 GLM。制作和独立复核期间再次返回明确五小时额度 HTTP403，未给准确重置时刻。完整联测尚未通过，原现场保留。

## 恢复现场

- 原 namespace `judex-ui-realhost-20261012105211-5d4b9c`、UID `79725dd1-d0de-4810-999c-4e1d6224a099` 与测试所有权标签保持；三个部署、五个 Pod 健康。宿主恢复后 Pod 曾重启，不声称连续无中断。
- 原 18182 转发已退出且端口空闲，经所有权核对后隐藏启动新的自有 kubectl 转发 PID35548。实际产品 system/model API 验证 `.20` 和两项 Kimi K3 配置，Helm revision18。
- B/C 正式工作读回：B11 accepted v6，B12 working v3，C11 working v2；总计仍为17/30任务接受、4/9计划整体验收。8计划、26任务、12实际会话已建立。
- 续原 KB02 `session_8e1da1b9-afe4-4591-b495-31a0c1f47a4b`、KC02 `session_a19d67a9-b20e-4ab3-b8a9-79ac41056a8f`。复用真实来源与制作源，B继续 SVG/PNG，C修正下降率符号后重算、渲染及独立核验；没有新开替代会话、重复正式任务或代模型写交付。

准入与现场恢复只证明当前可继续；交付、独立审阅、真实返工和固定页面正式确认将分别记录。上轮原失败和 `.20` 修复证据见[晨间恢复](KIMI-K3-RESUME-20261014.md)。

## 本轮真实制作与独立核对

- 原 KC02 完成 C11 四工作表 XLSX（1440条原始批次记录、24组统计、两图表）。下降率改为正口径，批次1/2为38.59%/37.02%；离线 LibreOffice7.4.7.2、独立profile的OOXMLRecalcMode=0实际重算，独立Python对照匹配。测试副本单个输入改变后，p95、下降率及图表数据同步改变，正式输入未改。
- C11 bare upload的三原件均ready：XLSX `eb25c30c-cf16-4081-9c26-407a099111bd`（SHA `8bb5ca2ec71c94f842a62452d69ca2e6c6ca343628f5a8370c8da9c2efacb00c`）、报告 `d692d475-8bbf-43f6-a830-ae4e21c720ff`、证据ZIP `008e3075-3a3d-465b-8955-b20cb465a7eb`。一次delivery后任务delivered v3，report `ebd7aa66-1f2c-4633-bfb9-311b94327293`。原 KC01 已独立下载三交付及五来源、解包并准备自身复算脚本；quota前尚未形成完整审阅报告或正式意图，不能计为接受。
- 操作员实看C11辅助PDF第1页，发现文字/图表跨页裁切；工作簿制作引擎的完整汇总视图可读。C11正式交付是XLSX，辅助PDF不是新业务成品，但版面及证据说明需由KC01恢复后明确核对，不能把抽样PDF称逐页完整检查通过。77页PDF仍保留原件。
- 原 KB02 生成真实原创SVG/1600×900 PNG。首张草稿的e与D/O/P内孔填充异常，操作员及制作人实际看图识别；制作人自行修轮廓合并与变量字重后，操作员再次看图并运行其检查器：两次PNG SHA均 `4d3984a789f2ce1a37c49acc9dc9551cee2df1ec6a2ad61e74ebf9d9bb21bff9`、RGB/sRGB、80px安全区、字标93px、十对颜色最小对比5.88:1，检查通过。此为working阶段制作内修订，没有正式品牌退回；quota前报告/许可说明/证据ZIP及CLI交付未完成，B12仍working v3。
- KC03真实新会话 `session_59814024-99d5-405d-a70a-d6f5068c1637` 建立并以本人device grant核对C项目/Office身份、C12 ready v2及C11未接受硬前置；实际探测离线Office/字体/库。未开始C12、未交付，也未声称原生Office许可恢复。累计13/14实际会话，KX02仍未建立。

## 路由、重放及真实失败

- 原KX01用真实系统时钟预登记七例（已绑定、仅计划、仅职位、仅正文、无适用候选、含糊、绑定冲突）及稳定clientSubmissionId，然后真实CLI提交。已绑定例完成，其余六例quota失败；全部source/analysis/batch已通过本人浏览器认证只读API恢复，不重新submit或重置预算。正式A12保持accepted v7、原aw1-be、原acceptance `efcd056b-870b-42de-a18c-30d0c519e8d5`。
- 已绑定例currentNodeCandidates保留正式绑定、没有修改决定，四个已知私人测试marker计数0；但公开摘要把历史delivery manifest称为“新材料”，并给出“第二份贡献未见”建议，操作员当前正式读取A15/A-P1均已接受。该例只关闭绑定保持与已知marker样本，来源时效/后续建议语义仍有缺口，不能把completed状态算整例语义通过。
- 原KX01后台驱动只对waiting/quota预期状态停，实际failed终态后仍提交了后五例，各自quota失败；原失败与开销保留。这是联测驱动缺陷，不能改述为六个路由判断通过，也不能把未知用量写0。
- 操作员使用同一真实已登记source做实际CLI并发重放：两个同键/同正文请求都返回 `3f43868a-e157-4e59-832b-5450225af54d`；同键改正文返回exit5。前后source/analysis/batch各1，没有新消息或预算重置。原body与前后事实保存于live-cli-idempotency.json。
- 新路由CLI驱动逐步落实际source，任意非queued/running/completed状态立即停止；显式retry只用原analysis，核对原batch，合并局部检查点时保留所有未访问来源。6项回归通过；在当前真实failed记录只读运行，停止于only_plan，无retry或后续submit。未来恢复必须通过该驱动，不再由临时后台循环继续刺激。

## 共用供方分类修复与隔离部署

- 真实Kimi明确五小时额度403此前被`classifyGatewayError`提前归auth。共用分类现要求`access_terminated_error`及正文同时含明确五小时达到上限、quota reset证据，才在403/流内错误识别为quota；类型单独出现、无重置/无额度证据、普通403、401、普通429及网络失败不改为quota。
- HTTP/流内回归、额度降级/非额度不降级回归及完整Agent包通过；本轮K3配置仍没有备用模型。V1设计同步，没有扩大权限、自动审批或引入GLM降级。
- `.21`发行构建/typecheck通过，56checksum、实际Windows CLI/Skill安装所有权验证通过。source fingerprint `c0dca424c9b5386cdf4e7953201c8a4f5a1e0895e6761eac98e8090d1b3d822a`；镜像manifest `ae955958e1a91c854e277b6a7f2ea3a8da81e8542dade1e041595b1bbd02625b`、index `b089172c8b92e51df7e59bd1e125ba7b881b568336778a3db63d58aef335b127`。
- 在原UID namespace升级为Helm revision19，values只改server image tag；实际system/model API验证 `.21` 和两K3配置，三个本地CLI及已安装/包内Skill均对齐 `.21`。滚动更新使旧forward先残留监听再退出，未误杀；端口空闲后隐藏恢复PID37548。首次本地安装从旧包源读到20，已改用冻结21发行包安装并逐目录回读确认，未影响凭据。
- 分类新分支是实际403证据驱动的受控回归，尚未做 `.21` 新K3调用复验；`.20` 新流程硬条件Schema的K3实调用也仍待额度。正常环境未发布、没有commit/push、没有清理未完成业务现场。

## 观测与继续入口

五次实际native调用共127次供方请求，124次usage/cache可判，124次响应model明确为k3；另3次403用量未知。可判输入12,455,513、输出84,260、缓存读取11,680,025，同集合加权93.77%；cache-write未提供，不作费用或全局冷缓存推论。明细保持24h TTL，永久仅脱敏聚合。平台七例有22次调用，已知16次（缓存可判14次）、失败403等未知6次；不同scope/kind分组保存，不能与Kimi调用混算。

当前仍17/30任务接受、4/9计划整体验收，8计划/26任务已创建；剩余13业务任务未接受，其中C11已交付未接受，B12未交付，A-P5四任务未创建。Image/Office正式往返、综合交付/接岗及其他专项仍未完成。平台queued/running=0、Kimi locks=0；18482健康、原4行DB SHA保持，正常18481未操作。

下次用户恢复先单次K3准入、核对21/原UID/转发；续原KB02补报告/许可与交付，原KC01复用其独立下载及复算脚本完成C11审阅，再实际有权固定页。原KC03等待真实C11接受和UI开始。KX01只用 `node tests/real-host/routing-case-runner.mjs --retry-existing` 恢复原六失败分析；completed例不得重开，额度不足或批次预算不足即停。不能把本检查点当完整验收，也不自动恢复或安排提醒。
