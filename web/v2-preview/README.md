# V2 UI preview source

Build and preview from the repository root:

```sh
npm run build:v2
npm run preview:v2
```

The self-contained output is `docs/plans/v2/demo.html`. Product scope and simulation boundaries are documented in [PREVIEW.md](../../docs/plans/v2/PREVIEW.md).

`App` owns the shell; `Projects`, `Sidebar`, `Chat` and `Files` compose the screens. `settings`, `Knowledge` and `dialogs` own focused dialogs. `context` owns local state and simulated interactions; `model` and `seed` contain types and sample data. `ui` is the shared HeroUI adapter. Page styles and tokens live in `web/src/styles/v2-preview*.css`, with bilingual copy in `web/src/i18n/v2Preview.ts`.

The shared acceptance scenarios in `tests/v2-preview/scenarios.mjs` run with either a Playwright Page or the CUA `tab.playwright` facade. The standard runner is `npm run test:v2`; it requires an installed Chromium browser. No V1 backend or model configuration is used.
