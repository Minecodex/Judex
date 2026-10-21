export type Copy = { zh: string; en: string };
export const c = (zh: string, en: string): Copy => ({ zh, en });
export type Person = { id: string; name: Copy; subtitle: Copy; tone: string; initials: string; online?: boolean };
export type Role = { id: string; name: Copy; prompt: Copy };
export type Workflow = { id: string; name: Copy; description: Copy };
export type Project = { id: string; name: Copy; description: Copy; tone: string; icon: string; members: string[]; roles: Role[]; workflows: Workflow[]; limit: number };
export type Group = { id: string; projectId: string; parentId?: string; name: Copy; purpose: Copy; icon: string; tone: string; members: Record<string, string[]>; workflows: string[]; archived: boolean; unread: number; preview: Copy; time: string };
export type FileRef = { fileId: string; version: number };
export type Version = { number: number; authorId: string; date: string; note: Copy; content?: string; kind?: SharedFile['kind'] };
export type SharedFile = { id: string; roomId: string; name: Copy; kind: 'brief' | 'table' | 'code' | 'text' | 'image' | 'binary'; versions: Version[]; removed?: boolean; mime?: string };
export type Message = { id: string; roomId: string; authorId: string; roleId?: string; text: Copy; time: string; kind?: 'system' | 'local'; files?: FileRef[]; tool?: Copy };
export type DM = { id: string; personId: string; unread: number; preview: Copy; time: string };
export type Discussion = { id: string; roomId: string; loops: number; limit: number; status: 'paused' | 'continued' | 'stopped'; feedback: string; people: string[] };
export type Notice = { id: string; text: Copy; roomId?: string; read: boolean };
export type DemoState = { projects: Project[]; groups: Group[]; files: SharedFile[]; messages: Message[]; dms: DM[]; discussions: Discussion[]; notices: Notice[]; personalPrompts: Record<string, string>; guests: Person[]; hiddenFiles: Record<string, string[]> };
export type ModalName = 'create' | 'group' | 'project' | 'library' | 'share' | 'push' | 'knowledge' | 'dm' | 'help' | 'notifications' | 'new-project' | null;
export type ModalState = { name: ModalName; parentId?: string; file?: FileRef };
export const uid = () => crypto.randomUUID();
export const now = () => new Date().toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
export function level(group: Group, groups: Group[]): number {
  const parent = groups.find(g => g.id === group.parentId);
  return parent ? 1 + level(parent, groups) : 1;
}
export function descendants(id: string, groups: Group[]): string[] {
  return [id, ...groups.filter(g => g.parentId === id).flatMap(g => descendants(g.id, groups))];
}
