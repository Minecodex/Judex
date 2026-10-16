继续 KA01 / U0 产品负责人原会话，仅用 .tools/KA01.cmd 和产品 identity 2a2888fd-4c5b-457b-a622-33b358ca640c。回读当前正式任务/计划与版本，不信旧会话或producer元数据中的版本/状态。平台当前发行 .14，原A13已由U4接受v8，不是还待审。A16 6e009085-d73c-4fb5-bc03-8328915abf7b 当前 delivered v4，report 83fbebce-5990-451f-85dd-1f67e30b17df；你是其实际reviewer。

下载固定发布 ZIP 82d277ac-3b8a-45e9-b7ea-eb840e2a3679（SHA57153933175b3cf211d4ae900fae7a78dda3f6758467a08082e8c3f440489a69）及构建报告 aadcea30-4c26-4941-8d9f-5721e74201ab（SHAade1253fee4672fa8c7d868c46e2cb938686a72b2dfd4b44c4d4e0b3955fff0b）到新 review/A16-original/，校验SHA、安全解包并核对SOURCES.json逐文件摘要。包内backend/web必须与已接受A12修订ZIP71b001d9、已接受A13主题修订ZIP3783470a逐字节一致，不能因README或producer自述就默认来源正确。A13包的pending描述只作为构建时快照，当前接受事实用实际task context确认。

实际独立检查：候选包没有db/config/凭据/会话/cache；README在干净解包目录可照步骤启动；执行API36+UI20单测（显式文件列表），使用自己临时端口/DB健康、页面、新建、Unicode过滤和跨重启持久化。你可以复制自身smoke脚本调整目标包，不使用producer的PASS摘要代替独立执行，不能停止18481公共测试实例或Judex部署。关闭本人临时进程。新主题已由U4的21/21真实Edge/API核对，下载固定QA新证据20985eae-57a1-45b0-8cf5-1e868deaa541/短复核19e614cc-1b1f-43d8-a1d9-029d2528d530校验并读实际JSON关键F-1，不把旧A15 18/19改写为通过。必要时看已选截图，不访问其他用户浏览器。

保存review/A16-owner-review.md、本人实际输出及 receipts/KA01-A16-review.json。当前只验收A16：符合实际条件就task accept --no-wait准备pending，否则task reject --reason具体失败--no-wait；保存 receipts/KA01-A16-review-intent.json（原CLI intent、snapshotBasis taskStatus delivered、taskVersion4、reviewerIdentityId本人、materialVersionIds两个A16版本、reviewDoc、recommendation）。最终由U0实际固定浏览器确认。不要提前创建plan accept意图，须A16实际accepted后再单独核对六任务事实和整体产品。平台自动分析此前失败单独标记，不能被任务接受遮盖。

附带元数据核对：producer回执cliVersion误写旧.7，操作员实测打包时.13；自己的回执用实际CLI返回，新版harness已提供本次实测版本/开始时间。旧producer回执不覆盖。版本等元数据错误要显性记录而非抄入本人回执，不新增交付/伪造业务状态。
