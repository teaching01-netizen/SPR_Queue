/* Warwick Staff Dashboard — dual-room operations board prototype
   Vanilla JS, mock data only. No network calls, no framework. */
(function () {
  'use strict';

  /* ---------- small helpers ---------- */
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const pad = (n) => String(n).padStart(2, '0');
  const T = (h, m, s = 0) => h * 3600 + m * 60 + s;
  const fmt = (sec) => {
    if (sec == null || !Number.isFinite(sec)) return '—';
    sec = Math.max(0, Math.floor(sec));
    return pad(Math.floor(sec / 3600) % 24) + ':' + pad(Math.floor((sec % 3600) / 60));
  };
  const fmtS = (sec) => {
    sec = Math.max(0, Math.floor(sec));
    return pad(Math.floor(sec / 3600) % 24) + ':' + pad(Math.floor((sec % 3600) / 60)) + ':' + pad(sec % 60);
  };
  const clockSpan = (sec) => {
    const t = Math.max(0, Math.floor(sec));
    return pad(Math.floor(t / 60)) + ':' + pad(t % 60);
  };
  const durMin = (a, b) => {
    if (a == null || b == null) return '—';
    return Math.max(0, Math.round((b - a) / 60)) + ' นาที';
  };
  const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const clone = (o) => JSON.parse(JSON.stringify(o));
  const otherStream = (s) => (s === 'Math' ? 'Verbal' : 'Math');
  const icon = (name, cls = 'ic') => `<svg class="${cls}"><use href="#i-${name}"/></svg>`;

  /* ---------- mock data ---------- */
  const R = (code, name, wcode, subject, status, o = {}) => ({
    id: o.id || code, code, name, wcode, subject, status,
    order: o.order == null ? null : o.order,
    calledAt: o.called == null ? null : o.called,
    startedAt: o.start == null ? null : o.start,
    endedAt: o.end == null ? null : o.end,
    archived: !!o.archived,
    reason: o.reason || '',
    cancelledAt: o.cancelled == null ? null : o.cancelled,
  });
  const P = (id, start, endM, endV, caps) => ({ id, start, endM, endV: endV == null ? endM : endV, caps });

  const DATA = {
    Beginner: {
      dateShort: '7 Nov 2026',
      dateLabel: '7 November 2026',
      mode: 'On-site',
      demoNow: T(17, 12, 20),
      periods: [
        P('P1300_1400', T(13, 0), T(14, 0), T(14, 0), { Math: 4 }),
        P('P1400_1530', T(14, 0), T(15, 30), T(15, 30), { Combined: 5 }),
        P('P1530_1700', T(15, 30), T(17, 0), T(17, 0), { Combined: 6 }),
        P('P1700_1800', T(17, 0), T(18, 0), T(18, 0), { Math: 4, Verbal: 4 }),
        P('P1800_2000', T(18, 0), T(20, 0), T(20, 0), { Math: 8, Verbal: 7 }),
      ],
      rows: {
        P1300_1400: {
          Math: [
            R('B-M-001', 'น้องปอ', 'W261001', 'Math', 'Completed', { start: T(13, 0), end: T(13, 8), archived: true }),
            R('B-M-002', 'น้องบิว', 'W261002', 'Math', 'Completed', { start: T(13, 10), end: T(13, 19) }),
            R('B-M-003', 'น้องหญิง', 'W261003', 'Math', 'Completed', { start: T(13, 22), end: T(13, 35) }),
            R('B-M-004', 'น้องโต', 'W261004', 'Math', 'No-show', { called: T(13, 39) }),
          ],
          Verbal: [],
        },
        P1400_1530: {
          Math: [
            R('B-MV-001', 'น้องซีล', 'W260194', 'Combined', 'Completed', { start: T(14, 2), end: T(14, 12) }),
            R('B-MV-002', 'น้องธีโอ', 'W260188', 'Combined', 'Completed', { start: T(14, 15), end: T(14, 25) }),
            R('B-MV-003', 'น้องเคบี', 'W260032', 'Combined', 'Completed', { start: T(14, 30), end: T(14, 40) }),
          ],
          Verbal: [
            R('B-MV-004', 'น้องตระกะ', 'W260228', 'Combined', 'Completed', { start: T(14, 0), end: T(14, 10) }),
            R('B-MV-005', 'น้องนีน', 'W250327', 'Combined', 'Completed', { start: T(14, 12), end: T(14, 22) }),
          ],
        },
        P1530_1700: {
          Math: [
            R('B-MV-006', 'น้องเอแคลร์', 'W260132', 'Combined', 'Completed', { start: T(15, 35), end: T(15, 45) }),
            R('B-MV-007', 'น้องแองเจิ้ล', 'W260232', 'Combined', 'Completed', { start: T(15, 50), end: T(16, 0) }),
            R('B-MV-008', 'น้องมิ้นท์', 'W260184', 'Combined', 'Completed', { start: T(16, 48), end: T(16, 58) }),
          ],
          Verbal: [
            R('B-MV-009', 'น้องปัน', 'W260206', 'Combined', 'Completed', { start: T(15, 32), end: T(15, 42) }),
            R('B-MV-010', 'น้องเรนา', 'W260214', 'Combined', 'Completed', { start: T(16, 5), end: T(16, 15) }),
            R('B-MV-011', 'น้องตัมตัม', 'W260190', 'Combined', 'Completed', { start: T(16, 53), end: T(17, 3) }),
          ],
        },
        P1700_1800: {
          Math: [
            R('B-M-005', 'น้องเจได', 'W260157', 'Math', 'Serving', { order: 1, called: T(17, 0), start: T(17, 1) }),
            R('B-M-006', 'น้องภูมิ', 'W260223', 'Math', 'Waiting', { order: 2 }),
            R('B-M-007', 'น้องลีอา', 'W260199', 'Math', 'Completed', { start: T(17, 0), end: T(17, 7) }),
            R('B-M-008', 'น้องคอปเตอร์', 'W260221', 'Math', 'Booked'),
            R('B-M-014', 'น้องแครอท', 'W261013', 'Math', 'Cancelled', { reason: 'ผู้ปกครองแจ้งยกเลิก', cancelled: T(16, 35) }),
          ],
          Verbal: [
            R('B-V-001', 'น้องอัลมอนด์', 'W260218', 'Verbal', 'Serving', { order: 1, called: T(17, 4), start: T(17, 4) }),
            R('B-V-002', 'น้องแอน', 'W260161', 'Verbal', 'Waiting', { order: 2 }),
            R('B-V-003', 'น้องเอมี่', 'W260138', 'Verbal', 'Completed', { start: T(17, 0), end: T(17, 9) }),
            R('B-V-004', 'น้องพิม', 'W260145', 'Verbal', 'Booked'),
            R('B-V-008', 'น้องส้ม', 'W261014', 'Verbal', 'Cancelled', { reason: 'ลงทะเบียนผิด', cancelled: T(16, 0) }),
          ],
        },
        P1800_2000: {
          Math: [
            R('B-M-009', 'น้องไอซ์', 'W261005', 'Math', 'Waiting', { order: 1 }),
            R('B-M-010', 'น้องเฟิร์น', 'W261006', 'Math', 'Waiting', { order: 2 }),
            R('B-M-011', 'น้องเจมส์', 'W261007', 'Math', 'Booked'),
            R('B-M-012', 'น้องข้าวหอม', 'W261008', 'Math', 'Booked'),
            R('B-M-013', 'น้องน้ำ', 'W261009', 'Math', 'Booked'),
          ],
          Verbal: [
            R('B-V-005', 'น้องปลื้ม', 'W261010', 'Verbal', 'Waiting', { order: 1 }),
            R('B-V-006', 'น้องกัน', 'W261011', 'Verbal', 'Booked'),
            R('B-V-007', 'น้องมีน', 'W261012', 'Verbal', 'Booked'),
            R('B-V-009', 'น้องสุกี้', 'W261015', 'Verbal', 'Cancelled', { reason: 'ผู้ปกครองแจ้งยกเลิก', cancelled: T(17, 30) }),
          ],
        },
      },
      handoffs: [
        { id: 'HO-1', code: 'B-MV-008', name: 'น้องมิ้นท์', wcode: 'W260184', from: 'Math', to: 'Verbal', finishedAt: T(16, 58), periodId: 'P1530_1700', order: null, materialized: false },
        { id: 'HO-2', code: 'B-MV-011', name: 'น้องตัมตัม', wcode: 'W260190', from: 'Verbal', to: 'Math', finishedAt: T(17, 3), periodId: 'P1530_1700', order: null, materialized: false },
      ],
    },
    'High Level': {
      dateShort: '6 Nov 2026',
      dateLabel: '6 November 2026',
      mode: 'Online via Zoom',
      demoNow: T(12, 8, 0),
      periods: [
        P('P1000_1130', T(10, 0), T(11, 30), T(11, 30), { Math: 7, Verbal: 7 }),
        P('P1155_1325', T(11, 55), T(13, 25), T(13, 40), { Math: 7, Verbal: 8 }),
        P('P1430_1610', T(14, 30), T(16, 10), T(16, 10), { Combined: 7 }),
        P('P1610_1810', T(16, 10), T(18, 10), T(18, 10), { Combined: 8 }),
      ],
      rows: {
        P1000_1130: {
          Math: [
            R('H-M-001', 'น้องเอิร์ธ', 'W262001', 'Math', 'Completed', { start: T(10, 0), end: T(10, 12), archived: true }),
            R('H-M-002', 'น้องเจ', 'W262002', 'Math', 'Completed', { start: T(10, 15), end: T(10, 28) }),
            R('H-M-003', 'น้องเฟย์', 'W262003', 'Math', 'Completed', { start: T(10, 31), end: T(10, 41) }),
            R('H-M-004', 'น้องข้าว', 'W262004', 'Math', 'No-show', { called: T(10, 44) }),
          ],
          Verbal: [
            R('H-V-001', 'น้องวี', 'W262005', 'Verbal', 'Completed', { start: T(10, 2), end: T(10, 13) }),
            R('H-V-002', 'น้องภัทร', 'W262006', 'Verbal', 'Completed', { start: T(10, 16), end: T(10, 29) }),
            R('H-V-003', 'น้องแพร', 'W262007', 'Verbal', 'Completed', { start: T(10, 31), end: T(10, 45) }),
          ],
        },
        P1155_1325: {
          Math: [
            R('H-M-005', 'น้องมีน', 'W262008', 'Math', 'Serving', { order: 1, called: T(11, 58), start: T(12, 1) }),
            R('H-M-006', 'น้องพีท', 'W262009', 'Math', 'Waiting', { order: 2 }),
            R('H-M-007', 'น้องนัท', 'W262010', 'Math', 'Booked'),
            R('H-M-008', 'น้องบีม', 'W262011', 'Math', 'Cancelled', { reason: 'ผู้ปกครองแจ้งยกเลิก', cancelled: T(11, 20) }),
          ],
          Verbal: [
            R('H-V-004', 'น้องครีม', 'W262012', 'Verbal', 'Serving', { order: 1, called: T(12, 2), start: T(12, 3) }),
            R('H-V-005', 'น้องโอ๊ต', 'W262013', 'Verbal', 'Waiting', { order: 2 }),
            R('H-V-006', 'น้องสาย', 'W262014', 'Verbal', 'Booked'),
          ],
        },
        P1430_1610: {
          Math: [
            R('H-MV-001', 'น้องฮาร์ท', 'W262015', 'Combined', 'Booked'),
            R('H-MV-002', 'น้องเกรซ', 'W262016', 'Combined', 'Booked'),
            R('H-MV-003', 'น้องเจน', 'W262017', 'Combined', 'Booked'),
          ],
          Verbal: [
            R('H-MV-004', 'น้องมายด์', 'W262018', 'Combined', 'Booked'),
            R('H-MV-005', 'น้องต้น', 'W262019', 'Combined', 'Booked'),
          ],
        },
        P1610_1810: {
          Math: [
            R('H-MV-006', 'น้องเปา', 'W262020', 'Combined', 'Booked'),
            R('H-MV-007', 'น้องแพรว', 'W262021', 'Combined', 'Booked'),
          ],
          Verbal: [
            R('H-MV-008', 'น้องกิ๊ก', 'W262022', 'Combined', 'Booked'),
            R('H-MV-009', 'น้องออม', 'W262023', 'Combined', 'Booked'),
          ],
        },
      },
      handoffs: [],
    },
  };

  /* ---------- state ---------- */
  const store = clone(DATA);
  const state = {
    level: 'Beginner',
    periodId: 'P1700_1800',
    view: 'board',
    dayMode: false,
    search: '',
    historyFilter: 'all',
    historyArchived: false,
    mobileLane: 'Math',
    dirty: { Math: false, Verbal: false },
    snapshots: { Math: null, Verbal: null },
    lastSaved: { Math: T(17, 5), Verbal: T(17, 4) },
    nowBase: DATA.Beginner.demoNow,
    tickStart: Date.now(),
    lastSync: null,
    drawer: null,
    modal: null,
    menu: null,
  };
  let toastTimer = null;
  let estVersion = 0;
  let estCache = {};

  /* ---------- derived helpers ---------- */
  const levelData = () => store[state.level];
  const periods = () => levelData().periods;
  const periodById = (id) => periods().find((p) => p.id === id);
  const currentPeriod = () => periodById(state.periodId) || periods()[0];
  const nowSec = () => state.nowBase + (Date.now() - state.tickStart) / 1000;
  const nowInt = () => Math.floor(nowSec());
  const rowsFor = (periodId, stream) => (levelData().rows[periodId] && levelData().rows[periodId][stream]) || [];
  const allRows = (stream) => {
    const out = [];
    for (const p of periods()) for (const r of rowsFor(p.id, stream)) out.push(r);
    return out;
  };
  const visibleRows = (stream) => allRows(stream).filter((r) => r.status !== 'Cancelled' && !r.archived);
  const laneLive = (stream) => visibleRows(stream).find((r) => r.status === 'Calling' || r.status === 'Serving') || null;
  const subjectLabel = (s) => (s === 'Combined' ? 'SAT Math + Verbal' : s === 'Math' ? 'SAT Math' : 'SAT Verbal');
  const findRow = (id) => {
    for (const p of periods()) {
      for (const s of ['Math', 'Verbal']) {
        const row = rowsFor(p.id, s).find((r) => r.id === id);
        if (row) return { row, stream: s, periodId: p.id };
      }
    }
    return null;
  };
  const findHandoff = (id) => levelData().handoffs.find((h) => h.id === id) || null;
  const rowMatches = (row) => {
    const q = state.search.trim().toLowerCase();
    if (!q) return true;
    return `${row.code} ${row.name} ${row.wcode} ${row.subject} ${subjectLabel(row.subject)}`.toLowerCase().includes(q);
  };

  function periodCounts(periodId) {
    const rows = [...rowsFor(periodId, 'Math'), ...rowsFor(periodId, 'Verbal')].filter((r) => r.status !== 'Cancelled');
    let m = 0, v = 0, c = 0;
    for (const r of rows) {
      if (r.subject === 'Combined') c++;
      else if (r.subject === 'Math') m++;
      else v++;
    }
    return { m, v, c, total: rows.length };
  }
  function bookedRows() {
    return [...rowsFor(state.periodId, 'Math'), ...rowsFor(state.periodId, 'Verbal')].filter((r) => r.status === 'Booked' && !r.archived);
  }
  function historyRows() {
    const out = [];
    for (const p of periods()) {
      for (const s of ['Math', 'Verbal']) {
        for (const r of rowsFor(p.id, s)) {
          if (r.status === 'Completed' || r.status === 'Cancelled') out.push({ row: r, period: p, stream: s });
        }
      }
    }
    return out;
  }

  function invalidateEstimates() {
    estVersion++;
    estCache = {};
  }
  function calcEstimates(stream, periodId) {
    const p = periodById(periodId);
    if (!p) return {};
    const rows = rowsFor(periodId, stream);
    const now = nowInt();
    const live = laneLive(stream);
    let cursor = now;
    if (live) {
      if (live.status === 'Serving' && live.startedAt != null) cursor = Math.max(cursor, live.startedAt + 600);
      else if (live.status === 'Calling' && live.calledAt != null) cursor = Math.max(cursor, live.calledAt + 600);
    }
    const end = stream === 'Math' ? p.endM : p.endV;
    let at = Math.max(cursor, p.start, now);
    at = Math.ceil(at / 60) * 60;
    const map = {};
    const waiting = rows
      .filter((r) => r.status === 'Waiting' && r.order > 0 && !r.archived)
      .sort((a, b) => a.order - b.order);
    for (const r of waiting) {
      map[r.id] = { start: at, late: at > end };
      at += 600;
    }
    return map;
  }
  function estimatesFor(stream, periodId = state.periodId) {
    const key = estVersion + '|' + periodId + '|' + stream;
    if (!estCache[key]) estCache[key] = calcEstimates(stream, periodId);
    return estCache[key];
  }
  function estimateFor(stream, row, periodId = state.periodId) {
    if (!row || row.status !== 'Waiting' || row.order == null) return null;
    return estimatesFor(stream, periodId)[row.id] || null;
  }

  function primarySpec(row, stream) {
    if (row.status === 'Booked') return { label: 'เช็กอิน' };
    if (row.status === 'Waiting') return row.order == null ? { label: 'จัดคิว' } : { label: 'เรียกคิว' };
    if (row.status === 'Calling') return { label: 'เริ่มปรึกษา' };
    if (row.status === 'Serving') {
      const pending = levelData().handoffs.find((h) => h.code === row.code && !h.materialized && h.from === stream);
      return { label: pending ? 'จบ ' + stream + ' → ' + otherStream(stream) : 'จบการปรึกษา' };
    }
    if (row.status === 'No-show') return { label: 'คืนคิว' };
    return null;
  }
  function shortPrimaryLabel(row, stream) {
    const spec = primarySpec(row, stream);
    if (!spec) return '';
    if (row.status === 'Serving') {
      const pending = levelData().handoffs.find((h) => h.code === row.code && !h.materialized && h.from === stream);
      return pending ? 'จบ → ' + otherStream(stream) : 'จบการปรึกษา';
    }
    return spec.label;
  }
  function lockReason(row, stream) {
    if (state.search.trim()) return 'ล้างคำค้นก่อน';
    if (state.dirty.Math || state.dirty.Verbal) return 'มีลำดับที่ยังไม่บันทึก';
    if (row.status === 'Waiting' && row.order != null && laneLive(stream)) return 'ห้องนี้กำลังให้บริการอยู่';
    return '';
  }

  function statusPill(status, row) {
    if (status === 'Waiting' && row && row.order == null) {
      return `<span class="pill pill-handoff">${icon('arrow-right')}รอจัดคิว</span>`;
    }
    const map = {
      Booked: ['pill-booked', 'clock', 'รอเช็กอิน'],
      Waiting: ['pill-waiting', 'clock', 'รอเรียกคิว'],
      Calling: ['pill-calling', 'phone', 'กำลังเรียก'],
      Serving: ['pill-serving', 'play', 'กำลังปรึกษา'],
      Completed: ['pill-completed', 'check', 'เสร็จสิ้น'],
      'No-show': ['pill-noshow', 'x', 'ไม่มา'],
      Cancelled: ['pill-cancelled', 'x', 'ยกเลิก'],
      Handoff: ['pill-handoff', 'arrow-right', 'รอจัดคิว'],
    };
    const item = map[status] || map.Waiting;
    return `<span class="pill ${item[0]}">${icon(item[1])}${item[2]}</span>`;
  }

  /* ---------- chrome ---------- */
  function renderChrome() {
    const d = levelData();
    $('#levelSel').value = state.level;
    $('#dateText').textContent = d.dateShort;
    $('#modeText').textContent = d.mode;
    const modeDot = $('#modeChip .dot');
    if (modeDot) modeDot.style.background = d.mode.indexOf('Online') >= 0 ? '#4f7fae' : '#4e9b78';
    $('#checkinCount').textContent = bookedRows().length;
    $('#historyCount').textContent = historyRows().length;
    $('#dayBtn').setAttribute('aria-pressed', state.dayMode ? 'true' : 'false');
    $('#tabBoard').setAttribute('aria-selected', state.view === 'board' ? 'true' : 'false');
    $('#tabHistory').setAttribute('aria-selected', state.view === 'history' ? 'true' : 'false');
    $('#controls').classList.toggle('is-history', state.view === 'history');
    const rb = $('#ribbon');
    if (rb) rb.innerHTML = renderRibbon();
    const si = $('#searchInput');
    if (si && si.value !== state.search) si.value = state.search;
    const sc = $('#searchClear');
    if (sc) sc.hidden = !state.search;
    const banner = $('#searchBanner');
    if (banner) {
      banner.hidden = !state.search.trim();
      if (state.search.trim()) $('#searchBannerText').textContent = 'กำลังค้นหา “' + state.search.trim() + '” · รายการอื่นจางลงเพื่อคงลำดับห้อง · ล้างคำค้นก่อนบันทึก';
    }
    for (const s of ['Math', 'Verbal']) {
      const waits = rowsFor(state.periodId, s).filter((r) => !r.archived && r.status === 'Waiting').length;
      const el = s === 'Math' ? $('#lsMath') : $('#lsVerbal');
      if (el) el.textContent = 'รอ ' + waits;
      $$('[data-lane-dot="' + s + '"]').forEach((dot) => dot.classList.toggle('live', !!laneLive(s)));
    }
    $$('#laneSwitch button').forEach((b) => b.setAttribute('aria-pressed', b.dataset.stream === state.mobileLane ? 'true' : 'false'));
    $('#demoClock').textContent = fmt(nowInt());
    if (!state.lastSync) state.lastSync = nowInt();
    $('#syncText').textContent = 'ซิงก์ ' + fmtS(nowInt()) + ' · ทุก 5 วิ';
  }

  function renderRibbon() {
    return periods().map((p) => {
      const counts = periodCounts(p.id);
      const active = state.view === 'board' && !state.dayMode && p.id === state.periodId;
      const caps = [];
      if (p.caps.Math) caps.push(`<i class="cap m">M ${counts.m}/${p.caps.Math}</i>`);
      if (p.caps.Verbal) caps.push(`<i class="cap v">V ${counts.v}/${p.caps.Verbal}</i>`);
      if (p.caps.Combined) caps.push(`<i class="cap mv">MV ${counts.c}/${p.caps.Combined}</i>`);
      const note = p.endM !== p.endV ? `<span class="p-note">Verbal ถึง ${fmt(p.endV)}</span>` : '';
      return `<button class="period ${active ? 'active' : ''}" data-action="period" data-period="${p.id}" aria-pressed="${active}" title="${p.id}">
        <span class="p-time">${fmt(p.start)}–${fmt(p.endM)}</span>${note}<span class="p-caps">${caps.join('')}</span>
      </button>`;
    }).join('');
  }

  /* ---------- board ---------- */
  function renderView() {
    $('#view').innerHTML = state.view === 'history' ? renderHistory() : renderBoard();
  }

  function renderBoard() {
    const isDay = state.dayMode;
    const lanes = ['Math', 'Verbal'].map((stream) => laneHtml(stream, isDay)).join('');
    return `<div class="board ${isDay ? 'day-board' : ''}">${lanes}</div>`;
  }

  function laneHead(stream, isDay, counts) {
    const teachers = stream === 'Math'
      ? `<div class="teachers"><div class="avatar-xs">B</div><div class="teacher-label"><strong>Aj. B</strong>ผู้สอน</div></div>`
      : `<div class="teachers"><div class="avatar-xs" style="margin-right:-8px">N</div><div class="avatar-xs">R</div><div class="teacher-label"><strong>Aj. Nice · Aj. Ryu</strong>ผู้สอน</div></div>`;
    return `<header class="lane-head">
      <div class="room-block">
        <div class="room-mark">${stream === 'Math' ? 'M' : 'V'}</div>
        <div>
          <div class="room-name">SAT ${stream}</div>
          <div class="room-kicker">${stream === 'Math' ? 'MATH ROOM' : 'VERBAL ROOM'}${isDay ? ' · ภาพรวมทั้งวัน' : ''}</div>
        </div>
      </div>
      <div class="lane-head-right">
        ${teachers}
        <div class="lane-counts"><b>${counts.wait}</b> รอ · <b>${counts.live}</b> Live</div>
      </div>
    </header>`;
  }

  function laneHtml(stream, isDay) {
    const p = currentPeriod();
    const all = isDay ? visibleRows(stream) : rowsFor(p.id, stream).filter((r) => r.status !== 'Cancelled' && !r.archived);
    const counts = {
      wait: all.filter((r) => r.status === 'Waiting').length,
      live: all.filter((r) => r.status === 'Calling' || r.status === 'Serving').length,
    };
    const active = stream === state.mobileLane ? ' is-mobile-active' : '';
    if (isDay) {
      const live = laneLive(stream);
      return `<section class="lane ${stream.toLowerCase()}${active}">
        ${laneHead(stream, true, counts)}
        <div class="lane-body">
          ${live ? `<div class="day-now"><span class="pulse"></span>กำลังให้บริการ ${esc(live.code)} · ${esc(live.name)} · เริ่ม ${fmt(live.startedAt || live.calledAt)}</div>` : ''}
          ${daySections(stream)}
          <div class="day-foot">
            <span class="foot-note">${icon('info')}คลิกแถวเพื่อดูรายละเอียด · ภาพรวมทุกช่วงเวลาของวัน</span>
            <span class="foot-note">ข้อมูล ณ ${fmt(nowInt())}</span>
          </div>
        </div>
      </section>`;
    }
    return `<section class="lane ${stream.toLowerCase()}${active}">
      ${laneHead(stream, false, counts)}
      <div class="lane-body">
        ${renderNow(stream)}
        ${renderUpNext(stream)}
        ${renderHandoff(stream)}
        ${renderQueueTable(stream, p)}
        ${renderLaneFoot(stream)}
      </div>
    </section>`;
  }

  function renderNow(stream) {
    const live = laneLive(stream);
    const p = currentPeriod();
    if (live) {
      const isServe = live.status === 'Serving';
      const elapsed = Math.max(0, nowInt() - (live.startedAt || nowInt()));
      const over = elapsed > 600;
      const spec = primarySpec(live, stream);
      const reason = lockReason(live, stream);
      return `<div class="now live">
        <div class="now-top">
          <span class="now-label"><span class="pulse"></span>${isServe ? 'กำลังให้บริการ' : 'กำลังเรียกคิว'}</span>
          <span class="now-expected">${isServe ? 'คาดจบ ' + fmt(live.startedAt + 600) : 'เรียก ' + fmt(live.calledAt)}</span>
        </div>
        <div class="now-main">
          <span class="now-code">${esc(live.code)}</span>
          <div class="now-person"><strong>${esc(live.name)}</strong><span>${subjectLabel(live.subject)}${live.wcode ? ' · ' + esc(live.wcode) : ''}</span></div>
        </div>
        <div class="now-timer">${isServe
          ? `เริ่ม ${fmt(live.startedAt)} <span class="sep">·</span> <span class="js-elapsed" data-start="${live.startedAt}">ใช้ไป ${clockSpan(elapsed)}</span> <span class="js-over over" data-start="${live.startedAt}" ${over ? '' : 'hidden'}>เกิน ${clockSpan(Math.max(0, elapsed - 600))}</span> <span class="sub">· ประมาณ 10 นาที</span>`
          : `เรียก ${fmt(live.calledAt)} <span class="sep">·</span> รอเริ่มปรึกษา`}</div>
        <div class="now-actions">
          <button class="lane-btn" data-action="primary" data-id="${live.id}" ${reason ? `disabled title="${esc(reason)}"` : ''}>${isServe ? icon('play') : icon('phone')}${spec.label}</button>
          <button class="now-menu" data-action="now-menu" data-id="${live.id}" aria-label="ตัวเลือกคิวนี้">${icon('more')}</button>
        </div>
      </div>`;
    }
    const cap = (p.caps[stream] || 0) + (p.caps.Combined || 0);
    const waiting = rowsFor(p.id, stream)
      .filter((r) => r.status === 'Waiting' && r.order > 0 && !r.archived)
      .sort((a, b) => a.order - b.order);
    const next = waiting[0];
    const est = next ? estimateFor(stream, next, p.id) : null;
    const reason = next
      ? (state.search.trim() ? 'ล้างคำค้นก่อน' : (state.dirty.Math || state.dirty.Verbal) ? 'บันทึกลำดับก่อน' : '')
      : 'ยังไม่มีคิวที่พร้อมเรียก';
    let hint;
    if (!cap) {
      const nextP = periods().find((x) => x.start > nowInt() && ((x.caps[stream] || 0) + (x.caps.Combined || 0)) > 0);
      hint = `รอบนี้ไม่มีการให้บริการ ${stream}${nextP ? ' · เปิดอีกครั้ง ' + fmt(nextP.start) + '–' + fmt(stream === 'Math' ? nextP.endM : nextP.endV) : ''}`;
    } else if (next) {
      hint = `ถัดไป <strong>${esc(next.code)} ${esc(next.name)}</strong> · ${est ? '~' + fmt(est.start) : 'รอจัดคิว'}`;
    } else {
      hint = 'ยังไม่มีคิวที่รอในรอบนี้';
    }
    return `<div class="now empty">
      <div class="now-top">
        <span class="now-label"><span class="pulse" style="background:#b9c5d2;box-shadow:none"></span>${cap ? 'ห้องว่าง' : 'ไม่มีการให้บริการ'}</span>
        <span class="now-expected">${fmt(p.start)}–${fmt(stream === 'Math' ? p.endM : p.endV)}</span>
      </div>
      <div class="now-main"><div class="empty-hint">${hint}</div></div>
      <div class="now-actions">
        <button class="lane-btn quiet" data-action="call-next" data-stream="${stream}" ${(!next || reason) ? `disabled title="${esc(reason)}"` : ''}>${icon('phone')}เรียกคิวถัดไป</button>
        <button class="now-menu" data-action="settings" aria-label="ตั้งเวลาต่อคิว">${icon('clock')}</button>
      </div>
    </div>`;
  }

  function renderUpNext(stream) {
    const p = currentPeriod();
    const waiting = rowsFor(p.id, stream)
      .filter((r) => r.status === 'Waiting' && r.order > 0 && !r.archived)
      .sort((a, b) => a.order - b.order)
      .slice(0, 2);
    if (!waiting.length) {
      return `<div class="upnext"><span class="upnext-label">ถัดไป</span><span class="upnext-empty">ไม่มีคิวรอในรอบนี้</span></div>`;
    }
    return `<div class="upnext"><span class="upnext-label">ถัดไป</span>${waiting.map((r) => {
      const est = estimateFor(stream, r, p.id);
      return `<span class="upnext-item"><b>${esc(r.code)}</b> ${esc(r.name)} <span class="at">${est ? '~' + fmt(est.start) : 'รอจัดคิว'}</span></span>`;
    }).join('')}</div>`;
  }

  function renderHandoff(stream) {
    const all = levelData().handoffs.filter((h) => h.to === stream);
    const visible = all.filter((h) => !(h.materialized && h.periodId === state.periodId));
    const sources = Array.from(new Set(visible.map((h) => h.from)));
    const label = visible.length ? `ส่งต่อจาก ${sources.join(' / ')}` : `ส่งต่อจาก ${otherStream(stream)}`;
    if (!visible.length) {
      return `<div class="handoff"><span class="handoff-label">${icon('arrow-right')}${label}</span><span class="handoff-empty">ไม่มีการส่งต่อข้ามห้อง</span></div>`;
    }
    return `<div class="handoff"><span class="handoff-label">${icon('arrow-right')}${label}</span>${visible.map((h) => {
      const source = periodById(h.periodId);
      let action;
      if (h.order == null) action = `<button class="mini-link" data-action="assign-handoff" data-ho="${h.id}">จัดคิว ${stream}</button>`;
      else if (!h.materialized) action = `<span class="handoff-done">จัดคิวแล้ว · รอบันทึก</span>`;
      else action = `<button class="mini-link" data-action="go-handoff" data-ho="${h.id}">เปิดรอบ ${source ? fmt(source.start) : ''}</button>`;
      return `<div class="handoff-card">
        <span class="handoff-code">${esc(h.code)}</span>
        <div class="handoff-person"><strong>${esc(h.name)}</strong><span>จบ ${h.from} ${fmt(h.finishedAt)} · Combined</span></div>
        ${action}
      </div>`;
    }).join('')}</div>`;
  }

  function queueList(stream, periodId) {
    const rows = rowsFor(periodId, stream).filter((r) => r.status !== 'Cancelled' && !r.archived);
    const active = rows
      .filter((r) => r.status === 'Waiting' || r.status === 'Calling' || r.status === 'Serving')
      .sort((a, b) => {
        const an = a.order == null ? 0 : 1;
        const bn = b.order == null ? 0 : 1;
        if (an !== bn) return an - bn;
        return (a.order || 0) - (b.order || 0);
      });
    const closed = rows
      .filter((r) => r.status === 'Completed' || r.status === 'No-show')
      .sort((a, b) => (a.startedAt == null ? a.calledAt || 0 : a.startedAt) - (b.startedAt == null ? b.calledAt || 0 : b.startedAt));
    const booked = rows.filter((r) => r.status === 'Booked');
    return [...active, ...closed, ...booked];
  }

  function timeCell(stream, row, period) {
    const est = estimateFor(stream, row, period.id);
    if (row.status === 'Serving') {
      const elapsed = Math.max(0, nowInt() - (row.startedAt || nowInt()));
      const over = elapsed > 600;
      return `<div class="q-time is-live"><strong>เริ่ม ${fmt(row.startedAt)}</strong><small><span class="js-elapsed" data-start="${row.startedAt}">ใช้ไป ${clockSpan(elapsed)}</span> <span class="js-over over" data-start="${row.startedAt}" ${over ? '' : 'hidden'}>เกิน ${clockSpan(Math.max(0, elapsed - 600))}</span></small></div>`;
    }
    if (row.status === 'Calling') return `<div class="q-time is-live"><strong>เรียก ${fmt(row.calledAt)}</strong><small>รอเริ่มปรึกษา</small></div>`;
    if (row.status === 'Completed') return `<div class="q-time"><strong>${fmt(row.startedAt)}–${fmt(row.endedAt)}</strong><small>${durMin(row.startedAt, row.endedAt)}</small></div>`;
    if (row.status === 'No-show') return `<div class="q-time"><strong>เรียก ${fmt(row.calledAt)}</strong><small>ไม่มาเข้าคิว</small></div>`;
    if (row.status === 'Booked') return `<div class="q-time"><strong>—</strong><small>รอเช็กอิน</small></div>`;
    if (row.order == null) return `<div class="q-time"><strong>—</strong><small>รอจัดคิว</small></div>`;
    if (!est) return `<div class="q-time is-pending"><strong>รอบันทึก</strong><small>กดบันทึกลำดับ</small></div>`;
    return `<div class="q-time"><strong>~${fmt(est.start)}</strong><small>~10 นาที ${est.late ? '<span class="over">เกินรอบ</span>' : ''}</small></div>`;
  }

  function renderQueueTable(stream, period) {
    const list = queueList(stream, period.id);
    if (!list.length) {
      const cap = (period.caps[stream] || 0) + (period.caps.Combined || 0);
      const nextP = periods().find((x) => x.start > nowInt() && ((x.caps[stream] || 0) + (x.caps.Combined || 0)) > 0);
      const msg = cap
        ? 'ยังไม่มีรายการในรอบนี้ · เช็กอินแล้วรายการจะแสดงที่นี่'
        : `รอบนี้ไม่มีคิว ${stream}${nextP ? ' · เปิดอีกครั้ง ' + fmt(nextP.start) + '–' + fmt(stream === 'Math' ? nextP.endM : nextP.endV) : ''}`;
      return `<div class="lane-empty"><span class="empty-ic">${icon('inbox')}</span><strong>${cap ? 'ยังไม่มีคิวในรอบนี้' : 'ไม่มีการให้บริการ ' + stream}</strong><p>${msg}</p></div>`;
    }
    return `<div class="q-table">
      <div class="q-head"><span>#</span><span>คิว / นักเรียน</span><span>เวลา</span><span>สถานะ</span><span class="q-actions" style="justify-content:flex-end">จัดการ</span></div>
      ${list.map((row) => renderQueueRow(stream, row, period)).join('')}
    </div>`;
  }

  function renderQueueRow(stream, row, period) {
    const orderedWait = rowsFor(period.id, stream)
      .filter((r) => r.status === 'Waiting' && r.order > 0)
      .sort((a, b) => a.order - b.order);
    const waitIndex = orderedWait.findIndex((r) => r.id === row.id);
    const dim = !!state.search.trim() && !rowMatches(row);
    const live = row.status === 'Calling' || row.status === 'Serving';
    const combined = row.subject === 'Combined' ? `<span class="q-combined">M+V</span>` : '';
    const orderCell = row.order != null
      ? `<div class="q-order"><span class="order-num">${row.order}</span>${row.status === 'Waiting' ? `<span class="order-tools">
          <button data-action="move" data-id="${row.id}" data-dir="-1" ${waitIndex <= 0 ? 'disabled' : ''} aria-label="เลื่อน ${esc(row.code)} ขึ้น">${icon('up')}</button>
          <button data-action="move" data-id="${row.id}" data-dir="1" ${waitIndex < 0 || waitIndex >= orderedWait.length - 1 ? 'disabled' : ''} aria-label="เลื่อน ${esc(row.code)} ลง">${icon('down')}</button>
        </span>` : ''}</div>`
      : `<div class="q-order"><span class="order-num" style="color:#b3bec9;background:#fbfcfd">—</span></div>`;
    let action = '<span class="q-done-label">ปิดรายการ</span>';
    if (row.status !== 'Completed' && row.status !== 'Cancelled') {
      const label = shortPrimaryLabel(row, stream);
      if (label) {
        const reason = lockReason(row, stream);
        const neutral = row.status === 'No-show' ? ' neutral' : '';
        action = `<button class="mini-btn${neutral}" data-action="primary" data-id="${row.id}" ${reason ? `disabled title="${esc(reason)}"` : ''}>${label}</button>`;
      }
    }
    return `<div class="q-row ${live ? 'live' : ''} ${dim ? 'is-dim' : ''}">
      ${orderCell}
      <button class="q-student" data-action="open-detail" data-id="${row.id}" title="เปิดรายละเอียด ${esc(row.code)}">
        <span class="q-student-line"><span class="q-code">${esc(row.code)}</span>${combined}</span>
        <span class="q-name-line"><span class="q-name">${esc(row.name)}</span><span class="q-wcode">· ${esc(row.wcode)}</span></span>
      </button>
      ${timeCell(stream, row, period)}
      <div class="q-status">${statusPill(row.status, row)}</div>
      <div class="q-actions">${action}<button class="menu-btn" data-action="row-menu" data-id="${row.id}" aria-label="ตัวเลือก ${esc(row.code)}">${icon('more')}</button></div>
    </div>`;
  }

  function renderLaneFoot(stream) {
    if (state.dirty[stream]) {
      return `<div class="savebar">
        <span class="savebar-info">${icon('alert')}ยังไม่บันทึก · มีการเปลี่ยนลำดับ</span>
        <span class="savebar-actions">
          <button class="mini-btn" data-action="cancel-draft" data-stream="${stream}">ยกเลิก</button>
          <button class="mini-btn primary" data-action="save-order" data-stream="${stream}" ${state.search.trim() ? 'disabled title="ล้างคำค้นก่อน"' : ''}>บันทึกลำดับ</button>
        </span>
      </div>`;
    }
    return `<div class="lane-foot">
      <span class="foot-note">${icon('check')}บันทึกลำดับล่าสุด ${fmt(state.lastSaved[stream])} · smoke-admin</span>
      <button class="save-ghost" data-action="save-order" data-stream="${stream}" disabled title="ไม่มีการเปลี่ยนแปลง">บันทึกลำดับ</button>
    </div>`;
  }

  /* ---------- day view ---------- */
  function daySections(stream) {
    const sections = [];
    for (const p of periods()) {
      const rows = rowsFor(p.id, stream).filter((r) => r.status !== 'Cancelled' && !r.archived);
      const handoffs = levelData().handoffs.filter((h) => h.to === stream && h.periodId === p.id && h.materialized);
      if (!rows.length && !handoffs.length) continue;
      const entries = rows.map((r) => ({ row: r, ho: null })).concat(handoffs.map((h) => ({ row: null, ho: h })));
      entries.sort((a, b) => {
        const ka = a.row ? dayKey(stream, a.row, p) : T(23, 59);
        const kb = b.row ? dayKey(stream, b.row, p) : T(23, 59);
        return ka - kb;
      });
      const cap = p.caps.Combined ? `MV ${periodCounts(p.id).c}/${p.caps.Combined}` : `${stream} ${periodCounts(p.id)[stream === 'Math' ? 'm' : 'v']}/${p.caps[stream] || 0}`;
      sections.push(`<div class="day-section">
        <div class="day-section-head"><strong>${fmt(p.start)}–${fmt(stream === 'Math' ? p.endM : p.endV)}</strong><span>${cap} · ${entries.length} รายการ</span></div>
        <div class="day-head"><span>เริ่ม</span><span>จบ</span><span>ระยะเวลา</span><span>นักเรียน</span><span>สถานะ</span></div>
        ${entries.map((entry) => entry.row ? dayRow(stream, entry.row, p) : dayHandoffRow(stream, entry.ho, p)).join('')}
      </div>`);
    }
    if (!sections.length) {
      return `<div class="lane-empty"><span class="empty-ic">${icon('inbox')}</span><strong>ไม่มีคิวในวันนี้</strong><p>ทั้งวันยังไม่มีรายการของห้องนี้</p></div>`;
    }
    return sections.join('');
  }
  function dayKey(stream, row, p) {
    if (row.startedAt != null) return row.startedAt;
    if (row.calledAt != null) return row.calledAt;
    const est = estimateFor(stream, row, p.id);
    return est ? est.start : T(23, 59);
  }
  function dayRow(stream, row, p) {
    const est = estimateFor(stream, row, p.id);
    let start = '—', end = '—', dur = '—';
    if (row.status === 'Completed') {
      start = fmt(row.startedAt);
      end = fmt(row.endedAt);
      dur = durMin(row.startedAt, row.endedAt);
    } else if (row.status === 'Serving') {
      start = fmt(row.startedAt);
      end = '~' + fmt(row.startedAt + 600);
      dur = 'กำลังทำ';
    } else if (row.status === 'Calling') {
      start = fmt(row.calledAt);
      dur = 'รอเริ่ม';
    } else if (row.status === 'Waiting') {
      if (est) {
        start = '~' + fmt(est.start);
        end = '~' + fmt(est.start + 600);
        dur = '~10 นาที';
      } else {
        dur = 'รอจัดคิว';
      }
    } else if (row.status === 'No-show') {
      start = fmt(row.calledAt);
    }
    const live = row.status === 'Calling' || row.status === 'Serving';
    return `<div class="day-row ${live ? 'live' : ''}" data-action="open-detail" data-id="${row.id}" tabindex="0">
      <span class="day-time ${start === '—' ? 'muted' : ''}">${start}</span>
      <span class="day-time ${end === '—' ? 'muted' : ''}">${end}</span>
      <span class="day-dur">${dur}</span>
      <span class="day-person"><strong>${esc(row.name)}</strong><span class="q-code">${esc(row.code)}</span>${row.subject === 'Combined' ? '<span class="q-combined">M+V</span>' : ''}</span>
      <span>${statusPill(row.status, row)}</span>
    </div>`;
  }
  function dayHandoffRow(stream, ho, p) {
    return `<div class="day-row" data-action="open-detail" data-id="${ho.code}" tabindex="0">
      <span class="day-time muted">—</span>
      <span class="day-time muted">—</span>
      <span class="day-dur">รอจัดคิว</span>
      <span class="day-person"><strong>${esc(ho.name)}</strong><span class="q-code">${esc(ho.code)}</span><span class="q-combined">M+V</span></span>
      <span>${statusPill('Handoff')}</span>
    </div>`;
  }

  /* ---------- history ---------- */
  function renderHistory() {
    let rows = historyRows();
    if (state.historyArchived) rows = rows.filter(({ row }) => row.archived);
    if (state.historyFilter === 'completed') rows = rows.filter(({ row }) => row.status === 'Completed');
    if (state.historyFilter === 'cancelled') rows = rows.filter(({ row }) => row.status === 'Cancelled');
    rows.sort((a, b) => (b.row.endedAt || b.row.cancelledAt || 0) - (a.row.endedAt || a.row.cancelledAt || 0));
    const chips = `<div class="history-filters">
      <button class="chip" data-action="history-filter" data-filter="all" aria-pressed="${state.historyFilter === 'all'}">ทั้งหมด</button>
      <button class="chip" data-action="history-filter" data-filter="completed" aria-pressed="${state.historyFilter === 'completed'}">เสร็จสิ้น</button>
      <button class="chip" data-action="history-filter" data-filter="cancelled" aria-pressed="${state.historyFilter === 'cancelled'}">ยกเลิก</button>
      <button class="chip" data-action="history-archived" aria-pressed="${state.historyArchived}">เก็บประวัติแล้ว</button>
    </div>`;
    const body = rows.length ? `<div class="history-table"><table>
      <thead><tr><th>รหัสคิว / นักเรียน</th><th>วิชา</th><th>รอบ</th><th>วันเวลา</th><th>สถานะ</th><th>หมายเหตุ</th></tr></thead>
      <tbody>${rows.map(({ row, period }) => {
        const when = row.status === 'Completed'
          ? `${fmt(row.startedAt)}–${fmt(row.endedAt)} · ${durMin(row.startedAt, row.endedAt)}`
          : `ยกเลิก ${fmt(row.cancelledAt)}`;
        const note = row.archived ? 'เก็บประวัติแล้ว' : row.status === 'Cancelled' ? esc(row.reason || '—') : '—';
        return `<tr data-action="open-detail" data-id="${row.id}" tabindex="0">
          <td><span class="history-person"><strong>${esc(row.name)}</strong><span>${esc(row.code)} · ${esc(row.wcode)}</span></span></td>
          <td>${subjectLabel(row.subject)}</td>
          <td>${fmt(period.start)}–${fmt(period.endM)}</td>
          <td>${when}</td>
          <td>${statusPill(row.status, row)}</td>
          <td style="color:#8b99aa">${note}</td>
        </tr>`;
      }).join('')}</tbody>
    </table></div>` : `<div class="history-empty">${icon('inbox')}<strong>ไม่พบรายการตามตัวกรอง</strong><p>ลองเลือก “ทั้งหมด” หรือเปลี่ยนระดับ / ตัวกรองประวัติ</p></div>`;
    return `<div class="history-card">
      <div class="history-head">
        <div><h2>${icon('clock')}ประวัติรายการ · ${esc(levelData().dateShort)}</h2><p>${esc(state.level)} · ${esc(levelData().mode)} · รายการเสร็จสิ้นและยกเลิกของทั้งวัน</p></div>
        ${chips}
      </div>
      ${body}
    </div>`;
  }

  /* ---------- drawer / modal ---------- */
  function renderOverlay() {
    const root = $('#overlayRoot');
    if (!state.drawer) {
      root.innerHTML = '';
      syncScrollLock();
      return;
    }
    const content = state.drawer.type === 'checkin' ? checkinDrawerHtml() : detailDrawerHtml(state.drawer.id);
    root.innerHTML = `<div class="scrim">${content}</div>`;
    syncScrollLock();
    requestAnimationFrame(() => {
      const close = root.querySelector('.drawer-close');
      if (close) close.focus();
    });
  }
  function syncScrollLock() {
    document.body.classList.toggle('no-scroll', !!(state.drawer || state.modal));
  }
  function openDetail(id) {
    state.drawer = { type: 'detail', id };
    renderOverlay();
  }
  function openCheckin() {
    state.drawer = { type: 'checkin' };
    renderOverlay();
  }
  function closeDrawer() {
    state.drawer = null;
    renderOverlay();
  }

  function detailDrawerHtml(id) {
    const found = findRow(id);
    if (!found) return `<aside class="drawer"><div class="drawer-head"><div><div class="eyebrow">ไม่พบข้อมูล</div><h2>รายการนี้ถูกปิดแล้ว</h2></div><button class="drawer-close" data-action="close-drawer" aria-label="ปิด">${icon('x')}</button></div><div class="drawer-body"></div></aside>`;
    const { row, stream, periodId } = found;
    const p = periodById(periodId) || currentPeriod();
    const handoff = levelData().handoffs.find((h) => h.code === row.code);
    const spec = primarySpec(row, stream);
    const cluster = row.subject === 'Combined' ? `math ${handoff && handoff.from === 'Math' ? 'done' : ''}` : '';
    const stageRows = row.subject === 'Combined' ? `<div class="detail-stages">
      <div class="detail-stage"><span>${icon('check')}SAT Math</span><b>${handoff && handoff.from === 'Math' ? 'เสร็จแล้ว ' + fmt(handoff.finishedAt) : row.status === 'Completed' ? 'เสร็จแล้ว' : 'รอดำเนินการ'}</b></div>
      <div class="detail-stage"><span>${icon('check')}SAT Verbal</span><b>${handoff && handoff.from === 'Verbal' ? 'เสร็จแล้ว ' + fmt(handoff.finishedAt) : handoff && handoff.to === 'Verbal' ? 'รอจัดคิว' : row.status === 'Completed' ? 'เสร็จแล้ว' : 'รอดำเนินการ'}</b></div>
    </div>` : '';
    const questions = (row.subject === 'Combined' ? [
      'อยากวางแผนทั้ง Math และ Verbal ให้ตรงกับเป้าหมายรอบถัดไป',
      'ควรเก็บวิชาไหนก่อน และต้องเตรียมอะไรบ้าง',
      'อยากให้ช่วยแนะนำตารางอ่านหนังสือช่วงปิดเทอม',
    ] : row.subject === 'Math' ? [
      'อยากวางแผนเก็บคะแนน SAT Math ให้ถึง 700 ภายในรอบถัดไป',
      'ควรเน้นเรื่องไหนก่อนระหว่าง Algebra กับ Geometry',
    ] : [
      'อยากพัฒนาการอ่านวิเคราะห์ให้ทันเวลาในข้อสอบ',
      'ควรฝึกทำ passage แบบไหนเพิ่มมากที่สุด',
    ]);
    const actions = spec ? `<button class="btn btn-primary" data-action="primary" data-id="${row.id}">${spec.label}</button>` : `<button class="btn btn-primary" data-action="close-drawer">ปิดรายละเอียด</button>`;
    return `<aside class="drawer" role="dialog" aria-modal="true" aria-labelledby="detailTitle">
      <div class="drawer-head">
        <div><div class="eyebrow">รายละเอียดการจอง</div><h2 id="detailTitle">${esc(row.code)}</h2><p>${esc(row.name)} · ${esc(row.wcode || 'ไม่มี Wcode')}</p></div>
        <button class="drawer-close" data-action="close-drawer" aria-label="ปิด">${icon('x')}</button>
      </div>
      <div class="drawer-body">
        <div class="drawer-section">
          <div class="detail-name">
            <span class="avatar-sm">${esc(row.name.replace('น้อง', '').slice(0, 1) || 'S')}</span>
            <div><strong>${esc(row.name)}</strong><span>${esc(row.level || state.level)} · ${subjectLabel(row.subject)}</span></div>
            <span style="margin-left:auto">${statusPill(row.status, row)}</span>
          </div>
          <div class="detail-grid">
            <div><dt>รอบที่จอง</dt><dd>${esc(levelData().dateShort)} · ${fmt(p.start)}–${fmt(p.endM)}</dd></div>
            <div><dt>รูปแบบ</dt><dd>${esc(levelData().mode)}</dd></div>
            <div><dt>โรงเรียน</dt><dd>Bangkok Christian College</dd></div>
            <div><dt>Email ผู้ปกครอง</dt><dd>parent.${esc(row.code.toLowerCase().replace(/[^a-z0-9]/g, ''))}@example.com</dd></div>
          </div>
          ${stageRows}
        </div>
        <div class="drawer-section">
          <h3>เวลาและสถานะ</h3>
          <div class="detail-times">
            <div><span>เรียกคิว</span><strong>${fmt(row.calledAt)}</strong></div>
            <div><span>เริ่มปรึกษา</span><strong>${fmt(row.startedAt)}</strong></div>
            <div><span>จบการปรึกษา</span><strong>${fmt(row.endedAt)}</strong></div>
            <div><span>ใช้เวลา</span><strong>${row.startedAt != null && row.endedAt != null ? durMin(row.startedAt, row.endedAt) : row.startedAt != null ? 'กำลังทำ' : '—'}</strong></div>
            ${row.status === 'Cancelled' ? `<div><span>เหตุผลยกเลิก</span><strong>${esc(row.reason || '—')}</strong></div>` : ''}
          </div>
        </div>
        <div class="drawer-section">
          <h3>ลิงก์ติดตามคิวของผู้ปกครอง</h3>
          <div class="tracking-box"><span>https://warwick.example/#/track/${esc(row.code.toLowerCase())}-demo</span><button data-action="copy-link" data-url="https://warwick.example/#/track/${esc(row.code.toLowerCase())}-demo" aria-label="คัดลอกลิงก์">${icon('copy')}</button></div>
          <div class="questions" style="margin-top:10px"><div class="question"><b>?</b><span>ลิงก์ตัวอย่างสำหรับต้นแบบเท่านั้น ไม่ผูกกับข้อมูลจริง</span></div></div>
        </div>
        <div class="drawer-section">
          <h3>คำถามถึงอาจารย์</h3>
          <div class="questions">${questions.map((q, i) => `<div class="question"><b>${i + 1}</b><span>${esc(q)}</span></div>`).join('')}</div>
        </div>
      </div>
      <div class="drawer-foot">
        ${actions}
        <button class="btn ghost" data-action="close-drawer">ปิด</button>
      </div>
    </aside>`;
  }

  function checkinDrawerHtml() {
    const p = currentPeriod();
    const rows = bookedRows().sort((a, b) => (a.subject === b.subject ? a.code.localeCompare(b.code) : a.subject.localeCompare(b.subject)));
    const items = rows.length ? rows.map((row) => `<div class="checkin-item">
      <div class="who"><strong>${esc(row.code)} ${row.subject === 'Combined' ? '<span class="q-combined">M+V</span>' : ''}</strong><span>${esc(row.name)} · ${esc(row.wcode)} · ${subjectLabel(row.subject)}</span></div>
      <button class="mini-btn" data-action="checkin" data-id="${row.id}">${icon('check')}เช็กอิน</button>
    </div>`).join('') : `<div class="checkin-empty">${icon('inbox')}<strong>ไม่มีรายการรอเช็กอิน</strong><p>ทุกรายการในรอบนี้เช็กอินแล้ว หรือยังไม่มีผู้ลงทะเบียน</p></div>`;
    return `<aside class="drawer" role="dialog" aria-modal="true" aria-labelledby="checkinTitle">
      <div class="drawer-head">
        <div><div class="eyebrow">CHECK-IN</div><h2 id="checkinTitle">เช็กอิน · ${rows.length} รายการ</h2><p>${esc(levelData().dateShort)} · ${esc(state.level)} · ${fmt(p.start)}–${fmt(p.endM)} · ${esc(levelData().mode)}</p></div>
        <button class="drawer-close" data-action="close-drawer" aria-label="ปิด">${icon('x')}</button>
      </div>
      <div class="drawer-body">
        <div class="checkin-list">${items}</div>
        <div class="drawer-note">${icon('info')}<span>เช็กอินแล้วรายการจะเข้าคิวของห้องนั้น · ต้องกด “จัดคิว” และ “บันทึกลำดับ” ก่อนเรียกคิว · รายการที่ยังไม่มาแสดงในช่อง “รอจัดคิว”</span></div>
      </div>
      <div class="drawer-foot"><button class="btn ghost" data-action="close-drawer">ปิด</button></div>
    </aside>`;
  }

  function renderModal() {
    const root = $('#modalRoot');
    if (!state.modal) {
      root.innerHTML = '';
      syncScrollLock();
      return;
    }
    let html = '';
    if (state.modal.type === 'cancel') {
      const found = findRow(state.modal.id);
      html = `<div class="modal-wrap"><div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
        <div class="modal-head"><h2 id="modalTitle">ยกเลิกการจอง</h2><button class="drawer-close" data-action="close-modal" aria-label="ปิด">${icon('x')}</button></div>
        <div class="modal-body">
          <p>เอา <strong>${found ? esc(found.row.name) + ' · ' + esc(found.row.code) : 'รายการนี้'}</strong> ออกจากคิวและคืนที่ว่าง รายการจะย้ายไปแท็บ “ประวัติ” และคืนการจองภายหลังได้</p>
          <label><span>เหตุผล</span><select id="cancelReason"><option>ผู้ปกครองแจ้งยกเลิก</option><option>ไม่มา / ไม่แจ้งยกเลิก</option><option>ลงทะเบียนผิด</option><option>อื่น ๆ</option></select></label>
          <p style="font-size:10px;color:#9aa7b5">ตัวอย่างในต้นแบบ: กดยืนยันแล้วรายการจะหายจากกระดานและปรากฏในประวัติทันที</p>
        </div>
        <div class="modal-foot"><button class="btn ghost" data-action="close-modal">ไม่ยกเลิก</button><button class="btn btn-danger" data-action="confirm-cancel">ยืนยันยกเลิก</button></div>
      </div></div>`;
    } else if (state.modal.type === 'settings') {
      html = `<div class="modal-wrap"><div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
        <div class="modal-head"><h2 id="modalTitle">ตั้งเวลาต่อคิว Consult</h2><button class="drawer-close" data-action="close-modal" aria-label="ปิด">${icon('x')}</button></div>
        <div class="modal-body">
          <p>เวลาต่อหนึ่งคิวใช้คำนวณเวลาของคิวถัดไปทุกห้อง · ค่าเริ่มต้น 10 นาที</p>
          <label><span>นาทีต่อคิว</span><input id="consultMinutes" type="number" min="1" max="60" value="10"></label>
          <p style="font-size:10px;color:#9aa7b5">ต้นแบบไม่ได้คำนวณเวลาใหม่จริง · ใช้เพื่อแสดงตำแหน่งของหน้าตั้งค่าเท่านั้น</p>
        </div>
        <div class="modal-foot"><button class="btn ghost" data-action="close-modal">ยกเลิก</button><button class="btn btn-primary" data-action="save-settings">บันทึก</button></div>
      </div></div>`;
    } else if (state.modal.type === 'help') {
      html = `<div class="modal-wrap"><div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
        <div class="modal-head"><h2 id="modalTitle">วิธีลองต้นแบบ</h2><button class="drawer-close" data-action="close-modal" aria-label="ปิด">${icon('x')}</button></div>
        <div class="modal-body">
          <p>ต้นแบบนี้เป็น HTML ไฟล์เดียว ใช้ข้อมูลจำลอง ไม่เชื่อมต่อ Backend</p>
          <ul class="help-list">
            <li><b>1</b><span>กด <strong>เรียกคิวถัดไป</strong> / <strong>เริ่มปรึกษา</strong> / <strong>จบการปรึกษา</strong> ในแต่ละห้อง แล้วดูสถานะและเวลาเปลี่ยน</span></li>
            <li><b>2</b><span>กด <strong>ทั้งวัน</strong> เพื่อดูตารางสองคอลัมน์แบบภาพรวมทั้งวัน (คล้ายภาพอ้างอิง) · กดรอบเพื่อกลับมาโฟกัสรอบ</span></li>
            <li><b>3</b><span>กด <strong>รอเช็กอิน</strong> เปิด drawer · ลองเช็กอินแล้วจัดคิว/บันทึกลำดับในเลน</span></li>
            <li><b>4</b><span>ลองย้ายลำดับด้วยลูกศรในช่อง # · จะมีแถบเตือน “ยังไม่บันทึก”</span></li>
            <li><b>5</b><span>ลองค้นหาในช่องค้นหา · แถวอื่นจะจางลงและปุ่มบันทึกลำดับถูกล็อก</span></li>
            <li><b>6</b><span>ย่อ/ขยายเมนูซ้ายด้วยปุ่มลูกศร · ย่อหน้าต่างให้แคบเพื่อดูเลนสลับห้องบนแท็บเล็ต</span></li>
          </ul>
        </div>
        <div class="modal-foot"><button class="btn btn-primary" data-action="close-modal">เข้าใจแล้ว</button></div>
      </div></div>`;
    } else if (state.modal.type === 'reset') {
      html = `<div class="modal-wrap"><div class="modal" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
        <div class="modal-head"><h2 id="modalTitle">รีเซ็ตข้อมูลต้นแบบ</h2><button class="drawer-close" data-action="close-modal" aria-label="ปิด">${icon('x')}</button></div>
        <div class="modal-body"><p>คืนค่าข้อมูลตัวอย่างทั้งหมด ล้างลำดับที่แก้และประวัติการทดลอง · ใช้เมื่อต้องการเริ่มสาธิตใหม่</p></div>
        <div class="modal-foot"><button class="btn ghost" data-action="close-modal">ยกเลิก</button><button class="btn btn-primary" data-action="confirm-reset">รีเซ็ตเดโม</button></div>
      </div></div>`;
    }
    root.innerHTML = html;
    syncScrollLock();
  }
  function closeModal() {
    state.modal = null;
    renderModal();
  }

  /* ---------- menus ---------- */
  function openRowMenu(id, el) {
    const rect = el.getBoundingClientRect();
    state.menu = { type: 'row', id, rect };
    renderMenu();
  }
  function openMoreMenu(el) {
    const rect = el.getBoundingClientRect();
    state.menu = { type: 'more', rect };
    renderMenu();
  }
  function closeMenu() {
    state.menu = null;
    renderMenu();
  }
  function renderMenu() {
    const root = $('#menuRoot');
    if (!state.menu) {
      root.innerHTML = '';
      return;
    }
    let items = [];
    if (state.menu.type === 'more') {
      items = [
        ['menu-help', 'info', 'วิธีลองต้นแบบ'],
        ['menu-reset', 'refresh', 'รีเซ็ตข้อมูลตัวอย่าง'],
        ['menu-logout', 'logout', 'ออกจากระบบ (เดโม)'],
      ];
    } else {
      const found = findRow(state.menu.id);
      if (!found) {
        root.innerHTML = '';
        return;
      }
      const { row, stream } = found;
      const bad = row.status === 'Completed' || row.status === 'Cancelled';
      items.push(['menu-detail', 'info', 'เปิดรายละเอียด']);
      items.push(['menu-edit', 'users', 'แก้ข้อมูลนักเรียน']);
      if (row.status === 'Booked' || row.status === 'Waiting' || row.status === 'No-show') {
        items.push(['menu-swap', 'refresh', 'สลับช่วงเวลากับอีกคน']);
      }
      if (row.status === 'No-show') items.push(['menu-restore', 'phone', 'คืนคิวเข้ารอ']);
      if (row.status === 'Waiting' || row.status === 'Calling') items.push(['menu-noshow', 'x', 'บันทึกว่าไม่มา']);
      if (row.status === 'Calling') items.push(['menu-hold', 'clock', 'พักคิว']);
      if (row.status === 'Completed') items.push(['menu-archive', 'inbox', row.archived ? 'นำกลับมาแสดง' : 'เก็บประวัติ']);
      if (!bad || row.status === 'Cancelled') {
        items.push(['divider']);
        items.push(['menu-cancel', 'alert', 'ยกเลิกการจอง', 'danger']);
      }
    }
    const height = items.reduce((sum, it) => sum + (it[0] === 'divider' ? 11 : 34), 14);
    const w = 224;
    const rect = state.menu.rect;
    let left = Math.min(Math.max(8, rect.right - w), window.innerWidth - w - 10);
    let top = rect.bottom + 6;
    if (top + height > window.innerHeight - 8) top = Math.max(8, rect.top - height - 6);
    root.innerHTML = `<div class="menu" style="left:${left}px;top:${top}px">${items.map((it) => {
      if (it[0] === 'divider') return `<div class="divider"></div>`;
      const [action, ic, label, cls] = it;
      return `<button data-action="${action}" data-id="${state.menu.id || ''}" class="${cls || ''}">${icon(ic)}${label}</button>`;
    }).join('')}</div>`;
  }

  /* ---------- actions ---------- */
  function paint() {
    renderChrome();
    renderView();
    renderOverlay();
    renderModal();
    syncScrollLock();
  }

  function startDirty(stream) {
    const other = otherStream(stream);
    if (state.dirty[other]) {
      toast('บันทึกลำดับห้อง ' + other + ' ก่อน');
      return false;
    }
    if (!state.dirty[stream]) {
      state.snapshots[stream] = { periodId: state.periodId, rows: clone(rowsFor(state.periodId, stream)) };
      state.dirty[stream] = true;
    }
    return true;
  }
  function saveDraft(stream) {
    if (!state.dirty[stream]) return;
    if (state.search.trim()) {
      toast('ล้างคำค้นก่อนบันทึกลำดับ');
      return;
    }
    materializeHandoffs(stream);
    state.dirty[stream] = false;
    state.snapshots[stream] = null;
    state.lastSaved[stream] = nowInt();
    invalidateEstimates();
    paint();
    toast('บันทึกลำดับห้อง ' + stream + ' แล้ว · ประมาณการเวลาใหม่พร้อมใช้งาน');
  }
  function materializeHandoffs(stream) {
    for (const h of levelData().handoffs) {
      if (h.order == null || h.materialized || h.to !== stream) continue;
      const bucket = levelData().rows[h.periodId] && levelData().rows[h.periodId][h.to];
      if (!bucket) continue;
      if (!bucket.some((r) => r.code === h.code && String(r.id).indexOf('::') >= 0)) {
        bucket.push({
          id: h.code + '::' + h.to, code: h.code, name: h.name, wcode: h.wcode, subject: 'Combined',
          status: 'Waiting', order: h.order, calledAt: null, startedAt: null, endedAt: null,
          archived: false, reason: '', cancelledAt: null, handoffFrom: h.from,
        });
      }
      h.materialized = true;
    }
  }
  function cancelDraft(stream) {
    const snap = state.snapshots[stream];
    if (snap && levelData().rows[snap.periodId]) {
      levelData().rows[snap.periodId][stream] = clone(snap.rows);
    }
    state.dirty[stream] = false;
    state.snapshots[stream] = null;
    invalidateEstimates();
    paint();
    toast('ยกเลิกการเปลี่ยนลำดับห้อง ' + stream + ' แล้ว');
  }
  function moveRow(id, dir) {
    const found = findRow(id);
    if (!found) return;
    const stream = found.stream;
    if (state.search.trim()) {
      toast('ล้างคำค้นก่อนจัดลำดับ');
      return;
    }
    if (state.dirty[otherStream(stream)]) {
      toast('บันทึกลำดับห้อง ' + otherStream(stream) + ' ก่อน');
      return;
    }
    const ordered = rowsFor(state.periodId, stream)
      .filter((r) => r.status === 'Waiting' && r.order > 0)
      .sort((a, b) => a.order - b.order);
    const i = ordered.findIndex((r) => r.id === id);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= ordered.length) return;
    if (!startDirty(stream)) return;
    const a = ordered[i];
    const b = ordered[j];
    const t = a.order;
    a.order = b.order;
    b.order = t;
    invalidateEstimates();
    paint();
  }
  function assignRow(row, stream) {
    if (!startDirty(stream)) return;
    const ordered = rowsFor(state.periodId, stream).filter((r) => r.status === 'Waiting' && r.order > 0);
    const max = ordered.reduce((m, r) => Math.max(m, r.order), 0);
    row.order = max + 1;
    invalidateEstimates();
    paint();
    toast('เพิ่ม ' + row.code + ' เข้าลำดับท้ายสุดแล้ว · กด “บันทึกลำดับ” เพื่อยืนยัน');
  }
  function assignHandoff(hoId) {
    const h = findHandoff(hoId);
    if (!h) return;
    const stream = h.to;
    if (!startDirty(stream)) return;
    const ordered = rowsFor(h.periodId, stream).filter((r) => r.status === 'Waiting' && r.order > 0);
    const max = ordered.reduce((m, r) => Math.max(m, r.order), 0);
    h.order = max + 1;
    invalidateEstimates();
    paint();
    toast('จัดคิว ' + h.code + ' เข้าห้อง ' + stream + ' แล้ว · กด “บันทึกลำดับ”');
  }
  function checkInRow(id) {
    const found = findRow(id);
    if (!found) return;
    found.row.status = 'Waiting';
    found.row.order = null;
    invalidateEstimates();
    paint();
    toast('เช็กอิน ' + found.row.code + ' แล้ว · กด “จัดคิว” ในห้อง ' + found.stream);
  }
  function callRow(row, stream) {
    if (laneLive(stream)) {
      toast('ห้อง ' + stream + ' กำลังให้บริการอยู่');
      return;
    }
    if (row.order == null) {
      toast('ต้องจัดคิวก่อนเรียก');
      return;
    }
    row.status = 'Calling';
    row.calledAt = nowInt();
    invalidateEstimates();
    paint();
    toast('เรียก ' + row.code + ' ' + row.name + ' แล้ว');
  }
  function startRow(row) {
    row.status = 'Serving';
    row.startedAt = nowInt();
    invalidateEstimates();
    paint();
    toast('เริ่มปรึกษา ' + row.code + ' แล้ว');
  }
  function finishRow(row, stream) {
    const pending = levelData().handoffs.find((h) => h.code === row.code && !h.materialized && h.from === stream);
    if (pending) {
      row.status = 'Completed';
      row.endedAt = nowInt();
      pending.order = null;
      pending.finishedAt = nowInt();
      invalidateEstimates();
      paint();
      toast('จบ ' + stream + ' แล้ว · ส่งต่อ ' + otherStream(stream) + ' รอจัดคิว');
      return;
    }
    row.status = 'Completed';
    row.endedAt = nowInt();
    row.order = null;
    invalidateEstimates();
    paint();
    toast('จบการปรึกษา ' + row.code + ' แล้ว');
  }
  function restoreRow(row) {
    row.status = 'Waiting';
    row.calledAt = null;
    invalidateEstimates();
    paint();
    toast('คืนคิว ' + row.code + ' เข้ารอแล้ว');
  }
  function performPrimary(id) {
    const found = findRow(id);
    if (!found) return;
    const { row, stream } = found;
    if (row.status === 'Booked') return checkInRow(id);
    if (row.status === 'Waiting') return row.order == null ? assignRow(row, stream) : callRow(row, stream);
    if (row.status === 'Calling') return startRow(row, stream);
    if (row.status === 'Serving') return finishRow(row, stream);
    if (row.status === 'No-show') return restoreRow(row);
  }
  function callNext(stream) {
    if (laneLive(stream)) {
      toast('ห้อง ' + stream + ' กำลังให้บริการอยู่');
      return;
    }
    if (state.search.trim()) {
      toast('ล้างคำค้นก่อนเรียกคิว');
      return;
    }
    if (state.dirty.Math || state.dirty.Verbal) {
      toast('บันทึกลำดับก่อนเรียกคิว');
      return;
    }
    const next = rowsFor(state.periodId, stream)
      .filter((r) => r.status === 'Waiting' && r.order > 0)
      .sort((a, b) => a.order - b.order)[0];
    if (!next) {
      toast('ยังไม่มีคิวที่พร้อมเรียกในรอบนี้');
      return;
    }
    callRow(next, stream);
  }
  function toggleDay() {
    if (state.dirty.Math || state.dirty.Verbal) {
      toast('บันทึกหรือยกเลิกลำดับก่อนเปลี่ยนมุมมอง');
      return;
    }
    state.dayMode = !state.dayMode;
    state.view = 'board';
    paint();
  }
  function goPeriod(id) {
    if (state.dirty.Math || state.dirty.Verbal) {
      toast('บันทึกหรือยกเลิกลำดับก่อนเปลี่ยนรอบ');
      return;
    }
    if (!periodById(id)) return;
    state.periodId = id;
    state.dayMode = false;
    state.view = 'board';
    const waits = { Math: rowsFor(id, 'Math').filter((r) => r.status === 'Waiting').length, Verbal: rowsFor(id, 'Verbal').filter((r) => r.status === 'Waiting').length };
    if ((id === 'P1300_1400' || id === 'P1400_1530' || id === 'P1530_1700') && waits.Math === 0 && waits.Verbal > 0) state.mobileLane = 'Verbal';
    paint();
  }
  function setLevel(level) {
    if (!DATA[level]) return;
    if (state.dirty.Math || state.dirty.Verbal) {
      toast('บันทึกหรือยกเลิกลำดับก่อนเปลี่ยนระดับ');
      renderChrome();
      return;
    }
    state.level = level;
    const d = levelData();
    state.nowBase = d.demoNow;
    state.tickStart = Date.now();
    state.lastSync = nowInt();
    state.periodId = level === 'Beginner' ? 'P1700_1800' : 'P1155_1325';
    state.dayMode = false;
    state.view = 'board';
    state.search = '';
    state.mobileLane = level === 'Beginner' ? 'Math' : 'Math';
    invalidateEstimates();
    paint();
    toast('สลับเป็น ' + level + ' · ' + d.dateLabel + ' (' + d.mode + ')');
  }
  function toggleTab(tab) {
    state.view = tab;
    paint();
  }
  function refresh() {
    state.lastSync = nowInt();
    renderChrome();
    toast('อัปเดตล่าสุดแล้ว · ข้อมูลจำลอง ณ ' + fmtS(nowInt()));
  }
  function resetDemo() {
    const fresh = clone(DATA);
    for (const k of Object.keys(fresh)) store[k] = fresh[k];
    state.level = 'Beginner';
    state.periodId = 'P1700_1800';
    state.view = 'board';
    state.dayMode = false;
    state.search = '';
    state.historyFilter = 'all';
    state.historyArchived = false;
    state.dirty = { Math: false, Verbal: false };
    state.snapshots = { Math: null, Verbal: null };
    state.lastSaved = { Math: T(17, 5), Verbal: T(17, 4) };
    state.nowBase = DATA.Beginner.demoNow;
    state.tickStart = Date.now();
    state.lastSync = nowInt();
    state.mobileLane = 'Math';
    invalidateEstimates();
    closeModal();
    paint();
    toast('รีเซ็ตข้อมูลต้นแบบแล้ว');
  }
  function toast(message) {
    const el = $('#toast');
    if (!el) return;
    el.textContent = message;
    el.classList.add('show');
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove('show'), 3600);
  }

  /* ---------- events ---------- */
  document.addEventListener('click', (event) => {
    const anchor = event.target.closest('a[href="#"]');
    if (anchor) event.preventDefault();

    const menuRoot = $('#menuRoot');
    if (state.menu && menuRoot && !event.target.closest('#menuRoot') &&
      !event.target.closest('[data-action="row-menu"],[data-action="now-menu"],[data-action="more-actions"]')) {
      closeMenu();
    }

    if (event.target.classList && event.target.classList.contains('scrim')) {
      closeDrawer();
      return;
    }
    if (event.target.classList && event.target.classList.contains('modal-wrap')) {
      closeModal();
      return;
    }

    const el = event.target.closest('[data-action]');
    if (!el) return;
    const action = el.dataset.action;
    if (action.indexOf('menu-') === 0) closeMenu();

    switch (action) {
      case 'toggle-rail':
        $('#app').classList.toggle('rail-open');
        break;
      case 'tab':
        toggleTab(el.dataset.tab);
        break;
      case 'period':
        goPeriod(el.dataset.period);
        break;
      case 'toggle-day':
        toggleDay();
        break;
      case 'clear-search':
        state.search = '';
        paint();
        break;
      case 'switch-lane':
        state.mobileLane = el.dataset.stream;
        paint();
        break;
      case 'open-checkin':
        openCheckin();
        break;
      case 'close-drawer':
        closeDrawer();
        break;
      case 'close-modal':
        closeModal();
        break;
      case 'primary':
        performPrimary(el.dataset.id);
        break;
      case 'call-next':
        callNext(el.dataset.stream);
        break;
      case 'row-menu':
        openRowMenu(el.dataset.id, el);
        break;
      case 'now-menu':
        openRowMenu(el.dataset.id, el);
        break;
      case 'save-order':
        saveDraft(el.dataset.stream);
        break;
      case 'cancel-draft':
        cancelDraft(el.dataset.stream);
        break;
      case 'move':
        moveRow(el.dataset.id, Number(el.dataset.dir));
        break;
      case 'assign-handoff':
        assignHandoff(el.dataset.ho);
        break;
      case 'go-handoff': {
        const h = findHandoff(el.dataset.ho);
        if (h) goPeriod(h.periodId);
        break;
      }
      case 'checkin':
        checkInRow(el.dataset.id);
        break;
      case 'open-detail':
        openDetail(el.dataset.id);
        break;
      case 'history-filter':
        state.historyFilter = el.dataset.filter;
        paint();
        break;
      case 'history-archived':
        state.historyArchived = !state.historyArchived;
        paint();
        break;
      case 'refresh':
        refresh();
        break;
      case 'export':
        toast('เดโม: ปุ่ม Export จะดาวน์โหลด Excel ของผู้ลงทะเบียนทุกวัน (ยังไม่เชื่อมต่อระบบจริง)');
        break;
      case 'settings':
        state.modal = { type: 'settings' };
        renderModal();
        break;
      case 'help':
        state.modal = { type: 'help' };
        renderModal();
        break;
      case 'more-actions':
        openMoreMenu(el);
        break;
      case 'copy-link':
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(el.dataset.url || '');
        }
        toast('คัดลอกลิงก์ติดตามคิวแล้ว (ตัวอย่าง)');
        break;
      case 'reset':
        state.modal = { type: 'reset' };
        renderModal();
        break;
      case 'confirm-reset':
        resetDemo();
        break;
      case 'save-settings':
        closeModal();
        toast('บันทึกเวลาต่อคิว 10 นาทีแล้ว (ตัวอย่าง)');
        break;
      case 'confirm-cancel': {
        const found = findRow(state.modal && state.modal.id);
        if (found) {
          const sel = $('#cancelReason');
          found.row.status = 'Cancelled';
          found.row.cancelledAt = nowInt();
          found.row.reason = sel ? sel.value : 'ผู้ปกครองแจ้งยกเลิก';
          found.row.order = null;
          if (found.row.archived) found.row.archived = false;
        }
        closeModal();
        invalidateEstimates();
        paint();
        toast('ยกเลิกการจองแล้ว · รายการย้ายไปแท็บประวัติ');
        break;
      }
      case 'menu-detail': {
        const id = el.dataset.id;
        openDetail(id);
        break;
      }
      case 'menu-edit':
        toast('เดโม: จะเปิดฟอร์มแก้ข้อมูลนักเรียน (ยังไม่แสดงในต้นแบบ)');
        break;
      case 'menu-swap':
        toast('เดโม: จะเปิดหน้าต่างสลับช่วงเวลากับอีกราย');
        break;
      case 'menu-noshow': {
        const found = findRow(el.dataset.id);
        if (found) {
          found.row.status = 'No-show';
          found.row.calledAt = found.row.calledAt || nowInt();
          invalidateEstimates();
          paint();
          toast('บันทึก ' + found.row.code + ' ว่าไม่มาแล้ว');
        }
        break;
      }
      case 'menu-hold': {
        const found = findRow(el.dataset.id);
        if (found) {
          found.row.status = 'Waiting';
          invalidateEstimates();
          paint();
          toast('พักคิว ' + found.row.code + ' แล้ว');
        }
        break;
      }
      case 'menu-restore': {
        const found = findRow(el.dataset.id);
        if (found) restoreRow(found.row);
        break;
      }
      case 'menu-archive': {
        const found = findRow(el.dataset.id);
        if (found) {
          found.row.archived = !found.row.archived;
          paint();
          toast(found.row.archived ? 'เก็บประวัติ ' + found.row.code + ' แล้ว' : 'นำ ' + found.row.code + ' กลับมาแสดงแล้ว');
        }
        break;
      }
      case 'menu-cancel':
        state.modal = { type: 'cancel', id: el.dataset.id };
        renderModal();
        break;
      case 'menu-help':
        state.modal = { type: 'help' };
        renderModal();
        break;
      case 'menu-reset':
        state.modal = { type: 'reset' };
        renderModal();
        break;
      case 'menu-logout':
        toast('เดโม: จะออกจากระบบและกลับไปหน้า Login');
        break;
      default:
        break;
    }
  });

  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    if (state.menu) closeMenu();
    else if (state.modal) closeModal();
    else if (state.drawer) closeDrawer();
  });
  window.addEventListener('resize', () => {
    if (state.menu) closeMenu();
  });
  $('#levelSel').addEventListener('change', (event) => setLevel(event.target.value));
  $('#searchInput').addEventListener('input', (event) => {
    state.search = event.target.value;
    renderChrome();
    renderView();
  });

  function tick() {
    const now = nowInt();
    const demo = $('#demoClock');
    if (demo) demo.textContent = fmt(now);
    $$('.js-elapsed').forEach((el) => {
      const start = Number(el.dataset.start);
      if (!Number.isFinite(start)) return;
      const diff = Math.max(0, now - start);
      el.textContent = 'ใช้ไป ' + clockSpan(diff);
      const over = el.parentElement ? el.parentElement.querySelector('.js-over') : null;
      if (over) {
        const od = diff - 600;
        if (od > 0) {
          over.hidden = false;
          over.textContent = 'เกิน ' + clockSpan(od);
          over.classList.add('over');
        } else {
          over.hidden = true;
        }
      }
    });
    if (now - (state.lastSync == null ? now : state.lastSync) >= 5) {
      state.lastSync = now;
      const st = $('#syncText');
      if (st) st.textContent = 'ซิงก์ ' + fmtS(now) + ' · ทุก 5 วิ';
    }
  }

  /* ---------- boot ---------- */
  state.lastSync = nowInt();
  paint();
  setInterval(tick, 1000);
})();
