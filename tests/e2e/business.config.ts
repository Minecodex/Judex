import { defineConfig } from "@playwright/test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));

// P6-03 双用户生产 E2E（docs/plans/v1/10）：production build + 真实 API。
// 前置（由 tests/e2e/business.setup.mjs 准备）：Docker PG + MinIO + 生产
// web 构建由 judex-server 托管。此配置只启动 Playwright。
export default defineConfig({
  testDir: fileURLToPath(new URL("./",import.meta.url)),
  testMatch: /business\..*\.spec\.ts/,
  timeout: 60000,
  retries: 0,
  workers: 1,
  use: {
    baseURL: process.env.JUDEX_E2E_BASE_URL ?? "http://127.0.0.1:18090",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  outputDir: process.env.JUDEX_E2E_ARTIFACT
    ? `${process.env.JUDEX_E2E_ARTIFACT}/${process.env.JUDEX_REAL_MODEL_SMOKE === '1' ? 'real-model-results' : 'business-results'}`
    : `${root}tests/results/business`,
  reporter: [["list"]],
});
