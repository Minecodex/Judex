import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'.',testMatch:'prerequisites.spec.ts',timeout:90000,workers:1,retries:0,
  use:{baseURL:'http://127.0.0.1:18182',viewport:{width:1440,height:1000},trace:'retain-on-failure',screenshot:'only-on-failure'},
  outputDir:`../../.cache/real-host/20261012-closure/prerequisite-browser/${process.env.JUDEX_BROWSER_ATTEMPT ?? 'attempt-2'}`,reporter:[['list']]});
