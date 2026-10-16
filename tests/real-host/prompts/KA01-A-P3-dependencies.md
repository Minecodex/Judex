继续KA01/U0产品，仅在修正后的A-P3模型草稿经真实浏览器提交及全部审批后执行。读取work-context.json与实际CLI确认新A-P3 plan/tasks；以实际完整UUID为准，不抄旧dcec invalid草稿clientRef。

通过一个项目范围message请平台真实GLM query_work并propose_changes生成恰两项set_requirements：A32开始硬依赖A31已接受；A33开始硬依赖A32已接受。change.targetType=task、targetId为当前真实目标task UUID、expectedVersion实读；fields仅requirements=[{kind:task_acceptance,phase:start,hard:true,targetId:实际源taskUUID,label:具体}]。A31现有A24依赖不动；不新建工作、不变归属/assignment/workflow/node/标准/性能门槛。不本地写changes充当模型结果。

回读同一submission-analysis的真实终态，核对实际两变更后proposal submit --no-wait。写receipts/KA01-A-P3-dependencies-draft.json、review/A-P3-dependencies-review.json、receipts/KA01-A-P3-dependencies-intent.json（使用intent字段保存完整CLI原JSON），停止待真实浏览器确认。依赖正式批准之前不开始三项任务，不代人确认，不派生子代理。
