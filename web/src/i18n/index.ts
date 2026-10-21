import {libraryZh,libraryEn} from './library';
import {readerZh,readerEn} from './reader';
import {settingsZh,settingsEn} from './settings';
import {workspaceZh,workspaceEn} from './workspace';
import {authZh,authEn} from './auth';
import {materialPreviewZh,materialPreviewEn} from './materialPreview';
import {commonZh,commonEn} from './common';
const modules=[{zh:libraryZh,en:libraryEn},{zh:readerZh,en:readerEn},{zh:settingsZh,en:settingsEn},{zh:workspaceZh,en:workspaceEn},{zh:authZh,en:authEn},{zh:materialPreviewZh,en:materialPreviewEn},{zh:commonZh,en:commonEn}];
export type Locale='zh-CN'|'en';
export type Key=keyof typeof libraryZh|keyof typeof readerZh|keyof typeof settingsZh|keyof typeof workspaceZh|keyof typeof authZh|keyof typeof materialPreviewZh|keyof typeof commonZh;
const zh=Object.assign({},...modules.map(m=>m.zh)) as Record<Key,string>;
const en=Object.assign({},...modules.map(m=>m.en)) as Record<Key,string>;
export function translate(locale:Locale,key:Key,values:Record<string,string|number>={}){let value=(locale==='zh-CN'?zh:en)[key];for(const [k,v] of Object.entries(values))value=value.replaceAll('{'+k+'}',String(v));return value;}
