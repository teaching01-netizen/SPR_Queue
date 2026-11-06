// Chapter 15–20: export, operator view, public display/kiosk, mobile screens,
// and edge states (invalid link, empty filter, expired session).
import {
  BASE, ADMIN, OPERATOR, check, failures, loadState, open, shotter, chapter,
  apiLogin, waitDashboard, selectView, setQueueScope, packet,
} from './lib.mjs';

const state = loadState();
const names = state.names;
// ---- Admin session: export + give the public board an active queue ----------
const admin = await open();
const { context, page } = admin;
const csrf = await apiLogin(context, ADMIN);
await page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(page);

chapter('c15', '15 · ส่งออก Excel ให้ทีมงาน', 'ข้อมูลคิวทั้งหมดดาวน์โหลดเป็นไฟล์ Excel ได้จากหน้า Staff');
const shotExport = shotter(page, 'c15');
const [download] = await Promise.all([
  page.waitForEvent('download', { timeout: 20000 }),
  page.getByRole('button', { name: 'Export Excel' }).click(),
]);
await page.waitForTimeout(1200);
const filename = download.suggestedFilename();
check('excel download produced a file', /^SPR_Queue_Registrations_.+\.xlsx$/.test(filename), filename);
await page.locator('.staff-header-actions').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotExport('01-export-notice', 'Export Excel สำเร็จ', `ไฟล์ ${filename} มีผู้ลงทะเบียนทุกวัน/ทุกวิชา เฉพาะที่ยังแสดงอยู่`, { full: false });

// A live queue makes the public board and mobile chapter meaningful.
await selectView(page, 'จัดคิว');
await setQueueScope(page, { level: 'High Level', date: '2026-11-06', stream: 'Math', periodText: '14:00' });
await page.waitForTimeout(500);
const mathRow = page.locator('.queue-card tbody tr', { hasText: names.queueB });
if (await mathRow.getByRole('button', { name: 'เรียกคิว' }).count()) {
  await mathRow.getByRole('button', { name: 'เรียกคิว' }).click();
  await page.waitForTimeout(1200);
  await mathRow.getByRole('button', { name: 'เริ่มปรึกษา' }).click();
  await page.waitForTimeout(1800);
}

chapter('c17', '17 · จอแสดงคิวที่หน้างาน', 'จอทีวีและหน้าเว็บสาธารณะแสดงคิวที่กำลังเรียก/ปรึกษา และคิวถัดไป');
const parent = await open({ width: 1440, height: 1100 });
const shotDisplay = shotter(parent.page, 'c17');
await parent.page.goto(`${BASE}/#/display?level=High%20Level&date=2026-11-06`, { waitUntil: 'networkidle' });
await parent.page.waitForSelector('.display-grid', { timeout: 15000 });
await parent.page.waitForTimeout(1400);
await shotDisplay('01-display-board', 'จอแสดงคิว · หน้าเว็บ', 'แยก SAT Math / SAT Verbal แสดง NOW CALLING และคิวถัดไปตามลำดับที่ Staff บันทึก');
const kiosk = await open({ width: 1920, height: 1080 });
const shotKiosk = shotter(kiosk.page, 'c17');
await kiosk.page.goto(`${BASE}/#/display?level=High%20Level&date=2026-11-06&kiosk=1`, { waitUntil: 'networkidle' });
await kiosk.page.waitForTimeout(1500);
await shotKiosk('02-display-kiosk', 'โหมด Kiosk สำหรับจอในสถานที่', 'หน้าจอเต็มไม่มีเมนู เหมาะกับทีวีที่หน้างาน อัปเดตเองทุก 5 วินาที', { full: false });
await kiosk.browser.close();

chapter('c16', '16 · มุมมอง Operator (ไม่ใช่ Admin)', 'สิทธิ์ Operator จัดคิวได้ แต่ไม่เห็นการตั้งค่าหรือเครื่องมือ Admin');
const operator = await open();
await apiLogin(operator.context, OPERATOR);
await operator.page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(operator.page);
const shotOperator = shotter(operator.page, 'c16');
await operator.page.evaluate(() => window.scrollTo(0, 0));
await operator.page.waitForTimeout(300);
await shotOperator('01-operator-dashboard', 'Operator Dashboard', 'หัวจอไม่มีปุ่ม "ตั้งเวลาต่อคิว" และไม่มีเมนู Admin');
await selectView(operator.page, 'รายการยกเลิก');
await operator.page.waitForTimeout(700);
const operatorRow = operator.page.locator('.checkin-section tbody tr').first();
if (await operatorRow.count()) {
  await operatorRow.scrollIntoViewIfNeeded();
  await operator.page.waitForTimeout(300);
  await shotOperator('02-operator-row-actions', 'Operator ไม่เห็นปุ่มลบถาวร', 'ปุ่มคืนการจองยังใช้ได้ แต่ปุ่มลบถาวร/คืนสถานะเป็นของ Admin เท่านั้น', { full: false });
}
await operator.browser.close();

chapter('c18', '18 · หน้าจอมือถือ', 'ผู้ปกครองและ Staff ใช้มือถือหน้างานได้ครบ')
const mobileParent = await open({ width: 390, height: 844, mobile: true });
const shotMobileParent = shotter(mobileParent.page, 'c18');
await mobileParent.page.goto(`${BASE}/#/book`, { waitUntil: 'networkidle' });
await mobileParent.page.waitForSelector('#field-email');
await mobileParent.page.waitForTimeout(900);
await shotMobileParent('01-mobile-booking', 'หน้าจองบนมือถือ', 'ฟอร์มจองใช้งานบนจอแคบได้โดยไม่ต้องซูม', { full: false });
await mobileParent.page.goto(`${BASE}/#/track/${state.tokens.queueB}`, { waitUntil: 'networkidle' });
await mobileParent.page.waitForSelector('.parent-ticket', { timeout: 15000 });
await mobileParent.page.waitForTimeout(1400);
await shotMobileParent('02-mobile-tracking-serving', 'ลิงก์ติดตามคิวบนมือถือ · กำลังปรึกษา', 'ผู้ปกครองเห็นสถานะล่าสุดและตำแหน่งคิวของตัวเอง', { full: false });
await mobileParent.browser.close();

