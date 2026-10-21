# 分栏、阅读器与确认卡片

日期：2026-10-20。承接[已确认讨论](WORKSPACE-READER-CARDS-DISCUSSION-20261020.md)。正式实现位于 `web/src` 和现有会话／文件服务，协议保持 2，数据迁移为 schema 15。

## 分栏

全局导航固定；群列表、聊天、文件库／阅读区可拖拽分配宽度。默认三栏 22／43／35；没有阅读区时额外空间交给聊天。两栏组合分别记忆宽度，恢复阅读区使用原三栏比例。低于区域最小可用宽度的 55% 时提示折叠并在松手后收起。

该版本固定导航仅保留一个「布局」图标菜单（v2.21 已改为[区域标题恢复入口](PANE-NAVIGATION-20261021.md)），支持恢复任意区域和重置。每栏标题有轻量收起图标，至少保留一个区域。分隔线支持左右方向键、Home 和双击复位；阅读聚焦退出恢复原布局。隐藏区域不卸载内容，草稿、滚动、文件与版本保留；点击文件或文件库明确展开右栏。偏好按用户持久化。

## 阅读范围与边界

格式注册表在 `readers/formats.ts`，共享统一工具栏、主题、错误和尺寸限制。扩展名用于选择阅读器，原件仍经过服务端验证／转换和会话授权。

| 类别 | 格式 | 实际能力 |
| --- | --- | --- |
| 文本、Markdown | txt、log、md、markdown | Markdown 排版和原文切换；原文行号、搜索、换行、复制、分页 |
| 代码 | go、py、js、ts、tsx、jsx、java、rs、sql、sh、bash、css、scss、vue、svelte、c、h、cpp、hpp、rb、php、swift、kt、kts、r、lua、bat、ps1 | 对应语言高亮、搜索、复制；未知语法可回退原文 |
| 结构数据 | json、jsonl、yaml、yml、xml、toml、ini、conf、env | 原文高亮；除 ini/conf/env 外提供结构树，解析错误可回到原文 |
| 分隔表格 | csv、tsv | 真正的行列、表头、编码与分隔符选择、搜索、排序、列宽、分页 |
| 工作簿 | xlsx、xls、ods | 工作表切换、保存的格式值、合并单元格、PDF 版式切换 |
| 文档、幻灯片 | pdf、docx、doc、odt、rtf、pptx、ppt、odp | 原生 PDF 或 LibreOffice 转换，逐页缩放、缩略图、搜索并定位匹配页 |
| 图片 | png、jpg、jpeg、gif、webp、svg、bmp、avif | 缩放、平移、适应区域；SVG 限制静态元素，BMP/AVIF 转 PNG |
| 网页 | html、htm、含根 index.html 的 zip | 已授权的隔离页面、相对资源、源码切换 |
| 音频 | mp3、wav、ogg、m4a、flac、aac、opus | 统一转 MP3，播放器支持时间轴和音量 |
| 视频 | mp4、webm、mov、mkv、m4v | 统一转 H.264/AAC MP4，播放、时间轴、全屏 |
| 图与归档 | mmd、mermaid；zip、tar、gz、tgz、7z | 严格模式图示；安全目录、可直接支持的条目预览与下载，嵌套归档不递归展开 |

文本读取上限 4 MiB，工作簿 20 MiB；表格最多 20,000 行、200 列，每页 100 行，工作线程超时明确失败。原文每页 300 行，高亮限前 500,000 字符，超过部分保留纯文本。超限明确提示截断或下载，不宣称完整读取。原件始终可按原授权下载。

工作簿只阅读已有值，不运行宏或重新计算公式；复杂 Office 排版由转换质量决定，幻灯片动画和交互对象不执行。归档内 Office 不额外启动转换，提供原件下载；嵌套压缩包也下载。PSD/AI/FIG/Sketch/CAD/三维、TIFF/HEIC/EPUB 不在本次原生支持范围，可上传导出的 PDF／图片。

文件处理仍在独立持久预览任务中执行；转换服务增加 ffmpeg/libarchive，不引入外部文档 SaaS。派生媒体和条目沿用固定版本、授权、Range 请求和不可变原件。归档拒绝危险路径、链接、重复和展开超限；SVG 不注入应用 DOM。

## 确认卡片

卡片由结构化人类提及 `requiresResponse:true` 产生，支持可选 `confirmation.title`；人和岗位 AI 均可在群中提出。界面采用浅色标题、标题／状态、来源／回应人、正文与固定附件、底部分体操作栏。人类输入框可选择「请求确认」并指定人；不靠正文关键词推断。

指定接收人分别选择「同意此建议」「需要修改」「仅提意见」，可编辑意见；其他成员可阅读结果。每人只能提交一次；幂等重放返回原结果，并发重复拒绝。普通明确引用回复仍有效，记为 comment。回复、结构化 choice、通知、公共知识与 SSE 在同一事务登记。

意见不表示授权执行外部操作，也不恢复旧审批状态机。默认不唤醒岗位；编辑区明确勾选「同时 @ 岗位继续讨论」才附加目标岗位提及。只解除该人的等待分支，其他人仍等待。暂停时意见和提及正常保存，仍使用同一讨论且不给新额度；继续／终止仍由原讨论控制卡操作。已终止讨论的请求卡显示关闭，拒绝迟到的结构化选择。

CLI：`message send --confirmation 标题 --mention-human 用户ID` 创建；`--reply-to 消息ID --decision agree|changes|comment` 回应，仍可显式 `--mention-ai 岗位ID`。新增 MessageInput/Message 合同与岗位 `send_message` 工具同步，无独立审批接口。

## 验收入口

`tests/v2/readers.mjs` 在已有隔离 namespace 中生成真实文件、执行 HTTP／CLI／受控岗位和浏览器测试；`JUDEX_V2_READERS=1` 可加入完整 cluster 流程。需设置已构建的 `JUDEX_CONVERTER_IMAGE`、对应原生 CLI 和证据目录。真实执行与本机版本见后续 records，不能仅凭此设计说明判定验收通过。
