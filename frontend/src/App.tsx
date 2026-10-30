import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode, FormEvent } from 'react';
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp, CalendarDays, Check, CheckCheck, ChevronRight, CircleHelp, Clock3, Copy, GraduationCap, LayoutDashboard, ListOrdered, MapPin, Monitor, Plus, Radio, RefreshCw, Search, ShieldCheck, Sparkles, Ticket, Users, X, ExternalLink, Mail, Video, BookOpen, CheckCircle2 } from 'lucide-react';
import { api } from './api';
import { Button as UIAction } from '@openai/apps-sdk-ui/components/Button';
import { Input as UIInput } from '@openai/apps-sdk-ui/components/Input';
import { Badge as UIBadge } from '@openai/apps-sdk-ui/components/Badge';
import type { StaffUser } from './api';
import { EVENT, configureEvent, allDates, capacityFor, SUBJECTS, DATE_LABELS, DEFAULT_SCOPE, STATUS_LABELS, allowedDates, modeFor, periodLabel, periodsFor, subjectLabel, seminarsFor } from './config';
import { inScope, queueCompare } from './domain';
import { useLive } from './hooks';
import type { Booking, Level, QueueScope, Status, Subject } from './types';

const message = (e: unknown) => e instanceof Error ? e.message : 'เกิดข้อผิดพลาด กรุณาลองอีกครั้ง';
const trackingUrl = (token: string) => `${location.href.split('#')[0]}#/track/${encodeURIComponent(token)}`;
function Badge({ status }: { status: Status }) { return <UIBadge color={status === 'Completed' ? 'success' : status === 'Calling' ? 'danger' : status === 'Serving' ? 'info' : status === 'Waiting' ? 'warning' : 'secondary'} className={`badge status-${status.toLowerCase()}`}>{['Calling', 'Serving'].includes(status) && <span className="dot" />}{STATUS_LABELS[status]}</UIBadge>; }
function Notice({ children, error = false }: { children: ReactNode; error?: boolean }) { return <div role={error ? 'alert' : 'status'} className={`notice ${error ? 'error' : ''}`}><CircleHelp size={17} /><div>{children}</div></div>; }
function SectionTitle({ number, title, note }: { number: string; title: string; note?: string }) { return <div className="section-title"><span>{number}</span><div><h2>{title}</h2>{note && <p>{note}</p>}</div></div>; }
function Header({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children?: ReactNode }) { return <header className="page-heading"><div><div className="eyebrow">{eyebrow}</div><h1>{title}</h1><p>{description}</p></div>{children}</header>; }
function Modal({ title, children, close }: { title: string; children: ReactNode; close: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => { const previous = document.activeElement as HTMLElement | null; ref.current?.querySelector<HTMLElement>('button')?.focus(); return () => previous?.focus(); }, []);
  function keyboard(e: React.KeyboardEvent) { if (e.key === 'Escape') close(); if (e.key === 'Tab') { const items = Array.from(ref.current?.querySelectorAll<HTMLElement>('button, input, textarea, select, a[href]') ?? []); const first = items[0], last = items.at(-1); if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus(); } else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus(); } } }
  return <div className="modal-overlay" onMouseDown={e => { if (e.target === e.currentTarget) close(); }}><div ref={ref} className="modal" role="dialog" aria-modal="true" aria-label={title} onKeyDown={keyboard}><div className="modal-header"><h2>{title}</h2><button className="icon-btn" onClick={close} aria-label="ปิด"><X size={20} /></button></div>{children}</div></div>;
}
export function App() {
  const [hash, setHash] = useState(location.hash || '#/book');
  const [ready,setReady] = useState(false); const [configError,setConfigError] = useState('');
  useEffect(()=>{let active=true;api.event().then(value=>{if(active){configureEvent(value);setReady(true);}}).catch(e=>{if(active)setConfigError(message(e));});return()=>{active=false;};},[]);
  useEffect(() => { const listener = () => { setHash(location.hash || '#/book'); window.scrollTo(0, 0); }; window.addEventListener('hashchange', listener); return () => window.removeEventListener('hashchange', listener); }, []);
  const page = hash.split('/')[1] || 'book'; const token = (() => { try { return decodeURIComponent(hash.split('/').slice(2).join('/')); } catch { return ''; } })();
  if (!ready) return <div className="empty"><h1>Warwick Private Consult</h1>{configError ? <><Notice error>เชื่อมต่อ Backend ไม่สำเร็จ กรุณาเปิด Go Server แล้วลองใหม่</Notice><button className="btn secondary" onClick={()=>location.reload()}>ลองใหม่</button></> : <p>กำลังโหลดตารางกิจกรรม…</p>}</div>;
  const nav = [{ id: 'book', icon: CalendarDays, name: 'จอง Private Consult', sub: 'BOOK A CONSULTATION' }, { id: 'track', icon: Ticket, name: 'ติดตามคิวของคุณ', sub: 'YOUR QUEUE' }, { id: 'staff', icon: LayoutDashboard, name: 'Staff Dashboard', sub: 'QUEUE MANAGEMENT' }, { id: 'display', icon: Monitor, name: 'จอแสดงคิว', sub: 'LIVE QUEUE DISPLAY' }];
  return <div className="app"><aside className="sidebar"><a href="#/book" className="brand"><div className="brand-mark">W<span /></div><div>WARWICK<span>INSTITUTE</span></div></a><div className="workspace-label">CONSULTATION PORTAL</div><nav aria-label="เมนูหลัก">{nav.map(n => <a key={n.id} href={`#/${n.id}`} className={`nav-item ${page === n.id ? 'selected' : ''}`} aria-current={page === n.id ? 'page' : undefined}><n.icon size={19} /><span>{n.name}<small>{n.sub}</small></span>{page === n.id && <ChevronRight size={15} />}</a>)}</nav><div className="sidebar-card"><Sparkles size={22} /><p>Every next step,<br /><strong>starts with a conversation.</strong></p><span>พร้อมที่สุด ให้สูงที่สุด</span></div><div className="sidebar-footer"><ShieldCheck size={16} /><span>Warwick Private Consult</span></div></aside><div className="app-main"><div className="topbar"><div className="breadcrumb">Warwick Institute <ChevronRight size={13} /> <strong>Private Consult</strong></div><div className="prototype"><span className="dot" /> CONNECTED PORTAL</div></div><div className="sample-banner"><span>{EVENT.sample ? EVENT.id.startsWith('local-test-') ? 'โหมดทดลองในเครื่อง · ข้อมูลทดสอบ' : 'ตารางกิจกรรมตัวอย่าง · สำหรับทดลองระบบ' : EVENT.title}</span><span>เชื่อมต่อระบบจองและคิวของ Warwick</span></div><main>{page === 'book' ? <BookPage /> : page === 'track' ? <TrackPage key={token} token={token} /> : page === 'staff' ? <StaffGate /> : page === 'display' ? <DisplayPage /> : <div className="empty"><h1>ไม่พบหน้านี้</h1><a href="#/book">กลับหน้าจอง</a></div>}</main><footer className="page-footer"><span>© Warwick Institute</span><span>Private Consult · Student Performance Report</span></footer></div></div>;
}

