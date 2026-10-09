import {expect, type Page} from '@playwright/test';
import {createHash, randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {command} from './cooperation-business-helpers';
export async function uploadMaterial(page: Page, root: string, name: string, content?: Buffer, purpose='用于真实资料验收', options?:{kind:string;entrypoint?:string}) {
  const raw = content ?? fs.readFileSync(path.resolve('tests/fixtures/materials', name));
  const digest = (bytes: Buffer) => createHash('sha256').update(bytes).digest('hex');
  const session = await command(page, 'get', '/auth/session');
  const up = await command(page, 'post', root + '/uploads', {name, size: raw.length, sha256: digest(raw), mime: 'application/octet-stream', kind: options?.kind??'file', entrypoint:options?.entrypoint, purpose});
  for (let n = 0; n < up.partCount; n++) {
    const part = raw.subarray(n * up.partSize, (n + 1) * up.partSize);
    const response = await page.request.put('/api/v1' + root + '/uploads/' + up.id + '/parts/' + (n + 1), {data: part, headers: {'X-CSRF-Token': session.csrfToken, 'X-Judex-Part-SHA256': digest(part)}});
    expect(response.ok(), await response.text()).toBe(true);
  }
  return command(page, 'post', root + '/uploads/' + up.id + '/complete', {});
}
export async function materialWrite(page: Page, method: string, path: string, body: unknown) {
  const session = await command(page, 'get', '/auth/session');
  return page.request.fetch('/api/v1' + path, {method, data: body, headers: {'X-CSRF-Token': session.csrfToken, 'Idempotency-Key': randomUUID()}});
}
export async function stableMaterialDialog(page: Page) {
  await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.locator('.judex-dialog-container[data-entering]')).toHaveCount(0);
}
export async function chooseMaterialContext(page: Page, index: number, value: string) {
  await page.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(index).click();
  await page.locator('[data-option-value="' + value + '"]').click();
}
