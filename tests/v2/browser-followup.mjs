const button=(page,name)=>{const locator=page.getByRole('button',{name,exact:true});return name==='关闭'?locator.first():locator;};
const dialog=(page,name)=>page.getByRole('dialog',{name,exact:true});
const visible=async locator=>{await locator.waitFor({state:'visible',timeoutMs:30000});};
async function equalsValue(locator,want){const end=Date.now()+30000;let value;do{value=await locator.evaluate(el=>el.value);if(value===want)return;await new Promise(r=>setTimeout(r,100));}while(Date.now()<end);throw new Error('Expected form value: '+want+'; received '+value);}

export async function runConcurrencyFlow(page,other,{url,peerURL,rootURL,peerEmail,password,controls}){
 const meta=await controls.setup(rootURL),checks=[];
 await page.goto(rootURL);await visible(page.locator('.judex-v2-chat-title'));await page.locator('.judex-v2-chat-title').click();
 const form=dialog(page,'群聊设置');await form.getByRole('textbox',{name:'群目的',exact:true}).fill('My retained group draft');
 await other.goto(peerURL+'/login');await other.locator('input[name="email"]').fill(peerEmail);await other.locator('input[name="password"]').fill(password);await button(other,'登录').click();await visible(other.getByRole('heading',{name:'把想法，聊成可能。',exact:true}));
 await other.goto(peerURL+'/?project='+meta.projectId+'&group='+meta.groupId);await visible(other.locator('.judex-v2-chat-title'));await other.locator('.judex-v2-chat-title').click();
 const remote=dialog(other,'群聊设置');await remote.getByRole('textbox',{name:'群目的',exact:true}).fill('Remote group purpose');await remote.getByRole('button',{name:/成员与任职/}).click();await remote.getByRole('checkbox',{name:meta.thirdName,exact:true}).press('Space');await button(remote,'保存修改').click();await remote.waitFor({state:'hidden'});
 await visible(form.getByText('其他人已更新这些设置。你的编辑已保留，保存时会检查原版本。',{exact:true}));await equalsValue(form.getByRole('textbox',{name:'群目的',exact:true}),'My retained group draft');await button(form,'保存修改').click();await visible(page.getByText('内容已被其他人修改。你的编辑已保留，请重新载入最新内容后重试。',{exact:true}));await controls.groupStayedRemote();
 await button(form,'重新载入最新内容并替换当前编辑').click();await equalsValue(form.getByRole('textbox',{name:'群目的',exact:true}),'Remote group purpose');await form.getByRole('button',{name:/成员与任职/}).click();if(await form.getByRole('checkbox',{name:meta.thirdName,exact:true}).getAttribute('data-selected')==='true')throw new Error('Removed colleague restored by reload');await button(form,'保存修改').click();await form.waitFor({state:'hidden'});await controls.groupStayedRemote();checks.push('two-human-SSE-config-conflict','retained-group-draft','removed-member-not-restored');
 await button(page,'项目设置').click();const mine=dialog(page,'项目设置 · Browser workspace');await mine.getByRole('textbox',{name:'项目描述',exact:true}).fill('My retained project draft');
 await button(other,'项目设置').click();const theirs=dialog(other,'项目设置 · Browser workspace');await theirs.getByRole('textbox',{name:'项目描述',exact:true}).fill('Remote project description');await button(theirs,'保存修改').click();await visible(other.getByText('已保存',{exact:true}));await button(theirs,'关闭').click();
 await visible(mine.getByText('其他人已更新这些设置。你的编辑已保留，保存时会检查原版本。',{exact:true}));await equalsValue(mine.getByRole('textbox',{name:'项目描述',exact:true}),'My retained project draft');await button(mine,'保存修改').click();await visible(page.getByText('内容已被其他人修改。你的编辑已保留，请重新载入最新内容后重试。',{exact:true}));await controls.projectStayedRemote();await button(mine,'重新载入最新内容并替换当前编辑').click();await equalsValue(mine.getByRole('textbox',{name:'项目描述',exact:true}),'Remote project description');await button(mine,'关闭').click();checks.push('project-draft-original-version-without-chat');return {...meta,rootURL,url,checks};
}

