import {collectPages, type CollectionPage} from './pagination';
import {createRequester} from './request';
export {APIError} from './request';

export const dataMode = import.meta.env.VITE_DATA_MODE === 'demo' ? 'demo' : 'api';
const base = (import.meta.env.VITE_API_BASE_URL || '') + '/api/v1';
const sessionClient = createRequester(base, () => window.dispatchEvent(new Event('judex:unauthorized')));
export const {request, setCSRFToken, clearCSRFToken} = sessionClient;

// EventSource uses the same API base as fetch.
export const apiUrl = (path: string) => base + path;

export function requestAll<T>(path: string, init?: RequestInit): Promise<T[]> {
  return collectPages<T>(path, url => request<CollectionPage<T>>(url, init));
}

export type SystemInfo = {
  name: string;
  version: string;
  commit?: string;
  protocolVersion: string;
  schemaRange: string;
  capabilities: Record<string, boolean>;
};
