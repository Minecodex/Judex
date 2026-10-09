import {test, expect} from '@playwright/test';
import fs from 'node:fs';
import {demo4Fixture} from './demo4-fixture';
const output = '.cache/demo4-review2';fs.mkdirSync(output, {recursive: true});

// Display projection agrees with the route without rewriting the business phase.
test('D4-STATUS a blocked task has the same display in its inspector and plan context', async ({page}) => {
  const fixture = demo4Fixture(); fixture.currentUser = '顾言';
  await page.addInitScript(s => localStorage.setItem('judex.web.preview.v1', JSON.stringify(s)), fixture);
  await page.goto('/projects/leaf/plans/leaf-first/route?task=build');
  await expect(page.getByTestId('execution-task-build')).toHaveAttribute('data-stage', 'blocked');
  await expect(page.getByTestId('task-inspector').locator('.judex-task-status')).toHaveText('等待前置');
  await expect(page.getByTestId('start-task')).toBeDisabled();
  await page.screenshot({path: output + '/consistent-blocked-inspector.png', fullPage: true});
  await page.goto('/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first');
  const item = page.locator('.judex-co-context-task').filter({hasText: '功能开发'});
  await expect(item.locator('.judex-task-status')).toHaveText('等待前置');
  fs.writeFileSync(output + '/blocked-status.json', JSON.stringify({task: 'build', prerequisite: 'demo4-review', graph: 'blocked', inspector: 'blocked', context: 'blocked',startEnabled:false}, null, 2));
});
