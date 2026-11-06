// Chapter 10–14: Combined handoff, no-show and recovery, cancellation/restore,
// admin recovery/permanent delete, and swapping two reservations.
import {
  BASE, ADMIN, check, failures, loadState, saveState, open, shotter, chapter,
  apiLogin, waitDashboard, selectView, setQueueScope, refreshStaff, packet,
} from './lib.mjs';

const state = loadState();
const names = state.names;
const { browser, context, page, errors } = await open();
await apiLogin(context, ADMIN);
await page.goto(`${BASE}/#/staff`, { waitUntil: 'networkidle' });
await waitDashboard(page);

const rowOf = (nickname, table = '.queue-card') => page.locator(`${table} tbody tr`, { hasText: nickname });
async function viewportShot(shot, name, title, detail, locator) {
  if (locator) await locator.scrollIntoViewIfNeeded();
  await page.waitForTimeout(350);
  await shot(name, title, detail, { full: false });
}
async function clickRowButton(nickname, label, table = '.queue-card') {
  await rowOf(nickname, table).getByRole('button', { name: label }).first().click();
  await page.waitForTimeout(1200);
}
async function scopeQueue(level, date, stream, periodText = '14:00') {
  await selectView(page, 'จัดคิว');
  await setQueueScope(page, { level, date, stream, periodText });
  await page.waitForTimeout(400);
}

chapter('c10', '10 · เคส Math + Verbal (Combined) ส่งต่อห้อง', 'ผู้ปกครองคนเดียวใช้สองห้องต่อเนื่อง ระบบสลับห้องให้อัตโนมัติเมื่องบวิชาแรก');
const shot = shotter(page, 'c10');

await scopeQueue('High Level', '2026-11-06', 'Math');
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await viewportShot(shot, '01-combined-queue-before', 'เคส Combined ในห้อง Math', 'ผู้ปกครองจอง Math + Verbal ระบบเริ่มที่ห้อง Math และยังไม่เคยถูกเรียก');
await page.locator('.queue-toolbar .btn.primary').click();
await page.waitForTimeout(1600);
await clickRowButton(names.combined, 'เรียกคิว');
await page.waitForTimeout(500);
await clickRowButton(names.combined, 'เริ่มปรึกษา');
await page.waitForTimeout(2000);
await viewportShot(shot, '02-combined-serving-math', 'Combined กำลังปรึกษา Math', 'ตัวจับเวลาเดิน และความคืบหน้าแสดงว่า Math กำลังทำอยู่ Verbal ยังรอ');
// The Combined label reads "จบ Math → Verbal" while the first room is ending.
await rowOf(names.combined).getByRole('button', { name: /^จบ/ }).first().click();
await page.waitForTimeout(1300);
await selectView(page, 'เช็กอิน');
await page.waitForTimeout(900);
await viewportShot(shot, '03-combined-handoff', 'จบ Math แล้วส่งต่อ Verbal', 'สถานะกลับเป็นรอเรียกคิวในห้อง Verbal พร้อมข้อความแจ้ง Staff ว่าต้องจัดคิวห้อง Verbal ก่อนเรียกครั้งแรก');

// The Verbal room now needs its own saved order for the handoff case.
await scopeQueue('High Level', '2026-11-06', 'Verbal');
await page.locator('.queue-toolbar .btn.primary').click();
await page.waitForTimeout(1700);
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shot('04-combined-verbal-queue', 'ห้อง Verbal หลังส่งต่อ', 'เคส Combined เข้าคิว Verbal ตามลำดับที่ Staff บันทึก');
await clickRowButton(names.combined, 'เรียกคิว');
await page.waitForTimeout(400);
await clickRowButton(names.combined, 'เริ่มปรึกษา');
await page.waitForTimeout(1600);
await viewportShot(shot, '05-combined-serving-verbal', 'Combined กำลังปรึกษา Verbal', 'ใช้เวลาต่อคิวเดิมต่อเนื่อง ไม่จ่าย slot ใหม่');
await clickRowButton(names.combined, 'จบการปรึกษา');
await page.waitForTimeout(700);
await selectView(page, 'รายการจบแล้ว');
await page.waitForTimeout(600);
const completedRow = rowOf(names.combined, '.checkin-section');
await completedRow.scrollIntoViewIfNeeded();
await completedRow.locator('.student-btn').click();
await page.waitForSelector('.modal .detail-list');
await page.waitForTimeout(500);
await shot('06-combined-completed-both-rooms', 'รายละเอียด Combined ครบทั้งสองห้อง', 'โมดัลแสดงเวลาเรียก/เริ่ม/จบของทั้ง Math และ Verbal พร้อมระยะเวลาจริง', { full: false });
await page.locator('.modal-header .icon-btn').click();

