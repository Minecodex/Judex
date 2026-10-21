// Shared desktop regression for the Kylon-inspired navigation. All effects use
// visible controls; API fixtures and provider assertions are supplied separately.
export async function runWorkspaceChromeFlow(page,{rootURL,fileName='acceptance.md'}){
 const checks=[],button=(name,scope=page)=>scope.getByRole('button',{name,exact:true}),visible=loc=>loc.waitFor({state:'visible',timeoutMs:30000});
 await page.goto(rootURL);await visible(page.locator('.judex-v2-chat-title'));
 if(await page.getByRole('textbox',{name:'搜索对话消息…',exact:true}).count())throw new Error('Message search should open on request');
 await button('会话菜单').click();await page.getByRole('menuitem',{name:'搜索对话消息…',exact:true}).click();const search=page.getByRole('textbox',{name:'搜索对话消息…',exact:true});await visible(search);await search.fill('nonexistent-ui-query');await button('关闭',page.locator('.judex-v2-chat-search')).click();if(await search.count())throw new Error('Search did not close');checks.push('conversation-menu-search-clear');
 const draft='Draft survives resource browsing',composer=page.getByRole('textbox',{name:'聊聊你的想法，或 @ 一位伙伴…',exact:true});await composer.fill(draft);
 await button('群聊文件').click();const library=page.getByRole('complementary',{name:'群聊文件',exact:true});await visible(library);if(await page.getByRole('dialog').count())throw new Error('File browsing must keep the conversation available');
 const fileSearch=library.getByRole('textbox',{name:'搜索文件…',exact:true});await fileSearch.fill('not-a-file');await visible(library.getByText('这里暂时没有内容',{exact:true}));await fileSearch.fill('');await library.getByRole('button').filter({hasText:fileName}).first().click();await visible(page.locator('.judex-v2-reader-paper'));
 if(await composer.evaluate(e=>e.value)!==draft)throw new Error('Resource browsing lost the draft');checks.push('inline-file-library','file-search-empty-and-reset','resource-browsing-preserves-draft');
 await button('版本记录').click();await visible(page.locator('.judex-v2-version-history'));await button('版本记录').click();await button('关闭阅读区').click();await composer.fill('');checks.push('reader-history-and-close');
 await button('会话菜单').click();await page.getByRole('menuitem',{name:'群公共知识',exact:true}).click();await visible(page.getByRole('dialog',{name:'群公共知识',exact:true}));await button('关闭',page.getByRole('dialog')).click();checks.push('knowledge-reachable-from-menu');
 const account=async(name,item)=>{await button(name).click();await page.getByRole('menuitem',{name:item,exact:true}).click();};
 await account('账号与偏好','切换深浅配色');if(await page.locator('html').getAttribute('data-theme')!=='dark')throw new Error('Theme mismatch');await account('账号与偏好','切换语言');await visible(button('Conversation menu'));await account('Account & preferences','Switch language');await account('账号与偏好','切换深浅配色');checks.push('account-menu-bilingual-theme');
 const body=await page.locator('.judex-v2-message-bubble').first().evaluate(e=>({size:+getComputedStyle(e).fontSize.replace('px',''),ink:getComputedStyle(e).color}));if(body.size<14.4)throw new Error('Chat type scale regressed');const frame=await page.locator('.judex-v2-frame').evaluate(e=>({width:e.clientWidth,scroll:e.scrollWidth}));if(frame.scroll>frame.width+1)throw new Error('Workspace horizontal overflow');checks.push('readable-chat-type','desktop-layout-no-overflow');
 return {status:'passed',checks};
}
