import type { Seat, WorkState } from './types.ts';

export const isMySeat = (state: WorkState, seat: Seat) =>
  (!seat.status || seat.status === 'active') &&
  (state.currentUserId ? seat.userId === state.currentUserId : seat.person === state.currentUser);

export const myPositions = (state: WorkState, projectId: string) =>
  state.positions.filter(position => position.projectId === projectId &&
    state.seats.some(seat => seat.positionId === position.id && isMySeat(state, seat)));

export const myPreference = (state: WorkState, projectId: string, positionId: string) =>
  state.preferences.find(preference => preference.projectId === projectId && preference.positionId === positionId &&
    (state.currentUserId ? preference.userId === state.currentUserId : preference.person === state.currentUser));