chapter('c11', '11 · ไม่มาเข้าคิว (No-show) และคืนคิว', 'เมื่อเรียกแล้วไม่มา Staff ปิดเคสได้ และคืนเข้ารอได้โดยต้องจัดลำดับใหม่');
const shotNo = shotter(page, 'c11');
await scopeQueue('High Level', '2026-11-06', 'Verbal');
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await viewportShot(shotNo, '01-waiting-before-noshow', 'คิวก่อนเกิด No-show', 'สถานะรอเรียกคิวพร้อมลำดับในห้อง Verbal');
await clickRowButton(names.noshow, 'ไม่มา');
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await viewportShot(shotNo, '02-noshow-row', 'ปิดเคสว่าไม่มา', 'สถานะ No-show ไม่กินคิวในห้อง และลำดับของคนอื่นไม่ถูกเลื่อนโดยพลการ');
const parentContext = await browser.newContext({ viewport: { width: 1440, height: 1100 }, locale: 'th-TH', timezoneId: 'Asia/Bangkok', baseURL: BASE });
const parent = await parentContext.newPage();
await parent.goto(`${BASE}/#/track/${state.tokens.noshow}`, { waitUntil: 'networkidle' });
await parent.waitForTimeout(1400);
const shotTrack = shotter(parent, 'c11');
await shotTrack('03-tracking-noshow', 'ผู้ปกครองเห็นสถานะไม่มาเข้าคิว', 'ลิงก์เดิมยังใช้ได้ ระบบแจ้งให้ติดต่อทีมงานเพื่อตรวจสอบ');
await scopeQueue('High Level', '2026-11-06', 'Verbal');
await clickRowButton(names.noshow, 'คืนคิวเข้ารอ');
await page.locator('.queue-card').scrollIntoViewIfNeeded();
await viewportShot(shotNo, '04-noshow-restored', 'คืนคิวเข้ารอแล้ว', 'ระบบล้างลำดับเดิมและให้ Staff บันทึกคิวใหม่ เพื่อไม่ให้ตำแหน่งชนกับคนอื่น');
await page.locator('.queue-toolbar .btn.primary').click();
await page.waitForTimeout(1500);
await viewportShot(shotNo, '05-noshow-resaved', 'บันทึกคิวใหม่หลังคืนสถานะ', 'ผู้ปกครองกลับเข้าคิวได้ตามลำดับใหม่ของห้อง');

chapter('c12', '12 · ยกเลิกการจองและคืนการจอง', 'ผู้ปกครองแจ้งยกเลิก Staff เก็บเหตุผลไว้ตรวจสอบ และคืนการจองภายหลังได้');
const shotCancel = shotter(page, 'c12');
await selectView(page, 'เช็กอิน');
await page.waitForTimeout(600);
const cancelRow = rowOf(names.cancelRestore, '.checkin-section');
await cancelRow.getByRole('button', { name: /เอาออก/ }).click();
await page.waitForSelector('.modal');
await page.waitForTimeout(400);
await shotCancel('01-cancel-modal', 'โมดัลยกเลิกการจอง', 'ต้องระบุเหตุผลก่อนยืนยัน ระบบคืนที่ว่างให้รอบนั้นทันที', { full: false });
await page.locator('.modal select').selectOption({ label: 'ผู้ปกครองแจ้งยกเลิก' });
await page.locator('.modal textarea').fill('ผู้ปกครองโทรแจ้งยกเลิกช่วงเช้า');
await page.waitForTimeout(300);
await shotCancel('02-cancel-reason', 'กรอกเหตุผลการยกเลิก', 'เหตุผลถูกบันทึกในประวัติและแสดงในแท็บรายการยกเลิก', { full: false });
await page.getByRole('button', { name: 'ยืนยันยกเลิกการจอง' }).click();
await page.waitForTimeout(1500);
await selectView(page, 'รายการยกเลิก');
await page.waitForTimeout(700);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotCancel('03-cancelled-tab', 'แท็บรายการยกเลิก', 'รายการที่ยกเลิกพร้อมเหตุผล และรหัสคิวถูกคืนเข้าระบบ');
const cancelRow2 = rowOf(names.cancelRestore, '.checkin-section');
await cancelRow2.getByRole('button', { name: 'คืนการจอง' }).click();
await page.waitForSelector('.modal');
await page.waitForTimeout(500);
await shotCancel('04-restore-modal', 'โมดัลคืนการจอง', 'เลือกคืนรอบเดิมหรือรอบใหม่ได้ทันที โดยยังใช้ข้อมูลและลิงก์ติดตามเดิม', { full: false });
await page.getByRole('button', { name: /ยืนยันคืนรอบเดิม/ }).click();
await page.waitForTimeout(1600);
await selectView(page, 'เช็กอิน');
await page.waitForTimeout(600);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotCancel('05-restored-row', 'คืนการจองสำเร็จ', 'รายการกลับมาเป็นรอเช็กอิน ใช้ลิงก์เดิม และต้องเช็กอิน/จัดคิวใหม่ก่อนเรียก');

