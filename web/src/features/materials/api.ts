import {useQuery, useQueryClient} from '@tanstack/react-query';
import {useWork} from '../work/store';
import {request, apiUrl} from '../../lib/api/client';
export {fileSize, materialDate, formatLabel, materialEvidence} from './materialModel';
export type Association = {type: string; id: string; name: string};
export type MaterialItem = {
  id: string; title: string; kind: string; visibility: string; currentVersionId: string;
  createdAt: string; versionId: string; revision: number; size: number; mime: string;
  format: string; authorId: string | null; authorName: string; uploadedAt: string | null;
  originalStatus: string; source: string; purpose: string; deletedAt: string | null;
  canDelete: boolean; previewStatus: string; associations: Association[];
};
export type MaterialUsage = {
  id: string; kind: string; description: string; actorId: string | null; actorName: string;
  createdAt: string | null; source: string; taskId: string | null; taskName: string | null;
  planId: string | null; planName: string | null; topicId: string | null; topicName: string | null;
};
export type PreviewInfo = {
  status: 'not_requested' | 'pending' | 'ready' | 'failed' | 'unsupported';
  kind: string; mime: string; size: number; error: string | null;
  contentUrl: string | null;
  thumbnail: {kind: string; url: string; page: number | null} | null;
};
export type ArchiveEntry = {name: string; size: number; directory: boolean};
export const versionPath = (project: string, version: string) => '/projects/' + project + '/material-versions/' + version;
export const materialURL = (url: string) => apiUrl(url.replace(/^\/api\/v1(?=\/)/, ''));
export const previewURL = (project: string, version: string) => apiUrl(versionPath(project, version) + '/preview/content');
export const originalURL = (project: string, version: string) => apiUrl(versionPath(project, version) + '/content');
export function useMaterialVersion(version: string) {
  const {project, state, mode} = useWork();
  return useQuery({queryKey: ['materials', project.id, state.currentUserId, 'version', version], enabled: mode === 'api' && !!version,
    queryFn: async () => (await request<{library: MaterialItem}>(versionPath(project.id, version))).library});
}
export function useMaterialPreview(version: string, enabled = true) {
  const {project, state, mode} = useWork(), client = useQueryClient();
  const key = ['materials', project.id, state.currentUserId, 'preview', version];
  const query = useQuery({queryKey: key, enabled: mode === 'api' && !!version && enabled,
    queryFn: () => request<PreviewInfo>(versionPath(project.id, version) + '/preview'),
    refetchInterval: q => q.state.data?.status === 'pending' ? 1500 : false});
  return {...query, prepare: async (retry = false) => {
    client.setQueryData([...key.slice(0,3),'reading',version],false);
    await request(versionPath(project.id, version) + '/preview' + (retry ? '?retry=true' : ''), {method: 'POST', idempotencyKey: crypto.randomUUID()});
    await client.invalidateQueries({queryKey: key});
    await client.invalidateQueries({predicate: query => query.queryKey[0] === 'materials' && query.queryKey[1] === project.id && query.queryKey.includes(version) && (query.queryKey.includes('contents') || query.queryKey.includes('cover'))});
  }};
}
// A local read failure (for example a disconnected browser) is shared by the
// cover and dialog, without marking a valid server cache as corrupt.
export function usePreviewReading(version:string){
  const {project,state}=useWork(),client=useQueryClient(),key=['materials',project.id,state.currentUserId,'reading',version];
  const query=useQuery({queryKey:key,queryFn:()=>false,enabled:false,initialData:false});
  return {failed:query.data,fail:()=>client.setQueryData(key,true),clear:()=>client.setQueryData(key,false)};
}
export function usePreviewContents(file: MaterialItem, info?: PreviewInfo, cover = false) {
  const {project, state} = useWork();
  return useQuery({queryKey: ['materials', project.id, state.currentUserId, cover ? 'cover' : 'contents', file.versionId, info?.status],
    enabled: info?.status === 'ready' && ['text', 'archive'].includes(info.kind),
    queryFn: async ({signal}) => {
      const response = await fetch(materialURL(info!.contentUrl!), {credentials: 'include', signal,
        headers: cover && info!.kind === 'text' ? {Range: 'bytes=0-4095'} : undefined});
      if (!response.ok) throw Error('Could not read material content');
      const raw = await response.text();
      return info!.kind === 'archive' ? {text: '', entries: JSON.parse(raw) as ArchiveEntry[]} : {text: !cover && file.format === 'json' ? JSON.stringify(JSON.parse(raw), null, 2) : raw, entries: [] as ArchiveEntry[]};
    }});
}
