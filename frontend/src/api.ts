import type { Booking, BookingInput, ParentView, QueueScope, Status } from './types';
const BASE = import.meta.env.VITE_API_URL ?? '';
let csrfToken = '';
export type StaffUser = { username: string; role: string };
export type StaffPacket = { canManageTestData: boolean; bookings: Booking[]; revision: number };
export type LiveQueue = { queueNo: string; state: 'Calling' | 'Serving' | 'Waiting'; counter?: string; periodStart?: string; periodEnd?: string };
export type ServerEvent = { seminars?: {subject:'Math'|'Verbal';date:string;start:string;end:string}[]; event: { id: string; title: string; cycle: string; venue: string; sample: boolean; timezone: string }; rules: { level: 'Beginner' | 'High Level'; subject: 'Math' | 'Verbal' | 'Combined'; date: string; mode: string; periods: {id: string; start: string; end: string; capacity: number}[] }[] };
export class ApiError extends Error { constructor(public code: string, message: string, public status: number) { super(message); } }
async function request<T>(path: string, method = 'GET', body?: unknown, headers: Record<string,string> = {}): Promise<T> {
  const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), 15000);
  try {
    const response = await fetch(`${BASE}${path}`, { method, credentials: 'include', cache: 'no-store', signal: controller.signal, headers: { Accept: 'application/json', ...(body === undefined ? {} : {'Content-Type':'application/json'}), ...(csrfToken ? {'X-CSRF-Token':csrfToken} : {}), ...headers }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    const json = await response.json().catch(() => null);
    if (!response.ok) { if (response.status === 401 && path !== '/api/auth/login') window.dispatchEvent(new Event('staff-session-expired')); throw new ApiError(json?.error?.code ?? 'HTTP_ERROR', json?.error?.message ?? 'เชื่อมต่อระบบไม่สำเร็จ', response.status); }
    if (!json) throw new Error('Backend ไม่ได้ส่งข้อมูล JSON กรุณาตรวจสอบการเชื่อมต่อ API');
    return json as T;
  } catch (e) { if (e instanceof TypeError) throw new Error('เชื่อมต่อระบบไม่ได้ กรุณาลองใหม่'); if (e instanceof DOMException && e.name === 'AbortError') throw new Error('การเชื่อมต่อใช้เวลานาน กรุณาลองใหม่'); throw e; }
  finally { clearTimeout(timer); }
}
const updated = () => window.dispatchEvent(new Event('queue-updated'));
async function staffExport() {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(`${BASE}/api/staff/export.xlsx`, { credentials: 'include', signal: controller.signal, cache: 'no-store', headers: { Accept: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' } });
    if (!response.ok) {
      if (response.status === 401) window.dispatchEvent(new Event('staff-session-expired'));
      const json = await response.json().catch(() => null);
      throw new ApiError(json?.error?.code ?? 'EXPORT_FAILED', json?.error?.message ?? 'Export Excel ไม่สำเร็จ กรุณาลองใหม่', response.status);
    }
    if (response.headers.get('Content-Type') !== 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet') throw new Error('ระบบไม่ได้ส่งไฟล์ Excel กรุณาอัปเดตเว็บแล้วลองใหม่');
    const blob = await response.blob();
    const filename = response.headers.get('Content-Disposition')?.match(/filename="(SPR_Queue_Registrations_[A-Za-z0-9_-]+\.xlsx)"/)?.[1] ?? 'SPR_Queue_Registrations.xlsx';
    return { blob, filename };
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw new Error('Export ใช้เวลานาน กรุณาลองใหม่');
    if (error instanceof TypeError) throw new Error('เชื่อมต่อระบบไม่ได้ กรุณาลองใหม่');
    throw error;
  } finally { clearTimeout(timer); }
}
export const api = {
  event: () => request<ServerEvent>('/api/event'),
  periods: (scope: Pick<BookingInput,'level'|'subject'|'date'>) => request<{id:string;start:string;end:string;available:boolean}[]>(`/api/periods?${new URLSearchParams(scope)}`),
  async book(input: BookingInput) { const {requestId,...payload} = input; const result = await request<{booking:Booking;token:string;trackingUrl:string}>('/api/bookings','POST',payload,{'Idempotency-Key':requestId}); updated(); return {...result.booking, token:result.token}; },
  track: (token:string) => request<ParentView>(`/api/track/${encodeURIComponent(token)}`),
  staffList: () => request<StaffPacket>('/api/staff/bookings'),
  staffExport,
  async swapBookings(id:string,input:{otherBookingId:string;expectedRevision:number;otherExpectedRevision:number;reason:string;confirmation:string}) {const r=await request<{bookings:Booking[]}>(`/api/staff/bookings/${encodeURIComponent(id)}/swap`,'POST',input);updated();return r.bookings;},
  async recoverBooking(id:string,input:{action:'cancel'|'reset-checkin';reason:string;startSubject?:'Math'|'Verbal';releaseQueueCode?:boolean;expectedRevision:number;confirmation:string}) {const b=await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/recovery`,'POST',input);updated();return b;},
  async deleteCancelledBooking(id:string,expectedRevision:number) {await request(`/api/staff/bookings/${encodeURIComponent(id)}/cancelled`,'DELETE',{expectedRevision,confirmation:`DELETE CANCELLED BOOKING ${id}`});updated();},
  async editStudentDetails(id:string,input:{wcode:string;nickname:string;school:string;email:string;expectedRevision:number}) { const b=await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/student-details`,'PATCH',input);updated();return b; },
  staffTrackingLink: (id:string) => request<{trackingUrl:string}>(`/api/staff/bookings/${encodeURIComponent(id)}/tracking-link`),
  display: (level:string,date:string) => request<Record<'Math'|'Verbal'|'MathNext'|'VerbalNext',LiveQueue[]>>(`/api/display?${new URLSearchParams({level,date})}`),
  async assign(scope:QueueScope,bookingIds:string[],expectedRevision:number) { await request('/api/staff/queue-order','PUT',{scope,bookingIds,expectedRevision}); updated(); },
  async status(id:string,status:Status,expectedRevision:number,reason='') { const b = await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/actions`,'POST',{status,expectedRevision,reason});updated();return b; },
  async startSubject(id:string,stream:'Math'|'Verbal',expectedRevision:number) { const b = await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/start-subject`,'POST',{stream,expectedRevision});updated();return b; },
  async editBooking(id:string,input:{subject:Booking['subject'];date:string;periodId:string;startSubject?:'Math'|'Verbal';seminarConfirmed:boolean;expectedRevision:number}) { const b=await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}`,'PATCH',input);updated();return b; },
  async restoreBooking(id:string,expectedRevision:number,target?:{date:string;periodId:string}) { const b=await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/restore`,'POST',{expectedRevision,...(target??{})});updated();return b; },
  async archiveCompleted(id:string,archived:boolean,expectedRevision:number) { const b=await request<Booking>(`/api/staff/bookings/${encodeURIComponent(id)}/archive`,'POST',{archived,expectedRevision});updated();return b; },
  async deleteTestBooking(id:string,expectedRevision:number) {await request(`/api/staff/bookings/${encodeURIComponent(id)}`,'DELETE',{expectedRevision,confirmation:'DELETE TEST BOOKING'});updated();},
  async clearTestData(expectedRevision:number,confirmation:string) {const r=await request<{deletedCount:number}>('/api/staff/test-data/clear','POST',{expectedRevision,confirmation});updated();return r;},
  async me() { const result=await request<{user:StaffUser;csrfToken:string}>('/api/auth/me');csrfToken=result.csrfToken;return result.user; },
  async login(username:string,password:string) { const result=await request<{user:StaffUser;csrfToken:string}>('/api/auth/login','POST',{username,password});csrfToken=result.csrfToken;return result.user; },
  async logout() {await request('/api/auth/logout','POST',{});csrfToken='';window.dispatchEvent(new Event('staff-session-expired'));},
};
