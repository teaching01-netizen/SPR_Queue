import { useCallback, useMemo, useState } from 'react';
import { ArrowRight, CheckCheck, CircleHelp, Clock3, Monitor, Phone, Radio, RefreshCw, Ticket, Users, X } from 'lucide-react';
import { api } from '../api';
import { clockLabel, freshnessBand } from '../freshness';
import type { Freshness } from '../freshness';
import { useLive, useNow, useOnline } from '../hooks';
import { displayHref } from '../routes';
import { Badge, Header, Notice } from '../ui';
import type { ParentView } from '../types';

const SUPPORT_PHONE = '02-658-4880';

export function FreshnessBanner({ band, updated }: { band: Freshness; updated: Date | null }) {
  const label = clockLabel(updated ? updated.getTime() : null);
  if (band === 'fresh') return null;
  if (band === 'ancient') {
    return (
      <div className="freshness-banner ancient" role="status">
        <CircleHelp size={16} />
        <span>ข้อมูลนี้เก่าเกินไป · อาจไม่ตรงกับสถานะปัจจุบัน กรุณาติดต่อทีมงาน {SUPPORT_PHONE}</span>
      </div>
    );
  }
  if (band === 'offline') {
    return (
      <div className="freshness-banner offline" role="status">
        <CircleHelp size={16} />
        <span>ออฟไลน์ · ข้อมูลล่าสุด {label} น. · ระบบจะอัปเดตอัตโนมัติเมื่อการเชื่อมต่อกลับมา</span>
      </div>
    );
  }
  return (
    <div className="freshness-banner delayed" role="status">
      <RefreshCw size={16} />
      <span>กำลังอัปเดตข้อมูล · ข้อมูลล่าสุด {label} น.</span>
    </div>
  );
}

export function TrackPage({ token }: { token: string }) {
  if (!token) return <TrackEntry />;
  return <TrackTicket token={token} />;
}