function BookPage() {
  const [level, setLevel] = useState<Level>('Beginner'); const [subject, setSubject] = useState<Subject>('Math');
  const [date, setDate] = useState(allowedDates('Beginner','Math')[0]); const [periodId, setPeriod] = useState('');
  const [busy, setBusy] = useState(false); const [error, setError] = useState(''); const [success, setSuccess] = useState<Booking | null>(null);
  const [copied, setCopied] = useState(false); const requestId = useRef(crypto.randomUUID());
  const fetcher = useCallback(() => api.periods({ level, subject, date }), [level, subject, date]);
  const { data: periods, error: periodError, refresh } = useLive(fetcher);
  useEffect(() => { if (periodId && periods?.some(p => p.id === periodId && !p.available)) setPeriod(''); }, [periods, periodId]);
  function chooseLevel(next: Level) { setLevel(next); setDate(allowedDates(next, subject)[0]); setPeriod(''); setError(''); }
  function chooseSubject(next: Subject) { setSubject(next); if (!allowedDates(level, next).includes(date)) setDate(allowedDates(level, next)[0]); setPeriod(''); setError(''); }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); if (busy) return; setError('');
    const data = new FormData(e.currentTarget);
    if (!periodId) { setError('กรุณาเลือกช่วงเวลาที่ต้องการ Consult'); return; }
    setBusy(true);
    try { const result = await api.book({ requestId: requestId.current, level, subject, date, periodId, email: String(data.get('email') ?? ''), wcode: String(data.get('wcode') ?? ''), nickname: String(data.get('nickname') ?? ''), school: String(data.get('school') ?? ''), questions: [1, 2, 3].map(n => String(data.get(`question${n}`) ?? '')), seminarConfirmed: data.get('seminar') === 'on' }); setSuccess(result); }
    catch (e) { setError(message(e)); await refresh(); } finally { setBusy(false); }
  }
  if (success) return <div className="success-page"><div className="success-icon"><CheckCheck size={32} /></div><div className="eyebrow">BOOKING CONFIRMED</div><h1>จองรอบ Consult เรียบร้อย</h1><p>ทีม Warwick จะจัดลำดับคิวให้ภายในรอบที่เลือก</p><div className="card success-details"><div><span>นักเรียน</span><strong>{success.nickname}</strong></div><div><span>วิชา</span><strong>{subjectLabel(success.subject)}</strong></div><div><span>วันที่ / รอบ</span><strong>{DATE_LABELS[success.date]} · {periodLabel(success.periodId, success.level, success.date)}</strong></div><div><span>รูปแบบ</span><strong>{success.mode}</strong></div></div><Notice>{success.emailStatus === 'Sent' ? 'ส่งอีเมลยืนยันแล้ว กรุณาเก็บลิงก์ติดตามคิวไว้' : success.emailStatus === 'Pending' ? 'บันทึกการจองแล้ว ระบบกำลังดำเนินการส่งอีเมล กรุณาเก็บลิงก์ติดตามคิวไว้ด้วย' : success.emailStatus === 'Failed' ? 'การจองยังมีผล แต่อีเมลยังส่งไม่สำเร็จ กรุณาเก็บลิงก์ติดตามคิวไว้' : 'การจองเรียบร้อยแล้ว ยังไม่ได้เปิดบริการอีเมล กรุณาเก็บลิงก์ติดตามคิวไว้'}</Notice><div className="success-actions"><a className="btn primary" href={`#/track/${success.token!}`}><Ticket size={18} /> ติดตามคิวของคุณ <ArrowRight size={17} /></a><button className="btn secondary" onClick={async () => { try { await navigator.clipboard.writeText(trackingUrl(success.token!)); setCopied(true); } catch { setError('คัดลอกไม่สำเร็จ กรุณาคัดลอกลิงก์จากช่องด้านล่าง'); } }}><Copy size={16} />{copied ? 'คัดลอกแล้ว' : 'คัดลอกลิงก์'}</button></div><input aria-label="ลิงก์ติดตามคิว" className="link-input" readOnly value={trackingUrl(success.token!)} onFocus={e => e.currentTarget.select()} />{error && <Notice error>{error}</Notice>}<button className="text-btn" onClick={() => { requestId.current = crypto.randomUUID(); setSuccess(null); setPeriod(''); setCopied(false); }}><Plus size={16} /> จองให้นักเรียนอีกคน</button></div>;
  return <><Header eyebrow="LET’S PLAN YOUR NEXT STEP" title="จอง Private Consult" description="พูดคุยกับอาจารย์ วางแผนการเรียนให้ตรงกับเป้าหมายของน้อง" /><div className="booking-hero"><div className="hero-copy"><span className="hero-tag"><BookOpen size={14} /> WARWICK INSTITUTE</span><h2 className="report-heading"><span>Student Performance Report</span><span>ประชุมรับผลการเรียนประจำ Cycle August - November 2026</span></h2><p>เลือกวิชาและช่วงเวลาที่สะดวก<br />แล้วให้ Warwick ช่วยวางแผนก้าวต่อไป</p><div className="hero-meta"><span><MapPin size={15} /> Warwick · 12A</span><span><Users size={15} /> จำนวน slots ตามรอบที่เลือก</span></div></div><div className="hero-art" aria-hidden="true"><div className="orbit orbit-one" /><div className="orbit orbit-two" /><div className="hero-ticket"><div className="ticket-top"><span>PRIVATE CONSULT</span><GraduationCap size={21} /></div><span className="ticket-line" /><div className="ticket-title">Your next<br /><em>chapter.</em></div><div className="ticket-bottom"><span>WARWICK INSTITUTE</span><ArrowUp size={28} /></div></div><div className="floating-seal"><Check size={19} /> Let's talk</div></div></div><div className="book-layout"><form onSubmit={submit} className="card booking-form"><SectionTitle number="01" title="ข้อมูลนักเรียนและผู้ปกครอง" note="กรุณากรอกข้อมูลให้ครบเพื่อเตรียมการปรึกษา" /><div className="field-grid"><label htmlFor="field-email">Email ผู้ปกครอง <b>*</b><UIInput id="field-email" type="email" name="email" size="xl" placeholder="parent@email.com" required maxLength={254} autoComplete="email" /></label><label htmlFor="field-wcode">Wcode <small>(ถ้ามี)</small><UIInput id="field-wcode" name="wcode" size="xl" placeholder="เช่น W1234" maxLength={40} /></label><label htmlFor="field-nickname">Nickname <b>*</b><UIInput id="field-nickname" name="nickname" size="xl" placeholder="ชื่อเล่นของน้อง" required maxLength={80} /></label><label htmlFor="field-school">School Name <b>*</b><UIInput id="field-school" name="school" size="xl" placeholder="ชื่อโรงเรียน" required maxLength={160} /></label></div><div className="form-divider" /><SectionTitle number="02" title="เลือกวิชาและช่วงเวลา Consult" note="เลือกช่วงเวลาที่สะดวก ทีมงานจะจัดลำดับคิวภายในช่วงเวลาที่เลือก" /><label className="field-label">ระดับการเรียน <b>*</b></label><div className="segmented level-picker">{(['Beginner', 'High Level'] as Level[]).map(l => <button type="button" aria-pressed={level === l} key={l} className={level === l ? 'active' : ''} onClick={() => chooseLevel(l)}><GraduationCap size={16} />{l}<span className="level-selected" aria-hidden="true">{level === l && <Check size={16} />}</span></button>)}</div><label className="field-label">วิชาที่ต้องการปรึกษา <b>*</b></label><div className="subject-grid">{SUBJECTS.map((s, i) => <button type="button" key={s.id} onClick={() => chooseSubject(s.id)} aria-pressed={subject === s.id} className={`subject-card ${subject === s.id ? 'chosen' : ''}`}><span className="subject-icon">{i === 0 ? '∑' : i === 1 ? 'Aa' : <BookOpen size={19} />}</span><strong>{s.label}</strong><span className="radio-mark">{subject === s.id && <span />}</span></button>)}</div><label className="field-label" htmlFor="consult-date">วันที่ Consult <b>*</b></label><div className="date-mode"><select id="consult-date" value={date} onChange={e => { setDate(e.target.value); setPeriod(''); }}>{allowedDates(level, subject).map(d => <option value={d} key={d}>{DATE_LABELS[d]}</option>)}</select><span>{modeFor(level, date) === 'On-site' ? <MapPin size={15} /> : <Video size={15} />}{modeFor(level, date)}</span></div><div className="period-heading"><label className="field-label">เลือกช่วงเวลา Private Consult <b>*</b></label><button type="button" className="text-btn" onClick={() => void refresh()} aria-label="รีเฟรชรอบว่าง"><RefreshCw size={13} />อัปเดตรอบว่าง</button></div><p id="consult-duration-note" className="consult-duration-note"><strong>1 ครอบครัวจะมีเวลา Consult บ้านละ 10 นาที</strong> เมื่อใกล้ถึงเวลาเจ้าหน้าที่จะติดต่อกลับไป</p>{periodError && <Notice error>{periodError}</Notice>}<div className="period-grid" role="group" aria-label="เลือกช่วงเวลา Private Consult" aria-describedby="consult-duration-note">{periods?.map(p => { const full = !p.available; return <button type="button" key={p.id} disabled={full} aria-pressed={periodId === p.id} className={`period-card ${full ? 'full' : ''} ${periodId === p.id ? 'chosen' : ''}`} onClick={() => setPeriod(p.id)}><Clock3 size={17} /><strong>{p.start}–{p.end}</strong><small className="period-capacity">{capacityFor(level, subject, date, p.id)} slots / รอบ</small><span>{full ? 'เต็มแล้ว' : periodId === p.id ? <><Check size={12} /> เลือกรอบนี้</> : 'ว่าง · เลือกได้'}</span></button>; }) ?? <p>กำลังตรวจสอบรอบว่าง…</p>}</div><p className="helper">รอบที่เลือกเป็นช่วงสำหรับเข้าคิว ลำดับคิวจะจัดสรรโดย Warwick Institute</p>{level === 'Beginner' && <div className="seminar-box" role="group" aria-labelledby="beginner-seminar-heading"><BookOpen size={24} aria-hidden="true" /><div><h3 id="beginner-seminar-heading">สำหรับนักเรียน Beginner</h3><p className="seminar-intro">เข้าร่วมสัมมนารวมก่อน Private Consult</p><div className="seminar-schedule">{seminarsFor(subject).map(s => <div className="seminar-time" key={s.subject}><span className="seminar-subject">SAT {s.subject}</span><strong className="seminar-date">{DATE_LABELS[s.date] ?? s.date}</strong><strong className="seminar-hour">{s.start}–{s.end} น.</strong></div>)}</div><label className="checkbox-label"><input type="checkbox" name="seminar" required /><span className="checkbox-mark" aria-hidden="true"><Check size={16} /></span><span>ยืนยันเข้าร่วมสัมมนารวมตามวิชาที่เลือก <b>*</b><small className="checkbox-state"><span className="unchecked-label">ยังไม่ได้ยืนยัน</span><span className="checked-label">ยืนยันแล้ว</span></small></span></label></div></div>}<div className="form-divider" /><SectionTitle number="03" title="คำถามถึงอาจารย์" note="อย่างน้อย 2 คำถาม เพื่อให้อาจารย์เตรียมคำแนะนำตรงประเด็น" /><div className="question-fields">{[1, 2, 3].map(n => <label key={n}>คำถามที่ {n} {n < 3 ? <b>*</b> : <small>(ไม่บังคับ)</small>}<textarea name={`question${n}`} required={n < 3} maxLength={1000} rows={2} placeholder={n === 1 ? 'เช่น น้องควรพัฒนาจุดไหนเพื่อไปถึงคะแนนเป้าหมาย?' : n === 2 ? 'เช่น ควรวางแผนเรียนและฝึกทำข้อสอบอย่างไรต่อไป?' : 'คำถามอื่น ๆ ที่อยากปรึกษาอาจารย์'} /></label>)}</div>{error && <Notice error>{error}</Notice>}<UIAction color="primary" pill={false} size="xl" type="submit" className="btn primary submit-booking" disabled={busy || !periodId}>{busy ? 'กำลังบันทึก…' : 'ยืนยันการจอง Private Consult'}<ArrowRight size={18} /></UIAction><p className="form-note"><ShieldCheck size={13} />ข้อมูลใช้สำหรับจัดกิจกรรมและเตรียมการปรึกษา</p></form><aside className="booking-aside"><div className="card summary-card"><div className="summary-heading"><Ticket size={19} /><h3>รอบที่คุณเลือก</h3></div><div className="summary-period">{periodId ? periodLabel(periodId, level, date) : 'เลือกช่วงเวลาที่สะดวก'}<span>{DATE_LABELS[date]}</span></div><dl><div><dt>ระดับ</dt><dd>{level}</dd></div><div><dt>วิชา</dt><dd>{subjectLabel(subject)}</dd></div><div><dt>รูปแบบ</dt><dd>{modeFor(level, date)}</dd></div>{periodId && <div><dt>จำนวน slots / รอบ</dt><dd>{capacityFor(level,subject,date,periodId)} slots</dd></div>}</dl><div className="summary-bottom"><ListOrdered size={18} /><p>หลังจองสำเร็จ ทีมงานจะจัดคิวให้<br /><strong>ติดตามคิวได้ผ่านลิงก์ของคุณ</strong></p></div></div><div className="how-it-works"><h3>เพียง 3 ขั้นตอน</h3>{[{ icon: CalendarDays, title: 'เลือกช่วงเวลาที่สะดวก', text: 'ดูจำนวน slots ที่ระบุในแต่ละรอบ' }, { icon: Mail, title: 'เก็บลิงก์ติดตามคิว', text: 'ใช้ลิงก์ส่วนตัวเพื่อดูคิวของคุณ' }, { icon: Users, title: 'พบอาจารย์เมื่อเรียกคิว', text: 'ทีมงานจัดลำดับภายในรอบที่เลือก' }].map((s, i) => <div className="how-step" key={s.title}><span><s.icon size={16} /></span><div><small>0{i + 1}</small><strong>{s.title}</strong><p>{s.text}</p></div></div>)}</div><div className="need-help"><CircleHelp size={19} /><div><strong>สอบถามเพิ่มเติม</strong><p>โทร. <a href="tel:026584880">02-658-4880</a></p></div></div></aside></div></>;
}

