// Chapter 3–9: staff signs in, checks in the families, builds the queue, calls,
// serves (including an overrun), changes the slot duration, ends and archives.
import {
  BASE, ADMIN, check, failures, loadState, saveState, open, shotter, chapter,
  waitDashboard, selectView, setQueueScope, refreshStaff, packet, backdateStage,
  staffApi,
} from './lib.mjs';

const state = loadState();
const names = state.names;
const { browser, context, page, errors } = await open();
const parentContext = await browser.newContext({
  viewport: { width: 1440, height: 1100 },
  locale: 'th-TH',
  timezoneId: 'Asia/Bangkok',
  baseURL: BASE,
});
const parent = await parentContext.newPage();

const rowOf = (nickname, table = '.queue-card') => page.locator(`${table} tbody tr`, { hasText: nickname });
async function viewportShot(shot, name, title, detail, locator) {
  if (locator) await locator.scrollIntoViewIfNeeded();
  await page.waitForTimeout(350);
  await shot(name, title, detail, { full: false });
}
async function clickRowButton(nickname, label, table = '.queue-card') {
  await rowOf(nickname, table).getByRole('button', { name: label }).first().click();
  await page.waitForTimeout(1100);
}
async function openParentTrack(token) {
  await parent.goto(`${BASE}/#/track/${token}`, { waitUntil: 'networkidle' });
  await parent.waitForTimeout(1400);
}
async function openDisplay({ kiosk = false } = {}) {
  const suffix = kiosk ? '&kiosk=1' : '';
  await parent.goto(`${BASE}/#/display?level=High%20Level&date=2026-11-06${suffix}`, { waitUntil: 'networkidle' });
  await parent.waitForSelector('.display-grid, .kiosk-board, .online-board', { timeout: 15000 });
  await parent.waitForTimeout(1200);
}

chapter('c03', '3 · Staff เปิดวันและเข้าสู่ระบบ', 'Login หน้า Staff Dashboard และรายการรอเช็กอินของทั้งวัน');
const shot = shotter(page, 'c03');

await page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await page.waitForSelector('.staff-login');
await page.waitForTimeout(500);
await shot('01-login', 'หน้า Staff Login', 'เฉพาะทีมงาน Warwick หน้านี้ไม่เกี่ยวกับลิงก์ผู้ปกครอง');
await page.locator('#staff-username').fill(ADMIN.username);
await page.locator('#staff-password').fill('wrong-password');
await page.getByRole('button', { name: 'เข้าสู่ระบบ' }).click();
await page.waitForSelector('.staff-login .notice.error', { timeout: 10000 });
await page.waitForTimeout(300);
await shot('02-login-error', 'รหัสผ่านไม่ถูกต้อง', 'ระบบตอบ 401 และไม่เปิดเผยว่ามีบัญชีนี้อยู่หรือไม่', { full: false });

await page.locator('#staff-password').fill(ADMIN.password);
await page.getByRole('button', { name: 'เข้าสู่ระบบ' }).click();
await waitDashboard(page);
await shot('03-dashboard-checkin', 'Staff Dashboard · เช็กอินรวมทุกช่วงเวลา', 'รายการจองทั้งหมดของวันนี้พร้อมปุ่มเช็กอิน ยังไม่มีคิวถูกจัด');

// Resolve the hero booking id and keep it for the later chapters.
const initial = await packet(page);
for (const [key, nickname] of Object.entries(names)) {
  const found = initial.bookings.find((booking) => booking.nickname === nickname);
  if (found) state.ids[key] = found.id;
}
saveState(state);

