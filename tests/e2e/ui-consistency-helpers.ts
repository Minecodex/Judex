import {expect,type Locator,type Page} from '@playwright/test';

export async function actionGeometry(group:Locator) {
  return group.locator('button:visible').evaluateAll(buttons=>buttons.map(button=>{
    const box=button.getBoundingClientRect(),style=getComputedStyle(button),icon=button.querySelector('svg');
    return {text:button.textContent?.trim(),height:box.height,width:box.width,font:style.fontSize,padding:style.paddingInlineStart,
      icon:icon?{width:icon.getBoundingClientRect().width,height:icon.getBoundingClientRect().height,dy:icon.getBoundingClientRect().y+icon.getBoundingClientRect().height/2-box.y-box.height/2}:null};
  }));
}

export async function settleOverlays(page:Page){
  await expect(page.locator('.judex-overlay[data-entering],.judex-overlay[data-exiting],.judex-overlay [data-entering],.judex-overlay [data-exiting]')).toHaveCount(0);
  await page.evaluate(async()=>{
    const animations=Array.from(document.querySelectorAll('.judex-dialog-container')).flatMap(element=>element.getAnimations({subtree:true}));
    await Promise.all(animations.filter(animation=>animation.playState==='running'&&animation.effect?.getComputedTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>{})));
  });
}

export async function assertActionGeometry(group:Locator,size=40) {
  const buttons=await actionGeometry(group);
  expect(buttons.length).toBeGreaterThan(0);
  for(const button of buttons){
    expect(Math.abs(button.height-size),JSON.stringify(button)).toBeLessThanOrEqual(1);
    if(button.icon){expect(Math.abs(button.icon.dy),JSON.stringify(button)).toBeLessThanOrEqual(1);expect(button.icon.width).toBe(button.icon.height);}
  }
  expect(new Set(buttons.map(b=>b.font)).size).toBe(1);
  return buttons;
}

export async function preferenceGeometry(page:Page) {
  await settleOverlays(page);
  return page.getByRole('dialog').evaluate(dialog=>{
    const body=dialog.querySelector('.judex-dialog-body')!,panel=dialog.querySelector('.judex-work-preferences .judex-work-panel')!,editor=dialog.querySelector('textarea')!,aside=dialog.querySelector('.judex-work-preferences aside')!;
    const box=(element:Element)=>{const r=element.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};};
    const bodyStyle=getComputedStyle(body);
    return {dialog:box(dialog),body:box(body),bodyWidth:body.clientWidth-parseFloat(bodyStyle.paddingLeft)-parseFloat(bodyStyle.paddingRight),panel:box(panel),editor:box(editor),aside:box(aside),pageTitles:dialog.querySelectorAll('h1').length};
  });
}

export async function assertPreferenceGeometry(page:Page){
  const geometry=await preferenceGeometry(page);
  expect(geometry.pageTitles).toBe(0);
  expect(Math.abs(geometry.panel.width-geometry.bodyWidth)).toBeLessThanOrEqual(2);
  expect(geometry.editor.width).toBeGreaterThan(geometry.bodyWidth*.85);
  expect(geometry.aside.y).toBeGreaterThanOrEqual(geometry.panel.bottom-1);
  expect(geometry.dialog.x).toBeGreaterThanOrEqual(0);
  expect(geometry.dialog.right).toBeLessThanOrEqual((page.viewportSize()?.width??0)+1);
  return geometry;
}