export async function runQueryFailureFlow(page,{groupId,rootURL,controls}){
 const checks=[],files=`/conversations/${groupId}/files`,snapshots=`/groups/${groupId}/snapshots`,context=`/groups/${groupId}/context`;
 controls.fail(files);await page.goto(rootURL);await visible(page.locator('.judex-v2-chat-title'));await button(page,'群聊文件').click();const library=page.getByRole('complementary',{name:'群聊文件',exact:true});await visible(library.getByRole('alert'));controls.restore(files);
 // An SSE invalidation may already have recovered the query after the fault
 // is removed. In that case the retry control correctly disappears.
 const recoveredFile=library.getByText('acceptance.md',{exact:true}).first();
 try{await button(library,'重试').click({timeout:2000,timeoutMs:2000});}catch{await visible(recoveredFile);}
 await visible(recoveredFile);await button(library,'关闭阅读区').click();checks.push('file-library-failure-recovery');
 controls.fail(snapshots);await button(page,'群公共知识').first().click();let knowledge=dialog(page,'群公共知识');await visible(knowledge.getByRole('alert'));controls.restore(snapshots);await button(knowledge,'重试').click();await visible(knowledge.getByText('尚未生成公共摘要，原文持续可读。',{exact:true}));await button(knowledge,'关闭').click();checks.push('snapshot-failure-retry');
 controls.fail(context);await page.goto(rootURL);await visible(page.locator('.judex-v2-chat-title'));await button(page,'群公共知识').first().click();knowledge=dialog(page,'群公共知识');await button(knowledge,'群聊天历史').click();await visible(knowledge.getByRole('alert'));controls.restore(context);await button(knowledge,'重试').click();await visible(knowledge.getByText(/#1 · message/));await button(knowledge,'关闭').click();checks.push('original-context-failure-retry');return {checks};
}

export async function runHumanResponseFlow(page,{url,projectId,controls}){
 const human=await controls.prepareHuman(),checks=[];await page.goto(`${url}/?project=${projectId}&group=${human.groupId}`);await visible(page.locator('.judex-v2-chat-title').filter({hasText:'Human reply UI'}));
 await button(page,'提及成员或岗位 AI').click();await page.getByRole('button').filter({hasText:human.roleLabel}).click();await page.getByRole('textbox',{name:'聊聊你的想法，或 @ 一位伙伴…',exact:true}).fill('HUMAN_WAIT_TEST');await button(page,'发送消息').click();
 const question=page.getByTestId('ai-message').filter({hasText:'HUMAN_WAIT_QUESTION'});await controls.partialReply(human.groupId);await visible(question);await visible(page.getByText(/等待人类回应/));
 await button(question,'回复这条消息').click();await visible(button(page,'取消回复'));await button(page,'取消回复').click();if(await button(page,'取消回复').count())throw new Error('Reply cancellation failed');await button(question,'回复这条消息').click();
 await page.getByRole('textbox',{name:'聊聊你的想法，或 @ 一位伙伴…',exact:true}).fill('HUMAN_REPLY_CONFIRMED');await button(page,'提及成员或岗位 AI').click();await page.getByRole('button').filter({hasText:human.roleLabel}).click();await button(page,'发送消息').click();await controls.settled(human.groupId);
 const answer=page.getByTestId('human-message').filter({hasText:'HUMAN_REPLY_CONFIRMED'});await visible(answer.locator('.judex-v2-reply-reference'));await page.locator('.judex-v2-discussion-result').waitFor({state:'hidden',timeoutMs:30000});checks.push('reply-selection-cancellation','human-explicit-reply-and-role-mention','human-wait-natural-settlement');return {checks,groupId:human.groupId};
}

export async function runRecoveryFlow(page,other,{controls,file}){
 const checks=[],title='项目设置 · Browser workspace',conflict='内容已被其他人修改。你的编辑已保留，请重新载入最新内容后重试。';
 const hidden=controls.hiddenCount();controls.hideConfiguration(true);
 try{
  await button(page,'项目设置').click();const mine=dialog(page,title);await mine.getByRole('textbox',{name:'项目描述',exact:true}).fill('Retained after missed event');
  await button(other,'项目设置').click();const theirs=dialog(other,title);await theirs.getByRole('textbox',{name:'项目描述',exact:true}).fill('Remote configuration without event');await button(theirs,'保存修改').click();await visible(other.getByText('已保存',{exact:true}));await button(theirs,'关闭').click();await controls.configurationHidden(hidden);
  if(await button(mine,'重新载入最新内容并替换当前编辑').count())throw Error('The controlled configuration event was not hidden');
  await button(mine,'保存修改').click();await visible(page.getByText(conflict,{exact:true}));await visible(button(mine,'重新载入最新内容并替换当前编辑'));await equalsValue(mine.getByRole('textbox',{name:'项目描述',exact:true}),'Retained after missed event');
  await button(mine,'关闭').click();await button(page,'项目设置').click();const reopened=dialog(page,title);await equalsValue(reopened.getByRole('textbox',{name:'项目描述',exact:true}),'Remote configuration without event');await reopened.getByRole('textbox',{name:'项目描述',exact:true}).fill('Successful save after conflict');await button(reopened,'保存修改').click();await controls.projectDescription('Successful save after conflict');await button(reopened,'关闭').click();checks.push('missed-config-event-conflict-refetch','conflict-close-reopen-latest','save-after-conflict');
 }finally{controls.hideConfiguration(false);}
 const evidence=[];
 for(const mode of ['truncated','envelope']){
  const body='Confirmed once after '+mode+' '+Date.now(),composer=page.getByRole('textbox',{name:'聊聊你的想法，或 @ 一位伙伴…',exact:true});controls.corruptMessage(mode);
  await composer.fill(body);await button(page,'发送消息').click();await visible(page.getByText('未能确认操作结果，输入已保留。重试同一操作可确认结果，不会重复提交。',{exact:true}));await equalsValue(composer,body);
  await button(page,'发送消息').click();await equalsValue(composer,'');evidence.push({mode,retry:await controls.messageEffects(body,1,2)});
  await composer.fill(body);await button(page,'发送消息').click();await equalsValue(composer,'');evidence.at(-1).newIntent=await controls.messageEffects(body,2,3);checks.push('committed-'+mode+'-response-safe-retry','acknowledged-'+mode+'-new-intent');
 }
 const before=await controls.fileCount();controls.corruptComplete();
 const choose=async()=>{const chooser=page.waitForEvent('filechooser');await button(page,'添加文件').click();await(await chooser).setFiles(file);};
 await choose();await visible(page.getByText('未能确认操作结果，输入已保留。重试同一操作可确认结果，不会重复提交。',{exact:true}));await choose();await visible(page.getByText('文件已发送。网页上传不会自动唤醒 AI。',{exact:true}));evidence.push({upload:await controls.uploadEffects(before)});checks.push('committed-upload-complete-safe-resume');
 return {checks,evidence};
}
