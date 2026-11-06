// Shared helpers for the Playwright consult-day storyboard run.
import { chromium } from 'playwright-core';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

export const BASE = process.env.FLOW_BASE ?? 'http://127.0.0.1:18081';
export const OUT = process.env.FLOW_OUT ?? new URL('../../docs/e2e-2026-11-06-full-flow/', import.meta.url).pathname;
export const STATE = process.env.FLOW_STATE ?? '/tmp/warwick-flow/state.json';
export const CAPS = process.env.FLOW_CAPS ?? '/tmp/warwick-flow/captions.json';
export const DB = process.env.FLOW_DB ?? 'warwick_flow_demo';
export const ADMIN = { username: 'warwick-admin', password: 'flow-demo-password-123' };
export const OPERATOR = { username: 'warwick-operator', password: 'operator-test-password-123' };

export const failures = [];
export function check(label, ok, detail = '') {
  console.log((ok ? 'PASS ' : 'FAIL ') + label + (detail && !ok ? ` :: ${detail}` : ''));
  if (!ok) failures.push(label);
}

export function sql(query) {
  return execFileSync('psql', ['-d', DB, '-q', '-t', '-A', '-c', query], { encoding: 'utf8' }).trim();
}
export function sqlLit(value) {
  return "'" + String(value).replace(/'/g, "''") + "'";
}
// Simulates time passing: moves a saved stage timestamp into the past.
export function backdateStage(bookingId, stream, field, msAgo) {
  const value = new Date(Date.now() - msAgo).toISOString();
  sql(`UPDATE bookings SET data=jsonb_set(data,'{stages,${stream},${field}}',to_jsonb(${sqlLit(value)}::text)) WHERE id=${sqlLit(bookingId)}`);
}
export function readStage(bookingId, stream, field) {
  const raw = sql(`SELECT data->'stages'->${sqlLit(stream)}->>${sqlLit(field)} FROM bookings WHERE id=${sqlLit(bookingId)}`);
  return raw;
}

export function loadState() {
  try { return JSON.parse(fs.readFileSync(STATE, 'utf8')); } catch { return {}; }
}
export function saveState(state) {
  fs.writeFileSync(STATE, JSON.stringify(state, null, 2));
}
export function loadCaps() {
  try { return JSON.parse(fs.readFileSync(CAPS, 'utf8')); } catch { return { chapters: {}, shots: [] }; }
}
export function saveCaps(caps) {
  fs.writeFileSync(CAPS, JSON.stringify(caps, null, 2));
}

export function resetOutput() {
  fs.mkdirSync(OUT, { recursive: true });
  for (const file of fs.readdirSync(OUT)) if (file.endsWith('.png')) fs.unlinkSync(path.join(OUT, file));
  saveCaps({ chapters: {}, shots: [] });
}

export async function open({ width = 1440, height = 1100, mobile = false } = {}) {
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  const context = await browser.newContext({
    viewport: { width, height },
    deviceScaleFactor: mobile ? 2 : 1,
    isMobile: mobile,
    hasTouch: mobile,
    locale: 'th-TH',
    timezoneId: 'Asia/Bangkok',
    baseURL: BASE,
  });
  const page = await context.newPage();
  // Only uncaught page errors count as failures. Console messages are recorded
  // separately because the public shell intentionally probes /api/auth/me (401)
  // and the SDK CSS references CDN fonts that the CSP blocks.
  const errors = [];
  const consoleErrors = [];
  page.on('pageerror', (error) => errors.push(String(error)));
  page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(message.text()); });
  return { browser, context, page, errors, consoleErrors };
}

export function shotter(page, chapter) {
  return async function shot(name, title, detail = '', options = {}) {
    await page.waitForTimeout(220);
    if (options.full !== false) {
      // Fixed-position chrome is painted at the scroll offset during a
      // full-page capture, so always start from the top.
      await page.evaluate(() => window.scrollTo(0, 0));
      await page.waitForTimeout(280);
    }
    const file = `${chapter}-${name}.png`;
    await page.screenshot({
      path: path.join(OUT, file),
      fullPage: options.full !== false,
      animations: 'disabled',
      ...(options.clip ? { clip: options.clip } : {}),
    });
    const caps = loadCaps();
    caps.shots = caps.shots.filter((item) => item.file !== file);
    caps.shots.push({ file, chapter, title, detail });
    saveCaps(caps);
    console.log('SHOT', file);
  };
}

