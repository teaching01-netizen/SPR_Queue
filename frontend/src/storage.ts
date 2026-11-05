// Versioned local cache for the public surfaces: last event config, last queue
// snapshot per tracking token, and the kiosk board scope. Payloads contain no
// personal data (queue codes/statuses only); the tracking token itself stays in
// the URL hash and is never sent to this layer from the server.
const PREFIX = 'spr.v1.';
const EVENT_KEY = `${PREFIX}event`;
const SNAPSHOT_PREFIX = `${PREFIX}snap.`;
const DISPLAY_KEY = `${PREFIX}display`;

type StorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function store(): StorageLike | null {
  try {
    const candidate = globalThis.localStorage;
    return candidate ?? null;
  } catch {
    return null;
  }
}

export function readJSON<T>(key: string): T | null {
  const target = store();
  if (!target) return null;
  try {
    const raw = target.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

export function writeJSON(key: string, value: unknown): void {
  const target = store();
  if (!target) return;
  try {
    target.setItem(key, JSON.stringify(value));
  } catch {
    // quota exceeded or storage disabled: cache is best effort
  }
}

export function removeKey(key: string): void {
  const target = store();
  if (!target) return;
  try {
    target.removeItem(key);
  } catch {
    // ignore
  }
}

export type EventConfigCache<T> = { value: T; savedAt: number };

export function readEventConfig<T>(): T | null {
  const cached = readJSON<EventConfigCache<T>>(EVENT_KEY);
  return cached && cached.value ? cached.value : null;
}

export function writeEventConfig<T>(value: T): void {
  const payload: EventConfigCache<T> = { value, savedAt: Date.now() };
  writeJSON(EVENT_KEY, payload);
}

export type Snapshot<T> = { data: T; fetchedAt: number };

export function snapshotKey(token: string): string {
  return `${SNAPSHOT_PREFIX}${token}`;
}

export function readSnapshot<T>(token: string): Snapshot<T> | null {
  const cached = readJSON<Snapshot<T>>(snapshotKey(token));
  if (!cached || typeof cached.fetchedAt !== 'number' || cached.data === null || cached.data === undefined) return null;
  return {
    data: cached.data,
    fetchedAt: cached.fetchedAt,
  };
}

export function writeSnapshot<T>(token: string, data: T): void {
  const payload: Snapshot<T> = { data, fetchedAt: Date.now() };
  writeJSON(snapshotKey(token), payload);
}

export function clearSnapshot(token: string): void {
  removeKey(snapshotKey(token));
}

export type DisplayScopeCache = { level: string; date: string };

export function readDisplayScope(): DisplayScopeCache | null {
  const cached = readJSON<DisplayScopeCache>(DISPLAY_KEY);
  if (!cached || typeof cached.level !== 'string' || typeof cached.date !== 'string') return null;
  return cached;
}

export function writeDisplayScope(scope: DisplayScopeCache): void {
  writeJSON(DISPLAY_KEY, scope);
}

// Staff identity cache: only a username/role label, used to render the portal shell
// immediately on reload. Authoritative checks always come from /api/auth/me.
export const STAFF_CACHE_KEY = `${PREFIX}staff`;

export type StaffIdentity = { username: string; role: string };

export function readStaffIdentity(): StaffIdentity | null {
  const cached = readJSON<StaffIdentity>(STAFF_CACHE_KEY);
  if (!cached || typeof cached.username !== 'string' || !cached.username) return null;
  return { username: cached.username, role: typeof cached.role === 'string' ? cached.role : '' };
}

export function writeStaffIdentity(identity: StaffIdentity): void {
  writeJSON(STAFF_CACHE_KEY, { username: identity.username, role: identity.role });
}

export function clearStaffIdentity(): void {
  removeKey(STAFF_CACHE_KEY);
}

// Last tracking token opened on this device. It lets the public board offer a
// "back to my queue" link without embedding the private token in a URL that can
// be shared, screenshotted or projected.
export const TRACK_TOKEN_KEY = `${PREFIX}track.token`;

export function isTrackToken(value: string | null | undefined): boolean {
  return typeof value === 'string' && value.length >= 32 && value.length <= 100;
}

export function readTrackToken(): string | null {
  const cached = readJSON<{ token?: string }>(TRACK_TOKEN_KEY);
  return isTrackToken(cached && cached.token) ? (cached as { token: string }).token : null;
}

export function writeTrackToken(token: string): void {
  if (!isTrackToken(token)) return;
  writeJSON(TRACK_TOKEN_KEY, { token });
}

export function clearTrackToken(): void {
  removeKey(TRACK_TOKEN_KEY);
}
