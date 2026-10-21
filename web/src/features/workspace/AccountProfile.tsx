import {useRef,useState} from 'react';
import {Camera,Trash2} from 'lucide-react';
import {useWorkspace} from './context';
import {Avatar,Button,ConfigChange,Field} from './ui';
import {useConfigDraft} from './configDraft';

async function portrait(file:File):Promise<string>{
 if(!['image/png','image/jpeg'].includes(file.type)||file.size>10*1024*1024)throw new Error('invalid');
 const bitmap=await createImageBitmap(file);
 try{if(bitmap.width>12000||bitmap.height>12000)throw new Error('invalid');const canvas=document.createElement('canvas');canvas.width=canvas.height=256;const ctx=canvas.getContext('2d');if(!ctx)throw new Error('invalid');const side=Math.min(bitmap.width,bitmap.height);ctx.drawImage(bitmap,(bitmap.width-side)/2,(bitmap.height-side)/2,side,side,0,0,256,256);return canvas.toDataURL('image/png');}finally{bitmap.close();}
}
export function AccountProfile(){
 const w=useWorkspace(),input=useRef<HTMLInputElement>(null),[error,setError]=useState(''),[reading,setReading]=useState(false);
 const config=useConfigDraft<{name:string;avatarData:string|undefined}>({name:w.user.displayName,avatarData:undefined},w.user.version),{name,avatarData}=config.draft;
 const choose=async(file?:File)=>{if(!file)return;setError('');setReading(true);try{config.update({avatarData:await portrait(file)});}catch{setError(w.t('pfInvalidImage'));}finally{setReading(false);if(input.current)input.current.value='';}};
 const save=async()=>{try{const user=await w.execute<{version:number}>('/me',{expectedVersion:config.baseVersion,displayName:name.trim(),...(avatarData!==undefined?{avatarData}:{})},'PATCH');config.replace({name:name.trim(),avatarData:undefined},user.version);await w.auth.refresh();w.setToast(w.t('v2Saved'));}catch{await w.auth.refresh();}};
 return <section className="judex-account-profile"><ConfigChange stale={config.stale} onReload={config.reload}/><div className="judex-account-portrait"><Avatar large name={name} userId={w.user.id} avatarVersion={avatarData!==undefined?0:w.user.avatarVersion} src={avatarData||undefined}/><div><strong>{w.t('pfPortrait')}</strong><p>{w.t('pfPortraitHint')}</p><div><Button variant="outline" isPending={reading} onPress={()=>input.current?.click()}><Camera/>{w.t('pfChangePortrait')}</Button><Button isDisabled={!avatarData&&!w.user.avatarVersion} onPress={()=>config.update({avatarData:''})}><Trash2/>{w.t('pfRemovePortrait')}</Button></div></div></div><input ref={input} type="file" accept="image/png,image/jpeg" className="judex-account-file" aria-label={w.t('pfChoosePortrait')} onChange={e=>void choose(e.target.files?.[0])}/>{error&&<p role="alert">{error}</p>}<Field label={w.t('pfNickname')} value={name} onChange={name=>config.update({name})} hint={w.t('pfNicknameHint')}/><p className="judex-settings-note">{w.user.email}</p><div className="judex-settings-savebar"><span>{w.t('pfSaveHint')}</span><Button variant="primary" isDisabled={!name.trim()||Array.from(name.trim()).length>80||reading||w.pending} onPress={()=>void save()}>{w.t('pfSave')}</Button></div></section>;
}
