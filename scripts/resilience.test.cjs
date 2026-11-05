const { test } = require('node:test');
const assert = require('node:assert/strict');

class MemoryStorage {
  constructor() { this.map = new Map(); this.failWrites = false; }
  getItem(key) { return this.map.has(key) ? this.map.get(key) : null; }
  setItem(key, value) {
    if (this.failWrites) throw new Error('QuotaExceededError');
    this.map.set(key, String(value));
  }
  removeItem(key) { this.map.delete(key); }
  clear() { this.map.clear(); }
}

const local = new MemoryStorage();
const session = new MemoryStorage();
Object.defineProperty(globalThis, 'localStorage', { value: local, configurable: true, writable: true });
Object.defineProperty(globalThis, 'sessionStorage', { value: session, configurable: true, writable: true });

const routesMod = import('../frontend/src/routes.ts');
const freshnessMod = import('../frontend/src/freshness.ts');
const storageMod = import('../frontend/src/storage.ts');

async function reset() {
  local.clear();
  local.failWrites = false;
  session.clear();
}

test('parseHash extracts the tracking token from the private link', async () => {
  const { parseHash } = await routesMod;
  const route = parseHash('#/track/abcdef0123456789abcdef0123456789');
  assert.equal(route.page, 'track');
  assert.equal(route.token, 'abcdef0123456789abcdef0123456789');
  assert.equal(route.kiosk, false);
});

test('parseHash keeps encoded tokens and rejects malformed encoding safely', async () => {
  const { parseHash } = await routesMod;
  assert.equal(parseHash('#/track/tok%2Fwith%2Fslashes').token, 'tok/with/slashes');
  assert.equal(parseHash('#/track/%E0%A4%A').token, '');
  assert.equal(parseHash('#/track/').token, '');
});

test('parseHash reads kiosk board scope from the query string', async () => {
  const { parseHash } = await routesMod;
  const route = parseHash('#/display?level=High%20Level&date=2026-11-07&kiosk=1');
  assert.equal(route.page, 'display');
  assert.equal(route.kiosk, true);
  assert.equal(route.level, 'High Level');
  assert.equal(route.date, '2026-11-07');
});

test('parseHash defaults an empty hash to the booking page', async () => {
  const { parseHash } = await routesMod;
  assert.equal(parseHash('').page, 'book');
  assert.equal(parseHash('#').page, 'book');
  assert.equal(parseHash('#/').page, 'book');
});

test('anonymous visitors never get the portal shell', async () => {
  const { shellFor } = await routesMod;
  assert.equal(shellFor('track'), 'parent');
  assert.equal(shellFor('display'), 'kiosk');
  assert.equal(shellFor('book'), 'parent');
  assert.equal(shellFor('staff'), 'auth');
  assert.equal(shellFor('unknown-route'), 'parent');
});

test('a signed-in staff member keeps the portal shell on every surface', async () => {
  const { shellFor } = await routesMod;
  const staff = { staff: true };
  assert.equal(shellFor('track', staff), 'portal');
  assert.equal(shellFor('display', staff), 'portal');
  assert.equal(shellFor('book', staff), 'portal');
  assert.equal(shellFor('staff', staff), 'portal');
  assert.equal(shellFor('unknown-route', staff), 'portal');
});

test('kiosk mode outranks an ambient staff session', async () => {
  const { shellFor } = await routesMod;
  assert.equal(shellFor('display', { staff: true, kiosk: true }), 'kiosk');
  assert.equal(shellFor('display', { staff: false, kiosk: true }), 'kiosk');
  assert.equal(shellFor('display', { staff: true, kiosk: false }), 'portal');
  // kiosk=1 on a non-board page must not turn it into a kiosk
  assert.equal(shellFor('track', { staff: false, kiosk: true }), 'parent');
});

test('the bare site URL is the staff entry point', async () => {
  const { isBaseEntry } = await routesMod;
  assert.equal(isBaseEntry(''), true);
  assert.equal(isBaseEntry('#'), true);
  assert.equal(isBaseEntry('#/'), true);
  assert.equal(isBaseEntry('#/book'), false);
  assert.equal(isBaseEntry('#/track/abc'), false);
  assert.equal(isBaseEntry('#/display?kiosk=1'), false);
});

test('the private-link guard applies to anonymous tabs only', async () => {
  const { shouldGuard } = await routesMod;
  assert.equal(shouldGuard('book', true, false), true);
  assert.equal(shouldGuard('staff', true, false), true);
  assert.equal(shouldGuard('book', true, true), false);
  assert.equal(shouldGuard('staff', true, true), false);
  assert.equal(shouldGuard('book', false, false), false);
  assert.equal(shouldGuard('display', true, false), false);
});

test('a private-link tab remembers that it is a parent surface and can leave for the staff login', async () => {
  const { markParentTab, readParentTab, clearParentTab, PARENT_TAB_KEY } = await routesMod;
  await reset();
  assert.equal(readParentTab(), false);
  markParentTab();
  assert.equal(session.getItem(PARENT_TAB_KEY), '1');
  assert.equal(readParentTab(), true);
  clearParentTab();
  assert.equal(readParentTab(), false);
  assert.equal(session.getItem(PARENT_TAB_KEY), null);
});

