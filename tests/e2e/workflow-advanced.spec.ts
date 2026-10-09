import {test,expect} from '@playwright/test';
import fs from 'node:fs';
import {chooseValue,selectPerson} from './workspace-helpers';
const catalog=JSON.parse(fs.readFileSync('internal/project/catalog/workflow-presets.json','utf8'));
const advanced=catalog.workflows.filter((p:{complexity:string})=>p.complexity==='advanced');

test('all researched full workflows render review gates, stage groups, labeled returns and reference links',async({page},info)=>{
  test.setTimeout(180000);
  await page.goto('/?project=leaf');await selectPerson(page,'林然');await page.goto('/?project=leaf&settings=flows');
  await page.getByTestId('open-workflow-presets').click();
  let current='';
  for(const preset of advanced){
    if(current!==preset.scenarioIds[0]){current=preset.scenarioIds[0];await chooseValue(page,'workflow-preset-scenario',current);}
    const previewButton=page.getByTestId('preview-workflow-'+preset.id);
    await expect(previewButton).toBeVisible();await previewButton.click();
    const preview=page.getByTestId('workflow-preset-full-preview');
    await expect(preview.locator('svg').first()).toBeVisible();
    await expect(preview.locator('g.node')).toHaveCount(preset.nodes.length);
    expect(await preview.locator('g.cluster').count()).toBeGreaterThanOrEqual(6);
    await expect(preview.getByTestId('workflow-source-links').locator('a').first()).toHaveAttribute('href',/^https:/);
    await page.getByTestId('workflow-natural-size').click();
    await expect(page.getByTestId('workflow-preview-viewport')).toHaveCSS('--diagram-scale','1');
    const font=await preview.locator('g.node .nodeLabel').first().evaluate(el=>el.getBoundingClientRect().height);
    expect(font).toBeGreaterThanOrEqual(12);
    if(preset.id==='software-delivery-advanced'){
      await page.screenshot({path:info.outputPath('advanced-software-natural.png'),fullPage:true});
      const viewport=page.getByTestId('workflow-preview-viewport'),rect=await viewport.boundingBox();
      await page.mouse.move(rect!.x+rect!.width/2,rect!.y+rect!.height/2);await page.mouse.down();
      await page.mouse.move(rect!.x+rect!.width/2+80,rect!.y+rect!.height/2+40);await page.mouse.up();
      await expect(viewport).toHaveCSS('--diagram-pan-x','80px');
      await page.getByRole('button',{name:'适应画布',exact:true}).click();
      await page.screenshot({path:info.outputPath('advanced-software-fit.png'),fullPage:true});
      await page.getByRole('button',{name:'纵向查看',exact:true}).click();
      await expect(preview.locator('svg').first()).toBeVisible();
    }
    await page.getByTestId('workflow-preview-back').click();
  }
});

test('full workflow sources, fit controls and fixed actions support both languages and themes on desktop',async({page},info)=>{
  test.setTimeout(120000);
  await page.goto('/?project=leaf');await selectPerson(page,'林然');
  for(const locale of ['zh-CN','en'])for(const theme of ['light','dark'])for(const width of [1120,1440,1920]){
    await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
    await page.setViewportSize({width,height:1000});await page.goto('/?project=leaf&settings=flows');
    await page.getByTestId('open-workflow-presets').click();await chooseValue(page,'workflow-preset-scenario','software');
    await page.getByTestId('preview-workflow-software-delivery-advanced').click();
    await expect(page.getByTestId('workflow-preset-full-preview').locator('svg').first()).toBeVisible();
    const bounds=await page.getByRole('dialog').evaluate(el=>{
      const r=el.getBoundingClientRect(),footer=el.querySelector('.judex-dialog-footer')!.getBoundingClientRect();
      return {width:el.clientWidth,scroll:el.scrollWidth,left:r.left,right:r.right,top:r.top,bottom:r.bottom,footer:footer.bottom,viewport:innerHeight};
    });
    expect(bounds.scroll).toBeLessThanOrEqual(bounds.width);expect(bounds.left).toBeGreaterThanOrEqual(0);
    expect(bounds.right).toBeLessThanOrEqual(width);expect(bounds.bottom).toBeLessThanOrEqual(bounds.viewport+1);
    expect(bounds.footer).toBeLessThanOrEqual(bounds.viewport+1);
    await page.getByTestId('workflow-natural-size').click();
    await expect(page.getByTestId('workflow-preview-viewport')).toHaveCSS('--diagram-scale','1');
    if(width===1440)await page.screenshot({path:info.outputPath('advanced-'+locale+'-'+theme+'.png'),fullPage:true});
    await page.getByTestId('workflow-preview-back').click();await page.keyboard.press('Escape');
  }
});
