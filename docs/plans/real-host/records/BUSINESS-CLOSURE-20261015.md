# 真实业务样本完成与剩余语义门槛

后续更新：本记录保留业务完成时点的失败与剩余门槛；其后8处修复和限定复测见[修复记录](REMEDIATION-20261015.md)，当前进度见[progress.json](../progress.json)。

验收结束：2026-10-15（北京时间）。供方为用户指定APIMart glm-5.3-flash，原生Kimi CLI 0.36.1，thinking enabled / reasoning_effort high。原角色会话继续，未自动降级。模型配置与实际wire校验分别留证，high不代表输出无需核对。

## 已完成的业务范围

3项目、8发布流程、11职位模板、12初始任职身份、14实际业务/专项Kimi会话。正式30任务均由指定reviewer接受，9计划均经owner另行固定页接受；六个错误归属的取消样本不计入30。公开当前事实见[验收快照](../evidence/apimart-business-acceptance-20261014.json)，其totals为30/30任务与9/9计划。

最终A-P5 accepted v6，含本计划A51v4/A52v5/A53v6/A54v7和原A32/A24/A33/A43四引用，不复制或转移原任务。B/C四计划分别接受；24视觉原件和15当前Office原件经独立源项目grant导出、目标A grant导入，SHA逐字节一致。主包包含优化后端、已验证主题、PNG/SVG、XLSX/DOCX/PPTX/PDF原件、许可/来源及两QA的独立证据。

测试甲沿原identity接岗U3/binding1→U5/binding2，历史/待办保持，旧绑定新决定403，旧A-only grant撤销后CLI exit3；同人B grant及browser正常。新人的个人偏好初始空，保存自己的revision1；真实模型输入hash匹配U5、另一QA匹配U4，两人未合并。受控私有回声两次被保护层拦截，公共工具/检查点/分析/运行manifest的controlled marker均0。此结论只针对明确样本，不能保证任意敏感信息或改写不泄露。

A53第一份U5独立贡献到位后workingv4，第二份U4到位后才deliveredv5，随后reviewer接受v6。第一份38API/56真实HTTP/8重启持久化；第二份38API/20UI/30实际CDP、1440×824 CSS/dpr1、四组合图/12页Office实看；源哈希与优化索引保留。测试侧超合同断言、工具问题和首次失败保留，未降低冻结门槛。

## 最终包与重放

主ZIP SHA256：66375099ec2936d7fde9a9d600c3c84f5070f5fbc4026b5cf9cb0134d72a6786，8,475,528字节、132条目。SOURCES.json实际存在，files131项逐SHA匹配，自身不递归自hash。replay ZIP SHA256：65ff03926561db530d2fcd5a106baab730e42c8bba079d00c634f79165e1500c，包含当前脚本、冻结清单及六已接受输入。

制作人不同label两build、干净目录两replay均同SHA；独立reviewer另在新目录两replay也同SHA。新旧主包只有SOURCES.json新增，131项业务/质量/QA字节未变；独立38API/20UI及跨重启smoke通过。首包漏SOURCES.json曾真实reject为reworkv5，新三固定件交付v6并独立复审接受v7，旧包/报告/失败仍保留。

本机最终交付目录：D:/workspace/judex-real-host/20261012-closure/deliverables/MiniIssue-v0.2.0-integrated。独立来源/接受证明为该目录acceptance-and-provenance.json。正式环境未发布，没有提交/推送本仓库或新建PR。

## 场景范围与尚未通过项

