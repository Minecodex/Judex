继续KA05/U4测试乙，使用本人identity846abe01-c033-40a1-8b80-63102eacf4ca和本人CLI。A32 558dea8f-1473-4618-b6c4-fc7e64d76869已delivery；本人context get实读当前版本/reviewer/A31 accepted前置。读取本轮receipts/KA02-A32-delivery.json，下载固定后端新包、报告/比较证据以及A31固定基准包和A24固定v0.1.1原件，逐SHA核对，安全解包到本人独立review/A32-qa-original，不可读未交付的开发者可变副本替代固定原件。

独立检查改动、索引/查询计划和全部A11语义：Unicode casefold、字面%/_、AND过滤、真实total、按id排序、普通分页、极大合法页200空items、400/409错误合同。新源可在本人新目录以固定A24 web及原测试组成可运行副本；记录文件来源/覆盖关系，不能修改制作人脚本或原包。运行原38 API和20 UI门槛，以及变更相关独立真实HTTP验证。不能仅认可开发者PASS。

性能用固定A31 seed_bench.py/run_bench.py（SHA不变），各100000同语义数据，基线和候选各8预热+60轮同批次复测，逐轮实际HTTP、六组结果digest和正确性控制、真实调度回读、EXPLAIN与数据摘要必须保留。A31旧client提权失效/负责人不可比复跑不能当作成功性能对照；不能直接跟旧固定14.543ms比较。目标p95下降至少20%，B1/B2/B4中位数恶化≤10%，目标CV必须<0.15，基线p50与原12.529偏差>10%时依原合同停止比较并报告。环境噪声不能抄成功；如确实不可判或行为回退，具体reject原因，不能修改门槛/脚本/制造通过。

写review/A32-qa-review.md和receipts/KA05-A32-review.json；准备task accept或reject --no-wait，receipts/KA05-A32-review-intent.json须含完整原CLI JSON和snapshotBasis对象：实际taskStatus=delivered、taskVersion、reviewerIdentityId、实际固定材料UUID列表、reviewDoc及recommendation。等待U4实际固定浏览器确认。只做任务独立验收，不提前开始A33、不验收计划、不派生子代理；清理本人独立临时进程，不停止18481/18482/Judex或他人进程。
