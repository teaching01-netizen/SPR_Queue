import { useCallback, useEffect, useState } from 'react';
import { Monitor, Radio, RefreshCw, Settings, Ticket } from 'lucide-react';
import { api } from '../api';
import { allDates, DATE_LABELS, modeFor } from '../config';
import { clockLabel, freshnessBand } from '../freshness';
import { useLive, useNow, useOnline, useWakeLock } from '../hooks';
import { trackHref } from '../routes';
import { readDisplayScope, readTrackToken, writeDisplayScope } from '../storage';
import { Header, Notice } from '../ui';
import { FreshnessBanner } from './TrackPage';
import type { Level } from '../types';

type DisplayProps = { level?: string; date?: string; kiosk: boolean };

function asLevel(value: string): Level | null {
  return value === 'Beginner' || value === 'High Level' ? value : null;
}

export function DisplayPage({ level: levelProp = '', date: dateProp = '', kiosk }: DisplayProps) {
  const remembered = readDisplayScope();
  const [level, setLevel] = useState<Level>(() => asLevel(levelProp) ?? asLevel(remembered?.level ?? '') ?? 'Beginner');
  const [dateState, setDateState] = useState(dateProp || remembered?.date || '');
  const [showControls, setShowControls] = useState(!kiosk);
  useWakeLock(kiosk);

  useEffect(() => {
    const next = asLevel(levelProp);
    if (next) setLevel(next);
  }, [levelProp]);
  useEffect(() => {
    if (dateProp) setDateState(dateProp);
  }, [dateProp]);

  const options = allDates(level);
  const date = options.includes(dateState) ? dateState : (options[0] ?? dateState);
  useEffect(() => {
    if (date) writeDisplayScope({ level, date });
  }, [level, date]);

  const fetcher = useCallback(() => api.display(level, date), [level, date]);
  const { data, error, refresh, updated } = useLive(fetcher, { cacheKey: `display.${level}.${date}` });
  const online = useOnline();
  const now = useNow(5000);
  const ageMs = updated ? now - updated.getTime() : null;
  const band = freshnessBand({ ageMs, offline: !online });
  const mode = modeFor(level, date);
  const onlineDay = mode.toLowerCase().includes('online');

  const controls = showControls ? (
    <div className="display-controls">
      <label>ระดับ<select value={level} onChange={e => { const next = e.target.value === 'High Level' ? 'High Level' : 'Beginner'; setLevel(next); setDateState(''); }}><option>Beginner</option><option>High Level</option></select></label>
      <label>วันกิจกรรม<select value={date} onChange={e => setDateState(e.target.value)}>{options.map(d => <option key={d} value={d}>{DATE_LABELS[d] ?? d}</option>)}</select></label>
      <div className="live-label"><span className="dot" /> อัปเดตอัตโนมัติ</div>
    </div>
  ) : null;

  const board = (
    <>
      <FreshnessBanner band={band} updated={updated} />
      {error && <Notice error>{error}</Notice>}
      {onlineDay ? (
        <div className="online-board">
          <Monitor size={38} />
          <h2>วันนี้เป็นรอบ Online ผ่าน Zoom</h2>
          <p>ไม่มีการเรียกคิวที่ห้อง · กรุณาติดตามลิงก์คิวของคุณและเข้าห้อง Zoom ตามเวลาที่จองไว้</p>
        </div>
      ) : (
        <div className="display-grid">
          {(['Math', 'Verbal'] as const).map(stream => {
            const queues = data?.[stream] ?? [];
            const upcoming = data?.[`${stream}Next`] ?? [];
            return (
              <div key={stream} className={`display-panel display-${stream.toLowerCase()}`}>
                <div className="display-panel-header"><span>SAT {stream}</span><Radio size={24} /></div>
                <div className="display-now">NOW CALLING / SERVING</div>
                {queues.length ? queues.map(b => <div className="display-number" key={b.queueNo}><strong>{b.queueNo}</strong><p>{b.state === 'Serving' ? 'กำลังปรึกษา' : 'กรุณาไปที่'} · {b.counter}</p></div>) : <div className="display-number"><strong>—</strong><p>กรุณารอประกาศเรียกคิว</p></div>}
                <div className="display-upcoming" aria-label={`คิวถัดไป SAT ${stream}`}>
                  <div className="display-upcoming-title"><span>คิวถัดไป · เตรียมพร้อม</span><small>ตามลำดับที่ Staff บันทึก</small></div>
                  {upcoming.length ? <ol>{upcoming.map((b, i) => <li key={b.queueNo}><span className="upcoming-index">{i + 1}</span><strong>{b.queueNo}</strong><span className="upcoming-period">{b.periodStart}–{b.periodEnd}</span></li>)}</ol> : <p className="upcoming-empty">ยังไม่มีคิวที่พร้อมเรียกถัดไป</p>}
                  <p className="upcoming-note">ลำดับอาจเปลี่ยนได้ กรุณารอประกาศเรียกคิว</p>
                </div>
                <div className="display-panel-footer"><Ticket size={18} />เมื่อถึงคิว กรุณาติดต่อทีมงานหน้าห้อง</div>
              </div>
            );
          })}
        </div>
      )}
      <p className="parent-note">อัปเดตล่าสุด {clockLabel(updated ? updated.getTime() : null)} น. · {band === 'offline' ? 'ออฟไลน์ แสดงข้อมูลล่าสุด' : 'อัปเดตอัตโนมัติทุก 5 วินาที'}</p>
    </>
  );

  if (kiosk) {
    return (
      <div className="kiosk-board">
        {controls}
        <button className="kiosk-gear icon-btn" aria-label="ตั้งค่าจอแสดงคิว" onClick={() => setShowControls(v => !v)}><Settings size={16} /></button>
        {board}
      </div>
    );
  }

  return (
    <>
      <Header eyebrow="LIVE AT WARWICK" title="จอแสดงคิว" description="ติดตามรหัสคิวที่กำลังเรียก แยก SAT Math และ SAT Verbal">
        <div className="header-actions">
          <a className="btn secondary" href={trackHref(readTrackToken())}><Ticket size={16} /> ติดตามคิวของคุณ</a>
          <button className="btn secondary" onClick={() => void refresh()}><RefreshCw size={16} /> รีเฟรช</button>
        </div>
      </Header>
      {controls}
      {board}
    </>
  );
}
