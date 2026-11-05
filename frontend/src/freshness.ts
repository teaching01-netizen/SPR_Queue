// Freshness model for queue data shown to parents and on the public board.
// The server snapshot is the only source of truth; these bands only describe its age.
export const FRESH_MS = 15_000;
export const ANCIENT_MS = 2 * 60 * 60 * 1000;

export type Freshness = 'fresh' | 'delayed' | 'offline' | 'ancient';

export type FreshnessInput = { ageMs: number | null; offline: boolean };

export function freshnessBand({ ageMs, offline }: FreshnessInput): Freshness {
  if (ageMs !== null && ageMs > ANCIENT_MS) return 'ancient';
  if (offline) return 'offline';
  if (ageMs === null || ageMs > FRESH_MS) return 'delayed';
  return 'fresh';
}

export function clockLabel(timestamp: number | null): string {
  if (!timestamp) return '—';
  try {
    return new Intl.DateTimeFormat('th-TH', {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      timeZone: 'Asia/Bangkok',
    }).format(new Date(timestamp));
  } catch {
    return '—';
  }
}

export type BannerTone = 'delayed' | 'offline' | 'ancient';

export function bannerTone(band: Freshness): BannerTone | null {
  if (band === 'fresh') return null;
  return band;
}