const mobileStaff = await open({ width: 390, height: 844, mobile: true });
await apiLogin(mobileStaff.context, ADMIN);
await mobileStaff.page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(mobileStaff.page);
const shotMobileStaff = shotter(mobileStaff.page, 'c18');
await mobileStaff.page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await mobileStaff.page.waitForTimeout(400);
await shotMobileStaff('03-mobile-checkin', 'Staff เช็กอินบนมือถือ', 'ตารางเช็กอินแปลงเป็นการ์ด อ่านง่ายและกดปุ่มได้สะดวก', { full: false });
await selectView(mobileStaff.page, 'จัดคิว');
await setQueueScope(mobileStaff.page, { level: 'High Level', date: '2026-11-06', stream: 'Math', periodText: '14:00' });
await mobileStaff.page.waitForTimeout(500);
await mobileStaff.page.locator('.queue-card').scrollIntoViewIfNeeded();
await mobileStaff.page.waitForTimeout(400);
await shotMobileStaff('04-mobile-queue', 'จัดคิวบนมือถือ', 'คอลัมน์เวลาคิวและปุ่มเรียก/เริ่ม/จบยังใช้งานได้บนมือถือ');
const overflow = await mobileStaff.page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
check('mobile staff page has no horizontal overflow', overflow <= 0, overflow);
await mobileStaff.page.getByRole('button', { name: /ตั้งเวลาต่อคิว/ }).click();
await mobileStaff.page.waitForSelector('#consult-minutes-input');
await mobileStaff.page.waitForTimeout(400);
await shotMobileStaff('05-mobile-settings', 'ตั้งเวลาต่อคิวบนมือถือ', 'Admin ตั้งค่าเวลาต่อคิวได้จากมือถือ', { full: false });
await mobileStaff.page.locator('.modal-header .icon-btn').click();
await mobileStaff.browser.close();

chapter('c19', '19 · สถานะขอบและข้อผิดพลาด', 'ลิงก์ผิด ค้นหาไม่เจอ และ session หมดอายุ ระบบบอกผู้ใช้ชัดเจน');
const edge = await open();
const shotEdge = shotter(edge.page, 'c19');
await edge.page.goto(`${BASE}/#/track/this-token-does-not-exist`, { waitUntil: 'networkidle' });
await edge.page.waitForSelector('.card.empty', { timeout: 15000 });
await edge.page.waitForTimeout(900);
await shotEdge('01-invalid-tracking-link', 'ลิงก์ติดตามคิวไม่ถูกต้อง', 'ระบบไม่เปิดเผยข้อมูลใด ๆ และให้ติดต่อทีมงานพร้อมเบอร์โทร');
await edge.browser.close();

// A clean browser profile so the private-link tab guard from the previous page
// does not affect the session-expiry scenario.
const staffEdge = await open();
const shotEdge2 = shotter(staffEdge.page, 'c19');
await apiLogin(staffEdge.context, ADMIN);
await staffEdge.page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(staffEdge.page);
await staffEdge.page.getByLabel('ค้นหานักเรียนหรือคิว').fill('ไม่มีรายการนี้แน่นอน');
await staffEdge.page.waitForTimeout(900);
await staffEdge.page.locator('.empty').first().scrollIntoViewIfNeeded();
await staffEdge.page.waitForTimeout(300);
await shotEdge2('02-search-empty', 'ค้นหาไม่พบรายการ', 'ตารางว่างพร้อมคำแนะนำให้เปลี่ยนเงื่อนไข');
// Expire the session, then force a reload so the shell notices it.
await staffEdge.context.clearCookies();
await staffEdge.page.reload({ waitUntil: 'networkidle' });
await staffEdge.page.waitForSelector('.staff-login', { timeout: 15000 });
await staffEdge.page.waitForTimeout(600);
await shotEdge2('03-session-expired', 'Session หมดอายุหรือถูกล้าง', 'ระบบกลับไปหน้าเข้าสู่ระบบโดยไม่ค้างข้อมูล Staff เก่า', { full: false });
await staffEdge.browser.close();

chapter('c20', '20 · สรุปภาพรวมหลังจบวัน', 'สถานะสุดท้ายของคิวและรายการทั้งหมด');
const wrap = await open();
await apiLogin(wrap.context, ADMIN);
await wrap.page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(wrap.page);
const shotWrap = shotter(wrap.page, 'c20');
await shotWrap('01-final-dashboard', 'ภาพรวมคิวหลังจบวัน', 'ผู้ปกครองที่จบแล้ว/ยกเลิก/รอคิว แยกสถานะชัดเจน พร้อมเวลาของทุกคน');
await selectView(wrap.page, 'รายการจบแล้ว');
await wrap.page.waitForTimeout(700);
await shotWrap('02-final-completed', 'รายการที่จบแล้วทั้งวัน', 'เก็บเวลาเริ่ม–จบของทุกรายไว้ตรวจสอบย้อนหลัง');
await wrap.browser.close();

const allErrors = [...admin.errors, ...parent.errors, ...edge.errors, ...staffEdge.errors, ...wrap.errors];
check('no page errors in misc chapters', allErrors.length === 0, allErrors.join(' | '));
console.log('FAILURES', failures);
process.exit(failures.length ? 1 : 0);
