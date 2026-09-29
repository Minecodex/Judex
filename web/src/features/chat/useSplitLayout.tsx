import { useWork } from "../work/store";
import { useSplitLayoutBase } from "./useSplitLayoutBase";
export { useSplitLayoutBase } from "./useSplitLayoutBase";
export function useSplitLayout(open: boolean) { const { t } = useWork(); return useSplitLayoutBase(open, t); }
