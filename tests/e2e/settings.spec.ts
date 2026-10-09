import {test,expect} from '@playwright/test';
import {openChat,demoState} from './cooperation-helpers';
import {openAccount,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
test('top account owns appearance and full settings return preserves chat and private drafts',async({page})=>{
 await openChat(page);await selectPerson(page,'林然');await expect(page.getByTestId('new-discussion')).toHaveText('新建会话');await expect(page.locator('.judex-chat-sidebar [data-testid=account-menu-button]')).toHaveCount(0);await page.getByTestId('work-discussion-input').fill('返回后仍保留');await toggleTheme(page);await expect(page.locator('html')).toHaveAttribute('data-theme','dark');await openSettings(page,'preferences');await page.getByTestId('next-personal-prompt').fill('只属于本人职位的草稿');await page.keyboard.press('Escape');await page.getByTestId('settings-back').click();await expect(page.getByTestId('work-discussion-input')).toHaveValue('返回后仍保留');await openSettings(page,'preferences');await expect(page.getByTestId('next-personal-prompt')).toHaveValue('只属于本人职位的草稿');await page.reload();await expect(page.getByTestId('next-personal-prompt')).toHaveValue('只属于本人职位的草稿');await expect(page.getByTestId('settings-section-preferences')).toHaveCount(0);
});
test('project card invitation requires management and stays separate from workspace tabs',async({page})=>{
 await page.goto('/');await selectPerson(page,'林然');const card=page.locator('.judex-co-project-card').filter({has:page.getByTestId('project-enter-leaf')});await card.getByRole('button',{name:'邀请用户',exact:true}).click();await page.getByTestId('next-invite-name').fill('新伙伴');await page.getByTestId('invite-position-build-role').click();await page.getByTestId('next-create-invite').click();expect((await demoState(page)).invites.some((v:any)=>v.person==='新伙伴')).toBe(true);await page.keyboard.press('Escape');await card.getByTestId('project-enter-leaf').click();await openSettings(page,'project');await expect(page.getByTestId('project-discussion-limit')).toBeEnabled();await page.getByTestId('settings-back').click();await selectPerson(page,'顾言');await page.locator('.judex-co-brand').click();await expect(page.locator('.judex-co-project-card').filter({has:page.getByTestId('project-enter-leaf')}).getByRole('button',{name:'邀请用户',exact:true})).toBeDisabled();await expect(page.getByTestId('workspace-tab-settings:project')).toHaveCount(0);
});
test('leaving preview retains work and reload does not silently sign in again',async({page})=>{
 await page.goto('/');await expect(page.getByTestId('account-menu-button')).toBeVisible();const before=await page.evaluate(()=>localStorage.getItem('judex.web.preview.v1'));await openAccount(page);await page.getByTestId('account-logout').click();await expect(page.getByText('已退出交互预览',{exact:true})).toBeVisible();await page.reload();await expect(page.getByText('已退出交互预览',{exact:true})).toBeVisible();expect(await page.evaluate(()=>localStorage.getItem('judex.web.preview.v1'))).toBe(before);await page.getByTestId('return-preview').click();await expect(page.getByTestId('account-menu-button')).toBeVisible();
});
test('nested account selection releases focus and pointer input after Escape',async({page})=>{
 // Slow frames exercise the asynchronous focus restoration of nested overlays.
 await page.addInitScript(()=>{
  const request=window.requestAnimationFrame.bind(window),cancel=window.cancelAnimationFrame.bind(window);
  let next=0;const pending=new Map<number,{timer:number;frame?:number}>();
  window.requestAnimationFrame=callback=>{
   const id=++next,entry:{timer:number;frame?:number}={timer:0};
   entry.timer=window.setTimeout(()=>{entry.frame=request(time=>{pending.delete(id);callback(time);});},80);
   pending.set(id,entry);return id;
  };
  window.cancelAnimationFrame=id=>{
   const entry=pending.get(id);if(!entry)return;
   window.clearTimeout(entry.timer);if(entry.frame!==undefined)cancel(entry.frame);pending.delete(id);
  };
 });
 await openChat(page);
 await page.addStyleTag({content:'.judex-ui-select-popover[data-exiting="true"] { animation-duration: 600ms !important; }'});
 await selectPerson(page,'林然');
 const account=page.getByTestId('account-menu-button');
 await expect(account).toHaveAttribute('aria-expanded','false');
 await expect(account).toBeFocused();
 await expect(page.locator('.judex-account-popover')).toHaveCount(0);
 await page.getByTestId('conversation-menu').click();
 await expect(page.getByTestId('conversation-project-settings')).toBeVisible();
});
