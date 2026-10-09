import { useLayoutEffect, useRef } from "react";
export function useReadingPosition(key: string, ready = true) {
  const ref = useRef<HTMLDivElement>(null),
    storage = "judex.chat.reading." + key;
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || !ready) return;
    let lastTop=0,restoring=true;
    try {lastTop=JSON.parse(sessionStorage.getItem(storage)||"null")?.top??0;}catch{}
    const restore=()=>{if(!restoring)return;el.scrollTop=lastTop;if(el.scrollTop>=lastTop-1)restoring=false;};
    restore();
    // Hydration and later pages can grow content after the first layout pass.
    const size=new ResizeObserver(restore);
    const observe=()=>{size.observe(el);for(const child of el.children)size.observe(child);restore();};
    const children=new MutationObserver(observe);children.observe(el,{childList:true,subtree:true});observe();
    const takeControl=()=>{restoring=false;lastTop=el.scrollTop;};
    el.addEventListener("wheel",takeControl);el.addEventListener("pointerdown",takeControl);el.addEventListener("keydown",takeControl);
    const persist = () => {
      try {
        sessionStorage.setItem(storage, JSON.stringify({ top: lastTop }));
      } catch {}
    };
    const save = () => {if(!restoring)lastTop=el.scrollTop;persist();};
    el.addEventListener("scroll", save);
    return () => {
      // React can replace a panel's contents before cleanup. Its shorter layout
      // must not overwrite the last position observed in the previous section.
      persist();
      size.disconnect();children.disconnect();el.removeEventListener("wheel",takeControl);el.removeEventListener("pointerdown",takeControl);el.removeEventListener("keydown",takeControl);
      el.removeEventListener("scroll", save);
    };
  }, [storage, ready]);
  return ref;
}
