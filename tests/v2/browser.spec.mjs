import {runFlowProfile} from './browser-flow-profile.mjs';import {readEnvironment} from './environment.mjs';
import {runWorkspaceChromeFlow} from './browser-appearance.mjs';
import {runPaneNavigationFlow} from './browser-pane-navigation.mjs';
import {runMenuStylingFlow} from './browser-menus.mjs';
import {runSettingsFlow} from './browser-settings.mjs';
import fs from 'node:fs';import os from 'node:os';import {runBatchRecoveryFlow} from './browser-batch.mjs';
import {test} from '@playwright/test';import path from 'node:path';import {client,base} from './api.mjs';import {runBrowserFlow} from './browser-flow.mjs';
import {startBrowserProxy} from './browser-proxy.mjs';import {browserControls} from './browser-controls.mjs';import {runConcurrencyFlow,runQueryFailureFlow,runHumanResponseFlow,runRecoveryFlow} from './browser-followup.mjs';
test('real V2 desktop workspace',async({page,context})=>{
 const peer=client(),stamp=Date.now(),peerEmail=`v2-browser-peer-${stamp}@test.local`,email=`v2-browser-owner-${stamp}@test.local`,password='V2-browser-testing-123',peerName='Browser peer';
 await peer.call('POST','/auth/register',{displayName:peerName,email:peerEmail,password});const proxy=await startBrowserProxy(base),other=await context.newPage();const batchDir=fs.mkdtempSync(path.join(os.tmpdir(),'judex-batch-')),batchFiles=['batch-a.md','batch-b.md'].map(name=>{const file=path.join(batchDir,name);fs.writeFileSync(file,'# '+name);return file;});
 try{
  const result=await runBrowserFlow(page,{url:proxy.url,email,peerEmail,peerName,password,file:path.resolve('tests/fixtures/materials/acceptance.md')});
  const appearance=await runWorkspaceChromeFlow(page,{rootURL:result.rootURL});
  const menus=await runMenuStylingFlow(page,{rootURL:result.rootURL});
  const paneNavigation=await runPaneNavigationFlow(page,{rootURL:result.rootURL});
  const projectSettings=await runSettingsFlow(page,{rootURL:result.rootURL});
  const owner=client();await owner.call('POST','/auth/login',{email,password});await owner.call('GET','/auth/session');const projectId=new URL(result.rootURL).searchParams.get('project'),project=await owner.call('GET','/projects/'+projectId);await owner.call('PATCH','/projects/'+projectId,{name:project.name,description:project.description,loopLimit:project.loopLimit,expectedVersion:project.version,defaultModelId:readEnvironment().models.loop||readEnvironment().models.small});const portrait=path.join(batchDir,'portrait.png');fs.writeFileSync(portrait,Buffer.from('iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAIAAAD8GO2jAAAAKklEQVR4nGMoKvenKWIYtWDUglELRi0YtWDUglELRi0YtWDUglELhooFAFOQ4D3tNHD9AAAAAElFTkSuQmCC','base64'));const flowProfile=await runFlowProfile(page,{rootURL:result.rootURL,portrait});
  const controls=browserControls(peer,{email,password,proxy}),settings=await runConcurrencyFlow(page,other,{url:proxy.url,peerURL:proxy.peerURL,rootURL:result.rootURL,peerEmail,password,controls});
  const recovery=await runRecoveryFlow(page,other,{...settings,controls,file:path.resolve('tests/fixtures/materials/acceptance.md')});
  const batch=await runBatchRecoveryFlow(page,{...settings,controls,files:batchFiles});await page.goto(settings.rootURL);
  const queries=await runQueryFailureFlow(page,{...settings,controls}),human=await runHumanResponseFlow(page,{...settings,controls});
  console.log(JSON.stringify({status:'passed',checks:[...result.checks,...appearance.checks,...menus.checks,...paneNavigation.checks,...projectSettings.checks,...flowProfile.checks,...settings.checks,...recovery.checks,...batch.checks,...queries.checks,...human.checks]}));
 }finally{await other.close();await proxy.close();fs.rmSync(batchDir,{recursive:true,force:true});}
});
