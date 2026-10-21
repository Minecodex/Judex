import {expect} from '@playwright/test';
import {evidencePath} from './environment.mjs';

export async function runPaneNavigationFlow(page,{rootURL}){
 const checks=[],button=(name,scope=page)=>scope.getByRole('button',{name,exact:true});
 const checkToggleIdentity=async(theme)=>{
  const shapes=[];
  for(const region of ['list','chat','reader']){
   const toggle=()=>page.locator(`.judex-pane-toggle[data-pane-target="${region}"]:visible`);
   await expect(toggle()).toHaveAttribute('aria-pressed','true');
   const icon=await toggle().locator('svg').innerHTML(),active=await toggle().evaluate(e=>getComputedStyle(e).backgroundColor);shapes.push(icon);
   await toggle().click();await expect(page.locator(`[data-pane="${region}"]`)).toBeHidden();await page.mouse.move(0,0);
   await expect(toggle()).toHaveAttribute('aria-pressed','false');expect(await toggle().locator('svg').innerHTML()).toBe(icon);
   await expect.poll(()=>toggle().evaluate(e=>getComputedStyle(e).backgroundColor)).not.toBe(active);
   await page.screenshot({path:evidencePath(`pane-toggle-${theme}-${region}.png`),animations:'disabled'});
   await toggle().focus();await page.keyboard.press('Enter');await expect(toggle()).toHaveAttribute('aria-pressed','true');
   expect(await toggle().locator('svg').innerHTML()).toBe(icon);
  }
  expect(new Set(shapes).size).toBe(3);checks.push(`${theme}-distinct-pane-icons-stable-across-selected-unselected-and-keyboard-toggle`);
 };
 const pane=name=>page.locator(`[data-pane="${name}"]`),controls=name=>page.locator(`[data-pane-controls="${name}"]`);
 await page.goto(rootURL);await expect(page.locator('.judex-v2-chat-title')).toBeVisible();
 await expect(button('布局')).toHaveCount(0);
 await expect(button('收起群聊',controls('list'))).toBeVisible();await expect(button('收起聊天',controls('chat'))).toBeVisible();
 checks.push('layout-controls-live-in-pane-headers-without-rail-menu');
 const composer=page.getByRole('textbox',{name:'聊聊你的想法，或 @ 一位伙伴…',exact:true});await composer.fill('布局与文件往返保留的草稿');
 await button('群聊文件').click();const library=page.locator('.judex-v2-resource-panel'),search=library.getByRole('textbox',{name:'搜索文件…',exact:true});
 await library.locator('input[type=file][multiple]').setInputFiles(Array.from({length:6},(_,i)=>({name:`navigation-${i+1}.md`,mimeType:'text/markdown',buffer:Buffer.from(`# Navigation ${i+1}\n\nReal uploaded file for pane navigation.`)})));
 await search.fill('navigation-');await expect(library.locator('.judex-library-card')).toHaveCount(6);
 const last=library.locator('.judex-library-open').last();await last.scrollIntoViewIfNeeded();
 const scroll=await library.locator('.judex-v2-resource-content').evaluate(e=>e.scrollTop);expect(scroll).toBeGreaterThan(0);
 await last.click();await expect(page.locator('.judex-reader-document')).toContainText('Real uploaded file');
 await checkToggleIdentity('light');
 await page.screenshot({path:evidencePath('pane-preview-light.png'),animations:'disabled'});
 await button('返回文件列表').click();await expect(search).toHaveValue('navigation-');
 await expect.poll(()=>library.locator('.judex-v2-resource-content').evaluate((e,previous)=>Math.abs(e.scrollTop-previous),scroll)).toBeLessThan(3);
 await expect(composer).toHaveValue('布局与文件往返保留的草稿');checks.push('reader-back-preserves-library-filter-scroll-and-message-draft');
 await search.fill('acceptance.md');await library.locator('.judex-library-open').click();
 await button('展开阅读').click();await expect(pane('chat')).toBeHidden();await button('返回文件列表').click();await expect(library).toBeVisible();await expect(pane('chat')).toBeVisible();checks.push('expanded-preview-back-restores-side-by-side-library');
 await button('收起群聊').click();await expect(pane('list')).toBeHidden();await expect(button('显示群聊',controls('chat'))).toBeVisible();
 await button('收起聊天').click();await expect(pane('chat')).toBeHidden();await expect(button('显示群聊',controls('reader'))).toBeVisible();await expect(button('显示聊天',controls('reader'))).toBeVisible();
 await button('显示聊天').click();await button('显示群聊').click();await expect(pane('list')).toBeVisible();await expect(composer).toHaveValue('布局与文件往返保留的草稿');checks.push('single-visible-pane-can-restore-both-neighbours');
 await button('关闭阅读区').click();await expect(button('显示内容预览',controls('chat'))).toBeVisible();await button('显示内容预览').click();await expect(library).toBeVisible();checks.push('reader-header-hide-and-neighbour-restore');
 const divider=page.locator('.judex-pane-divider').first(),box=await divider.boundingBox();await page.mouse.move(box.x,box.y+150);await page.mouse.down();await page.mouse.move(62,box.y+150,{steps:12});await page.mouse.up();await expect(pane('list')).toBeHidden();await button('显示群聊').click();await expect(pane('list')).toBeVisible();checks.push('drag-collapse-restores-from-neighbour-header');
 await library.locator('.judex-library-open').click();await button('关闭阅读区').click();await button('显示内容预览').click();await expect(page.locator('.judex-v2-reader-header')).toContainText('acceptance.md');checks.push('hide-restore-retains-selected-file');
 await button('账号与偏好').click();await page.getByRole('menuitem',{name:'切换深浅配色',exact:true}).click();await button('账号与偏好').click();await page.getByRole('menuitem',{name:'切换语言',exact:true}).click();
 await expect(button('Back to files')).toBeVisible();await checkToggleIdentity('dark');await button('Hide Groups').click();await expect(button('Show Groups',controls('chat'))).toBeVisible();await page.screenshot({path:evidencePath('pane-preview-dark-en.png'),animations:'disabled'});await button('Show Groups').click();await button('Back to files').click();checks.push('dark-english-back-and-local-pane-controls');
 await button('Account & preferences').click();await page.getByRole('menuitem',{name:'Switch language',exact:true}).click();await button('账号与偏好').click();await page.getByRole('menuitem',{name:'切换深浅配色',exact:true}).click();
 await button('关闭阅读区').click();await composer.fill('');
 await page.locator('.judex-v2-rail').getByRole('button',{name:'聊天',exact:true}).click();await expect(page.locator('.judex-v2-chat-empty')).toBeVisible();
 await button('收起会话列表').click();await expect(button('显示会话列表',controls('chat'))).toBeVisible();await button('显示会话列表').click();await expect(pane('list')).toBeVisible();checks.push('empty-private-chat-retains-list-restore-control');
 await page.goto(rootURL);await expect(page.locator('.judex-v2-chat-title')).toBeVisible();
 return {status:'passed',checks};
}