function TrackEntry() {
  const [link, setLink] = useState('');
  const [inputError, setInputError] = useState('');
  return (
    <>
      <Header eyebrow="YOUR PRIVATE QUEUE" title="ติดตามคิวของคุณ" description="เปิดลิงก์ติดตามคิวที่ได้รับหลังจองสำเร็จ" />
      <div className="card tracking-entry">
        <div className="round-icon"><Ticket size={28} /></div>
        <h2>คิวของคุณ อยู่ที่นี่</h2>
        <p>ใช้ลิงก์ส่วนตัวเพื่อดูรหัสคิวของคุณ<br />และคิวที่กำลังเรียกในวิชาที่ปรึกษา</p>
        <form
          onSubmit={e => {
            e.preventDefault();
            const match = link.match(/#\/track\/(.+)$/);
            if (!match) { setInputError('กรุณาวางลิงก์ติดตามคิวให้ครบ'); return; }
            location.hash = `#/track/${match[1]}`;
          }}
        >
          <label htmlFor="tracking-link">ลิงก์ติดตามคิว</label>
          <input id="tracking-link" type="text" placeholder="วางลิงก์ที่ได้รับหลังการจอง" value={link} onChange={e => setLink(e.target.value)} required />
          <button className="btn primary">เปิดคิวของคุณ<ArrowRight size={17} /></button>
        </form>
        {inputError && <Notice error>{inputError}</Notice>}
        <p className="parent-note">
          ลืมลิงก์หรือเปิดไม่ได้? ติดต่อทีม Warwick ที่ <a href={`tel:${SUPPORT_PHONE.replace(/-/g, '')}`}>{SUPPORT_PHONE}</a>
          <br /><a className="text-btn" href={displayHref('', '')}><Monitor size={14} /> ดูจอแสดงคิวที่ห้อง</a>
        </p>
      </div>
    </>
  );
}

function TrackTicket({ token }: { token: string }) {
  const fetcher = useCallback(() => api.track(token), [token]);
  const { data, error, errorStatus, loading, updated, refresh } = useLive(fetcher, { refreshOnResume: true, cacheKey: token });
  const online = useOnline();
  const now = useNow(5000);
  const ageMs = updated ? now - updated.getTime() : null;
  const band = useMemo(() => freshnessBand({ ageMs, offline: !online }), [ageMs, online]);
  const announcement = useMemo(() => {
    if (!data) return '';
    if (data.status === 'Calling') return `ถึงคิวของคุณแล้ว กรุณาไปที่ ${data.counter || `${data.currentSubject} Room`}`;
    if (data.status === 'Serving') return 'กำลังรับการปรึกษาอยู่';
    if (data.status === 'Completed') return 'การปรึกษาเสร็จสิ้นแล้ว';
    if (data.status === 'Cancelled') return 'การจองถูกยกเลิก';
    if (data.status === 'No-show') return 'กรุณาติดต่อทีมงานเพื่อตรวจสอบสถานะคิว';
    return '';
  }, [data]);

  const header = (
    <Header eyebrow="YOUR PRIVATE QUEUE" title="ติดตามคิวของคุณ" description="กรุณารอติดตามรหัสคิว เมื่อถึงคิวทีมงานจะเรียกเข้าพบอาจารย์">
      <div className="header-actions">
        <a className="btn secondary" href={displayHref('', '')}><Monitor size={15} /> จอคิว</a>
        <button className="btn secondary" onClick={() => void refresh()}><RefreshCw size={16} /> รีเฟรช</button>
      </div>
    </Header>
  );

  if (errorStatus === 404) {
    return (
      <>
        {header}
        <div className="card empty">
          <CircleHelp size={34} />
          <h2>ลิงก์นี้ใช้ไม่ได้แล้ว</h2>
          <p>ไม่พบการจองสำหรับลิงก์นี้ · ติดต่อทีม Warwick ที่ {SUPPORT_PHONE}</p>
          <a className="btn secondary" href="#/track">วางลิงก์ติดตามคิวอีกครั้ง</a>
        </div>
      </>
    );
  }

  if (!data) {
    if (loading) return <>{header}<div className="card empty">กำลังโหลดสถานะคิว…</div></>;
    return (
      <>
        {header}
        <FreshnessBanner band="offline" updated={updated} />
        <div className="card empty">
          <CircleHelp size={34} />
          <h2>ยังโหลดสถานะคิวไม่ได้</h2>
          <p>{error || 'เชื่อมต่อระบบไม่ได้'} · ระบบจะลองใหม่ให้อัตโนมัติ</p>
          <button className="btn secondary" onClick={() => void refresh()}>ลองอีกครั้ง</button>
          <a className="btn secondary" href="#/track">กลับหน้าติดตามคิว</a>
        </div>
      </>
    );
  }

  return (
    <>
      {header}
      <FreshnessBanner band={band} updated={updated} />
      <p className="sr-only" aria-live="polite">{announcement}</p>
      <div className="tracking-layout">
        <div className="parent-ticket">
          <div className="parent-ticket-header"><span><Ticket size={18} /> YOUR QUEUE</span><span>SAT {data.currentSubject}</span></div>
          <p>คิวของคุณ</p>
          <div className={`parent-queue ${!data.queueNo ? 'preparing' : ''}`}>{data.queueNo || 'กำลังจัดคิว'}</div>
          <div className="parent-state"><Badge status={data.status} /></div>
          <div className="ticket-perforation" />
          <div className="parent-bottom">
            {data.status === 'Calling' ? <><Radio size={23} /><h2>ถึงคิวของคุณแล้ว</h2><p>กรุณาไปที่ {data.counter || `${data.currentSubject} Room`}</p></>
              : data.status === 'Serving' ? <><Users size={23} /><h2>กำลังปรึกษาอาจารย์</h2><p>ทีม Warwick พร้อมช่วยวางแผนก้าวต่อไป</p></>
              : data.status === 'Completed' ? <><CheckCheck size={23} /><h2>เสร็จสิ้นการปรึกษา</h2><p>ขอบคุณที่มาพูดคุยกับ Warwick</p></>
              : data.status === 'Cancelled' ? <><X size={23} /><h2>การจองถูกยกเลิก</h2><p>ติดต่อทีม Warwick หากต้องการสอบถาม</p></>
              : data.status === 'No-show' ? <><CircleHelp size={23} /><h2>กรุณาติดต่อทีมงาน</h2><p>ทีมงานจะช่วยตรวจสอบสถานะคิว</p></>
              : <><Clock3 size={23} /><h2>{data.queueNo ? 'กรุณารอเรียกคิว' : 'ยืนยันการจองแล้ว'}</h2><p>{data.queueNo ? 'ติดตามรหัสคิวปัจจุบันได้จากหน้านี้' : 'ทีมงานกำลังเตรียมรหัสคิวของคุณ'}</p></>}
          </div>
        </div>
        <div className="card current-queue-card">
          <div className="live-label"><span className="dot" /> LIVE QUEUE</div>
          <h2>คิวปัจจุบัน</h2>
          <p>SAT {data.currentSubject}</p>
          {data.currentQueues.length ? data.currentQueues.map(q => <div className="current-number" key={q.queueNo}><strong>{q.queueNo}</strong><span>{q.state === 'Serving' ? 'กำลังปรึกษา' : 'กำลังเรียก'}</span></div>) : <div className="current-number"><strong className="no-queue">—</strong><span>ยังไม่มีคิวที่กำลังเรียก</span></div>}
          {data.queueCountState !== 'inactive' && (
            <div className="queue-remaining" aria-live="polite">
              <h3>อีกกี่คิวถึงคุณ · SAT {data.currentSubject}</h3>
              {data.queuesUntilTurn !== null && data.queuesUntilTurn !== undefined ? (data.queuesUntilTurn === 0 ? <p className="remaining-current">ถึงคิวของคุณแล้ว</p> : <><div className="remaining-value"><span>เหลืออีก</span><strong>{data.queuesUntilTurn}</strong><span>คิวถึงคุณ</span></div><p>{data.queuesUntilTurn === 1 ? 'คุณเป็นคิวถัดไป · เตรียมพร้อมรอประกาศ' : 'นับตามลำดับคิวปัจจุบันของห้องนี้'}</p></>) : <p className="remaining-unavailable">{data.queueCountState === 'awaiting-check-in' ? 'เช็กอินกับ Staff ก่อน ระบบจึงจะคำนวณคิวให้' : data.queueCountState === 'staff-review' ? 'กรุณาติดต่อ Staff เพื่อตรวจลำดับคิวของคุณ' : 'กำลังจัดลำดับคิวสำหรับวิชานี้'}</p>}
              <small>จำนวนปรับตามการเรียก ข้าม ยกเลิก และจัดลำดับใหม่ · กรุณารอประกาศเรียกคิว</small>
            </div>
          )}
          <div className="live-footer"><RefreshCw size={13} />อัปเดตล่าสุด {clockLabel(updated ? updated.getTime() : null)} น. · อัปเดตอัตโนมัติทุก 5 วินาที</div>
        </div>
      </div>
      <p className="parent-note">
        หน้าติดตามคิวจะอัปเดตเองเมื่อการเชื่อมต่อกลับมา · สอบถามทีมงาน <a href={`tel:${SUPPORT_PHONE.replace(/-/g, '')}`}><Phone size={12} /> {SUPPORT_PHONE}</a>
      </p>
    </>
  );
}