chapter('c04', '4 · ผู้ปกครองมาถึงและเช็กอิน', 'Staff เช็กอินให้ผู้ปกครอง แล้วสถานะเปลี่ยนเป็นรอเรียกคิว');
const shotCheckin = shotter(page, 'c04');
await viewportShot(shotCheckin, '01-checkin-before', 'แถวของน้องมีนก่อนเช็กอิน', 'สถานะ "รอเช็กอิน" ยังไม่มีคิวหรือเวลาคิว', rowOf(names.hero, '.checkin-section'));
await rowOf(names.hero, '.checkin-section').getByRole('button', { name: 'เช็กอิน' }).click();
await page.waitForTimeout(1200);
await viewportShot(shotCheckin, '02-checkin-after', 'เช็กอินสำเร็จ', 'สถานะเปลี่ยนเป็น "รอเรียกคิว" และขึ้นเครื่องหมายเช็กอินแล้ว', rowOf(names.hero, '.checkin-section'));

// The rest of the day's families check in too; this is the normal morning rush.
for (const nickname of [names.queueB, names.queueC, names.combined, names.noshow, names.verbal1, names.verbal2, names.verbal3]) {
  const row = rowOf(nickname, '.checkin-section');
  const button = row.getByRole('button', { name: 'เช็กอิน' });
  if (await button.count()) { await button.click(); await page.waitForTimeout(750); }
}
await refreshStaff(page);
await openParentTrack(state.tokens.hero);
const shotTrack = shotter(parent, 'c04');
await shotTrack('03-tracking-after-checkin', 'ผู้ปกครองเห็นสถานะหลังเช็กอิน', 'ลิงก์เดิมอัปเดตเองทุก 5 วินาที · ตอนนี้ยังไม่มีลำดับคิวจนกว่า Staff จะบันทึกคิว');

chapter('c05', '5 · จัดลำดับคิวและประกาศเวลาคิว', 'Staff เรียงลำดับ บันทึกคิว แล้วผู้ปกครอง/จอแสดงคิวเห็นรหัสและเวลาคิว');
const shotQueue = shotter(page, 'c05');
await selectView(page, 'จัดคิว');
await setQueueScope(page, { level: 'High Level', date: '2026-11-06', stream: 'Math', periodText: '14:00' });
await page.waitForTimeout(600);

// Put the first-arriving family at the front, as the staff would, then save.
for (let n = 0; n < 3; n++) {
  await rowOf(names.hero).getByRole('button', { name: `เลื่อน ${names.hero} ขึ้น` }).click();
  await page.waitForTimeout(250);
}
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await shotQueue('01-queue-before-save', 'จัดลำดับก่อนบันทึก', 'ยังไม่มีรหัสคิว ลำดับถูกเลื่อนไว้ชั่วคราว และคอลัมน์เวลาคิวเป็น "—"');
await page.locator('.queue-toolbar .btn.primary').click();
await page.waitForTimeout(1600);
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await shotQueue('02-queue-after-save', 'บันทึกคิวแล้ว · รหัสและเวลาคิวประมาณ', 'ทุกแถวได้รหัสคิว ลำดับ 1–3 และเวลาคิวโดยประมาณห่างกันตามเวลาต่อคิว');

await openParentTrack(state.tokens.hero);
const shotTrackQueue = shotter(parent, 'c05');
await shotTrackQueue('03-tracking-queue-position', 'ผู้ปกครองเห็นจำนวนคิวที่เหลือ', 'น้องมีนเป็นคิวแรก ระบบบอกว่าเป็นคิวถัดไปและให้เตรียมตัว');
await openDisplay();
const shotDisplay = shotter(parent, 'c05');
await shotDisplay('04-display-upcoming', 'จอแสดงคิว · คิวถัดไป', 'จอประชาสัมพันธ์แสดงรหัสคิวที่รอในห้อง Math ตามลำดับที่ Staff บันทึก');

