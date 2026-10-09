import catalog from '../../../../internal/project/catalog/position-presets.json' with { type: 'json' };
import type { components } from '../../lib/api/schema';
import type { Position, Result, WorkState } from './types.ts';
import { words } from './types.ts';
import { manage } from './selectors.ts';
import { uid } from './seed.ts';

export type PositionPresetCatalog = components['schemas']['PositionPresetCatalog'];
export type PositionPreset = components['schemas']['PositionPreset'];
export type ImportPresetsRequest = components['schemas']['ImportPositionPresetsRequest'];
export const builtinPositionPresets: PositionPresetCatalog = catalog;
const normalized = (value: string) => value.trim().toLowerCase();

export function existingPresetPosition(positions: Position[], preset: PositionPreset) {
  const names = [normalized(preset.name.zh), normalized(preset.name.en)];
  return positions.find(position => position.presetId === preset.id ||
    [position.name.zh, position.name.en].some(name => names.includes(normalized(name))));
}

export function importDemoPositionPresets(state: WorkState, projectId: string, request: ImportPresetsRequest): Result & {createdCount?: number; skippedCount?: number} {
  if (!manage(state, projectId)) return {error: 'permission'};
  const scenario = builtinPositionPresets.scenarios.find(item => item.id === request.scenarioId);
  if (request.catalogVersion !== builtinPositionPresets.version) return {error: 'stale'};
  if (!scenario || !['zh-CN', 'en'].includes(request.locale) || !request.roleIds.length || request.roleIds.length > 32 ||
    new Set(request.roleIds).size !== request.roleIds.length || request.roleIds.some(id => !scenario.roleIds.includes(id))) return {error: 'scope'};
  const next = structuredClone(state);
  const language = request.locale === 'en' ? 'en' : 'zh';
  let createdCount = 0, skippedCount = 0;
  for (const id of request.roleIds) {
    const preset = builtinPositionPresets.roles.find(role => role.id === id);
    if (!preset) return {error: 'scope'};
    if (existingPresetPosition(next.positions.filter(position => position.projectId === projectId), preset)) {
      skippedCount++;
      continue;
    }
    next.positions.push({id: uid(), projectId, presetId: id, name: words(preset.name[language]),
      prompt: words(preset.prompt[language]), publicSummary: words(preset.summary[language]), tone: 'mint', bindings: []});
    createdCount++;
  }
  return {state: next, createdCount, skippedCount};
}
