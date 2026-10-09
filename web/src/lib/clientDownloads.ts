export type DownloadArtifact = {filename:string;url:string;size:number;sha256:string;architectures:string[]};
export type ClientDownloads = {version:string;commit:string;sourceHash:string;artifacts:Record<'windows'|'macos'|'skills',DownloadArtifact>};

export async function clientDownloads():Promise<ClientDownloads> {
  const response=await fetch('/downloads/manifest.json',{cache:'no-cache'});
  if(!response.ok)throw new Error('Client downloads are unavailable');
  const data=await response.json() as ClientDownloads;
  if(!data||typeof data.version!=='string'||!/^[0-9A-Za-z][0-9A-Za-z.+-]*$/.test(data.version))throw new Error('Invalid client catalog');
  for(const key of ['windows','macos','skills'] as const){
    const artifact=data.artifacts?.[key];
    if(!artifact||artifact.filename!==`judex-${key}-${data.version}.zip`||artifact.url!=='/downloads/'+artifact.filename||!Number.isSafeInteger(artifact.size)||artifact.size<=0||!/^[a-f0-9]{64}$/.test(artifact.sha256))throw new Error('Invalid download artifact');
  }
  return data;
}
export function downloadArtifact(artifact:DownloadArtifact) {
  const link=document.createElement('a');link.href=artifact.url;link.download=artifact.filename;
  document.body.appendChild(link);link.click();link.remove();
}