function TrackPage({ token }: { token: string }) {
  const fetcher = useCallback(() => token ? api.track(token) : Promise.resolve(null), [token]);
  const { data, error, loading, refresh } = useLive(fetcher, { enabled: !!token }); const [link, setLink] = useState(''); const [inputError, setInputError] = useState('');
  if (!token) return <><Header eyebrow="YOUR PRIVATE QUEUE" title="ติดตามคิวของคุณ" description="เปิดลิงก์ติดตามคิวที่ได้รับหลังจองสำเร็จ" /><div className="card tracking-entry"><div className="round-icon"><Ticket size={28} /></div><h2>คิวของคุณ อยู่ที่นี่</h2><p>ใช้ลิงก์ส่วนตัวเพื่อดูรหัสคิวของคุณ<br />และคิวที่กำลังเรียกในวิชาที่ปรึกษา</p><form onSubmit={e => { e.preventDefault(); const match = link.match(/#\/track\/(.+)$/); if (!match) { setInputError('กรุณาวางลิงก์ติดตามคิวให้ครบ'); return; } location.hash = `#/track/${match[1]}`; }}><label htmlFor="tracking-link">ลิงก์ติดตามคิว</label><input id="tracking-link" type="text" placeholder="วางลิงก์ที่ได้รับหลังการจอง" value={link} onChange={e => setLink(e.target.value)} required /><button className="btn primary">เปิดคิวของคุณ<ArrowRight size={17} /></button></form>{inputError && <Notice error>{inputError}</Notice>}</div></>;
  return <><Header eyebrow="YOUR PRIVATE QUEUE" title="ติดตามคิวของคุณ" description="กรุณารอติดตามรหัสคิว เมื่อถึงคิวทีมงานจะเรียกเข้าพบอาจารย์"><button className="btn secondary" onClick={() => void refresh()}><RefreshCw size={16} />รีเฟรช</button></Header>{error ? <div className="card empty"><CircleHelp size={34} /><h2>โหลดสถานะคิวไม่สำเร็จ</h2><p>{error}</p><button className="btn secondary" onClick={() => void refresh()}>ลองอีกครั้ง</button><a className="btn secondary" href="#/track">กลับหน้าติดตามคิว</a></div> : loading || !data ? <div className="card empty">กำลังโหลดสถานะคิว…</div> : <div className="tracking-layout"><div className="parent-ticket"><div className="parent-ticket-header"><span><Ticket size={18} /> YOUR QUEUE</span><span>SAT {data.currentSubject}</span></div><p>คิวของคุณ</p><div className={`parent-queue ${!data.queueNo ? 'preparing' : ''}`}>{data.queueNo || 'กำลังจัดคิว'}</div><div className="parent-state"><Badge status={data.status} /></div><div className="ticket-perforation" /><div className="parent-bottom">{data.status === 'Calling' ? <><Radio size={23} /><h2>ถึงคิวของคุณแล้ว</h2><p>กรุณาไปที่ {data.counter || `${data.currentSubject} Room`}</p></> : data.status === 'Serving' ? <><Users size={23} /><h2>กำลังปรึกษาอาจารย์</h2><p>ทีม Warwick พร้อมช่วยวางแผนก้าวต่อไป</p></> : data.status === 'Completed' ? <><CheckCheck size={23} /><h2>เสร็จสิ้นการปรึกษา</h2><p>ขอบคุณที่มาพูดคุยกับ Warwick</p></> : data.status === 'Cancelled' ? <><X size={23} /><h2>การจองถูกยกเลิก</h2><p>ติดต่อทีม Warwick หากต้องการสอบถาม</p></> : data.status === 'No-show' ? <><CircleHelp size={23} /><h2>กรุณาติดต่อทีมงาน</h2><p>ทีมงานจะช่วยตรวจสอบสถานะคิว</p></> : <><Clock3 size={23} /><h2>{data.queueNo ? 'กรุณารอเรียกคิว' : 'ยืนยันการจองแล้ว'}</h2><p>{data.queueNo ? 'ติดตามรหัสคิวปัจจุบันได้จากหน้านี้' : 'ทีมงานกำลังเตรียมรหัสคิวของคุณ'}</p></>}</div></div><div className="card current-queue-card"><div className="live-label"><span className="dot" /> LIVE QUEUE</div><h2>คิวปัจจุบัน</h2><p>SAT {data.currentSubject}</p>{data.currentQueues.length ? data.currentQueues.map(q => <div className="current-number" key={q.queueNo}><strong>{q.queueNo}</strong><span>{q.state === 'Serving' ? 'กำลังปรึกษา' : 'กำลังเรียก'}</span></div>) : <div className="current-number"><strong className="no-queue">—</strong><span>ยังไม่มีคิวที่กำลังเรียก</span></div>}{data.queueCountState !== 'inactive' && <div className="queue-remaining" aria-live="polite"><h3>อีกกี่คิวถึงคุณ · SAT {data.currentSubject}</h3>{data.queuesUntilTurn !== null && data.queuesUntilTurn !== undefined ? data.queuesUntilTurn === 0 ? <p className="remaining-current">ถึงคิวของคุณแล้ว</p> : <><div className="remaining-value"><span>เหลืออีก</span><strong>{data.queuesUntilTurn}</strong><span>คิวถึงคุณ</span></div><p>{data.queuesUntilTurn === 1 ? 'คุณเป็นคิวถัดไป · เตรียมพร้อมรอประกาศ' : 'นับตามลำดับคิวปัจจุบันของห้องนี้'}</p></> : <p className="remaining-unavailable">{data.queueCountState === 'awaiting-check-in' ? 'เช็กอินกับ Staff ก่อน ระบบจึงจะคำนวณคิวให้' : data.queueCountState === 'staff-review' ? 'กรุณาติดต่อ Staff เพื่อตรวจลำดับคิวของคุณ' : 'กำลังจัดลำดับคิวสำหรับวิชานี้'}</p>}<small>จำนวนปรับตามการเรียก ข้าม ยกเลิก และจัดลำดับใหม่ · กรุณารอประกาศเรียกคิว</small></div>}<div className="live-footer"><RefreshCw size={13} />อัปเดตอัตโนมัติทุก 5 วินาทีเมื่อเปิดหน้านี้</div></div></div>}</>;
}

function StaffGate() {
  const [user,setUser]=useState<StaffUser | null>(null); const [checking,setChecking]=useState(true); const [error,setError]=useState(''); const [busy,setBusy]=useState(false);
  useEffect(()=>{let active=true;const expired=()=>setUser(null);window.addEventListener('staff-session-expired',expired);api.me().then(u=>{if(active)setUser(u);}).catch(()=>{}).finally(()=>{if(active)setChecking(false);});return()=>{active=false;window.removeEventListener('staff-session-expired',expired);};},[]);
  async function login(e:FormEvent<HTMLFormElement>){e.preventDefault();const form=new FormData(e.currentTarget);setBusy(true);setError('');try{setUser(await api.login(String(form.get('username')),String(form.get('password'))));}catch(e){setError(message(e));}finally{setBusy(false);}}
  if(checking)return <div className="empty">กำลังตรวจสอบสิทธิ์ Staff…</div>;
  if(!user)return <><Header eyebrow="STAFF ACCESS" title="เข้าสู่ระบบ Staff" description="สำหรับทีม Warwick เพื่อจัดลำดับและควบคุมคิว"/><div className="card tracking-entry"><div className="round-icon"><ShieldCheck size={28}/></div><h2>Staff Login</h2><form onSubmit={login}><label htmlFor="staff-username">ชื่อผู้ใช้<UIInput id="staff-username" size="xl" name="username" required autoComplete="username" maxLength={100}/></label><label className="login-password" htmlFor="staff-password">รหัสผ่าน<UIInput id="staff-password" size="xl" type="password" name="password" required autoComplete="current-password" maxLength={72}/></label>{error&&<Notice error>{error}</Notice>}<UIAction color="primary" pill={false} size="xl" type="submit" className="btn primary" disabled={busy}>{busy?'กำลังเข้าสู่ระบบ…':'เข้าสู่ระบบ'}</UIAction></form></div></>;
  return <><div className="staff-login-bar"><span><ShieldCheck size={15}/> {user.username}</span><button className="text-btn" onClick={async()=>{try{await api.logout();setUser(null);}catch(e){setError(message(e));}}}>ออกจากระบบ</button></div>{error&&<Notice error>{error}</Notice>}<StaffPage user={user}/></>;
}

function StaffTrackingLink({ bookingId }: { bookingId: string }) {
  const [url,setUrl] = useState(''); const [error,setError] = useState('');
  const [loading,setLoading] = useState(true); const [attempt,setAttempt] = useState(0);
  const [copied,setCopied] = useState(false); const [copyError,setCopyError] = useState('');
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    let active=true; setUrl('');setError('');setCopyError('');setCopied(false);setLoading(true);
    api.staffTrackingLink(bookingId).then(result => {if(active)setUrl(result.trackingUrl);})
      .catch(e => {if(active)setError(message(e));}).finally(() => {if(active)setLoading(false);});
    return () => {active=false;};
  },[bookingId,attempt]);
  async function copy() {
    setCopied(false);setCopyError('');
    try {await navigator.clipboard.writeText(url);setCopied(true);}
    catch {input.current?.focus();input.current?.select();setCopyError('คัดลอกอัตโนมัติไม่ได้ กรุณาคัดลอกลิงก์จากช่องด้านบน');}
  }
  return <section className="staff-tracking-link" aria-label="ลิงก์ติดตามคิว">
    <h3>ลิงก์ติดตามคิวของผู้ปกครอง</h3>
    {loading ? <p role="status">กำลังโหลดลิงก์…</p> : error ? <><Notice error>{error}</Notice><button className="btn secondary" onClick={() => setAttempt(n => n+1)}>ลองโหลดลิงก์ใหม่</button></> : <>
      <label htmlFor="staff-tracking-url">ลิงก์สำหรับรายการนี้<input ref={input} id="staff-tracking-url" readOnly value={url} onFocus={e => e.currentTarget.select()} /></label>
      <button className="btn secondary" disabled={!url} onClick={() => void copy()}><Copy size={16}/>{copied ? 'คัดลอกลิงก์แล้ว' : 'คัดลอกลิงก์ติดตามคิว'}</button>
      <p role="status">{copyError || (copied ? 'นำลิงก์ไปส่งให้ผู้ปกครองของรายการนี้ได้เลย' : 'ใช้ลิงก์เดิมของการจอง คัดลอกส่งให้ผู้ปกครองได้')}</p>
    </>}
  </section>;
}

type SubjectFilter = Subject | 'All';
const otherSubject = (stream: 'Math' | 'Verbal') => stream === 'Math' ? 'Verbal' : 'Math';
function canChooseStart(b: Booking) {
  return b.subject === 'Combined' && ['Booked', 'Waiting', 'No-show'].includes(b.status)
    && !Object.values(b.stages).some(s => s?.calledAt || s?.startedAt || s?.endedAt);
}
function StageProgress({ booking: b }: { booking: Booking }) {
  if (b.subject !== 'Combined') return null;
  return <div className="stage-progress">{(['Math', 'Verbal'] as const).map(s =>
    <span key={s} className={b.stages[s]?.endedAt ? 'done' : b.currentSubject === s ? 'current' : ''}>
      {b.stages[s]?.endedAt && <Check size={12} aria-hidden="true" />}{s}: {b.stages[s]?.endedAt ? 'เสร็จแล้ว' : b.currentSubject === s ? 'วิชาปัจจุบัน' : 'รอต่อ'}
    </span>)}
  </div>;
}
function canEditBooking(b: Booking) {
  return ['Booked', 'Waiting', 'No-show', 'Cancelled'].includes(b.status)
    && !Object.values(b.stages).some(s => s?.startedAt || s?.endedAt);
}
type EditValues = Parameters<typeof api.editBooking>[1];
function BookingEditModal({ booking: b, busy, save, close }: { booking: Booking; busy: boolean; save: (values: EditValues) => Promise<void>; close: () => void }) {
  const [subject, setSubject] = useState(b.subject);
  const [date, setDate] = useState(allowedDates(b.level, b.subject).includes(b.date) ? b.date : allowedDates(b.level, b.subject)[0] ?? '');
  const [periodId, setPeriodId] = useState(capacityFor(b.level, b.subject, b.date, b.periodId) ? b.periodId : '');
  const [startSubject, setStartSubject] = useState<'Math' | 'Verbal'>(b.subject === 'Combined' ? b.currentSubject : 'Math');
  const [seminarConfirmed, setSeminarConfirmed] = useState(b.seminarConfirmed);
  const [error, setError] = useState('');
  const fetcher = useCallback(() => api.periods({ level: b.level, subject, date }), [b.level, subject, date]);
  const { data: periods, error: periodError, loading } = useLive(fetcher);
  const available = (p: {id:string;available:boolean}) => p.available || b.status === 'Cancelled' || (subject === b.subject && date === b.date && p.id === b.periodId);
  async function submit(e: FormEvent) {
    e.preventDefault(); setError('');
    try { await save({ subject, date, periodId, seminarConfirmed, expectedRevision: b.revision ?? -1, ...(subject === 'Combined' ? { startSubject } : {}) }); }
    catch (e) { setError(message(e)); }
  }
  return <Modal title="แก้วัน / เวลา / วิชา" close={close}>
    <p><strong>{b.nickname}</strong> · {b.level}<br />รายการเดิม: {subjectLabel(b.subject)} · {DATE_LABELS[b.date] ?? b.date} · {periodLabel(b.periodId, b.level, b.date) === '—' ? b.periodId : periodLabel(b.periodId, b.level, b.date)}</p>
    <form className="booking-edit-form" onSubmit={submit}>
      <label>วิชาที่จอง<select aria-label="วิชาใหม่" value={subject} onChange={e => { const next = e.target.value as Subject; setSubject(next); const dates = allowedDates(b.level, next); if (!dates.includes(date)) setDate(dates[0] ?? ''); setPeriodId(''); setStartSubject('Math'); if (b.level === 'Beginner') setSeminarConfirmed(false); }}>{SUBJECTS.map(s => <option key={s.id} value={s.id}>{s.label}</option>)}</select></label>
      <label>วันที่<select aria-label="วันที่ใหม่" value={date} onChange={e => { setDate(e.target.value); setPeriodId(''); }}>{allowedDates(b.level, subject).map(d => <option key={d} value={d}>{DATE_LABELS[d] ?? d}</option>)}</select></label>
      <label>ช่วงเวลา<select aria-label="ช่วงเวลาใหม่" required value={periodId} disabled={loading || !!periodError} onChange={e => setPeriodId(e.target.value)}><option value="">เลือกช่วงเวลา</option>{(periods ?? []).map(p => <option key={p.id} value={p.id} disabled={!available(p)}>{p.start}–{p.end}{!p.available && !(subject === b.subject && date === b.date && p.id === b.periodId) ? ' · เต็ม' : ''}</option>)}</select></label>
      {subject === 'Combined' && <label>วิชาเริ่มต้น<select aria-label="วิชาเริ่มต้นใหม่" value={startSubject} onChange={e => setStartSubject(e.target.value as 'Math' | 'Verbal')}><option value="Math">Math ก่อน → Verbal</option><option value="Verbal">Verbal ก่อน → Math</option></select></label>}
      {b.level === 'Beginner' && <label className="checkbox-label"><input type="checkbox" checked={seminarConfirmed} required onChange={e => setSeminarConfirmed(e.target.checked)} /><span className="checkbox-mark" aria-hidden="true"><Check size={16} /></span><span>ผู้ปกครองยืนยันสัมมนาตามวิชาใหม่แล้ว</span></label>}
      <Notice>ใช้รายการและลิงก์ติดตามเดิม ต้องจัดลำดับคิวใหม่หลังแก้{subject !== b.subject ? ' · เปลี่ยนวิชาแล้วจะออกรหัสคิวใหม่ตอนจัดคิว' : ''}{date !== b.date ? ' · เปลี่ยนวันแล้วต้องเช็กอินใหม่' : ''}</Notice>
      {b.status === 'Cancelled' && <Notice>รายการยังอยู่ในแท็บยกเลิก การแก้รอบยังไม่จองที่ ต้องกดคืนการจองอีกครั้ง และรอบนั้นต้องมีที่ว่าง</Notice>}
      {(periodError || error) && <Notice error>{periodError || error}</Notice>}
      <button type="submit" className="btn primary wide" disabled={busy || loading || !!periodError || !periodId || !periods?.some(p => p.id === periodId && available(p)) || (b.level === 'Beginner' && !seminarConfirmed)}>บันทึกการแก้ไข</button>
    </form>
  </Modal>;
}

function StaffPage({ user }: { user: StaffUser }) {
  const fetcher = useCallback(() => api.staffList(), []);
  const { data: packet, error: loadError, loading, updated, refresh } = useLive(fetcher);
  const bookings = packet?.bookings ?? [];
  const [view, setView] = useState<'checkin' | 'queue' | 'cancelled' | 'completed'>('checkin');
  const [subjectFilter, setSubjectFilter] = useState<SubjectFilter>('All');
  const [checkLevel, setCheckLevel] = useState<Level | 'All'>('All');
  const [checkDate, setCheckDate] = useState('All');
  const [scope, setScope] = useState<QueueScope>(DEFAULT_SCOPE);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('All');
  const [draft, setDraft] = useState<string[] | null>(null);
  const draftRevision = useRef<number | null>(null);
  const [notice, setNotice] = useState(''); const [error, setError] = useState('');
  const [busy, setBusy] = useState(false); const busyRef = useRef(false);
  const [detailId, setDetailId] = useState<string | null>(null);
  const detail = bookings.find(b => b.id === detailId);
  const [cancel, setCancel] = useState<Booking | null>(null); const [reason, setReason] = useState('');
  const [startChoice, setStartChoice] = useState<Booking | null>(null);
  const [edit, setEdit] = useState<Booking | null>(null);
  const [restore, setRestore] = useState<Booking | null>(null);
  const [restoreError, setRestoreError] = useState('');
  const [archivedOnly,setArchivedOnly] = useState(false);
  const [deleteTest,setDeleteTest] = useState<Booking|null>(null);
  const [deleteConfirmed,setDeleteConfirmed] = useState(false);
  const [clearTest,setClearTest] = useState<{revision:number;count:number}|null>(null);
  const [clearText,setClearText] = useState('');
  const canManageTestData=packet?.canManageTestData === true;
  const hasDraft = !!draft;
  const scopeBookings = useMemo(() => bookings.filter(b => b.status !== 'Cancelled' && !b.archivedAt && inScope(b, scope)).sort(queueCompare), [packet, scope]);
  const eligible = scopeBookings.filter(b => ['Booked', 'Waiting'].includes(b.status));
  const signature = eligible.map(b => b.id).sort().join('|');
  useEffect(() => { setDraft(null); draftRevision.current = null; }, [signature]);
  function clearDraft() { setDraft(null); draftRevision.current = null; }
  function matches(b: Booking) {
    const term = search.trim().toLowerCase();
    return (subjectFilter === 'All' || b.subject === subjectFilter)
      && (statusFilter === 'All' || b.status === statusFilter)
      && (!term || `${b.nickname} ${b.queueNo ?? ''} ${b.wcode} ${b.school} ${b.email}`.toLowerCase().includes(term));
  }
  const checkRows = bookings.filter(b => (view === 'cancelled' ? b.status === 'Cancelled' : view === 'completed' ? b.status === 'Completed' && (!archivedOnly || !!b.archivedAt) : b.status !== 'Cancelled' && !b.archivedAt) && (checkLevel === 'All' || b.level === checkLevel) && (checkDate === 'All' || b.date === checkDate) && matches(b));
  const sections = new Map<string, Booking[]>();
  for (const b of [...checkRows].sort((a, b) => a.date.localeCompare(b.date) || a.level.localeCompare(b.level) || a.periodId.localeCompare(b.periodId) || a.nickname.localeCompare(b.nickname))) {
    const key = `${b.date}|${b.level}|${b.periodId}`;
    sections.set(key, [...(sections.get(key) ?? []), b]);
  }
  const ordered = draft ? [...draft.map(id => scopeBookings.find(b => b.id === id)!).filter(Boolean), ...scopeBookings.filter(b => !draft.includes(b.id))] : scopeBookings;
  const list = ordered.filter(matches);
  const visibleEligible = eligible.filter(matches);
  // A room queue must be saved in full; a view filter must never silently reorder hidden rows.
  const canOrder = !search.trim() && statusFilter === 'All' && visibleEligible.length === eligible.length;
  const physicalLive = bookings.filter(b => b.level === scope.level && b.date === scope.date && ['Calling', 'Serving'].includes(b.status));
  const live = physicalLive.filter(b => b.currentSubject === scope.stream);
  const inactive = busy || !!loadError || loading;
  const countRows = view !== 'queue' ? checkRows : list;
  const counts = [countRows.length, countRows.filter(b => b.checkedInAt && !['Cancelled', 'No-show'].includes(b.status)).length, countRows.filter(b => b.status === 'Waiting').length, countRows.filter(b => b.status === 'Completed').length];
  const dates = [...new Set([...allDates('Beginner'), ...allDates('High Level'), ...bookings.map(b => b.date)])].sort();
  function queuePeriods(level = scope.level, date = scope.date, stream = scope.stream, filter = subjectFilter) {
    const entries = periodsFor(level, date, stream).filter(p => filter === 'All' || capacityFor(level, filter, date, p.id) > 0);
    for (const b of bookings.filter(b => b.level === level && b.date === date && b.currentSubject === stream && (filter === 'All' || b.subject === filter))) {
      if (!entries.some(p => p.id === b.periodId)) entries.push({ id: b.periodId, start: b.periodId, end: '', capacity: 0 });
    }
    return entries.sort((a, b) => a.start.localeCompare(b.start));
  }
  function updateScope(next: Partial<QueueScope>, filter = subjectFilter) {
    setScope(old => { const merged = { ...old, ...next }; const periods = queuePeriods(merged.level, merged.date, merged.stream, filter);
      return { ...merged, periodId: periods.some(p => p.id === merged.periodId) ? merged.periodId : periods[0]?.id ?? '' }; });
    clearDraft(); setNotice(''); setError('');
  }
  function chooseFilter(filter: SubjectFilter) {
    setSubjectFilter(filter); clearDraft();
    if (view === 'queue') {
      const stream = filter === 'Math' || filter === 'Verbal' ? filter : scope.stream;
      const dates = allDates(scope.level, stream);
      const date = dates.includes(scope.date) ? scope.date : dates[0] ?? scope.date;
      updateScope({ stream, date }, filter);
    }
  }
  async function run(work: () => Promise<unknown>, success: string) {
    if (busyRef.current) return;
    busyRef.current = true; setBusy(true); setError(''); setNotice('');
    try { await work(); await refresh(); setNotice(success); }
    catch (e) { setError(message(e)); clearDraft(); await refresh(); }
    finally { busyRef.current = false; setBusy(false); }
  }
  function action(b: Booking, status: Status) {
    if (status === 'Cancelled') { setDetailId(null); setCancel(b); setReason(b.status === 'No-show' ? 'ไม่มา / ไม่แจ้งยกเลิก' : 'ผู้ปกครองแจ้งยกเลิก'); setError(''); return; }
    const handoff = status === 'Completed' && b.subject === 'Combined' && !b.stages[otherSubject(b.currentSubject)]?.endedAt;
    void run(() => api.status(b.id, status, b.revision ?? -1), handoff ? `จบ ${b.currentSubject} แล้ว · รอจัดลำดับ ${otherSubject(b.currentSubject)}` : `อัปเดต ${b.queueNo || b.nickname} เรียบร้อย`);
  }
  function openQueue(b: Booking) {
    clearDraft(); setSearch(''); setStatusFilter('All'); setSubjectFilter(b.subject); setView('queue');
    setScope({ level: b.level, date: b.date, periodId: b.periodId, stream: b.currentSubject }); setNotice(''); setError('');
  }
  function move(id: string, direction: number) {
    if (!draft) draftRevision.current = packet?.revision ?? -1;
    const ids = draft ?? eligible.map(b => b.id); const index = ids.indexOf(id);
    if (index < 0 || index + direction < 0 || index + direction >= ids.length) return;
    const copy = [...ids]; [copy[index], copy[index + direction]] = [copy[index + direction], copy[index]]; setDraft(copy);
  }
  function startButton(b: Booking) {
    return canChooseStart(b) ? <button className="mini-btn" disabled={inactive || hasDraft} onClick={() => setStartChoice(b)}>เริ่ม {otherSubject(b.currentSubject)} ก่อน</button> : null;
  }
  function managementButtons(b: Booking) {
    return <>
      {canEditBooking(b) && <button className="mini-btn" disabled={inactive || hasDraft} onClick={() => { setError(''); setDetailId(null); setEdit(b); }}>แก้วัน / เวลา / วิชา</button>}
      {['Booked', 'Waiting', 'Calling', 'No-show'].includes(b.status) && <button className="mini-btn" disabled={inactive || hasDraft} onClick={() => action(b, 'Cancelled')}>เอาออก / ยกเลิก</button>}
      {b.status === 'Completed' && <button className="mini-btn" disabled={inactive || hasDraft} onClick={() => void run(() => api.archiveCompleted(b.id,!b.archivedAt,b.revision ?? -1),b.archivedAt ? 'นำรายการกลับมาแสดงแล้ว · สถานะยังเป็นเสร็จสิ้น' : 'เก็บรายการแล้ว · เปิดดูได้ในแท็บรายการจบแล้ว')}>{b.archivedAt ? 'นำกลับมาแสดง' : 'เก็บรายการจบแล้ว'}</button>}
      {canManageTestData && <button className="mini-btn danger" disabled={inactive || hasDraft} onClick={() => {setError('');setDetailId(null);setDeleteTest(b);setDeleteConfirmed(false);}}>ลบข้อมูลทดสอบ</button>}
    </>;
  }
  async function saveEdit(values: EditValues) {
    if (!edit || busyRef.current) return;
    busyRef.current = true; setBusy(true); setError(''); setNotice('');
    try {
      const changed = await api.editBooking(edit.id, values);
      await refresh(); setEdit(null); clearDraft(); setView(changed.status === 'Cancelled' ? 'cancelled' : 'checkin');
      setSubjectFilter(changed.subject); setCheckLevel(changed.level); setCheckDate(changed.date); setStatusFilter('All'); setSearch('');
      setNotice('แก้รายการเดิมแล้ว · ต้องบันทึก / จัดคิวใหม่ก่อนเรียก');
    } catch (e) { await refresh(); throw e; }
    finally { busyRef.current = false; setBusy(false); }
  }
  async function restoreBooking() {
    if (!restore || busyRef.current) return;
    busyRef.current = true; setBusy(true); setRestoreError(''); setNotice('');
    try {
      const changed = await api.restoreBooking(restore.id, restore.revision ?? -1);
      await refresh(); setRestore(null); clearDraft(); setView('checkin'); setSubjectFilter(changed.subject);
      setCheckLevel(changed.level); setCheckDate(changed.date); setStatusFilter('All'); setSearch('');
      setNotice('คืนการจองแล้ว · ต้องบันทึก / จัดคิวใหม่ก่อนเรียก');
    } catch (e) { setRestoreError(message(e)); await refresh(); }
    finally { busyRef.current = false; setBusy(false); }
  }
  function rowActions(b: Booking, checkin = false) {
    if(b.status === 'Completed') return <div className="row-actions">{b.archivedAt && <span className="helper">เก็บประวัติแล้ว</span>}{managementButtons(b)}</div>;
    if (b.status === 'Cancelled') return <div className="row-actions">
      <button className="mini-btn accent" disabled={inactive} onClick={() => { setRestore(b); setRestoreError(''); }}>คืนการจอง</button>
      {managementButtons(b)}
    </div>;
    if (checkin) return <div className="row-actions">
      {b.status === 'Booked' && <button className="mini-btn accent" disabled={inactive} onClick={() => action(b, 'Waiting')}><Check size={14} />เช็กอิน</button>}
      {b.status === 'No-show' && <button className="mini-btn" disabled={inactive} onClick={() => action(b, 'Waiting')}>คืนคิวเข้ารอ</button>}
      {b.checkedInAt && !['No-show', 'Cancelled'].includes(b.status) && <span className="checkin-done"><CheckCheck size={14} />เช็กอินแล้ว</span>}
      {startButton(b)}{managementButtons(b)}<button className="mini-btn" disabled={inactive} onClick={() => openQueue(b)}>เปิดคิวรอบนี้</button>
    </div>;
    const roomBusy = physicalLive.some(l => l.currentSubject === b.currentSubject);
    return <div className="row-actions">
      {b.status === 'Booked' && <button disabled={inactive || hasDraft} className="mini-btn" onClick={() => action(b, 'Waiting')}><Check size={13} />เช็กอิน</button>}
      {b.status === 'Waiting' && <><button disabled={inactive || hasDraft || !b.queueNo || !b.orders[b.currentSubject] || roomBusy} className="mini-btn accent" onClick={() => action(b, 'Calling')}>เรียกคิว</button><button disabled={inactive || hasDraft} className="mini-btn" onClick={() => action(b, 'No-show')}>ไม่มา</button></>}
      {b.status === 'Calling' && <><button disabled={inactive || hasDraft} className="mini-btn accent" onClick={() => action(b, 'Serving')}>เริ่มปรึกษา</button><button disabled={inactive || hasDraft} className="mini-btn" onClick={() => action(b, 'Waiting')}>พักคิว</button><button disabled={inactive || hasDraft} className="mini-btn" onClick={() => action(b, 'No-show')}>ไม่มา</button></>}
      {b.status === 'Serving' && <button disabled={inactive || hasDraft} className="mini-btn accent" onClick={() => action(b, 'Completed')}>{b.subject === 'Combined' && !b.stages[otherSubject(b.currentSubject)]?.endedAt ? `จบ ${b.currentSubject} → ${otherSubject(b.currentSubject)}` : 'จบการปรึกษา'}</button>}
      {b.status === 'No-show' && <button disabled={inactive || hasDraft} className="mini-btn" onClick={() => action(b, 'Waiting')}>คืนคิวเข้ารอ</button>}
      {startButton(b)}
      {managementButtons(b)}
      {['Completed', 'Cancelled'].includes(b.status) && <span className="helper">ปิดรายการแล้ว</span>}
    </div>;
  }
  const periodText = (b: Booking) => { const text = periodLabel(b.periodId, b.level, b.date); return text === '—' ? `รอบเดิม ${b.periodId}` : text; };
  const nameCell = (b: Booking) => <button className="student-btn" onClick={() => setDetailId(b.id)}><strong>{b.queueNo || 'ยังไม่จัดคิว'}</strong><span>{b.nickname} <small>· {b.wcode || 'ไม่มี Wcode'}</small></span></button>;
  return <>
    <Header eyebrow="WARWICK OPERATIONS" title="Staff Dashboard" description="เช็กอินจากทุกช่วงเวลา แล้วเปิดรอบเพื่อจัดคิวและเรียกเข้าห้อง"><button className="btn secondary" onClick={() => void refresh()}><RefreshCw size={16} />รีเฟรช</button>{canManageTestData && <button className="btn secondary" disabled={inactive} onClick={() => {setError('');setClearText('');setClearTest({revision:packet?.revision ?? -1,count:bookings.length});}}>ล้างงานทดสอบนี้</button>}</Header>
    <div className="staff-view-tabs" role="group" aria-label="หน้าทำงาน Staff">
      <button aria-pressed={view === 'checkin'} className={view === 'checkin' ? 'active' : ''} onClick={() => { setView('checkin'); setStatusFilter('All'); clearDraft(); }}><CheckCheck size={18} />เช็กอิน · รวมทุกช่วงเวลา</button>
      <button aria-pressed={view === 'queue'} className={view === 'queue' ? 'active' : ''} onClick={() => { setView('queue'); setStatusFilter('All'); updateScope({}); }}><ListOrdered size={18} />จัดคิว / เรียกคิว</button>
      <button aria-pressed={view === 'completed'} className={view === 'completed' ? 'active' : ''} onClick={() => {setView('completed');setArchivedOnly(false);setSubjectFilter('All');setCheckLevel('All');setCheckDate('All');setSearch('');setStatusFilter('All');clearDraft();}}><CheckCheck size={18} />รายการจบแล้ว ({bookings.filter(b => b.status === 'Completed').length})</button>
      <button aria-pressed={view === 'cancelled'} className={view === 'cancelled' ? 'active' : ''} onClick={() => { setView('cancelled'); setSubjectFilter('All'); setCheckLevel('All'); setCheckDate('All'); setSearch(''); setStatusFilter('All'); clearDraft(); }}><X size={18} />รายการยกเลิก ({bookings.filter(b => b.status === 'Cancelled').length})</button>
    </div>
    <div className="stats-grid">{['การจองตามตัวกรอง', 'เช็กอินแล้ว', 'รอเรียกคิว', 'เสร็จสิ้น'].map((label, i) => <div className="card stat" key={label}><span>{label}</span><strong>{counts[i]}<small>รายการ</small></strong><span className={`stat-line stat-line-${i}`} /></div>)}</div>
    <div className="card staff-controls">
      <div className="subject-filter" role="group" aria-label="กรองวิชาที่จอง">{(['All', 'Math', 'Verbal', 'Combined'] as const).map(s => <button key={s} aria-pressed={subjectFilter === s} className={subjectFilter === s ? 'active' : ''} onClick={() => chooseFilter(s)}>{s === 'All' ? 'All · ทุกวิชา' : subjectLabel(s)}</button>)}</div>
      <p className="helper">ตัวกรองใช้วิชาที่จอง · Math + Verbal แสดงเป็นกลุ่มของตัวเอง</p>
      {view !== 'queue' ? <div className="control-grid">
        <label>ระดับ<select aria-label="ระดับสำหรับเช็กอิน" value={checkLevel} onChange={e => setCheckLevel(e.target.value as Level | 'All')}><option value="All">ทุกระดับ</option><option>Beginner</option><option>High Level</option></select></label>
        <label>วันที่<select aria-label="วันที่สำหรับเช็กอิน" value={checkDate} onChange={e => setCheckDate(e.target.value)}><option value="All">ทุกวัน</option>{dates.map(d => <option key={d} value={d}>{DATE_LABELS[d] ?? d}</option>)}</select></label>
        <div className="checkin-summary"><Clock3 size={17} /><span>{view === 'completed' ? 'ประวัติการปรึกษา' : view === 'cancelled' ? 'รายการที่เก็บไว้' : 'รวมทุกช่วงเวลา'}<br /><strong>{view === 'completed' ? 'เก็บรายการ / เปิดดูย้อนหลังได้' : view === 'cancelled' ? 'ค้นหาแล้วคืนการจองได้' : 'ค้นหาแล้วเช็กอินได้เลย'}</strong></span></div>
        <label>ผู้ดูแลคิว<input value={user.username} readOnly /></label>
      </div> : <>
        <div className="control-grid">
          <label>ระดับ<select aria-label="ระดับสำหรับจัดคิว" value={scope.level} onChange={e => { const level = e.target.value as Level; const date = allDates(level, scope.stream)[0] ?? allDates(level)[0]; updateScope({ level, date, periodId: '' }); }}><option>Beginner</option><option>High Level</option></select></label>
          <label>วันที่<select aria-label="วันที่สำหรับจัดคิว" value={scope.date} onChange={e => updateScope({ date: e.target.value, periodId: '' })}>{[...new Set([...allDates(scope.level, scope.stream), ...bookings.filter(b => b.level === scope.level && b.currentSubject === scope.stream).map(b => b.date)])].sort().map(d => <option key={d} value={d}>{DATE_LABELS[d] ?? d}</option>)}</select></label>
          <label>ห้องที่ให้บริการ<select aria-label="ห้องที่ให้บริการ" value={scope.stream} onChange={e => { const stream = e.target.value as 'Math' | 'Verbal'; const date = allDates(scope.level, stream).includes(scope.date) ? scope.date : allDates(scope.level, stream)[0]; updateScope({ stream, date }); }}><option value="Math">ห้อง Math</option><option value="Verbal">ห้อง Verbal</option></select></label>
          <label>ผู้ดูแลคิว<input value={user.username} readOnly /></label>
        </div>
        <div className="staff-periods">{queuePeriods().map(p => {
          const groups = bookings.filter(b => b.level === scope.level && b.date === scope.date && b.periodId === p.id && b.status !== 'Cancelled');
          return <button key={p.id} aria-pressed={scope.periodId === p.id} className={scope.periodId === p.id ? 'selected' : ''} onClick={() => updateScope({ periodId: p.id })}><Clock3 size={15} /><strong>{p.end ? `${p.start}–${p.end}` : `รอบเดิม ${p.id}`}</strong><span>{(['Math', 'Verbal', 'Combined'] as const).filter(s => subjectFilter === 'All' || subjectFilter === s).map(s => {
            const cap = capacityFor(scope.level, s, scope.date, p.id); return cap ? <span className="capacity-item" key={s}>{s === 'Combined' ? 'MV' : s[0]} {groups.filter(b => b.subject === s).length}/{cap}</span> : null;
          })}</span></button>;
        })}</div>
      </>}
    </div>
    <div className="table-filters staff-search"><div className="search-field"><Search size={16} /><input aria-label="ค้นหานักเรียนหรือคิว" placeholder="ชื่อ / Wcode / รหัสคิว / Email" value={search} onChange={e => { setSearch(e.target.value); clearDraft(); }} /></div><select aria-label="กรองสถานะ" value={statusFilter} onChange={e => { setStatusFilter(e.target.value); clearDraft(); }}><option value="All">ทุกสถานะ</option>{Object.entries(STATUS_LABELS).filter(([key]) => view === 'completed' ? key === 'Completed' : view === 'cancelled' ? key === 'Cancelled' : key !== 'Cancelled').map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select><span>{countRows.length} รายการ</span></div>
    {(error || loadError) && <Notice error>{error || loadError}</Notice>}{notice && <Notice>{notice}</Notice>}
    <p className="staff-updated">{loading ? 'กำลังโหลดรายการ…' : updated ? `อัปเดตล่าสุด ${updated.toLocaleTimeString('th-TH')} · อัปเดตอัตโนมัติทุก 5 วินาที` : ''}</p>
    {view === 'completed' && <><Notice>รายการจบแล้วเก็บสถานะ เวลา และรหัสคิวเดิม การเก็บประวัติไม่คืน slot และไม่เปิดเคสใหม่</Notice><label className="completed-history-filter">ดูรายการจบแล้ว<select aria-label="กรองรายการจบแล้ว" value={archivedOnly ? 'archived' : 'all'} onChange={e => setArchivedOnly(e.target.value === 'archived')}><option value="all">ทั้งหมด · รวมที่เก็บไว้</option><option value="archived">เก็บประวัติแล้ว</option></select></label></>}
    {view === 'cancelled' && <Notice>รายการที่เอาออกเก็บไว้ที่นี่และคืนที่ว่างแล้ว · เปลี่ยนใจกลับมาได้ด้วยปุ่มคืนการจอง โดยต้องมีที่ว่างอีกครั้ง</Notice>}
    {view !== 'queue' ? <div className="checkin-sections">{[...sections.entries()].map(([key, rows]) => {
      const first = rows[0]; const checked = rows.filter(b => b.checkedInAt && !['Cancelled', 'No-show'].includes(b.status)).length;
      return <section className="card checkin-section" key={key}>
        <div className="checkin-section-heading"><div><h2><Clock3 size={18} />{periodText(first)}</h2><p>{DATE_LABELS[first.date] ?? first.date} · {first.level} · {first.mode}</p></div><span>{rows.length} รายการ · เช็กอินแล้ว {checked}</span></div>
        <div className="table-wrap"><table><thead><tr><th>รหัสคิว / นักเรียน</th><th>วิชาที่จอง / ความคืบหน้า</th><th>สถานะ</th><th>{view === 'cancelled' ? 'คืนการจอง / แก้รายการ' : 'เช็กอิน / จัดการรายการ'}</th></tr></thead><tbody>{rows.map(b => <tr key={b.id}><td>{nameCell(b)}</td><td><span className="subject-pill">{subjectLabel(b.subject)}</span><StageProgress booking={b} /></td><td><Badge status={b.status} />{b.status === 'Cancelled' && <span className="cancel-reason">{b.cancellationReason}</span>}</td><td>{rowActions(b, true)}</td></tr>)}</tbody></table></div>
      </section>;
    })}{!loading && !checkRows.length && <div className="card empty"><Users size={28} /><h3>ไม่พบรายการตามตัวกรอง</h3><p>ลองเลือก All หรือค้นหาด้วย Email / Wcode</p></div>}</div> : <>
      <div className="staff-live"><div><div className="live-label"><span className="dot" />กำลังให้บริการ · ห้อง {scope.stream} · รวมทุกวิชาที่จอง / ทุกรอบของระดับและวันนี้</div>{live.length ? live.map(b => <div key={b.id} className="staff-live-info"><strong>{b.queueNo}</strong><span>{b.nickname} · {subjectLabel(b.subject)}</span><Badge status={b.status} /></div>) : <div className="staff-live-info"><strong>—</strong><span>ยังไม่มีคิวที่กำลังเรียกหรือปรึกษา</span></div>}</div><button className="btn primary" disabled={inactive || hasDraft || live.length > 0 || !list.some(b => b.status === 'Waiting' && b.queueNo && b.orders[b.currentSubject])} onClick={() => { const next = list.find(b => b.status === 'Waiting' && b.queueNo && b.orders[b.currentSubject]); if (next) action(next, 'Calling'); }}><Radio size={17} />เรียกคิวถัดไปตามตัวกรอง</button></div>
      <div className="card queue-card"><div className="queue-toolbar"><div><h2><ListOrdered size={19} />จัดลำดับและควบคุมคิว · ห้อง {scope.stream}</h2><p>ลูกศรจัดลำดับภายในรอบ · รหัสคิวคงเดิมเมื่อส่งต่อวิชา</p></div><button className="btn primary" disabled={inactive || !eligible.length || !canOrder} onClick={() => void run(async () => { await api.assign(scope, draft ?? eligible.map(b => b.id), draftRevision.current ?? packet?.revision ?? -1); clearDraft(); }, 'บันทึกลำดับและจัดสรรรหัสคิวเรียบร้อย')}><Check size={16} />{draft ? 'บันทึกลำดับใหม่' : 'บันทึก / จัดคิว'}</button></div>
        {!canOrder && eligible.length > 0 && <div className="draft-bar">จัดลำดับต้องเห็นคิวที่รอในห้อง / รอบนี้ครบ <button onClick={() => { setSubjectFilter('All'); setSearch(''); setStatusFilter('All'); clearDraft(); }}>ดูทั้งหมดเพื่อจัดลำดับ</button></div>}
        {draft && <div className="draft-bar">มีการเปลี่ยนลำดับที่ยังไม่ได้บันทึก <button onClick={clearDraft}>ยกเลิกการเปลี่ยน</button></div>}
        <div className="table-wrap"><table><thead><tr><th>ลำดับรอ</th><th>รหัสคิว / นักเรียน</th><th>วิชาที่จอง / ความคืบหน้า</th><th>สถานะ</th><th>จัดคิว</th><th>การดำเนินการ</th></tr></thead><tbody>{list.map(b => {
          const ids = draft ?? eligible.map(e => e.id); const index = ids.indexOf(b.id);
          return <tr key={b.id} className={['Calling', 'Serving'].includes(b.status) ? 'live-row' : ''}><td><span className="row-order">{['Booked', 'Waiting'].includes(b.status) ? draft?.includes(b.id) ? draft.indexOf(b.id) + 1 : b.orders[b.currentSubject] ?? '—' : '—'}</span></td><td>{nameCell(b)}</td><td><span className="subject-pill">{subjectLabel(b.subject)}</span><StageProgress booking={b} /></td><td><Badge status={b.status} /></td><td><div className="move-buttons"><button className="icon-btn" aria-label={`เลื่อน ${b.nickname} ขึ้น`} disabled={inactive || !canOrder || index <= 0} onClick={() => move(b.id, -1)}><ArrowUp size={15} /></button><button className="icon-btn" aria-label={`เลื่อน ${b.nickname} ลง`} disabled={inactive || !canOrder || index < 0 || index >= ids.length - 1} onClick={() => move(b.id, 1)}><ArrowDown size={15} /></button></div></td><td>{rowActions(b)}</td></tr>;
        })}</tbody></table>{!loading && !list.length && <div className="empty"><Users size={28} /><h3>ยังไม่มีรายการในเงื่อนไขนี้</h3><p>ลองเปลี่ยนรอบ วิชาที่จอง ห้อง หรือคำค้นหา</p></div>}</div>
      </div>
    </>}
    <p className="staff-footnote">โควตานับตามวิชาที่จอง · Math + Verbal มีโควตาแยกและเลือกวิชาเริ่มต้นได้ก่อนเรียกครั้งแรก · ห้อง Math/Verbal ใช้ร่วมกับผู้จองวิชาเดียว</p>
    {detail && <Modal title="รายละเอียดการจอง" close={() => setDetailId(null)}><div className="detail-heading"><strong>{detail.nickname}</strong><Badge status={detail.status} /></div><StageProgress booking={detail} /><dl className="detail-list"><div><dt>รหัสคิว</dt><dd>{detail.queueNo || 'ยังไม่จัดคิว'}</dd></div><div><dt>ระดับ / วิชา</dt><dd>{detail.level} · {subjectLabel(detail.subject)}</dd></div><div><dt>รอบที่จอง</dt><dd>{DATE_LABELS[detail.date] ?? detail.date} · {periodText(detail)}</dd></div><div><dt>โรงเรียน</dt><dd>{detail.school}</dd></div><div><dt>Email</dt><dd>{detail.email}</dd></div><div><dt>สถานะอีเมล</dt><dd>{detail.emailStatus === 'Sent' ? 'Sent · ส่งแล้ว' : detail.emailStatus === 'Pending' ? 'Pending · รอส่ง' : detail.emailStatus === 'Failed' ? 'Failed · ส่งไม่สำเร็จ' : 'ยังไม่ได้ตั้งค่าการส่ง'}</dd></div><div><dt>Wcode</dt><dd>{detail.wcode || '—'}</dd></div><div><dt>รูปแบบ</dt><dd>{detail.mode}</dd></div></dl>{detail.status === 'Cancelled' && <Notice>เหตุผลยกเลิก: {detail.cancellationReason}</Notice>}<StaffTrackingLink key={detail.id} bookingId={detail.id}/><div className="row-actions">{managementButtons(detail)}{detail.status === 'Cancelled' && <button className="mini-btn accent" disabled={inactive} onClick={() => { setDetailId(null); setRestore(detail); setRestoreError(''); }}>คืนการจอง</button>}</div><h3>คำถามถึงอาจารย์</h3>{detail.questions.filter(Boolean).map((q, i) => <div className="question-display" key={i}><span>{i + 1}</span><p>{q}</p></div>)}</Modal>}
    {startChoice && <Modal title="เลือกวิชาเริ่มต้น Math + Verbal" close={() => setStartChoice(null)}><p><strong>{startChoice.nickname}</strong> จะเริ่ม <strong>{otherSubject(startChoice.currentSubject)}</strong> ก่อน แล้วส่งต่อ {startChoice.currentSubject} เมื่อวิชาแรกเสร็จ</p>{error && <Notice error>{error}</Notice>}<Notice>รหัสคิวและการเช็กอินคงเดิม ต้องบันทึก / จัดคิวในห้องใหม่อีกครั้งก่อนเรียก</Notice><button className="btn primary wide" disabled={inactive} onClick={() => void run(async () => { await api.startSubject(startChoice.id, otherSubject(startChoice.currentSubject), startChoice.revision ?? -1); setStartChoice(null); }, `เลือกเริ่ม ${otherSubject(startChoice.currentSubject)} แล้ว · เปิดห้องนี้เพื่อจัดคิว`)}>ยืนยันเริ่ม {otherSubject(startChoice.currentSubject)} ก่อน</button></Modal>}
    {edit && <BookingEditModal booking={edit} busy={inactive} save={saveEdit} close={() => setEdit(null)} />}
    {restore && <Modal title="คืนการจอง" close={() => setRestore(null)}><p>คืนรายการของ <strong>{restore.nickname}</strong><br />{subjectLabel(restore.subject)} · {DATE_LABELS[restore.date] ?? restore.date} · {periodText(restore)}</p><Notice>ตรวจที่ว่างใหม่ เก็บข้อมูลและลิงก์เดิม ถ้าเคยเช็กอินแล้วในวันเดิมจะกลับเข้ารอ และต้องจัดลำดับคิวอีกครั้ง</Notice>{restoreError && <Notice error>{restoreError}</Notice>}<button className="btn primary wide" disabled={inactive} onClick={() => void restoreBooking()}>ยืนยันคืนการจอง</button>{canEditBooking(restore) && <button className="btn secondary wide" disabled={inactive} onClick={() => { setRestore(null); setEdit(restore); }}>เลือกวัน / เวลา / วิชาอื่น</button>}</Modal>}
    {deleteTest && <Modal title="ลบข้อมูลทดสอบรายการนี้" close={() => setDeleteTest(null)}><p>ลบ <strong>{deleteTest.nickname}</strong> · {deleteTest.queueNo || 'ยังไม่จัดคิว'}<br />{subjectLabel(deleteTest.subject)} · {DATE_LABELS[deleteTest.date]} · {periodText(deleteTest)}</p><Notice>ใช้เฉพาะฐานข้อมูลเทส ลบการจองและประวัติรายการนี้ถาวร คืนที่ว่าง และลิงก์ติดตามเดิมใช้ไม่ได้ รวมถึงรายการที่ปรึกษาจบแล้ว ลำดับเลขคิวอื่นคงเดิม</Notice>{['Calling','Serving'].includes(deleteTest.status) && <Notice>คิวนี้กำลังเรียกหรือปรึกษาอยู่ การลบจะเอาคิวนี้ออกจากห้องทดสอบด้วย</Notice>}{error && <Notice error>{error}</Notice>}<label className="test-delete-confirm"><input type="checkbox" checked={deleteConfirmed} onChange={e => setDeleteConfirmed(e.target.checked)} />ยืนยันว่าต้องการลบรายการทดสอบนี้ถาวร</label><button className="btn destructive wide" disabled={inactive || !deleteConfirmed || !canManageTestData} onClick={() => void run(async () => {await api.deleteTestBooking(deleteTest.id,deleteTest.revision ?? -1);setDeleteTest(null);clearDraft();},'ลบรายการทดสอบแล้ว · ข้อมูลรายการอื่นคงเดิม')}>ยืนยันลบข้อมูลทดสอบ</button></Modal>}
    {clearTest && <Modal title="ล้างการจองของงานทดสอบนี้" close={() => setClearTest(null)}><p>ล้าง <strong>{clearTest.count} รายการ</strong> ทั้งงาน รวมรายการจบแล้ว เก็บประวัติ ยกเลิก และคิวที่กำลังทำงาน ขอบเขตคือ <strong>{EVENT.title}</strong></p><Notice>เป็นการลบถาวร ล้างการจอง คิว ประวัติและงานเมลของงานทดสอบนี้ เริ่มเลขคิวใหม่ ลิงก์เดิมใช้ไม่ได้ เก็บตารางกิจกรรม บัญชี Staff และการ Login ไว้ ไม่มีผลต่อรอบจริง</Notice>{error && <Notice error>{error}</Notice>}<label>พิมพ์ CLEAR TEST DATA เพื่อยืนยัน<input aria-label="ยืนยันล้างข้อมูลทดสอบ" value={clearText} onChange={e => setClearText(e.target.value)} autoComplete="off" /></label><button className="btn destructive wide" disabled={inactive || clearText !== 'CLEAR TEST DATA' || !canManageTestData} onClick={() => void run(async () => {const r=await api.clearTestData(clearTest.revision,clearText);setClearTest(null);setDeleteTest(null);setDetailId(null);setCancel(null);setRestore(null);setEdit(null);setStartChoice(null);clearDraft();setView('checkin');setSubjectFilter('All');setCheckLevel('All');setCheckDate('All');setStatusFilter('All');setSearch('');setNotice(`ล้างข้อมูลทดสอบ ${r.deletedCount} รายการแล้ว`);},'ล้างงานทดสอบแล้ว · จองรายการใหม่ได้ทันที')}>ยืนยันล้างงานทดสอบนี้</button></Modal>}
    {cancel && <Modal title="เอาออก / ยกเลิกการจอง" close={() => setCancel(null)}><p>เอา <strong>{cancel.nickname}</strong> ออกจากรายการใช้งานและคืนที่ว่าง เก็บข้อมูลไว้ในแท็บรายการยกเลิกเพื่อคืนการจองภายหลัง</p>{error && <Notice error>{error}</Notice>}<label>เหตุผล<select aria-label="เหตุผลยกเลิก" value={['ผู้ปกครองแจ้งยกเลิก', 'ไม่มา / ไม่แจ้งยกเลิก', 'ลงทะเบียนผิด'].includes(reason) ? reason : 'อื่น ๆ'} onChange={e => setReason(e.target.value === 'อื่น ๆ' ? '' : e.target.value)}><option>ผู้ปกครองแจ้งยกเลิก</option><option>ไม่มา / ไม่แจ้งยกเลิก</option><option>ลงทะเบียนผิด</option><option>อื่น ๆ</option></select></label><label>เหตุผลที่ยกเลิก <b>*</b><textarea value={reason} onChange={e => setReason(e.target.value)} rows={3} maxLength={500} /></label><button className="btn destructive wide" disabled={!reason.trim() || inactive} onClick={() => void run(async () => { await api.status(cancel.id, 'Cancelled', cancel.revision ?? -1, reason); setCancel(null); }, 'ยกเลิกแล้ว · คืนที่ว่างในรอบนี้เรียบร้อย')}>ยืนยันยกเลิกการจอง</button></Modal>}
  </>;
}

function DisplayPage() {
  const [level, setLevel] = useState<Level>('Beginner'); const [date, setDate] = useState(allDates('Beginner')[0]);
  const fetcher = useCallback(() => api.display(level,date), [level,date]); const { data, error, refresh } = useLive(fetcher);
  return <><Header eyebrow="LIVE AT WARWICK" title="จอแสดงคิว" description="ติดตามรหัสคิวที่กำลังเรียก แยก SAT Math และ SAT Verbal"><button className="btn secondary" onClick={() => void refresh()}><RefreshCw size={16} />รีเฟรช</button></Header><div className="display-controls"><label>ระดับ<select value={level} onChange={e => { const level = e.target.value as Level; setLevel(level); setDate(allDates(level)[0]); }}><option>Beginner</option><option>High Level</option></select></label><label>วันกิจกรรม<select value={date} onChange={e => setDate(e.target.value)}>{allDates(level).map(d => <option key={d} value={d}>{DATE_LABELS[d]}</option>)}</select></label><div className="live-label"><span className="dot" />อัปเดตอัตโนมัติ</div></div>{error && <Notice error>{error}</Notice>}<div className="display-grid">{(['Math', 'Verbal'] as const).map(stream => { const queues = data?.[stream] ?? []; const upcoming = data?.[`${stream}Next`] ?? []; return <div key={stream} className={`display-panel display-${stream.toLowerCase()}`}><div className="display-panel-header"><span>SAT {stream}</span><Radio size={24} /></div><div className="display-now">NOW CALLING / SERVING</div>{queues.length ? queues.map(b => <div className="display-number" key={b.queueNo}><strong>{b.queueNo}</strong><p>{b.state === 'Serving' ? 'กำลังปรึกษา' : 'กรุณาไปที่'} · {b.counter}</p></div>) : <div className="display-number"><strong>—</strong><p>กรุณารอประกาศเรียกคิว</p></div>}<div className="display-upcoming" aria-label={`คิวถัดไป SAT ${stream}`}><div className="display-upcoming-title"><span>คิวถัดไป · เตรียมพร้อม</span><small>ตามลำดับที่ Staff บันทึก</small></div>{upcoming.length ? <ol>{upcoming.map((b,i) => <li key={b.queueNo}><span className="upcoming-index">{i+1}</span><strong>{b.queueNo}</strong><span className="upcoming-period">{b.periodStart}–{b.periodEnd}</span></li>)}</ol> : <p className="upcoming-empty">ยังไม่มีคิวที่พร้อมเรียกถัดไป</p>}<p className="upcoming-note">ลำดับอาจเปลี่ยนได้ กรุณารอประกาศเรียกคิว</p></div><div className="display-panel-footer"><Ticket size={18} />เมื่อถึงคิว กรุณาติดต่อทีมงานหน้าห้อง</div></div>; })}</div></>;
}
