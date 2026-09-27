import { useLayoutEffect, useRef } from "react";
export function useReadingPosition(key: string) {
  const ref = useRef<HTMLDivElement>(null),
    storage = "judex.chat.reading." + key;
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    try {
      const value = JSON.parse(sessionStorage.getItem(storage) || "null");
      if (value) el.scrollTop = value.top || 0;
    } catch {}
    const save = () => {
      try {
        sessionStorage.setItem(storage, JSON.stringify({ top: el.scrollTop }));
      } catch {}
    };
    el.addEventListener("scroll", save);
    return () => {
      save();
      el.removeEventListener("scroll", save);
    };
  }, [storage]);
  return ref;
}
