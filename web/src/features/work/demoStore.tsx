import {WorkContext as Context} from "./store";
import {dataMode} from "../../lib/api/client";
import {ensureDemoMainTopics} from "../chat/demoCollaboration";
import {
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Button } from "../../components/ui/Button";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { seedWork, WORK_KEY } from "./seed";
import { normalizeWork } from "./normalize";
import { allPeople, manage, member } from "./selectors";
import type { View, WorkState } from "./types";
import { demoActions } from "./actionRegistry";
import type { ActFn, WorkStore } from "./storeTypes";
import { readRoute, useWorkbenchBase } from "./storeBase";

import { translate, type Key } from "../../i18n";
import { usePreferences } from "../../stores/preferences";
import {
  loadWorkspace,
  persistPreview,
  workspaceKey,
} from "../../lib/api/workspace";


function initialPreview() {
  try {
    return (
      normalizeWork(JSON.parse(localStorage.getItem(WORK_KEY) || "null")) ||
      normalizeWork(seedWork())!
    );
  } catch {
    return normalizeWork(seedWork())!;
  }
}
function useDemoWorkbench(): WorkStore {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: workspaceKey,
    queryFn: loadWorkspace,
    initialData: initialPreview,
    staleTime: Infinity,
  });
  const state = query.data;
  const ref = useRef(state);
  ref.current = state;
  const {
    route,
    go,
    toast,
    setToast,
    locale,
    setLocale,
    theme,
    setTheme,
    t,
    text,
  } = useWorkbenchBase();
  const [storageError, setStorageError] = useState(false);
  const commit = (next: WorkState) => {
 next=ensureDemoMainTopics(next);
    try {
      persistPreview(next);
      ref.current = next;
      client.setQueryData(workspaceKey, next);
      setStorageError(false);
      return true;
    } catch {
      setStorageError(true);
      return false;
    }
  };
  useEffect(() => {
    if (dataMode === "demo") {
      try {
        if (!localStorage.getItem(WORK_KEY)) persistPreview(ref.current);
      } catch {
        setStorageError(true);
      }
    }
  }, []);
  useEffect(() => {
    const sync = (e: StorageEvent) => {
      if (e.key !== WORK_KEY || !e.newValue) return;
      try {
        const next = normalizeWork(JSON.parse(e.newValue));
        if (next)
          client.setQueryData(workspaceKey, {
            ...next,
            currentUser: ref.current.currentUser,
          });
      } catch {
        /* Ignore invalid preview snapshots. */
      }
    };
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, [client]);
  const act: ActFn = async (name, payload, opts) => {
    if (dataMode !== "demo") {
      setToast(t("shellUnavailable"));
      return { ok: false };
    }
    const result = demoActions[name](ref.current, payload as never);
    if (result.error) {
      setToast(
        t(
          ("workError" +
            result.error[0].toUpperCase() +
            result.error.slice(1)) as Key,
        ),
      );
      return { ok: false };
    }
    if (!commit(result.state!)) return { ok: false };
    if (opts?.toast !== false) setToast(t("workSaved"));
    return { ok: true, id: result.id, createdCount: result.createdCount, skippedCount: result.skippedCount };
  };
  const switchPerson = (name: string) => {
    if (commit({ ...ref.current, currentUser: name }))
      go({ view:route.scopeTaskId?"task":route.scopePlanId?"plan":route.view,id:route.scopeTaskId??route.scopePlanId??route.id,editor:undefined,invite:false });
  };
  const reset = () => {
    if (commit(seedWork())) {
      go({ projectId: "leaf", view: "home", id: undefined });
      setToast(t("resetDone"));
    }
  };
  const project =
    state.projects.find((p) => p.id === route.projectId) || state.projects[0];
  return {
    mode:"demo",
    state,
    route,
    go,
    locale,
    setLocale,
    theme,
    setTheme,
    toast,
    setToast,
    storageError,
    text,
    t,
    act,
    switchPerson,
    reset,
    project,
    membership: member(state, project.id),
    management: manage(state, project.id),
    people: allPeople(state),
  };
}

export default function DemoWorkProvider({children}:{children:ReactNode}){return <Context.Provider value={useDemoWorkbench()}>{children}</Context.Provider>;}
