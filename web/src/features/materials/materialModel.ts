import type {Evidence} from '../work/types';
import type {MaterialItem} from './api';
export const fileSize = (size: number) => size >= 1048576 ? (size / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(size / 1024)) + ' KB';
export function materialDate(value: string | null | undefined, locale: string, missing: string) {
  const date = value ? new Date(value) : null;
  return date && Number.isFinite(date.getTime()) ? date.toLocaleString(locale) : missing;
}
export function formatLabel(file: Pick<MaterialItem, 'format' | 'title'>) {
  const ext = file.title.split('.').at(-1)?.toUpperCase();
  return file.format === 'image' ? ext || 'IMAGE' : file.format === 'archive' ? 'ZIP' : file.format === 'markdown' ? 'MD' : file.format === 'other' ? ext || 'FILE' : file.format.toUpperCase();
}
export function materialEvidence(file: MaterialItem): Evidence {
  return {id: file.versionId, versionId: file.versionId, name: file.title, text: '', author: file.authorName, at: file.uploadedAt ? Date.parse(file.uploadedAt) : 0, type: file.mime};
}
