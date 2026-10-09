import {test,expect} from '@playwright/test';
import {seedWork} from '../../web/src/features/work/seed';
import {ensureDemoMainTopics} from '../../web/src/features/chat/demoCollaboration';
import {assertActionGeometry,assertPreferenceGeometry,actionGeometry,preferenceGeometry,settleOverlays} from './ui-consistency-helpers';

for(const width of [1120,1440,1920,3840])for(const locale of ['zh-CN','en'])for(const theme of ['light','dark'])for(const scale of [1,1.25]){
test(`UI-C01 shared action sizes and private preference dialog · ${width}/${locale}/${theme}/${scale}`,async({browser},info)=>{
  const context=await browser.newContext({viewport:{width,height:width===3840?1926:1000},deviceScaleFactor:scale});
  try{
    const seed=ensureDemoMainTopics(seedWork());seed.currentUser='林然';
    await context.addInitScript(({seed,locale,theme})=>{
      localStorage.setItem('judex.web.preview.v1',JSON.stringify(seed));localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);
    },{seed,locale,theme});
    const page=await context.newPage(),errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
    const evidence:any={width,locale,theme,scale,views:[]};
    for(const [name,url,group,size] of [
      ['projects','/','.judex-co-project-card .card__footer',32],
      ['hub','/projects/leaf','.judex-co-page-heading .judex-co-actions',40],
      ['route','/projects/leaf/plans/leaf-first/route','.judex-co-page-heading .judex-co-actions',40],
      ['team','/projects/leaf/settings/team','.judex-page-actions',40],
      ['flows','/projects/leaf/settings/flows','.judex-page-actions',40],
      ['task','/projects/leaf?tab=deliveries&view=task&item=build','.judex-co-inspector-body > .judex-co-actions',32],
    ] as const){
      await page.goto(url);await expect(page.locator(group).first()).toBeVisible();await page.evaluate(()=>document.fonts.ready);await settleOverlays(page);
      const groups=page.locator(group);const geometry=[];
      for(const holder of await groups.all())geometry.push(await actionGeometry(holder));
      evidence.views.push({name,geometry});await info.attach(name+'-before-assertion',{body:JSON.stringify(geometry),contentType:'application/json'});
      for(const holder of await groups.all())await assertActionGeometry(holder,size);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth)).toBe(true);
    }
    await page.getByTestId('simulate-local-report').click();
    await expect(page.getByTestId('collaboration-report-body')).toBeVisible();
    await settleOverlays(page);
    evidence.reportActions=await assertActionGeometry(page.locator('.judex-collab-dialog-actions').filter({visible:true}),40);
    await page.goto('/projects/leaf/settings/team');
    await page.locator('[data-testid^="position-preferences-"]').first().click();
    await expect(page.getByTestId('next-personal-prompt')).toBeVisible();await page.evaluate(()=>document.fonts.ready);
    evidence.preferences=await preferenceGeometry(page);await info.attach('preferences-before-assertion',{body:JSON.stringify(evidence.preferences),contentType:'application/json'});
    await assertPreferenceGeometry(page);
    await page.getByTestId('next-personal-prompt').fill('UI-C01 private draft');
    await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);
    await page.locator('[data-testid^="position-preferences-"]').first().click();
    await expect(page.getByTestId('next-personal-prompt')).toHaveValue('UI-C01 private draft');
    await assertPreferenceGeometry(page);expect(errors).toEqual([]);
    if(width===1440&&scale===1){await page.screenshot({path:info.outputPath(`preferences-${locale}-${theme}.png`)});}
    await info.attach('geometry',{body:JSON.stringify(evidence),contentType:'application/json'});
  }finally{await context.close();}
});
}


test('UI-C02 private preference dialog has one title and a full width editor',async({page},info)=>{
  const seed=ensureDemoMainTopics(seedWork());seed.currentUser='林然';
  await page.addInitScript(seed=>{localStorage.setItem('judex.web.preview.v1',JSON.stringify(seed));},seed);
  await page.goto('/projects/leaf/settings/team');
  await page.locator('[data-testid^="position-preferences-"]').first().click();
  await expect(page.getByTestId('next-personal-prompt')).toBeVisible();
  await info.attach('before-assertion',{body:JSON.stringify(await preferenceGeometry(page)),contentType:'application/json'});
  await assertPreferenceGeometry(page);
});
