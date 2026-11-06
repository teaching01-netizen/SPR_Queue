const { test } = require('node:test');
const assert = require('node:assert/strict');
const mod = import('../frontend/src/consult-times.ts');

test('elapsed time is anchored to the server clock, not the device clock', async () => {
  const { elapsedMs } = await mod;
  // Server says the consult started at 05:50 and its clock is 06:00 when the
  // packet was produced: ten minutes used, no matter what the device claims.
  const started = '2026-11-06T05:50:00Z';
  const server = '2026-11-06T06:00:00Z';
  assert.equal(elapsedMs(started, server, 5_000, 5_000), 600_000);
  // Three seconds after the packet arrived, the timer has advanced by three
  // seconds only - a wildly wrong device clock cannot move the anchor.
  assert.equal(elapsedMs(started, server, 5_000, 8_000), 603_000);
  // A device clock far ahead/behind still only measures the delivery gap.
  assert.equal(elapsedMs(started, server, 900_000, 902_000), 602_000);
});

test('elapsed time falls back to the device clock when the server clock is absent', async () => {
  const { elapsedMs } = await mod;
  assert.equal(elapsedMs('2026-11-06T05:50:00Z', null, 0, Date.parse('2026-11-06T05:55:00Z')), 300_000);
});

test('elapsed time refuses unknown or negative input', async () => {
  const { elapsedMs, parseStamp } = await mod;
  assert.equal(elapsedMs(undefined, '2026-11-06T06:00:00Z', 0, 0), null);
  assert.equal(elapsedMs('not-a-time', '2026-11-06T06:00:00Z', 0, 0), null);
  assert.equal(elapsedMs('2026-11-06T06:00:00Z', '2026-11-06T05:00:00Z', 0, 0), 0);
  assert.equal(parseStamp(''), null);
  assert.equal(parseStamp(null), null);
  assert.equal(parseStamp('2026-11-06T06:00:00Z'), Date.parse('2026-11-06T06:00:00Z'));
});

test('running consult interval renders as mm:ss', async () => {
  const { intervalLabel } = await mod;
  assert.equal(intervalLabel(0), '00:00');
  assert.equal(intervalLabel(252_000), '04:12');
  assert.equal(intervalLabel(63_400), '01:03');
  assert.equal(intervalLabel(-5_000), '00:00');
  assert.equal(intervalLabel(null), '—');
});

test('finished consult duration renders in whole minutes', async () => {
  const { durationLabel, MINUTE_MS } = await mod;
  assert.equal(MINUTE_MS, 60_000);
  assert.equal(durationLabel(21 * MINUTE_MS), '21 นาที');
  assert.equal(durationLabel(14 * MINUTE_MS + 20_000), '14 นาที');
  assert.equal(durationLabel(0), '0 นาที');
  assert.equal(durationLabel(null), '—');
});

test('estimate and clock labels use the event time zone', async () => {
  const { estimateLabel, clockFor } = await mod;
  // 10:20 and 10:07 in Asia/Bangkok.
  assert.equal(estimateLabel({ startAt: '2026-11-06T03:20:00Z' }), '~10:20');
  assert.equal(estimateLabel({ startAt: '2026-11-06T03:20:00Z', late: true }), '~10:20');
  assert.equal(estimateLabel(undefined), '—');
  assert.equal(estimateLabel(null), '—');
  assert.equal(clockFor('2026-11-06T03:07:00Z'), '10:07');
  assert.equal(clockFor(''), '—');
});
