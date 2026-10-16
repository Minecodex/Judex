export type CollectionPage<T> = {items: T[]; nextCursor?: string | null};

// Preserve filters and an explicit page size without appending duplicate keys.
export function collectionURL(path: string, cursor?: string, limit = 100): string {
  const [pathname, query = ''] = path.split('?', 2);
  const params = new URLSearchParams(query);
  if (!params.has('limit')) params.set('limit', String(limit));
  if (cursor !== undefined) {
    if (cursor) params.set('cursor', cursor);
    else params.delete('cursor');
  }
  return pathname + '?' + params;
}

// Configuration reads must finish every page before returning or writing.
// A failed page or repeated cursor is an error, never a partial collection.
export async function collectPages<T>(path: string, read: (url: string) => Promise<CollectionPage<T>>): Promise<T[]> {
  const items: T[] = [];
  const seen = new Set<string>();
  let url = collectionURL(path);
  const initial = new URLSearchParams(url.split('?')[1]).get('cursor');
  if (initial) seen.add(initial);
  for (;;) {
    const page = await read(url);
    items.push(...page.items);
    if (!page.nextCursor) return items;
    if (seen.has(page.nextCursor)) throw new Error('Repeated collection cursor');
    seen.add(page.nextCursor);
    url = collectionURL(path, page.nextCursor);
  }
}
