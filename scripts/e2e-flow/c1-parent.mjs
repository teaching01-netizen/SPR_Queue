// Chapter 1–2: the parent books a slot, then opens the tracking link before arriving.
import {
  BASE, check, failures, loadState, saveState, resetOutput, open, shotter, chapter,
  bookViaApi, bookingInput,
} from './lib.mjs';

const state = loadState();
state.tokens = {};
state.ids = {};
state.names = {
  hero: 'น้องมีน',
  queueB: 'น้องภูมิ',
  queueC: 'น้องเฟิร์น',
  combined: 'น้องเจ',
  noshow: 'น้องออม',
  cancelRestore: 'น้องกัน',
  cancelDelete: 'น้องโบว์',
  swapA: 'น้องสวย',
  swapB: 'น้องต้น',
  verbal1: 'น้องวี 1',
  verbal2: 'น้องวี 2',
  verbal3: 'น้องวี 3',
};
saveState(state);

resetOutput();
const { browser, context, page, errors } = await open();

// Families used by the staff chapters below. Only the hero booking is made
// through the browser form; the rest are seeded so the day already has a queue.
const seed = [
  { key: 'queueB', input: { nickname: state.names.queueB, email: 'beat@example.com', wcode: 'W1002', periodId: 'P1400_1530' } },
  { key: 'queueC', input: { nickname: state.names.queueC, email: 'fern@example.com', wcode: 'W1003', periodId: 'P1400_1530' } },
  { key: 'combined', input: { nickname: state.names.combined, email: 'jay@example.com', wcode: 'W1004', periodId: 'P1400_1530', subject: 'Combined' } },
  { key: 'noshow', input: { nickname: state.names.noshow, email: 'aom@example.com', wcode: 'W1005', periodId: 'P1400_1530', subject: 'Verbal' } },
  { key: 'cancelRestore', input: { nickname: state.names.cancelRestore, email: 'kan@example.com', wcode: 'W1006', periodId: 'P1530_1700' } },
  { key: 'cancelDelete', input: { nickname: state.names.cancelDelete, email: 'bow@example.com', wcode: 'W1007', periodId: 'P1530_1700' } },
  { key: 'verbal1', input: { nickname: 'น้องวี 1', email: 'vee1@example.com', wcode: 'W1011', periodId: 'P1400_1530', subject: 'Verbal' } },
  { key: 'verbal2', input: { nickname: 'น้องวี 2', email: 'vee2@example.com', wcode: 'W1012', periodId: 'P1400_1530', subject: 'Verbal' } },
  { key: 'verbal3', input: { nickname: 'น้องวี 3', email: 'vee3@example.com', wcode: 'W1013', periodId: 'P1400_1530', subject: 'Verbal' } },
  { key: 'swapA', input: { nickname: state.names.swapA, email: 'suay@example.com', wcode: 'W1008', level: 'Beginner', subject: 'Math', date: '2026-11-07', periodId: 'P1300_1400' } },
  { key: 'swapB', input: { nickname: state.names.swapB, email: 'ton@example.com', wcode: 'W1009', level: 'Beginner', subject: 'Math', date: '2026-11-07', periodId: 'P1700_1800' } },
];
for (const item of seed) {
  const created = await bookViaApi(context, bookingInput(item.input), `seed-${item.key}`);
  state.ids[item.key] = created.booking.id;
  state.tokens[item.key] = created.token;
}
// Fill the 13:00 Beginner Math period (capacity 4) so a full round is visible.
for (const n of [1, 2, 3]) {
  const created = await bookViaApi(context, bookingInput({
    nickname: `น้องทดสอบ ${n}`, email: `filler${n}@example.com`, wcode: `W20${n}`,
    level: 'Beginner', subject: 'Math', date: '2026-11-07', periodId: 'P1300_1400',
  }), `seed-filler-${n}`);
  state.ids[`filler${n}`] = created.booking.id;
}
saveState(state);

chapter('c01', '1 · ผู้ปกครองจองรอบ Consult', 'เปิดหน้าจอง กรอกข้อมูล เลือกรอบ รับลิงก์ติดตามคิว และเห็นกรณีรอบถูกแย่งที่');
const shot = shotter(page, 'c01');

await page.goto(`${BASE}/#/book`, { waitUntil: 'networkidle' });
await page.waitForSelector('#field-email');
await page.waitForTimeout(800);
await shot('01-booking-page', 'หน้าจอง Private Consult', 'หน้าจองสาธารณะแสดงรอบว่าง จำนวน slots และเวลาต่อคิวที่ดึงจาก Backend · รอบ 13:00 ที่เต็มแล้วแสดงสถานะ "เต็มแล้ว"');

await page.getByLabel('Email ผู้ปกครอง').fill('parent.meen@example.com');
await page.locator('#field-nickname').fill(state.names.hero);
await page.locator('#field-school').fill('Demo School');
await page.locator('#field-wcode').fill('W1001');
await page.locator('textarea[name=question1]').fill('น้องควรโฟกัสอะไรก่อนสอบครั้งต่อไป');
await page.locator('textarea[name=question2]').fill('ควรวางแผนอ่านหนังสืออย่างไรให้ทันเวลา');

