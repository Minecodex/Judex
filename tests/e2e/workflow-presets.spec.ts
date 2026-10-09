import {test,expect} from '@playwright/test';
import {chooseValue,openTool,selectPerson} from './workspace-helpers';

test('workflow preset chooser previews independently, creates editable drafts, publishes and prevents duplicate imports',async({page},info)=>{
  await page.goto('/?project=leaf');
  await selectPerson(page,'林然');
  await openTool(page,'flows');
  await page.getByTestId('open-workflow-presets').click();
  await expect(page.getByTestId('import-workflow-presets')).toBeDisabled();
  await chooseValue(page,'workflow-preset-scenario','software');
  await expect(page.locator('.judex-workflow-choice')).toHaveCount(7);
  await expect(page.locator('.judex-workflow-thumbnail svg')).toHaveCount(7);
  await page.getByTestId('workflow-preset-card-software-bug').click();
  await page.getByTestId('preview-workflow-software-standard').click();
  await expect(page.getByTestId('workflow-preset-full-preview')).toBeVisible();
  await expect(page.getByTestId('workflow-preset-full-preview').locator('svg').first()).toBeVisible();
  await page.getByRole('button',{name:'放大流程',exact:true}).click();
  await expect(page.getByTestId('workflow-preview-viewport')).toHaveCSS('--diagram-scale','1.25');
  await page.getByRole('button',{name:'适应画布',exact:true}).click();
  await page.screenshot({path:info.outputPath('workflow-full-preview.png'),fullPage:true});
  await page.getByTestId('workflow-preview-back').click();
  await expect(page.getByTestId('workflow-preset-selection-count')).toHaveText('已选择 1 个流程');
  await page.getByTestId('workflow-preset-card-software-standard').click();
  await page.getByTestId('import-workflow-presets').click();
  await expect(page.getByTestId('workflow-draft-editor')).toBeVisible();
  await page.getByRole('button',{name:'标准软件研发',exact:true}).click();
  await page.getByTestId('workflow-draft-name').fill('定制研发流程');
  await page.getByRole('button',{name:/1\.\s*需求确认/}).click();
  await page.getByTestId('workflow-node-name-step1').fill('本项目需求确认');
  await expect(page.getByTestId('publish-workflow-draft')).toBeDisabled();
  await page.getByTestId('save-workflow-draft').click();
  await expect(page.getByTestId('publish-workflow-draft')).toBeEnabled();
  await page.reload();
  await expect(page.getByTestId('workflow-draft-name')).toHaveValue('定制研发流程');
  await page.getByTestId('publish-workflow-draft').click();
  await expect(page.getByTestId('workflow-draft-editor')).toHaveCount(0);
  await expect(page.locator('.judex-flow-title')).toContainText('已发布');
  await page.getByTestId('open-workflow-presets').click();
  await chooseValue(page,'workflow-preset-scenario','software');
  await expect(page.getByTestId('workflow-preset-card-software-standard').getByRole('checkbox')).toBeDisabled();
  await expect(page.getByTestId('workflow-preset-card-software-bug').getByRole('checkbox')).toBeDisabled();
  await page.getByTestId('workflow-preset-card-software-release').click();
  await chooseValue(page,'workflow-preset-scenario','publishing');
  await expect(page.getByTestId('workflow-preset-selection-count')).toHaveText('已选择 0 个流程');
  await page.getByTestId('workflow-preset-select-all').click();
  await expect(page.getByTestId('workflow-preset-selection-count')).toHaveText('已选择 4 个流程');
});

test('workflow template dialog and full-screen preview fit the desktop matrix in both languages and themes',async({page},info)=>{
  test.setTimeout(120000);
  await page.goto('/?project=leaf');await selectPerson(page,'林然');
  for(const locale of ['zh-CN','en'])for(const theme of ['light','dark'])for(const width of [1120,1280,1440,1920,3840]){
    await page.evaluate(({locale,theme})=>{localStorage.setItem('argus.locale',locale);localStorage.setItem('judex.theme',theme);},{locale,theme});
    await page.setViewportSize({width,height:width===3840?1926:900});
    await page.goto('/?project=leaf&settings=flows');
    await page.getByTestId('open-workflow-presets').click();await chooseValue(page,'workflow-preset-scenario','software');
    await expect(page.locator('.judex-workflow-thumbnail svg')).toHaveCount(7);
    const inspect=()=>page.getByRole('dialog').evaluate(el=>{
      const r=el.getBoundingClientRect(),footer=el.querySelector('.judex-dialog-footer')!.getBoundingClientRect();
      return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:el.clientWidth,scroll:el.scrollWidth,
        viewportWidth:innerWidth,viewportHeight:innerHeight,footerBottom:footer.bottom};
    });
    const dialog=await inspect();
    expect(dialog.left).toBeGreaterThanOrEqual(0);expect(dialog.right).toBeLessThanOrEqual(width);
    expect(dialog.scroll).toBeLessThanOrEqual(dialog.width);expect(dialog.footerBottom).toBeLessThanOrEqual(dialog.viewportHeight);
    if(width===1440)await page.screenshot({path:info.outputPath('workflow-templates-'+locale+'-'+theme+'.png'),fullPage:true});
    await page.getByTestId('preview-workflow-software-standard').click();
    await expect(page.getByTestId('workflow-preset-full-preview')).toBeVisible();
    const preview=await inspect();
    expect(preview.right-preview.left).toBeGreaterThanOrEqual(width-2);
    expect(preview.top).toBeGreaterThanOrEqual(0);expect(preview.bottom).toBeLessThanOrEqual(preview.viewportHeight+1);
    expect(preview.scroll).toBeLessThanOrEqual(preview.width);expect(preview.footerBottom).toBeLessThanOrEqual(preview.viewportHeight+1);
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('workflow-presets-dialog')).toBeVisible();
    await page.keyboard.press('Escape');
  }
});
