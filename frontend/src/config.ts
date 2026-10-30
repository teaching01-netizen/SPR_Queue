import type { ServerEvent } from './api';
import type { Level, Period, Subject, QueueScope } from './types';
// Current event schedule is loaded from the Go API before rendering.
export const EVENT = { id: 'performance-nov-2026', title: 'Student Performance Report', cycle: 'November 2026', venue: 'Warwick Institute · ชั้น 12A', sample: true };
export const SUBJECTS: { id: Subject; label: string; description: string }[] = [
  { id: 'Math', label: 'SAT Math', description: 'วางแผนพัฒนาคะแนนคณิตศาสตร์' },
  { id: 'Verbal', label: 'SAT Verbal', description: 'วางแผนพัฒนาการอ่านและการเขียน' },
  { id: 'Combined', label: 'Math + Verbal', description: 'ปรึกษาทั้งสองวิชา · ทีมงานเลือกวิชาเริ่มต้น' },
];
export const DATE_LABELS: Record<string, string> = { '2026-11-06': '6 November 2026', '2026-11-07': '7 November 2026' };
let liveRules: ServerEvent['rules'] | null = null;
let liveSeminars: NonNullable<ServerEvent['seminars']> = [];
export function seminarsFor(subject:Subject) { return liveSeminars.filter(s=>subject === 'Combined' || s.subject === subject).sort((a,b)=>a.date.localeCompare(b.date)||a.start.localeCompare(b.start)); }
export function configureEvent(value: ServerEvent) {
  Object.assign(EVENT, value.event); liveRules = value.rules; liveSeminars = value.seminars ?? [];
  for (const r of value.rules) DATE_LABELS[r.date] = new Intl.DateTimeFormat('en-GB', {day:'numeric',month:'long',year:'numeric',timeZone:'UTC'}).format(new Date(r.date + 'T00:00:00Z'));
  const rule = value.rules[0]; Object.assign(DEFAULT_SCOPE, {level:rule.level,date:rule.date,periodId:rule.periods[0].id,stream:rule.subject === 'Verbal' ? 'Verbal' : 'Math'});
}
export function allDates(level:Level,stream?: 'Math' | 'Verbal') { return liveRules ? [...new Set(liveRules.filter(r=>r.level===level && (!stream || r.subject===stream || r.subject==='Combined')).map(r=>r.date))].sort() : []; }
export function allowedDates(level: Level, subject: Subject) {
  if (liveRules) return [...new Set(liveRules.filter(r=>r.level===level && r.subject===subject).map(r=>r.date))].sort();
  return [];
}
export function modeFor(level: Level, date: string) { if (liveRules) return liveRules.find(r=>r.level===level && r.date===date)?.mode ?? ''; return ''; }
export function periodsFor(level: Level, date: string, stream?: 'Math' | 'Verbal'): Period[] {
  if (liveRules) { const entries = liveRules.filter(r=>r.level===level && r.date===date && (!stream || r.subject===stream || r.subject==='Combined')).flatMap(r=>r.periods); return [...new Map(entries.map(p=>[p.id,p])).values()].sort((a,b)=>a.start.localeCompare(b.start)); }
  return []; // The UI waits for the server event configuration before rendering.
}
export const periodLabel = (id: string, level: Level, date: string) => { const p = periodsFor(level, date).find(p => p.id === id); return p ? `${p.start}–${p.end}` : '—'; };
export const subjectLabel = (id: Subject) => SUBJECTS.find(s => s.id === id)?.label ?? id;
export const STATUS_LABELS = { Booked: 'รอเช็กอิน', Waiting: 'รอเรียกคิว', Calling: 'กำลังเรียก', Serving: 'กำลังปรึกษา', Completed: 'เสร็จสิ้น', 'No-show': 'ไม่มาเข้าคิว', Cancelled: 'ยกเลิก' };
export const DEFAULT_SCOPE: QueueScope = { level: 'Beginner', date: '2026-11-07', periodId: 'P1300_1400', stream: 'Math' };

export function capacityFor(level:Level,subject:Subject,date:string,periodId:string) { if (!liveRules) return 4; return liveRules.find(r=>r.level===level && r.subject===subject && r.date===date)?.periods.find(p=>p.id===periodId)?.capacity ?? 0; }
