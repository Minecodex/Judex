import { expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import path from 'node:path';

// SQL fixtures may only mutate the database owned by this acceptance run.
export function fixtureSQL(sql: string): string {
  const artifact = path.basename(process.env.JUDEX_E2E_ARTIFACT ?? '');
  const container = process.env.JUDEX_E2E_PG_CONTAINER;
  const options = { encoding: 'utf8' as const, windowsHide: true };
  if (container) {
    const runId = process.env.JUDEX_E2E_RUN_ID;
    expect(runId).toMatch(/^judex-e2e-\d+-\d+$/);
    expect(artifact).toBe(runId);
    expect(container).toBe(`${runId}-pg`);
    const info = JSON.parse(execFileSync('docker', ['inspect', container], options))[0];
    expect(info.Config.Labels['judex.test-run']).toBe(runId);
    return execFileSync('docker', ['exec', container, 'psql', '-U', 'postgres', '-d', 'judex', '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], options);
  }

  expect(artifact).toMatch(/^judex-ui-[a-f0-9]{8}$/);
  const ns = JSON.parse(execFileSync('kubectl', ['get', 'namespace', artifact, '-o', 'json'], options));
  expect(ns.metadata.labels['judex.dev/test-run']).toBe(artifact);
  return execFileSync('kubectl', ['-n', artifact, 'exec', 'statefulset/ui-postgresql', '--', 'psql', '-U', 'judex', '-d', 'judex', '-v', 'ON_ERROR_STOP=1', '-At', '-c', sql], options);
}
