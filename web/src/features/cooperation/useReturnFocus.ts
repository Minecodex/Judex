import {useRef} from 'react';

type Opener = {element: HTMLElement | null; key?: string};
export function useReturnFocus() {
  const opener = useRef<Opener>({element: null});
  const capture = (fallbackKey?: string, preserveDrawerOpener = false) => {
    const element = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    // Switching sections or following a prerequisite inside the drawer keeps
    // the original page trigger as the return destination.
    if (preserveDrawerOpener && element?.closest('[data-testid="task-details-drawer"]')) return;
    opener.current = {element, key: element?.getAttribute('data-focus-key') ?? fallbackKey};
  };
  const restore = (fallbackKey?: string | string[]) => {
    const saved = opener.current;
    requestAnimationFrame(() => {
      const keys = [saved.key, ...(Array.isArray(fallbackKey) ? fallbackKey : [fallbackKey])].filter((key): key is string => !!key);
      const candidates = [saved.element, ...keys.flatMap(key => [...document.querySelectorAll<HTMLElement>('[data-focus-key="' + CSS.escape(key) + '"]')])];
      const target = candidates.find(el => el?.isConnected && el !== document.body && el.getClientRects().length && !el.hasAttribute('disabled') && !el.closest('[inert],[hidden]'));
      target?.focus({preventScroll: true});
    });
  };
  return {capture, restore};
}
