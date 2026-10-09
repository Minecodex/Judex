import { expect, test } from '@playwright/test';
import {assertVisibleSurfaceBorders} from './border-helpers';

for(const locale of ['zh-CN','en']) for(const theme of ['light','dark']) for(const width of [1120,1280,1440,1920,3840]) {
test(`shared visual system covers work/configuration/dialogs · ${locale}/${theme}/${width}`,async({page},info)=>{
  test.setTimeout(120000);
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  await page.goto('/');
  await expect.poll(()=>page.evaluate(()=>localStorage.getItem('judex.web.preview.v1'))).not.toBeNull();
  const fixture=await page.evaluate(()=>{
    const state=JSON.parse(localStorage.getItem('judex.web.preview.v1')!);
    const project=state.projects.find((p:any)=>p.id==='studio')??state.projects[0];
    const owner=project.members.find((m:any)=>m.role==='owner')??project.members[0];
    state.currentUser=owner.name;localStorage.setItem('judex.web.preview.v1',JSON.stringify(state));
    return {projectId:project.id,taskId:state.tasks.find((t:any)=>t.projectId===project.id)?.id,planId:state.plans.find((p:any)=>p.projectId===project.id)?.id,topicId:state.topics.find((t:any)=>t.projectId===project.id)?.id};
  });
  const views=[{name:'projects',path:'/'},...['plans','decisions','deliveries','materials'].map(tab=>({name:tab,path:'/projects/'+fixture.projectId+'?tab='+tab})),{name:'task',path:'/projects/'+fixture.projectId+'?tab=deliveries&view=task&item='+fixture.taskId},{name:'route',path:'/projects/'+fixture.projectId+'/plans/'+fixture.planId+'/route'},{name:'chat',path:'/projects/'+fixture.projectId+'/chat/'+fixture.topicId},...['project','team','flows','local'].map(section=>({name:section,path:'/projects/'+fixture.projectId+'/settings/'+section}))];
  {
    await page.setViewportSize({width,height:width===3840?1926:1000});
    await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
    for(const view of views) {
      await page.goto(view.path);
      await expect(page.locator(view.path.includes('/settings/') ? '.judex-settings-page' : '.judex-co-app')).toBeVisible();
      await page.evaluate(()=>document.fonts.ready);
      await expect(page.locator('.modal__backdrop[data-entering],.modal__backdrop[data-exiting]')).toHaveCount(0);

      for(const holder of await page.locator('.judex-empty-icon:visible').all()) {
        const centers=await holder.evaluate(element=>{const a=element.getBoundingClientRect(),b=element.querySelector('svg')!.getBoundingClientRect();return {dx:(a.x+a.width/2)-(b.x+b.width/2),dy:(a.y+a.height/2)-(b.y+b.height/2)};});
        expect(Math.abs(centers.dx)).toBeLessThanOrEqual(1);expect(Math.abs(centers.dy)).toBeLessThanOrEqual(1);
      }
      const geometry=await page.evaluate(()=>({width:document.documentElement.clientWidth,scrollWidth:document.documentElement.scrollWidth,lang:document.documentElement.lang,theme:document.documentElement.dataset.theme,
        fields:Array.from(document.querySelectorAll('.judex-chat-panel-body .input,.judex-chat-panel-body .textarea,.judex-settings-content .input,.judex-settings-content .select__trigger')).filter((field)=>field.getClientRects().length).map((field)=>({left:field.getBoundingClientRect().left,right:field.getBoundingClientRect().right}))}));
      expect(geometry.lang).toBe(locale);expect(geometry.theme).toBe(theme);
      expect(geometry.scrollWidth,`${view.name}/${width}/${locale}/${theme}`).toBeLessThanOrEqual(geometry.width);
      for(const field of geometry.fields) { expect(field.left).toBeGreaterThanOrEqual(0);expect(field.right,`Clipped field: ${view.name}/${width}`).toBeLessThanOrEqual(width); }
      await assertVisibleSurfaceBorders(page);
      if(width===1440) await page.screenshot({path:info.outputPath(`${view.name}-${locale}-${theme}.png`)});
    }
    await page.goto(`/projects/${fixture.projectId}/chat/${fixture.topicId}`);
    await page.getByTestId('new-discussion').click();
    const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();
    const box=await dialog.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(width);
    await page.keyboard.press('Escape');await expect(dialog).toBeHidden();
  }
  expect(errors).toEqual([]);
});

}
