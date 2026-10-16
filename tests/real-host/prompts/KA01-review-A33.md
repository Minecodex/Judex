继续原KA01/U0产品负责人，身份2a2888fd-4c5b-457b-a622-33b358ca640c。A33 f118d0a6-cb9c-42fa-8a78-60947a1bf739已delivery；本人CLI context get核对当前delivered/版本、reviewer本人及A32 accepted硬前置。读本轮receipts/KA05-A33-delivery.json，下载报告与证据固定原件，逐SHA核对，仅安全解包至review/A33-owner-original。

本轮独立复核固定结果，不必再做一轮全量硬件基准。本人应读逐轮JSON并按冻结run_bench.py实际统计算法重算各批次p50/p95/CV/改进与控制回退比例，对照summary/报告；不是仅转抄QA声明。核对源A24/候选A32版本和文件SHA、固定seed/run脚本SHA、100000行语义摘要及索引造成的schema/物理库差异、8+60轮完整性、六组HTTP状态/响应摘要等价、真实调度回读与时间。所有批次保留；有效批次必须满足基线与原12.529ms偏差≤10%、目标CV<0.15、p95改善≥20%和B1/B2/B4中位数恶化≤10%，错误语义保持。正向/反向批次若存在均分别核算，不能择优隐去失败。

核对38API、20UI和实际Edge断言、22×2独立HTTP证据的正确来源与统计范围（Node测试条目数不等于浏览器断言数）；原开发者不可测批次、A31提权失效与owner不可比复跑不算本次成功对照。任何统计或来源不符需具体reject，不能替QA改写冻结原件。合格则task accept --no-wait。

写review/A33-owner-review.md、receipts/KA01-A33-review.json及receipts/KA01-A33-review-intent.json。snapshotBasis用对象：实际taskStatus=delivered、taskVersion、reviewerIdentityId、reviewDoc、recommendation；materialVersionIds只列A33本次delivery正式挂入验收manifest的原件UUID，上游基准/候选以及其他独立证据用referenceMaterialVersionIds另列，不能混入本任务正式原件。intent保留完整原CLI JSON。待U0实际固定浏览器确认，不自己决定、不注入故障、不验收计划、不派生子代理。