export function chapter(id, title, intro) {
  const caps = loadCaps();
  caps.chapters[id] = { title, intro };
  saveCaps(caps);
}

export async function apiLogin(context, user = ADMIN) {
  const response = await context.request.post(`${BASE}/api/auth/login`, {
    data: { username: user.username, password: user.password },
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok()) throw new Error(`login failed: ${response.status()} ${await response.text()}`);
  const me = await context.request.get(`${BASE}/api/auth/me`);
  const session = await me.json();
  return session.csrfToken;
}

export function staffApi(context, csrf) {
  const send = async (method, pathname, data) => {
    const response = await context.request.fetch(`${BASE}${pathname}`, {
      method,
      data,
      headers: { 'X-CSRF-Token': csrf, 'Content-Type': 'application/json' },
    });
    if (!response.ok()) throw new Error(`${method} ${pathname} -> ${response.status()} ${await response.text()}`);
    return response;
  };
  return {
    action: (id, status, revision, reason = '') => send('POST', `/api/staff/bookings/${id}/actions`, { status, expectedRevision: revision, reason }),
    assign: (scope, bookingIds, expectedRevision) => send('PUT', '/api/staff/queue-order', { scope, bookingIds, expectedRevision }),
    settings: (minutes) => send('PUT', '/api/staff/settings/consult-minutes', { minutes }),
    archive: (id, archived, expectedRevision) => send('POST', `/api/staff/bookings/${id}/archive`, { archived, expectedRevision }),
    swap: (id, body) => send('POST', `/api/staff/bookings/${id}/swap`, body),
    recovery: (id, body) => send('POST', `/api/staff/bookings/${id}/recovery`, body),
    deleteCancelled: (id, expectedRevision) => send('DELETE', `/api/staff/bookings/${id}/cancelled`, { expectedRevision, confirmation: `DELETE CANCELLED BOOKING ${id}` }),
  };
}

export async function bookViaApi(context, input, key) {
  const response = await context.request.post(`${BASE}/api/bookings`, {
    data: input,
    headers: { 'Idempotency-Key': key },
  });
  if (!response.ok()) throw new Error(`booking failed: ${response.status()} ${await response.text()}`);
  return response.json();
}

export function bookingInput({ nickname, email, level = 'High Level', subject = 'Math', date = '2026-11-06', periodId = 'P1400_1530', wcode = '' }) {
  return {
    email,
    wcode,
    nickname,
    school: 'Demo School',
    level,
    subject,
    date,
    periodId,
    questions: ['น้องควรโฟกัสอะไรก่อนสอบครั้งต่อไป', 'ควรวางแผนอ่านหนังสืออย่างไรให้ทันเวลา'],
    seminarConfirmed: true,
  };
}

export async function packet(page) {
  return page.evaluate(() => fetch('/api/staff/bookings', { cache: 'no-store' }).then((response) => response.json()));
}

export async function findBooking(page, nickname) {
  const view = await packet(page);
  const booking = view.bookings.find((item) => item.nickname === nickname);
  if (!booking) throw new Error(`booking not found: ${nickname}`);
  return { booking, view };
}

export async function reload(page, hash = '#/staff') {
  await page.goto(`${BASE}/${hash}`);
  await page.evaluate(() => { location.hash = location.hash || '#/staff'; });
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForTimeout(500);
}

export async function openTrack(page, token) {
  await page.goto(`${BASE}/#/track/${token}`);
  await page.waitForTimeout(1300);
}

export async function waitDashboard(page) {
  await page.waitForSelector('.staff-view-tabs', { timeout: 20000 });
  await page.waitForTimeout(900);
}

export async function selectView(page, name) {
  await page.locator('.staff-view-tabs button').filter({ hasText: name }).first().click();
  await page.waitForTimeout(700);
}

export async function setQueueScope(page, { level, date, stream, periodText }) {
  await page.getByLabel('ระดับสำหรับจัดคิว').selectOption(level);
  await page.waitForTimeout(250);
  await page.getByLabel('วันที่สำหรับจัดคิว').selectOption(date);
  await page.waitForTimeout(250);
  await page.getByLabel('ห้องที่ให้บริการ').selectOption(stream);
  await page.waitForTimeout(250);
  if (periodText) {
    await page.locator('.staff-periods button').filter({ hasText: periodText }).first().click();
    await page.waitForTimeout(350);
  }
}

export async function refreshStaff(page) {
  await page.getByRole('button', { name: 'รีเฟรช' }).first().click();
  await page.waitForTimeout(1100);
}