test('displayHref only includes the parameters that are set', async () => {
  const { displayHref } = await routesMod;
  assert.equal(displayHref('', ''), '#/display');
  assert.equal(displayHref('Beginner', '2026-11-07'), '#/display?level=Beginner&date=2026-11-07');
  assert.equal(displayHref('Beginner', '2026-11-07', true), '#/display?level=Beginner&date=2026-11-07&kiosk=1');
});

test('freshness bands describe age and connectivity, not guesses', async () => {
  const { freshnessBand, FRESH_MS, ANCIENT_MS } = await freshnessMod;
  assert.equal(freshnessBand({ ageMs: 0, offline: false }), 'fresh');
  assert.equal(freshnessBand({ ageMs: FRESH_MS, offline: false }), 'fresh');
  assert.equal(freshnessBand({ ageMs: FRESH_MS + 1, offline: false }), 'delayed');
  assert.equal(freshnessBand({ ageMs: null, offline: false }), 'delayed');
  assert.equal(freshnessBand({ ageMs: 1000, offline: true }), 'offline');
  assert.equal(freshnessBand({ ageMs: ANCIENT_MS + 1, offline: true }), 'ancient');
  assert.equal(freshnessBand({ ageMs: ANCIENT_MS - 1, offline: true }), 'offline');
});

test('clockLabel formats a timestamp in Bangkok time', async () => {
  const { clockLabel } = await freshnessMod;
  const label = clockLabel(Date.UTC(2026, 10, 5, 7, 5));
  assert.match(label, /14/);
  assert.match(label, /05/);
  assert.equal(clockLabel(null), '—');
});

test('a queue snapshot survives a reload and is cleared when the token stops existing', async () => {
  const { readSnapshot, writeSnapshot, clearSnapshot } = await storageMod;
  await reset();
  const token = 'token-abc';
  assert.equal(readSnapshot(token), null);
  writeSnapshot(token, { queueNo: 'Q12', status: 'Calling' });
  const cached = readSnapshot(token);
  assert.equal(cached.data.queueNo, 'Q12');
  assert.equal(cached.data.status, 'Calling');
  assert.equal(typeof cached.fetchedAt, 'number');
  clearSnapshot(token);
  assert.equal(readSnapshot(token), null);
});

test('corrupt or partial cache entries are ignored instead of crashing the page', async () => {
  const { readSnapshot, readEventConfig, snapshotKey } = await storageMod;
  await reset();
  local.setItem(snapshotKey('t1'), '{not json');
  assert.equal(readSnapshot('t1'), null);
  local.setItem(snapshotKey('t2'), JSON.stringify({ fetchedAt: 'soon', data: { a: 1 } }));
  assert.equal(readSnapshot('t2'), null);
  local.setItem(snapshotKey('t3'), JSON.stringify({ data: { a: 1 } }));
  assert.equal(readSnapshot('t3'), null);
  local.setItem('spr.v1.event', JSON.stringify({ savedAt: 1 }));
  assert.equal(readEventConfig(), null);
});

test('event config is cached so the tracking page can render without the API', async () => {
  const { readEventConfig, writeEventConfig } = await storageMod;
  await reset();
  const config = { event: { id: 'performance-nov-2026' }, rules: [{ level: 'Beginner' }] };
  writeEventConfig(config);
  assert.deepEqual(readEventConfig(), config);
});

test('kiosk board scope is remembered across reloads', async () => {
  const { readDisplayScope, writeDisplayScope } = await storageMod;
  await reset();
  assert.equal(readDisplayScope(), null);
  writeDisplayScope({ level: 'Beginner', date: '2026-11-07' });
  assert.deepEqual(readDisplayScope(), { level: 'Beginner', date: '2026-11-07' });
});

test('the staff identity cache renders the portal shell before the probe resolves', async () => {
  const { readStaffIdentity, writeStaffIdentity, clearStaffIdentity } = await storageMod;
  await reset();
  assert.equal(readStaffIdentity(), null);
  writeStaffIdentity({ username: 'RD1', role: 'admin' });
  assert.deepEqual(readStaffIdentity(), { username: 'RD1', role: 'admin' });
  local.setItem('spr.v1.staff', JSON.stringify({ role: 'admin' }));
  assert.equal(readStaffIdentity(), null);
  writeStaffIdentity({ username: 'SM2', role: 'staff' });
  clearStaffIdentity();
  assert.equal(readStaffIdentity(), null);
});

test('a storage quota failure never breaks writing to the cache', async () => {
  const { writeSnapshot, readSnapshot } = await storageMod;
  await reset();
  local.failWrites = true;
  assert.doesNotThrow(() => writeSnapshot('quota', { a: 1 }));
  assert.equal(readSnapshot('quota'), null);
});
