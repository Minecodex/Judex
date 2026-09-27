import {openTool} from './workspace-helpers';
import {test,expect} from '@playwright/test';
test('preview is explicit, HeroUI is loaded, and the chat context stays beside project tools',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));await page.goto('/');
 await expect(page.locator(".judex-preview-banner")).toHaveCount(0);await expect(page.getByText("AI 演示 · 未连接模型",{exact:true})).toBeVisible();await expect(page.locator('.button').first()).toBeVisible();
 await page.screenshot({path:test.info().outputPath('workspace.png'),fullPage:true});
 for(const view of ['plans','decisions','team','flows','resources']){await openTool(page,view);if(await page.getByTestId("settings-page").isVisible())await page.getByTestId("settings-back").click();await expect(page.locator('.judex-chat-panel-body')).not.toBeEmpty();await expect(page.getByTestId('chat-thread')).toBeVisible();}
 expect(errors).toEqual([]);
});
test('authentication form calls actual Gin API and never fakes account creation',async({page})=>{
 await page.goto('/register');await expect(page.getByTestId('backend-status')).toContainText('骨架 API 已连接');
 await page.getByRole('textbox',{name:/姓名/}).fill('测试用户');await page.getByRole('textbox',{name:/工作邮箱/}).fill('example@example.test');await page.getByLabel(/密码/).fill('temporary-test-password');
 const response=page.waitForResponse(r=>r.url().endsWith('/api/v1/auth/register'));await page.getByRole('button',{name:'提交',exact:true}).click();expect((await response).status()).toBe(501);await expect(page.getByRole('alert')).toContainText('尚未接入');await expect(page.getByLabel(/密码/)).toHaveValue('');
 expect(await page.evaluate(()=>JSON.stringify(localStorage))).not.toContain('temporary-test-password');
});
test('production web build served by Go never silently enables demo mode',async({page})=>{
 await page.goto('http://127.0.0.1:18080/projects/example');await expect(page.getByText('真实接口模式',{exact:false})).toBeVisible();await expect(page.locator('.judex-preview-banner')).toHaveCount(0);
 await page.screenshot({path:test.info().outputPath('api-entry.png'),fullPage:true});
 const system=await page.request.get('http://127.0.0.1:18080/api/v1/system');expect((await system.json()).capabilities.workspace).toBe(false);
 const unknown=await page.request.get('http://127.0.0.1:18080/api/v1/unknown');expect(unknown.status()).toBe(404);expect(unknown.headers()['content-type']).toContain('application/json');
});
