import type { SlotEstimate } from './types';
import { clockLabel } from './freshness.ts';

// Display helpers for the staff consult clock. Pure functions so the
// server-anchored elapsed timer can be verified without a browser.
export const MINUTE_MS = 60_000;

export function parseStamp(value?: string | null): number | null {
  if (!value) return null;
  const at = Date.parse(value);
  return Number.isNaN(at) ? null : at;
}

// Server-anchored elapsed time. The packet's server clock measures the consult;
// the local clock only adds the time passed since the packet arrived, so a
// skewed staff device never changes a reported duration.
export function elapsedMs(
  startedAt: string | null | undefined,
  serverTime: string | null | undefined,
  receivedAtMs: number,
  nowMs: number,
): number | null {
  const started = parseStamp(startedAt);
  if (started === null) return null;
  const server = parseStamp(serverTime);
  if (server === null) return Math.max(0, nowMs - started);
  return Math.max(0, server - started + (nowMs - receivedAtMs));
}

// mm:ss for a running consult ("04:12").
export function intervalLabel(ms: number | null): string {
  if (ms === null || !Number.isFinite(ms)) return '—';
  const total = Math.max(0, Math.floor(ms / 1000));
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}

// Whole minutes for a finished consult ("21 นาที").
export function durationLabel(ms: number | null): string {
  if (ms === null || !Number.isFinite(ms)) return '—';
  return `${Math.max(0, Math.round(ms / MINUTE_MS))} นาที`;
}

// Event-local clock label for a saved timestamp ("10:07"), "—" when unknown.
export function clockFor(value?: string | null): string {
  return clockLabel(parseStamp(value));
}

// Predicted start of a queued slot ("~10:20"), "—" when there is no estimate.
export function estimateLabel(estimate?: SlotEstimate | null): string {
  const at = parseStamp(estimate?.startAt);
  return at === null ? '—' : `~${clockLabel(at)}`;
}
