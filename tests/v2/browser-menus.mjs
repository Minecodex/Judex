import {expect} from '@playwright/test';
import {evidencePath} from './environment.mjs';

export async function runMenuStylingFlow(page,{rootURL}){
 const checks=[],button=(name,scope=page)=>scope.getByRole('button',{name,exact:true});
 await page.goto(rootURL);await expect(button('会话菜单')).toBeVisible();
 const inspect=async(item,panel,keyboard)=>{
  await expect(item).toBeVisible();
  await expect.poll(()=>item.evaluate(e=>getComputedStyle(e).outlineStyle)).toBe('none');
  if(keyboard){await expect(item).toHaveAttribute('data-focus-visible','true');await expect.poll(()=>item.evaluate(e=>getComputedStyle(e).boxShadow)).toContain('inset');}
  else await expect.poll(()=>item.evaluate(e=>getComputedStyle(e).boxShadow)).toBe('none');
  const row=await item.boundingBox(),box=await panel.boundingBox();expect(row.x).toBeGreaterThan(box.x);expect(row.x+row.width).toBeLessThan(box.x+box.width);
 };
 for(const theme of ['light','dark']){
  if(theme==='dark'){await button('账号与偏好').click();await page.getByRole('menuitem',{name:'切换深浅配色',exact:true}).click();}
  await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
  await button('会话菜单').click();let panel=page.locator('.judex-v2-action-popover'),row=page.getByRole('menuitem',{name:'群公共知识',exact:true});await row.hover();await inspect(row,panel,false);
  await panel.screenshot({path:evidencePath(`menu-${theme}-pointer.png`),animations:'disabled'});await page.keyboard.press('Escape');await expect(panel).toHaveCount(0);
  await button('会话菜单').press('ArrowDown');await expect(panel.getByRole('menuitem').first()).toBeFocused();await page.keyboard.press('ArrowDown');row=panel.locator('[data-focus-visible=true]').first();await inspect(row,panel,true);await panel.screenshot({path:evidencePath(`menu-${theme}-keyboard.png`),animations:'disabled'});await page.keyboard.press('Escape');await expect(panel).toHaveCount(0);await expect(button('会话菜单')).toBeFocused();checks.push(`${theme}-menu-hover-no-external-border`,`${theme}-keyboard-inset-focus-and-escape`);
  await button('账号与偏好').click();await page.getByRole('menuitem',{name:'账号与偏好',exact:true}).click();const dialog=page.getByRole('dialog',{name:'账号与偏好',exact:true}),select=dialog.locator('.select__trigger').first();await select.click();panel=page.locator('.judex-v2-choice-popover');row=panel.getByRole('option').first();await row.hover();await inspect(row,panel,false);await page.keyboard.press('ArrowDown');row=panel.locator('[data-focus-visible=true]').first();await inspect(row,panel,true);await panel.screenshot({path:evidencePath(`select-${theme}-keyboard.png`),animations:'disabled'});await page.keyboard.press('Escape');await expect(panel).toHaveCount(0);await expect(select).toBeFocused();await button('关闭',dialog).click();checks.push(`${theme}-select-matches-menu-and-keeps-keyboard-selection`);
 }
 await button('账号与偏好').click();await page.getByRole('menuitem',{name:'切换深浅配色',exact:true}).click();
 await button('群聊文件').click();await button('切换文件版本 · acceptance.md').click();let panel=page.locator('.judex-v2-action-popover');const version=panel.getByRole('menuitemradio').first();await version.hover();await inspect(version,panel,false);await page.keyboard.press('Escape');await expect(panel).toHaveCount(0);await button('切换文件版本 · acceptance.md').press('ArrowDown');await expect(version).toBeFocused();await inspect(version,panel,true);await page.keyboard.press('Enter');await expect(panel).toBeHidden();await button('关闭阅读区').click();checks.push('file-version-menu-shares-focus-treatment-and-enter-selection');
 return {status:'passed',checks};
}
