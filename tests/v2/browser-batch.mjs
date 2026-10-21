import assert from 'node:assert/strict';
const button=(page,name)=>page.getByRole('button',{name,exact:true});
const visible=async item=>item.waitFor({state:'visible',timeoutMs:30000});

export async function runBatchRecoveryFlow(page,{url,projectId,controls,files}){
 const checks=[],evidence=[];
 const choose=async selected=>{
  const pending=page.waitForEvent('filechooser');
  await button(page,'添加文件').click();const chooser=await pending;
  assert.ok(chooser.isMultiple());await chooser.setFiles(selected);
 };
 for(const mode of ['before-whole','after-subset','after-resume']){
  const id=await controls.prepareBatch(mode);
  await page.goto(`${url}/?project=${projectId}&group=${id}`);
  await visible(page.locator('.judex-v2-chat-title').filter({hasText:'Batch recovery '+mode}));
  await choose(files);const recovery=page.getByTestId('upload-recovery');await visible(recovery);
  await visible(recovery.getByText('文件上传 · 1/2 已确认',{exact:true}));
  const before=await controls.batchEffects(id,mode==='before-whole'?1:2);
  if(mode==='before-whole'){
   // Reselecting only the already-confirmed item cannot discard the batch.
   await choose([files[0]]);await visible(recovery);await controls.batchEffects(id,1);
   await choose(files);
  }else if(mode==='after-subset')await choose([files[1]]);
  else await button(recovery,'继续本批上传').click();
  await recovery.waitFor({state:'hidden',timeoutMs:30000});
  const after=await controls.batchEffects(id,2);
  assert.ok(before.every(f=>after.some(a=>a.id===f.id)));
  await choose([files[0]]);await controls.batchEffects(id,3);
  evidence.push({mode,before,confirmed:after});checks.push('batch-'+mode+'-no-duplicate','batch-'+mode+'-explicit-new-intent');
 }
 return {checks,evidence};
}
