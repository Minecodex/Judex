import { useEffect, useState } from "react";
import { useWork } from "../work/store";
export function useWorkspaceDraft<T>(slot: string, initial: T) {
  const { project, state } = useWork();
  const key =
      "judex.workspace.draft." +
      project.id +
      ":" +
      state.currentUser +
      ":" +
      slot,
    base = JSON.stringify(initial);
  const [value, setValue] = useState<T>(() => {
    try {
      const saved = JSON.parse(sessionStorage.getItem(key) || "null");
      return saved?.base === base ? saved.value : initial;
    } catch {
      return initial;
    }
  });
  useEffect(() => {
    try {
      if (JSON.stringify(value) === base) sessionStorage.removeItem(key);
      else sessionStorage.setItem(key, JSON.stringify({ base, value }));
    } catch {}
  }, [key, base, value]);
  return [value, setValue] as const;
}
