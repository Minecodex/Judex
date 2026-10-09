import {defineConfig} from "@playwright/test";
export default defineConfig({
 testDir:".",testMatch:"*.spec.ts",timeout:45000,workers:1,retries:0,
 use:{baseURL:"http://127.0.0.1:5197",viewport:{width:1440,height:1000},screenshot:"only-on-failure",trace:"retain-on-failure"},
 outputDir:"../../.cache/material-preview-20261008/test-results",reporter:[["list"]],
 webServer:{command:"node tests/material-preview/server.mjs",cwd:"../..",url:"http://127.0.0.1:5197",reuseExistingServer:true,timeout:10000},
});
