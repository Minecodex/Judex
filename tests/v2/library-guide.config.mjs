import {defineConfig} from '@playwright/test';
import {evidencePath} from './environment.mjs';
export default defineConfig({testDir:'.',testMatch:'library-guide.spec.mjs',workers:1,timeout:180000,use:{actionTimeout:15000,viewport:{width:1600,height:1000},trace:'retain-on-failure',screenshot:'only-on-failure'},reporter:[['list'],['json',{outputFile:evidencePath('library-guide-results.json')}]],outputDir:evidencePath('library-guide-artifacts')});
