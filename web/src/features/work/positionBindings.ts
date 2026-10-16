import type { PositionValue } from './storeTypes';
import type { Position } from './types';

export function positionBindings(value: Pick<PositionValue, 'bindings' | 'flowId' | 'nodeId'>): Position['bindings'] | null {
  const bindings = (value.bindings ?? (value.flowId || value.nodeId
    ? [{ flowId: value.flowId ?? '', nodeId: value.nodeId ?? '' }] : []))
    .filter(b => b.flowId || b.nodeId);
  if (bindings.some(b => !b.flowId || !b.nodeId)) return null;
  if (new Set(bindings.map(b => `${b.flowId}/${b.nodeId}`)).size !== bindings.length) return null;
  return bindings.map(b => ({ flowId: b.flowId, nodeId: b.nodeId }));
}