chapter('c06', '6 · เรียกคิว', 'Staff เรียกคิวถัดไป ผู้ปกครองเห็นสถานะถูกเรียก และจอแสดงคิวเปลี่ยนเป็น Now Calling');
const shotCall = shotter(page, 'c06');
await clickRowButton(names.hero, 'เรียกคิว');
await viewportShot(shotCall, '01-called-row', 'แถวที่ถูกเรียก', 'สถานะ "กำลังเรียก" เวลาเรียกถูกบันทึก และมีปุ่มเริ่มปรึกษา/พักคิว/ไม่มา', rowOf(names.hero));
await page.locator('.staff-live').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotCall('02-live-banner', 'แบนเนอร์ห้องที่กำลังให้บริการ', 'ห้อง Math แสดงคิวที่กำลังเรียก ไม่ให้เรียกซ้อนอีกคิวในห้องเดียวกัน', { full: false });
await openParentTrack(state.tokens.hero);
const shotTrackCall = shotter(parent, 'c06');
await shotTrackCall('03-tracking-called', 'ผู้ปกครองถูกเรียก', 'หน้าติดตามคิวขึ้น "ถึงคิวของคุณแล้ว" พร้อมบอกให้ไปที่ห้อง');
await openDisplay();
const shotDisplayCall = shotter(parent, 'c06');
await shotDisplayCall('04-display-calling', 'จอแสดงคิวยามเรียก', 'NOW CALLING แสดงรหัสที่กำลังเรียก และ "คิวถัดไป" ยังคงเตรียมพร้อม');

chapter('c07', '7 · เริ่มปรึกษา · จับเวลา · ใช้เวลาเกิน', 'เวลาเริ่มถูกบันทึกและเดินจริง ทุกคนเห็นเวลาเดียวกัน');
const shotServe = shotter(page, 'c07');
await clickRowButton(names.hero, 'เริ่มปรึกษา');
await page.waitForTimeout(2200);
await viewportShot(shotServe, '01-serving-timer', 'เริ่มปรึกษาแล้ว · ตัวจับเวลาเดินจริง', 'แถวแสดง "เริ่ม HH:mm · ใช้ไป mm:ss · คาดจบ HH:mm" โดยอิงนาฬิกาเซิร์ฟเวอร์', rowOf(names.hero));

// Simulate time passing: the consult has been running for 20 minutes
// (called a minute before it started, as in a real session).
backdateStage(state.ids.hero, 'Math', 'calledAt', 21 * 60 * 1000);
backdateStage(state.ids.hero, 'Math', 'startedAt', 20 * 60 * 1000);
await refreshStaff(page);
await page.waitForTimeout(900);
await viewportShot(shotServe, '02-serving-overrun', 'คิวใช้เวลาเกินกำหนด', 'เมื่อเกินเวลาต่อคิว ตัวเลขเปลี่ยนเป็น "เกิน mm:ss" และคิวถัดไปถูกเลื่อนตามเวลาจริง', rowOf(names.hero));
await openParentTrack(state.tokens.hero);
const shotTrackServe = shotter(parent, 'c07');
await shotTrackServe('03-tracking-serving', 'ผู้ปกครองเห็นว่ากำลังปรึกษา', 'หน้าติดตามคิวบอกว่ากำลังรับการปรึกษาอยู่');
await openDisplay();
const shotDisplayServe = shotter(parent, 'c07');
await shotDisplayServe('04-display-serving', 'จอแสดงคิวยามปรึกษา', 'NOW CALLING / SERVING แสดงรหัสที่กำลังรับบริการ');

chapter('c08', '8 · Admin ปรับเวลาต่อคิว', 'เวลาต่อคิวใช้คำนวณทุกห้อง มีผลทันทีโดยไม่ต้อง deploy');
const shotSettings = shotter(page, 'c08');
await page.locator('.staff-header-actions').scrollIntoViewIfNeeded();
await page.getByRole('button', { name: /ตั้งเวลาต่อคิว/ }).click();
await page.waitForSelector('#consult-minutes-input');
await page.waitForTimeout(300);
await shotSettings('01-settings-modal', 'หน้าตั้งเวลาต่อคิว (Admin เท่านั้น)', 'ค่าเริ่มต้น 10 นาที ใช้กับทุกห้องและทุกช่วงเวลาของกิจกรรมนี้', { full: false });
await page.locator('#consult-minutes-input').fill('20');
await page.getByRole('button', { name: 'บันทึกเวลาต่อคิว' }).click();
await page.waitForTimeout(1500);
await page.locator('.staff-header-actions').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotSettings('02-settings-saved', 'บันทึกค่าใหม่แล้ว', 'ปุ่มบนหัวจอแสดง 20 นาที ทุกเครื่องของ Staff เห็นค่าเดียวกันภายใน 5 วินาที', { full: false });

