// Approved scenario structure, used only to review actual model/UI results.
export const scenes={
 B:{folder:'B-VisualAssets',ownerSession:'KB01',title:'MiniIssue B 视觉资产闭环',workflowCount:2,taskCount:5,
  positions:[
   ['owner','设计负责人','明确MiniIssue视觉要求、来源与验收约束，核对设计/适配成果和整体目标；正式工作安排及验收仍由有权人确认。'],
   ['design','视觉设计师','制作PNG主视觉、SVG标识可编辑源、尺寸适配和透明素材；报告真实文件、版本、尺寸、透明度和实际看图证据，不以模型描述代替成品。'],
   ['brand','品牌审阅','独立看图核对品牌、可读性、尺寸、透明度与导出清单，明确返工和未覆盖项；只分析本项目固定原件。'],
  ],members:[['U2',['design']],['U3',['brand']]]},
 C:{folder:'C-OfficeReports',ownerSession:'KC01',title:'MiniIssue C 质量与汇报闭环',workflowCount:2,taskCount:5,
  positions:[
   ['owner','报告负责人','核对来源数据、XLSX/DOCX/PPTX/PDF的一致性、版面与范围；区分文件解析、实际重算和原生软件限制，整体验收由有权人确认。'],
   ['data','数据分析师','以固定原件制作多工作表XLSX和公式图表，实际重算并核对数据来源与结果；未知结果不填零，不把静态缓存当重算证明。'],
   ['office','Office制作','制作实际DOCX/PPTX/PDF并逐页检查版面和字体，依据已核对的数据，保留退回/修订与许可证或工具限制。'],
  ],members:[['U4',['data']],['U2',['office']]]},
};
