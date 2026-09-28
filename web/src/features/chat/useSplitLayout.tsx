import {
  useLayoutEffect,
  useEffect,
  useRef,
  useState,
  type PointerEvent,
  type KeyboardEvent,
} from "react";
import {
  DEFAULT_SPLIT,
  SPLIT_KEY,
  readSplit,
  fitSplit,
  type SplitRatio,
} from "./splitLayout";
import { useWork } from "../work/store";
export type SplitTranslator = (key: "chatResizeLeft" | "chatResizeRight" | "chatResizeHint") => string;

// useSplitLayoutBase 与数据源无关：三栏拖拽/比例持久化/最大化逻辑共用，
// demo 树与真实 API 工作区各自传入翻译函数。
export function useSplitLayoutBase(open: boolean, t: SplitTranslator) {
  const ref = useRef<HTMLDivElement>(null);
  const [ratio, setRatio] = useState(() => {
    try {
      return readSplit(JSON.parse(localStorage.getItem(SPLIT_KEY) || "null"));
    } catch {
      return { ...DEFAULT_SPLIT };
    }
  });
  const [box, setBox] = useState({
    width: 1440,
    limits: { left: 200, center: 360, right: 320, divider: 8 },
  });
  const [resizing, setResizing] = useState(false),
    [expanded, setExpanded] = useState(false);
  const latest = useRef(ratio);
  latest.current = ratio;
  const drag = useRef<{
    side: "left" | "right";
    x: number;
    start: ReturnType<typeof fitSplit>;
    ratio: SplitRatio;
    el: HTMLDivElement;
    id: number;
  } | null>(null);
  const persist = (value: SplitRatio) => {
    try {
      localStorage.setItem(SPLIT_KEY, JSON.stringify(value));
    } catch {
      /* Layout remains usable when storage is unavailable. */
    }
  };
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      if (!el.clientWidth) return;
      const style = getComputedStyle(el);
      const n = (key: string) => Number.parseFloat(style.getPropertyValue(key));
      setBox({
        width: el.clientWidth,
        limits: {
          left: n("--split-left-min"),
          center: n("--split-center-min"),
          right: n("--split-right-min"),
          divider: n("--split-divider"),
        },
      });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const fitted = fitSplit(box.width, ratio, box.limits, open);
  const cancel = () => {
    const d = drag.current;
    if (!d) return;
    drag.current = null;
    latest.current = d.ratio;
    setRatio(d.ratio);
    setResizing(false);
    if (d.el.hasPointerCapture(d.id)) d.el.releasePointerCapture(d.id);
  };
  useEffect(() => {
    const key = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape" && drag.current) {
        e.preventDefault();
        cancel();
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
  const adjust = (side: "left" | "right", pixels: number, start = fitted) => {
    const max =
      box.width -
      box.limits.divider * (open ? 2 : 1) -
      box.limits.center -
      (side === "left" ? start.right : start.left);
    const size = Math.max(box.limits[side], Math.min(max, pixels));
    const next = {
      ...latest.current,
      [side]: size / box.width,
      [side === "left" ? "right" : "left"]:
        (side === "left"
          ? start.right || box.width * latest.current.right
          : start.left) / box.width,
    };
    latest.current = next;
    setRatio(next);
    return next;
  };
  const reset = (side: "left" | "right") =>
    persist(adjust(side, box.width * DEFAULT_SPLIT[side]));
  const divider = (side: "left" | "right") => {
    const name = t(side === "left" ? "chatResizeLeft" : "chatResizeRight");
    return (
      <div
        className="judex-pane-divider"
        role="separator"
        aria-label={name}
        title={t("chatResizeHint")}
        aria-orientation="vertical"
        aria-valuemin={box.limits[side]}
        aria-valuemax={Math.floor(
          box.width -
            box.limits.center -
            (side === "left" ? fitted.right : fitted.left) -
            box.limits.divider * (open ? 2 : 1),
        )}
        aria-valuenow={Math.round(fitted[side])}
        tabIndex={0}
        data-testid={"resize-" + side}
        onDoubleClick={() => reset(side)}
        onPointerDown={(e: PointerEvent<HTMLDivElement>) => {
          if (e.button !== 0) return;
          e.preventDefault();
          e.currentTarget.setPointerCapture(e.pointerId);
          drag.current = {
            side,
            x: e.clientX,
            start: fitted,
            ratio: { ...ratio },
            el: e.currentTarget,
            id: e.pointerId,
          };
          setResizing(true);
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (d && d.side === side)
            adjust(
              side,
              d.start[side] + (e.clientX - d.x) * (side === "left" ? 1 : -1),
              d.start,
            );
        }}
        onPointerUp={(e) => {
          if (!drag.current) return;
          drag.current = null;
          setResizing(false);
          persist(latest.current);
          if (e.currentTarget.hasPointerCapture(e.pointerId))
            e.currentTarget.releasePointerCapture(e.pointerId);
        }}
        onPointerCancel={cancel}
        onLostPointerCapture={() => {
          if (drag.current) cancel();
        }}
        onKeyDown={(e: KeyboardEvent) => {
          if (
            ["ArrowLeft", "ArrowRight", "Home", "End", "Enter"].includes(e.key)
          ) {
            e.preventDefault();
            if (e.key === "Enter") {
              reset(side);
              return;
            }
            const delta =
              (e.shiftKey ? 48 : 12) *
              (e.key === "ArrowLeft" ? -1 : 1) *
              (side === "left" ? 1 : -1);
            persist(
              adjust(
                side,
                e.key === "Home"
                  ? box.limits[side]
                  : e.key === "End"
                    ? box.width
                    : fitted[side] + delta,
              ),
            );
          }
        }}
      >
        <span />
      </div>
    );
  };
  useEffect(() => {
    if (!open) setExpanded(false);
  }, [open]);
  const toggleWide = () => setExpanded((value) => !value);
  const maximized = open && expanded;
  return {
    ref,
    resizing,
    expanded: maximized,
    restore: () => setExpanded(false),
    toggleWide,
    divider,
    columns: maximized
      ? `${fitted.left}px ${box.limits.divider}px minmax(0,1fr)`
      : open
        ? `${fitted.left}px ${box.limits.divider}px minmax(0,1fr) ${box.limits.divider}px ${fitted.right}px`
        : `${fitted.left}px ${box.limits.divider}px minmax(0,1fr)`,
  };
}

export function useSplitLayout(open: boolean) {
  const { t } = useWork();
  return useSplitLayoutBase(open, t);
}
