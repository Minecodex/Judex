export type SplitRatio = { left: number; right: number };
export type SplitLimits = {
  left: number;
  center: number;
  right: number;
  divider: number;
};
export const DEFAULT_SPLIT: SplitRatio = { left: 0.2, right: 0.3 };
export const SPLIT_KEY = "judex.chat.layout.v2";
export function readSplit(value: unknown): SplitRatio {
  const v = value as SplitRatio;
  return v &&
    [v.left, v.right].every(
      (n) => typeof n === "number" && Number.isFinite(n) && n > 0 && n < 1,
    )
    ? { left: v.left, right: v.right }
    : { ...DEFAULT_SPLIT };
}
export function fitSplit(
  width: number,
  ratio: SplitRatio,
  limits: SplitLimits,
  open: boolean,
) {
  const available = width - limits.divider * (open ? 2 : 1),
    sideSpace = available - limits.center;
  let left = Math.max(limits.left, width * ratio.left),
    right = open ? Math.max(limits.right, width * ratio.right) : 0;
  if (left + right > sideSpace) {
    const leftExtra = left - limits.left,
      rightExtra = open ? right - limits.right : 0,
      total = leftExtra + rightExtra;
    const room = Math.max(
      0,
      sideSpace - limits.left - (open ? limits.right : 0),
    );
    left = limits.left + (total ? (room * leftExtra) / total : 0);
    right = open ? limits.right + (total ? (room * rightExtra) / total : 0) : 0;
  }
  return { left, right, center: available - left - right };
}
