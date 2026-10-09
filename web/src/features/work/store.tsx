import {createContext,useContext,lazy,Suspense,type ReactNode} from "react";
import {Button} from "../../components/ui/Button";
import {useApiWorkbench} from "./apiStore";
import {readRoute} from "./storeBase";
import {usePreferences} from "../../stores/preferences";
import {translate} from "../../i18n";
import {dataMode} from "../../lib/api/client";
import type {WorkStore} from "./storeTypes";
import type {View} from "./types";
export const WorkContext=createContext<WorkStore|null>(null);
const Context=WorkContext;
const DemoProvider=import.meta.env.VITE_DATA_MODE==="demo"?lazy(()=>import("./demoStore")):null;
export function WorkProvider({children,mode,projectId}:{children:ReactNode;mode?:"demo"|"api";projectId?:string}){
 if((mode??dataMode)==="api"||!DemoProvider)return <ApiWorkProvider projectId={projectId??readRoute().projectId}>{children}</ApiWorkProvider>;
 return <Suspense fallback={<main className="judex-entry"/>}><DemoProvider>{children}</DemoProvider></Suspense>;
}
function ApiWorkProvider({
  projectId,
  children,
}: {
  projectId: string;
  children: ReactNode;
}) {
  const { store, failed, retry } = useApiWorkbench(projectId);
  const { locale } = usePreferences();
  if (!store)
    return (
      <main className="judex-entry min-h-screen flex flex-col items-center justify-center gap-4">
        <p className="judex-workspace-status" role={failed ? "alert" : undefined}>
          {translate(locale, failed ? "errServer" : "shellLoading")}
        </p>
        {failed && (
          <Button variant="secondary" onPress={retry}>
            {translate(locale, "shellRetry")}
          </Button>
        )}
      </main>
    );
  return <Context.Provider value={store}>{children}</Context.Provider>;
}
export function useWork() {
  const value = useContext(Context);
  if (!value) throw new Error("WorkProvider required");
  return value;
}

export function WorkView({
  view,
  id,
  section,
  children,
}: {
  view: View;
  id?: string;
  section?:import('./runtimeTypes').TaskSection;
  children: ReactNode;
}) {
  const base = useWork();
  return (
    <Context.Provider value={{ ...base, route: { ...base.route, view, id,taskSection:section??base.route.taskSection } }}>
      {children}
    </Context.Provider>
  );
}