// The Verbal room shows the new 20-minute spacing; the last family falls past
// the period end and is flagged.
await selectView(page, 'จัดคิว');
await setQueueScope(page, { level: 'High Level', date: '2026-11-06', stream: 'Verbal', periodText: '14:00' });
await page.waitForTimeout(400);
await page.locator('.queue-toolbar .btn.primary').click();
await page.waitForTimeout(1600);
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotSettings('03-queue-late-spacing', 'คิวห้อง Verbal ใช้ช่วง 20 นาที', 'คิวที่เลยเวลาจบรอบจะขึ้น "เกินรอบ" เพื่อให้ Staff เห็นว่าต้องบริหารเวลา');

chapter('c09', '9 · จบการปรึกษา · เก็บประวัติ', 'Staff กดจบ ระบบบันทึกช่วงเวลาและระยะเวลาจริงของผู้ปกครองรายนั้น');
const shotDone = shotter(page, 'c09');
await setQueueScope(page, { level: 'High Level', date: '2026-11-06', stream: 'Math', periodText: '14:00' });
await page.waitForTimeout(400);
await clickRowButton(names.hero, 'จบการปรึกษา');
await viewportShot(shotDone, '01-completed-row', 'แถวที่จบการปรึกษาแล้ว', 'แสดงช่วงเวลาเริ่ม–จบ และจำนวนนาทีที่ใช้จริง', rowOf(names.hero));
await rowOf(names.hero).locator('.student-btn').click();
await page.waitForSelector('.modal .detail-list');
await page.waitForTimeout(500);
await shotDone('02-detail-modal', 'รายละเอียดการจอง · เวลาของแต่ละห้อง', 'โมดัลแสดงเวลาปัจจุบัน เวลาของแต่ละห้อง และเวลาต่อคิวที่ใช้อยู่ในกิจกรรมนี้', { full: false });
await page.locator('.modal-header .icon-btn').click();
await page.waitForTimeout(300);
await openParentTrack(state.tokens.hero);
const shotTrackDone = shotter(parent, 'c09');
await shotTrackDone('03-tracking-completed', 'ผู้ปกครองเห็นว่าปรึกษาเสร็จสิ้น', 'สถานะเสร็จสิ้นพร้อมข้อความขอบคุณ และไม่มีการคำนวณคิวต่อ');

// Completed tab + archive keeps the history without reopening the case.
await selectView(page, 'รายการจบแล้ว');
await page.waitForTimeout(600);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
const shotCompleted = shotter(page, 'c09');
await shotCompleted('04-completed-tab', 'แท็บรายการจบแล้ว', 'รายการเสร็จสิ้นพร้อมเวลาและปุ่มเก็บประวัติ');
await rowOf(names.hero, '.checkin-section').getByRole('button', { name: 'เก็บรายการจบแล้ว' }).click();
await page.waitForTimeout(1200);
await page.getByLabel('กรองรายการจบแล้ว').selectOption('archived');
await page.waitForTimeout(800);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotCompleted('05-archived-filter', 'กรองดูเฉพาะที่เก็บประวัติแล้ว', 'การเก็บประวัติไม่คืน slot ไม่เปิดเคสใหม่ และยังตรวจย้อนหลังได้');

check('no page errors in staff chapters', errors.length === 0, errors.join(' | '));
console.log('FAILURES', failures);
await browser.close();
process.exit(failures.length ? 1 : 0);
