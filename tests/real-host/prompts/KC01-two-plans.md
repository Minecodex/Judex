继续原KC01/U0报告负责人。C两个流程已由U0实际编辑发布并挂到三职位的多个节点，A→C五件来源（含manifest）已由本人真实CLI下载/逐SHA核对/bare upload并ready。读team/workflow-context、receipts/KC01-source-handover.json和toolchain-context.json，实读本人身份与C选择。纠正源标签边界：A24是本机MiniIssue v0.1.1补丁包的构建报告，不是正常/生产部署；保留manifest原词并在后续质量报告明确这一区别。

项目范围发送一次业务message，真实K3协调者list_agent/query_work核对职位、发布流程与C已接收固定来源，propose_changes生成恰两计划+五任务（七项create）。不手写提案JSON或直接建正式对象，不建新讨论。

- C-P1 质量分析报告，owner报告负责人，workflow C-W1；C11 多工作表XLSX公式图表（数据分析师参加，报告负责人reviewer）；C12 DOCX质量报告及PDF（Office制作参加，报告负责人reviewer）；C13 来源/数值一致性核对报告（报告负责人参加，数据分析师reviewer）。C11以已移交的真实A24/A33/A43原件及A33证据ZIP为输入，至少来源、实际批次数据、计算/图表三个有用工作表，公式计算p95改善/控制恶化与复核，实际重算并保存数值/方法来源；不能虚构缺少的分母、次数或把未知当0。C12以C11实际接受的固定数据写真实DOCX/PDF，逐页渲染、看图查中文字体/裁切与数据一致性，不声称原生Office许可恢复。C13独立下载本轮固定原件、逐SHA、从source/raw-rounds复算关键数值并交叉核对XLSX/DOCX/PDF，不抄maker PASS，保留旧失败/方法限制。
- C-P2 发布汇报材料，owner报告负责人，workflow C-W2；C21 PPTX汇报及PDF（Office制作参加，报告负责人reviewer）；C22 图表/数值/版面核对报告（报告负责人参加，数据分析师reviewer，数据分析师提供已核对的C11数值依据）。C21用已接受报告/工作簿固定来源制作真实中文PPTX/PDF和可编辑图表或清楚标注静态快照来源，逐页渲染并实际看图；C22独立检查来源、数值、图表、页数/字体/裁切与两个格式一致，记录未覆盖。不代替A生产部署或整体平台验收。

每任务仅上述一个participant和一个reviewer；fields.planId指对应create_plan.clientRef，workflowId与nodeId为本项目当前发布真实节点且允许参加职位，字段只能nodeId。创建时五任务requirements先空，取得实际UUID后另用第二提案补C11→C12→C13→C21→C22开始硬前置；未补齐前不开始工作，不凭clientRef编造UUID。计划标准包含三/两任务分别接受、固定来源/真公式重算/原件+真实PDF导出/独立复核/至少一轮真实退回修订及整体人工验收。

当前工具链是已安装的捆绑文件库和离线LibreOffice7.4.7.2（networknone/pullnever），原生Excel/Word许可仍过期只读。普通转存未重算（999缓存首轮失败）；独立临时profile设OOXMLRecalcMode=0已通过合成7/3.5挑战，真实C11仍需自己重算、实际验证，不拿合成探针当业务验收。

保存receipts/KC01-two-plans-draft.json及实际提交/分析/提案引用，同提交submission-analysis核对完成；若CLI没有proposal show/review，如实记录并依据实际coordinator调用记录核对（随后操作员固定浏览器另读原正式草稿）。核对恰两计划五任务、身份/节点/归属/来源标准正确才proposal submit --no-wait，receipts/KC01-two-plans-submit-intent.json的intent字段存完整原CLI JSON。失败保留，不重复另发消息，不自己确认/开始/验收，不派生子代理。
