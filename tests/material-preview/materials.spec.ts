import {expect,test} from "@playwright/test";
import {mkdir} from "node:fs/promises";
import {resolve} from "node:path";
import {pathToFileURL} from "node:url";
const evidence=".cache/material-preview-20261008";
test.beforeEach(async({page})=>{await page.goto("/demo3.html");});
test("library filters, all eight content formats, purpose and provenance",async({page})=>{
 await expect(page.locator(".judex-mp-library-grid .judex-mp-file-card")).toHaveCount(8);
 await mkdir(evidence,{recursive:true});await page.setViewportSize({width:1440,height:1320});await page.screenshot({path:evidence+"/library-light.png"});await page.setViewportSize({width:1440,height:1000});
 await page.getByRole("button",{name:"表格",exact:true}).click();await expect(page.locator(".judex-mp-library-grid .judex-mp-file-card")).toHaveCount(1);
 await page.getByRole("button",{name:"全部文件",exact:true}).click();
 const expected:Record<string,string>={brief:"共享资料体验方案",requirements:"资料预览需求说明",checklist:"研发与验收清单",architecture:"同一份资料，贯穿每次讨论。",notes:"共享资料实现约定",review:"让资料回到工作中。",schema:'"materialId"',assets:"design-assets /"};
 for(const [id,content] of Object.entries(expected)){
  await page.locator(`.judex-mp-library-grid [data-file-id="${id}"] .judex-mp-cover-button`).click();
  await expect(page.getByRole("dialog")).toBeVisible();await expect(page.locator(".judex-mp-reader-canvas")).toContainText(content);
  if(id==="brief"){await page.getByRole("button",{name:"下一页",exact:true}).click();await expect(page.locator(".judex-mp-document h1")).toHaveText("材料用途与关联");await page.screenshot({path:evidence+"/preview-pdf.png"});}
  await page.getByRole("button",{name:"关闭",exact:true}).last().click();
 }
 await page.locator('.judex-mp-library-grid [data-file-id="brief"] .judex-mp-card-footer').getByRole("button",{name:/详情/}).click();
 const details=page.getByTestId("material-details");await expect(details).toContainText("这份材料用来做什么");await expect(details).toContainText("协同体验优化");await expect(details).toContainText("共享资料体验设计");await expect(details).toContainText("由 周屿 关联");await expect(details).toContainText("2026-10-08 10:05");
 await expect(page.getByRole("dialog")).toBeVisible();await expect(page.locator('.judex-dialog-container[data-entering]')).toHaveCount(0);await page.screenshot({path:evidence+"/details.png",animations:"disabled"});
});
test("discussion reuses file cards; sharing records usage; deletion preserves historical references",async({page})=>{
 await page.getByRole("navigation").getByRole("button",{name:"计划讨论",exact:true}).click();
 await expect(page.locator('.judex-mp-message-files [data-file-id="brief"]')).toHaveCount(1);await expect(page.locator('.judex-mp-panel-grid [data-file-id="brief"]')).toHaveCount(1);
 await page.screenshot({path:evidence+"/discussion-light.png"});
 await page.getByRole("button",{name:"添加共享资料",exact:true}).click();await page.getByRole("dialog").locator(".judex-mp-picker-card").filter({hasText:"研发与验收清单.xlsx"}).click();await expect(page.getByRole("checkbox",{name:/研发与验收清单.xlsx/})).toBeChecked();await page.getByRole("button",{name:"添加到消息",exact:true}).click();
 await page.getByRole("textbox",{name:"补充想法，或选择资料发送到讨论…"}).fill("补充验收清单，请确认预览与详情。");await page.getByRole("button",{name:"发送",exact:true}).click();
 await expect(page.locator('.judex-mp-message-files [data-file-id="checklist"]')).toHaveCount(1);
 await page.locator('.judex-mp-message-files [data-file-id="checklist"] .judex-mp-card-footer').getByRole("button",{name:/详情/}).click();await expect(page.getByTestId("material-details")).toContainText("在讨论中分享，供当前工作参考。");await page.getByRole("button",{name:"关闭",exact:true}).last().click();
 await page.locator('.judex-mp-panel-grid [data-file-id="checklist"] .judex-mp-card-footer').getByRole("button",{name:/删除/}).click();await expect(page.getByRole("dialog")).toContainText("1 条讨论消息");await page.getByRole("button",{name:"取消",exact:true}).click();
 await expect(page.locator('.judex-mp-panel-grid [data-file-id="checklist"]')).toHaveCount(1);
 await page.locator('.judex-mp-panel-grid [data-file-id="checklist"] .judex-mp-card-footer').getByRole("button",{name:/删除/}).click();await page.getByRole("button",{name:"确认删除",exact:true}).click();
 await expect(page.locator('.judex-mp-panel-grid [data-file-id="checklist"]')).toHaveCount(0);await expect(page.locator('.judex-mp-message-files [data-file-id="checklist"]')).toContainText("资料已删除");
 await page.getByRole("navigation").getByRole("button",{name:"共享资料",exact:true}).click();await expect(page.locator('.judex-mp-library-grid [data-file-id="checklist"]')).toHaveCount(0);
 await page.getByRole("button",{name:"重置示例",exact:true}).click();await expect(page.locator(".judex-mp-library-grid .judex-mp-file-card")).toHaveCount(8);
});
test("local text upload is previewable and downloadable; no API mutation",async({page})=>{
 const apiRequests:string[]=[];page.on("request",r=>{if(r.url().includes("/api/"))apiRequests.push(r.url());});
 await page.getByRole("button",{name:"上传资料",exact:true}).click();await page.locator('input[type="file"]').setInputFiles({name:"联调记录.txt",mimeType:"text/plain",buffer:Buffer.from("这是本地上传的真实文本内容。\npreview = shared")});
 await page.getByRole("textbox",{name:"说明材料用途",exact:true}).fill("用于任务联调核对。");await page.getByRole("button",{name:"加入共享资料",exact:true}).click();
 const file=page.getByRole("article",{name:"联调记录.txt",exact:true});await expect(file).toBeVisible();await file.locator(".judex-mp-cover-button").click();await expect(page.locator(".judex-mp-reader-canvas")).toContainText("这是本地上传的真实文本内容。");
 const downloading=page.waitForEvent("download");await page.getByRole("button",{name:"下载原文件",exact:true}).click();expect((await downloading).suggestedFilename()).toBe("联调记录.txt");expect(apiRequests).toEqual([]);
});
test("desktop light/dark and Chinese/English layouts, modal bounds, panel and draft preservation",async({page})=>{
 for(const width of [1120,1440,1920,3840]){
  await page.setViewportSize({width,height:1000});
  for(const locale of ["zh-CN","en"]){
   const lang=await page.locator("html").getAttribute("lang");if(lang!==locale)await page.locator(".judex-preference-controls button").first().click();
   for(const theme of ["light","dark"]){
    const current=await page.locator("html").getAttribute("data-theme");if(current!==theme)await page.locator(".judex-preference-controls button").last().click();
    for(const mode of ["library","plan","task"]){
     await page.goto(`/demo3.html#${mode}`);await expect(page.locator("html")).toHaveAttribute("data-theme",theme);
     expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
     const escaped=await page.locator(".judex-mp-file-card").evaluateAll(cards=>cards.filter(c=>{const box=c.getBoundingClientRect();return box.left<0||box.right>innerWidth;}).length);expect(escaped).toBe(0);
     if(mode==="library"){
       await page.locator('[data-file-id="brief"] .judex-mp-cover-button').click();await expect(page.locator('.judex-dialog-container[data-entering]')).toHaveCount(0);const modal=await page.getByRole("dialog").boundingBox();expect(modal!.x).toBeGreaterThanOrEqual(0);expect(modal!.x+modal!.width).toBeLessThanOrEqual(width);await page.getByRole("button",{name:locale==="en"?"Close":"关闭",exact:true}).last().click();
     }
    }
   }
  }
 }
 await page.setViewportSize({width:1440,height:1000});await page.goto("/demo3.html#task");await page.screenshot({path:evidence+"/discussion-dark-en.png"});
 await page.getByRole("textbox",{name:"Add a thought, or share a file in this discussion…",exact:true}).fill("Keep my draft.");await page.getByRole("button",{name:"Expand file panel",exact:true}).click();await expect(page.locator(".judex-mp-conversation")).toBeHidden();await page.getByRole("button",{name:"Restore file panel",exact:true}).click();await expect(page.getByRole("textbox",{name:"Add a thought, or share a file in this discussion…",exact:true})).toHaveValue("Keep my draft.");
 await page.getByRole("button",{name:"Hide file panel",exact:true}).first().click();await expect(page.locator(".judex-mp-shared-panel")).toHaveCount(0);await page.getByRole("button",{name:"Open shared files",exact:true}).click();await expect(page.locator(".judex-mp-shared-panel")).toBeVisible();
});
test("generated HTML opens directly offline without external assets",async({page})=>{
 const external:string[]=[];page.on("request",request=>{if(/^https?:/.test(request.url()))external.push(request.url());});
 await page.goto(pathToFileURL(resolve("demo3.html")).href);await expect(page.locator(".judex-mp-library-grid .judex-mp-file-card")).toHaveCount(8);
 await page.locator('[data-file-id="architecture"] .judex-mp-cover-button').click();await expect(page.getByRole("dialog")).toBeVisible();await expect(page.locator(".judex-mp-reader-canvas svg")).toBeVisible();expect(external).toEqual([]);
});
