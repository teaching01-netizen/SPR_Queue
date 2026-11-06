const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const file = 'file://' + path.resolve('/Users/rd-cream/Downloads/SPR_sch/prototype/staff-dual-room-board.html');
const out = '/Users/rd-cream/Downloads/SPR_sch/prototype/shots';
fs.mkdirSync(out, { recursive: true });

(async () => {
  const browser = await chromium.launch({ executablePath: process.env.HOME + '/Library/Caches/ms-playwright/chromium-1217/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing' });
  const errors = [];
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });
  page.on('pageerror', (e) => errors.push('PAGEERROR: ' + e.message));
  page.on('console', (m) => { if (m.type() === 'error') errors.push('CONSOLE: ' + m.text()); });
  await page.goto(file, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(600);
  await page.screenshot({ path: out + '/01-board-1440.png' });

  // check-in drawer
  await page.click('[data-action="open-checkin"]');
  await page.waitForTimeout(300);
  await page.screenshot({ path: out + '/02-checkin-drawer.png' });
  await page.keyboard.press('Escape');
  await page.waitForTimeout(200);

  // detail drawer
  await page.click('.q-student');
  await page.waitForTimeout(300);
  await page.screenshot({ path: out + '/03-detail-drawer.png' });
  await page.keyboard.press('Escape');
  await page.waitForTimeout(200);

  // day mode
  await page.click('[data-action="toggle-day"]');
  await page.waitForTimeout(400);
  await page.screenshot({ path: out + '/04-day-mode.png' });
  await page.click('[data-action="period"][data-period="P1700_1800"]');
  await page.waitForTimeout(300);

  // history
  await page.click('[data-action="tab"][data-tab="history"]');
  await page.waitForTimeout(300);
  await page.screenshot({ path: out + '/05-history.png' });

  // high level
  await page.click('[data-action="tab"][data-tab="board"]');
  await page.selectOption('#levelSel', 'High Level');
  await page.waitForTimeout(400);
  await page.screenshot({ path: out + '/06-high-level.png' });

  // mobile
  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  mobile.on('pageerror', (e) => errors.push('MOBILE PAGEERROR: ' + e.message));
  await mobile.goto(file, { waitUntil: 'load' });
  await mobile.evaluate(() => document.fonts.ready);
  await mobile.waitForTimeout(700);
  await mobile.screenshot({ path: out + '/07-mobile-390.png' });

  const tablet = await browser.newPage({ viewport: { width: 1024, height: 900 }, deviceScaleFactor: 1 });
  await tablet.goto(file, { waitUntil: 'load' });
  await tablet.evaluate(() => document.fonts.ready);
  await tablet.waitForTimeout(500);
  await tablet.screenshot({ path: out + '/08-tablet-1024.png' });

  console.log('ERRORS:', errors.length ? '\n' + errors.join('\n') : 'none');
  await browser.close();
})();
