继续 KA01/U0 产品负责人，仅用本人 .tools/KA01.cmd。A16已由U0实际浏览器正式accepted v5，A11~A16六任务均已接受；A-P1仍active，先不要验收计划。读本次实际CLI版本/task context，不抄旧状态。

新的真实问题：操作员对hash匹配的已交付A12后端做只读HTTP预探测，page=999999999999999999999999999999&page_size=1返回500/internal_error；page=1000000和9223372036854775808控制组200空items。A11 §4明确page整数≥1，超范围页返回空items且total真实，没有页码上限。证据 .work/large-page-prefind.json 是操作员预发现，不能算测试甲A21正式贡献，也不代表根因已确定。不能为通过验收给合法页码另加上限或放松200空列表约定。

按已批准的场景A-P2执行缺陷流程，并顺延A-P1整体验收到修补闭环后。现在通过实际submit项目范围message，把业务目标交给平台真实GLM协调者query当前正式事实、propose_changes生成一个A-P2计划和四任务的草稿。不要本地写changes JSON代替模型输出，不使用HTTP/SQL建正式工作。

计划：A-P2 MiniIssue v0.1.1 极大页码缺陷修复；owner产品identity 2a2888fd-4c5b-457b-a622-33b358ca640c；正式已发布A-W2 217ef1ef-c862-5dd1-9bb1-83e5633d7fea。四任务各fields.planId明确指同次create_plan的clientRef，dependsOn仅创建顺序。当前只五项create变更，不改旧任务/计划，不新增/发布流程。
- A21 极大页码缺陷复现：测试甲af739ffe-a283-4caf-b0ab-67e903e55704参加，产品身份reviewer；node aw2-repro。从固定A16发布包82d277ac或A12修订71b001d9下载、SHA核对、隔离临时实例独立复现真实HTTP失败与正常控制，引用A11原需求，输出复现脚本/来源/实际错误与根因待确认。开始硬前置A16 6e009085-d73c-4fb5-bc03-8328915abf7b task_acceptance，kind=task_acceptance phase=start hard=true label具体。
- A22 极大页码修复：后端4b02a3ee-4c71-488a-a985-827d641558f7参加，测试乙846abe01-c033-40a1-8b80-63102eacf4ca reviewer；node aw2-fix。依据A21正式复现修后端分页溢出；合法极大页码仍200空items/真实total，普通分页/过滤/负值非法400不回退，增加边界测试，保留原36API与QA门槛；新固定版本交付。
- A23 补丁独立回归：测试乙846abe01参加，产品reviewer；node aw2-reg。新固定补丁真实HTTP逐例极大页码/普通边界/非法参数、真实Edge关键页面与主题回归，独立来源与实际结果；不能拿后端自测代替贡献。
- A24 v0.1.1补丁交付包：运维b861062d-ef8c-48ea-a591-0445000bb32d参加，产品reviewer；node aw2-patch。从A22已接受新后端与A13已接受主题前端固定原件生成可重复ZIP、v0.1.1版本/逐文件SHA/来源/README；干净目录实际启动及极大页码smoke，新固定材料/报告。

注意三项后继开始硬依赖A22←A21、A23←A22、A24←A23：当前这些实际UUID尚未生成，不能把clientRef字符串塞进要求uuid的requirements.targetId。先创五项草稿并经实际审批得到任务UUID，后续另让模型形成恰好3项set_requirements草稿再审批；在依赖批准之前不开始A22~A24。计划验收须四项任务人工接受且新版本满足原标准，旧失败保留。A-P1最终应通过正式reference_task引用A24修补交付（到有实际A24后另走模型提案和审批），不直接搬任务归属或抹掉旧首版缺陷；这也是跨流程相关证据核对，不改旧任务正式workflow/node。

用同一submission-analysis合理退避回读到完成或实际失败，记录sourceId/analysisId/proposalId至 receipts/KA01-A-P2-draft.json。核对模型实际草稿恰好1计划4任务、owner/participant/reviewer、planId及已发布node正确，A21真实A16硬依赖正确；可复核时 proposal submit --no-wait只创建pending意图，写 receipts/KA01-A-P2-submit-intent.json（完整CLI JSON）与 review/A-P2-draft-review.json。不能自己浏览器确认、审批、开始任务，不派生子代理。分析失败不得写成完成；若已经有可复核模型草稿，保留实际分析状态与草稿分别报告。