chapter('c13', '13 · Admin กู้คืนเคสและลบรายการยกเลิก', 'สิทธิ์ Admin เท่านั้นที่ย้อนสถานะหรือลบถาวรได้ พร้อมการยืนยันและเหตุผล');
const shotAdmin = shotter(page, 'c13');
await selectView(page, 'รายการจบแล้ว');
await page.waitForTimeout(700);
const archivedRow = rowOf(names.hero, '.checkin-section');
await archivedRow.scrollIntoViewIfNeeded();
await archivedRow.getByRole('button', { name: /คืนสถานะ/ }).click();
await page.waitForSelector('.modal');
await page.waitForTimeout(400);
await shotAdmin('01-recovery-modal', 'โมดัลคืนสถานะเฉพาะราย (Admin)', 'เลือกคืนเป็นรอเช็กอินหรือยกเลิกเคสที่จบแล้ว พร้อมผลข้างเคียงที่ชัดเจน', { full: false });
await page.locator('.modal select').selectOption('cancel');
await page.locator('.modal textarea').fill('ทีมงานกดจบเคสผิด จึงยกเลิกเพื่อคืนที่ว่าง');
await page.locator('.modal .checkbox-label input').check({ force: true });
await page.waitForTimeout(300);
await page.getByRole('button', { name: /ยืนยันยกเลิกและคืนที่ว่าง/ }).click();
await page.waitForTimeout(1800);
await selectView(page, 'รายการยกเลิก');
await page.waitForTimeout(700);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotAdmin('02-recovered-cancelled', 'เคสที่จบแล้วถูกย้อนเป็นยกเลิก', 'การกู้คืนไม่ลบประวัติเดิม และคืน slot ให้รอบนั้น');

// Permanently delete a cancelled record (Admin, confirmation phrase).
await selectView(page, 'เช็กอิน');
await page.waitForTimeout(600);
const bowRow = rowOf(names.cancelDelete, '.checkin-section');
await bowRow.getByRole('button', { name: /เอาออก/ }).click();
await page.waitForSelector('.modal');
await page.locator('.modal textarea').fill('ลงทะเบียนซ้ำกับรายการอื่น');
await page.getByRole('button', { name: 'ยืนยันยกเลิกการจอง' }).click();
await page.waitForTimeout(1500);
await selectView(page, 'รายการยกเลิก');
await page.waitForTimeout(700);
const bowCancelled = rowOf(names.cancelDelete, '.checkin-section');
await bowCancelled.getByRole('button', { name: /ลบถาวร/ }).click();
await page.waitForSelector('.modal');
await page.waitForTimeout(400);
await shotAdmin('03-delete-modal', 'โมดัลลบรายการยกเลิกถาวร (Admin)', 'ต้องยืนยันรายการเจาะจง ลบประวัติและงานเมลของรายการนี้เท่านั้น', { full: false });
await page.locator('.modal .checkbox-label input').check({ force: true });
await page.waitForTimeout(300);
await page.getByRole('button', { name: 'ยืนยันลบรายการนี้ถาวร' }).click();
await page.waitForTimeout(1800);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(300);
await shotAdmin('04-deleted-gone', 'ลบถาวรแล้วเหลือเฉพาะรายการยกเลิกอื่น', 'รหัสของรายการที่ลบถูกคืนเข้าระบบ รายการอื่นไม่ได้รับผลกระทบ');

chapter('c14', '14 · สลับช่วงเวลาระหว่างสองครอบครัว', 'Staff สลับรอบให้ผู้ปกครองสองรายพร้อมกันโดยไม่ต้องยกเลิกการจอง');
const shotSwap = shotter(page, 'c14');
await selectView(page, 'เช็กอิน');
await page.waitForTimeout(700);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
const swapRow = rowOf(names.swapA, '.checkin-section');
await swapRow.getByRole('button', { name: 'สลับช่วงเวลากับอีกคน' }).click();
await page.waitForSelector('.modal');
await page.waitForTimeout(500);
await shotSwap('01-swap-modal', 'โมดัลสลับช่วงเวลา', 'ค้นหาและเลือกผู้ปกครองอีกคนที่ระดับ/วิชาเดียวกันต่างรอบ', { full: false });
await page.locator('#swap-search').fill(names.swapB);
await page.waitForTimeout(400);
const swapOption = await page.locator('#swap-other option').filter({ hasText: names.swapB }).first().getAttribute('value');
await page.locator('#swap-other').selectOption(swapOption);
await page.waitForTimeout(600);
await shotSwap('02-swap-confirm', 'ตรวจผลก่อนยืนยัน', 'ระบบแสดงผลลัพธ์ทั้งสองฝ่ายและต้องยืนยันว่าทั้งคู่ตกลงสลับกัน', { full: false });
await page.locator('#swap-reason').fill('ผู้ปกครองทั้งสองรายตกลงสลับช่วงเวลากัน');
await page.locator('.modal .checkbox-label input').check({ force: true });
await page.getByRole('button', { name: 'ยืนยันสลับช่วงเวลาทั้งสองราย' }).click();
await page.waitForTimeout(2000);
await page.locator('.checkin-section').first().scrollIntoViewIfNeeded();
await page.waitForTimeout(400);
await shotSwap('03-swap-result', 'สลับช่วงเวลาสำเร็จ', 'ทั้งสองรายได้รอบใหม่พร้อมกัน รหัสคิว/ข้อมูล/ลิงก์ติดตามยังเป็นของแต่ละคน');

check('no page errors in case chapters', errors.length === 0, errors.join(' | '));
console.log('FAILURES', failures);
await browser.close();
process.exit(failures.length ? 1 : 0);