// Realistic race: this family picked a Verbal round, but another family takes
// the last seats before the submit reaches the server. The UI keeps showing the
// availability it last read, and the server rejects the booking.
await page.locator('.subject-card').filter({ hasText: 'SAT Verbal' }).click();
await page.waitForTimeout(900);
const stalePeriods = await (await context.request.get(`${BASE}/api/periods?level=Beginner&subject=Verbal&date=2026-11-07`)).text();
await page.route('**/api/periods*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: stalePeriods }));
await page.locator('input[name=seminar]').check({ force: true });
await page.locator('.period-card').filter({ hasText: '17:00' }).first().click();
await page.waitForTimeout(400);
for (const n of [1, 2, 3, 4]) {
  await bookViaApi(context, bookingInput({
    nickname: `น้องแย่งที่ ${n}`, email: `race${n}@example.com`, wcode: `W30${n}`,
    level: 'Beginner', subject: 'Verbal', date: '2026-11-07', periodId: 'P1700_1800',
  }), `seed-race-${n}`);
}
await page.locator('button.submit-booking').click();
await page.waitForSelector('.booking-form .notice.error', { timeout: 15000 });
await page.locator('.booking-form .notice.error').scrollIntoViewIfNeeded();
await page.waitForTimeout(500);
await shot('02-booking-period-taken', 'รอบถูกแย่งที่ก่อนกดยืนยัน', 'เซิร์ฟเวอร์ตอบ PERIOD_FULL หน้าจองแจ้ง "ช่วงเวลานี้เต็มแล้ว กรุณาเลือกช่วงอื่น" และไม่ตัดยอดจองเดิม', { full: false });
await page.unroute('**/api/periods*');

// Hero booking on 6 November, then the success page carries the private link.
await page.goto(`${BASE}/#/book`, { waitUntil: 'networkidle' });
await page.reload({ waitUntil: 'networkidle' });
await page.waitForSelector('#field-email');
await page.getByLabel('Email ผู้ปกครอง').fill('parent.meen@example.com');
await page.locator('#field-nickname').fill(state.names.hero);
await page.locator('#field-school').fill('Demo School');
await page.locator('#field-wcode').fill('W1001');
await page.locator('textarea[name=question1]').fill('น้องควรโฟกัสอะไรก่อนสอบครั้งต่อไป');
await page.locator('textarea[name=question2]').fill('ควรวางแผนอ่านหนังสืออย่างไรให้ทันเวลา');
await page.locator('.level-picker button').filter({ hasText: 'High Level' }).click();
await page.waitForTimeout(700);
await page.locator('.subject-card').filter({ hasText: 'SAT Math' }).click();
await page.waitForTimeout(800);
await page.locator('.period-card').filter({ hasText: '14:00' }).first().click();
await page.waitForTimeout(500);
await shot('03-booking-filled', 'กรอกข้อมูลครบและเลือกรอบ 14:00–15:30', 'สรุปรอบที่เลือก จำนวน slots และรูปแบบ On-site แสดงในการ์ดด้านข้างก่อนกดยืนยัน');
await page.locator('button.submit-booking').click();
await page.waitForSelector('.success-page', { timeout: 15000 });
await page.waitForTimeout(700);
const successText = await page.locator('.success-details').innerText();
check('hero booking is High Level Math 14:00', successText.includes('SAT Math') && successText.includes('14:00'), successText);
const trackHref = await page.locator('.success-actions a').first().getAttribute('href');
state.tokens.hero = decodeURIComponent(trackHref.split('#/track/')[1]);
saveState(state);
await shot('04-booking-success', 'จองสำเร็จ พร้อมลิงก์ติดตามคิว', 'หน้ายืนยันการจองแสดงวิชา รอบ และสถานะอีเมล พร้อมลิงก์ติดตามคิวส่วนตัวที่ผู้ปกครองต้องเก็บไว้');

chapter('c02', '2 · ผู้ปกครองเปิดลิงก์ติดตามคิวก่อนถึงวัน', 'สถานะก่อนเช็กอินยังไม่มีการคำนวณคิว');
const shotTrack = shotter(page, 'c02');
await page.goto(`${BASE}/#/track/${state.tokens.hero}`, { waitUntil: 'networkidle' });
await page.waitForSelector('.parent-ticket', { timeout: 15000 });
await page.waitForTimeout(1300);
await shotTrack('01-tracking-before-checkin', 'ลิงก์ติดตามคิวก่อนเช็กอิน', 'ผู้ปกครองเห็นสถานะรอเช็กอิน และข้อความ "เช็กอินกับ Staff ก่อน ระบบจึงจะคำนวณคิวให้"');

const mobile = await open({ width: 390, height: 844, mobile: true });
const shotMobile = shotter(mobile.page, 'c02');
await mobile.page.goto(`${BASE}/#/track/${state.tokens.hero}`, { waitUntil: 'networkidle' });
await mobile.page.waitForSelector('.parent-ticket', { timeout: 15000 });
await mobile.page.waitForTimeout(1300);
await shotMobile('02-tracking-mobile', 'ลิงก์ติดตามคิวบนมือถือ', 'หน้าติดตามคิวคือหน้าแรกที่ผู้ปกครองเปิดตอนมาถึงหน้างาน');
await mobile.browser.close();

check('no browser errors in parent chapters', errors.length === 0, errors.join(' | '));
console.log('FAILURES', failures);
await browser.close();
process.exit(failures.length ? 1 : 0);
