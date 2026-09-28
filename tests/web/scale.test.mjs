// P6-04 规模样本（工程测量而非容量承诺）：10k 消息分页切片性能。
import { test } from "node:test";
assertPerformance();

function assertPerformance() {
  const messages = Array.from({ length: 10000 }, (_, i) => ({
    id: `m${i}`,
    seq: i + 1,
    content: "消息 ".repeat(20),
    createdAt: new Date(2026, 0, 1, 0, 0, i).toISOString(),
  }));
  test("10k 消息按 seq keyset 分页切片 < 5ms/页", () => {
    const pageSize = 50;
    const start = Date.now();
    let cursor = 0;
    let pages = 0;
    while (cursor < messages.length) {
      const page = messages.filter((m) => m.seq > cursor && m.seq <= cursor + pageSize);
      cursor += pageSize;
      pages++;
      if (page.length === 0) break;
    }
    const elapsed = Date.now() - start;
    if (pages !== 200) throw new Error(`pages=${pages}`);
    if (elapsed > 500) throw new Error(`10k 分页耗时 ${elapsed}ms 超预算`);
  });
  test("窗口渲染取最近 100 条切片正确", () => {
    const recent = messages.slice(-100);
    if (recent.length !== 100 || recent[99].seq !== 10000) throw new Error("slice wrong");
  });
}
