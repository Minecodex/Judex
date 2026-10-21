export type ReaderKind='markdown'|'text'|'table'|'workbook'|'image'|'pdf'|'html'|'audio'|'video'|'diagram'|'archive'|'unsupported';
export const formats:Record<Exclude<ReaderKind,'unsupported'>,string[]>={
 markdown:['md','markdown'],text:['txt','log','go','py','js','ts','tsx','jsx','java','rs','sql','sh','bash','css','scss','json','jsonl','yaml','yml','xml','toml','ini','conf','env','vue','svelte','c','h','cpp','hpp','rb','php','swift','kt','kts','r','lua','bat','ps1'],table:['csv','tsv'],workbook:['xlsx','xls','ods'],image:['png','jpg','jpeg','gif','webp','svg','bmp','avif'],pdf:['pdf','docx','doc','odt','rtf','pptx','ppt','odp'],html:['html','htm'],audio:['mp3','wav','ogg','m4a','flac','aac','opus'],video:['mp4','webm','mov','mkv','m4v'],diagram:['mmd','mermaid'],archive:['zip','tar','gz','tgz','7z']};
export const extension=(name:string)=>name.split('.').at(-1)?.toLowerCase()||'';
export function readerKind(name:string,kind?:string):ReaderKind{
 if(kind==='unsupported')return 'unsupported';
 if(kind==='html')return 'html';
 const ext=extension(name);
 if(formats.workbook.includes(ext))return 'workbook';
 if(kind==='pdf')return 'pdf';
 for(const [reader,extensions] of Object.entries(formats))if(extensions.includes(ext))return reader as ReaderKind;
 return kind==='text'?'text':kind==='image'?'image':kind==='archive'?'archive':'unsupported';
}
export const languages:Record<string,string>={go:'go',py:'python',js:'javascript',ts:'typescript',tsx:'typescript',jsx:'javascript',java:'java',rs:'rust',sql:'sql',sh:'bash',bash:'bash',css:'css',scss:'scss',json:'json',jsonl:'json',yaml:'yaml',yml:'yaml',xml:'xml',html:'xml',htm:'xml',toml:'ini',ini:'ini',conf:'ini',env:'ini',vue:'xml',svelte:'xml',c:'c',h:'c',cpp:'cpp',hpp:'cpp',rb:'ruby',php:'php',swift:'swift',kt:'kotlin',kts:'kotlin',r:'r',lua:'lua',bat:'dos',ps1:'powershell'};
