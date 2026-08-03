// ─── Agent step round navigation ───
import type { AgentStep } from './types';

/** Filter steps by round. round < 0 means all steps. */
export function filterStepsByRound(steps: AgentStep[], round: number): AgentStep[] {
  return round < 0 ? steps : steps.filter(s => s.round === round);
}

/** Navigate to previous round. Returns new round index or -1 if already at start. */
export function prevRoundIndex(current: number): number {
  return current > 0 ? current - 1 : -1;
}

/** Navigate to next round. Returns new round index or -1 if already at max. */
export function nextRoundIndex(current: number, max: number): number {
  return current < max ? current + 1 : -1;
}