| 范围 | 当前结论与限制 |
| --- | --- |
| S01宿主/会话 | 3工作区、14原生CLI角色、同会话恢复及真实Skill使用通过已批准CLI路径；不冒充Kimi桌面GUI创建验收。 |
| S02业务创建 | 建项、模型流程草稿/发布、计划任务安排与各有权账号固定页确认有实际全链；不将AI草稿视为正式决定。 |
| S03软件 | 首版/主题/缺陷/优化/受控故障/综合包实际往返与独立验收通过其明示范围。 |
| S04隔离/接岗 | 当前身份、旧决定/旧grant、A-only越项目负控制及同人B正控制、私人偏好与literal guard有证据；岗位汇总语义仍有下面的缺口。 |
| S05路由 | 原七例全部运行结束，5语义通过、2partial。限定后续第一例通过，第二例native判pass-annotated但操作员保留范围偏离：性能计划请求夹入另一计划A52的aw1-int。[独立判断](ROUTING-CLARIFIED-FOLLOWUP-20261014.md)保持，通用路由未关闭。 |
| S06决定 | 实际设备授权、固定审阅、退回/重开、过期续签/旧绑定和撤销grant、独立task/plan接受通过已执行范围；不扩充未实测权限组合。 |
| S07视觉 | 真实SVG矢量制作/PNG光栅化、原生模型看图、透明1×/2×、受控新增DPI往返/独立像素与许可核验；不称照片生图服务或平台PNG元数据解析等同视觉理解。 |
| S08Office | 固定原件、真实LO公式重算/导出、中文/页表/图表/源定位、真实数字退回/修订通过离线路径；原生Office许可未恢复。扫描PDF明确needsOCRorVision且空文本，损坏样本octet-stream明确unsupported MIME，不宣称OCR。 |
| S09综合 | 源码+视觉+Office+两必需贡献+可重放包+单独计划接受闭环。C21源ID/C22单元格/A54漏清单各真实往返；未受影响版本按同SHA复用。 |
| S10故障/恢复 | 实际额度历史、断流/连接重置/超时、partial上传/会话续作、版本冲突、同键并发/改载荷、取消投影修复有范围证据；未新增真实供方429、CLI自然token到期和所有outbox/重启交叉组合，不称全矩阵通过。 |
| S11长会话 | 原角色长会话/恢复、重新读取当前事实、绑定/偏好变化保护有实际范围；high准入旧标记返回和120秒边界失败仍保留，不能称所有压缩/流程更新组合通过。 |
| S12缓存 | 实际Kimi/APIMart与平台供方分开统计，missing与0分开。435自然到期模型行的临时observation/requestID清零，durable账留存；全变因缓存因果矩阵/计费节省未证明。 |
| S13安装/持续 | 同版CLI/发行与安装Skill25、默认发现加载及自然24h窗口有证据；持续大负载与桌面重启全组合未验收。 |

另一未通过项：受控接岗征询批次曾因private-context guard等待人工审阅，核已知效果/0 unknown后U0公开运营API续一轮，原预算保持、运行completed；最终summary为__SLOT_B__，没有有效汇总语义。输入隔离与保护层证据成立，completed不能替代内容通过。原两不安全输出的失败与后续clean role outcomes并列留证。

因此状态为“完整业务样本已接受，整体AI语义/持续运行门槛仍部分未关闭”，不是全平台发布准入或全部S01–S13通过。接入探测、小样本成功、业务接受、运行结束、模型语义通过分别记录。

## 归档与清理

两临时namespace经各自UID、test-run、Helm来源核验后删除：主namespace UID79725dd1-d0de-4810-999c-4e1d6224a099；sandbox UIDe352155e-bb0c-4b7b-8100-4dc7773f4072，三个项目PVC已先读只读备份。受保护正常judex/judex-sandboxes/Argus没有列入删除，正常Judex四deployment清理后仍1/1。

忽略目录.cache/real-host/20261012-closure/export保存：数据库SQL（model_calls临时字段不入dump）、725行durable模型账JSONL（observation移除、requestID置空）、Seaweed原始对象tar、三沙箱工作区tar及SHA/namespace清理证据。四gzip可读、源对象完整性校验，provider Secret没有导出。原件/工作源/原Kimi会话保留；临时原始观测按限定所有权清理，长期只存脱敏统计与业务结果。

本次只停止各owner记录匹配的18482故障实例、18483集成实例和18182自有port-forward；QA自有18485–18488已分别清理。18481在清理前已connection refused，本次没有停止/重启它，也不将其称健康。
