import { defineConfig } from "@playwright/test";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../../", import.meta.url));
export default defineConfig({
  testDir: ".",
  testMatch: ["ui-consistency.spec.ts","cooperation.spec.ts","cooperation-closure.spec.ts","cooperation-remediation.spec.ts","cooperation-scope-fixes.spec.ts","position-presets.spec.ts","workflow-presets.spec.ts","workflow-advanced.spec.ts","team-assignment.spec.ts","tab-feedback.spec.ts","surface-borders.spec.ts","work.spec.ts","chat-layout.spec.ts","workspace-maximize.spec.ts","discussion-policy.spec.ts","collaboration.spec.ts","chat.spec.ts","chat-tabs.spec.ts","workspace-tabs.spec.ts","settings.spec.ts","visual-system.spec.ts"],
  timeout: 45000,
  workers: 2,
  outputDir: "../results/chat",
  reporter: [
    ["list"],
    ["html", { outputFolder: "../reports/chat", open: "never" }],
  ],
  use: {
    baseURL: "http://127.0.0.1:5176",
    channel: process.env.PLAYWRIGHT_CHANNEL ?? "msedge",
    viewport: { width: 1440, height: 1000 },
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command:
      "npm exec --workspace @judex/web -- vite --mode e2e --host 127.0.0.1 --port 5176 --strictPort",
    cwd: root,
    url: "http://127.0.0.1:5176",
    reuseExistingServer: false,
    timeout: 60000,
  },
});
