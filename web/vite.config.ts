import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import {pdfSupportAssets} from './src/build/pdfSupportAssets';
const api=process.env.JUDEX_API_PROXY||'http://127.0.0.1:8080';
export default defineConfig({resolve:{dedupe:["react","react-dom"]},plugins:[react(),tailwindcss(),pdfSupportAssets()],optimizeDeps:{include:['mermaid']},server:{proxy:{'/api':api,'/downloads':api,'/healthz':api,'/readyz':api}},build:{sourcemap:false}});
