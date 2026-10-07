export const lifecycleZh = {
  lcLoadMore: "加载更多",
  lcReceiptHint: "接收交接不会自动验收来源任务。",
} as const;
export const lifecycleEn: Record<keyof typeof lifecycleZh, string> = {
  lcLoadMore: "Load more",
  lcReceiptHint: "Receiving a handoff does not accept the source task.",
};
