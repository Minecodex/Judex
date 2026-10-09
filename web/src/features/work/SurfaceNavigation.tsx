import {createContext, useContext, type ReactNode} from 'react';
import {useWork} from './store';
import type {Route, Task} from './types';

type NavigationBoundary = {
  beforeNavigate: () => void;
  locatePlan?: (task: Task) => boolean;
};
const Boundary = createContext<NavigationBoundary | null>(null);

// Nested record, rule and material views use this boundary when leaving a
// viewer. In ordinary pages the same controls keep the normal workbench route.
export function SurfaceNavigation({children, ...value}: NavigationBoundary & {children: ReactNode}) {
  return <Boundary.Provider value={value}>{children}</Boundary.Provider>;
}
export function useSurfaceNavigation() {
  const {go} = useWork(), boundary = useContext(Boundary);
  const navigate = (action: () => void) => {
    boundary?.beforeNavigate();
    action();
  };
  return {go: (route: Partial<Route>) => navigate(() => go(route)), navigate, locatePlan: boundary?.locatePlan};
}
