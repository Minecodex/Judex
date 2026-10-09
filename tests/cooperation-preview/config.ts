import {defineConfig} from "@playwright/test";
export default defineConfig({
 testDir:".",testMatch:"*.spec.ts",timeout:45000,workers:1,retries:0,
 use:{baseURL:"http://127.0.0.1:5195",viewport:{width:1440,height:1000},screenshot:"only-on-failure",trace:"retain-on-failure"},
 outputDir:"../results/cooperation-preview",reporter:[["list"]],
 webServer:{command:"node tests/cooperation-preview/server.mjs",cwd:"../..",url:"http://127.0.0.1:5195",reuseExistingServer:false,timeout:10000},
});
