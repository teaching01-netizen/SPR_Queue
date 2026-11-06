export type Level = 'Beginner' | 'High Level';
export type Subject = 'Math' | 'Verbal' | 'Combined';
export type Stream = 'Math' | 'Verbal';
export type Status = 'Booked' | 'Waiting' | 'Calling' | 'Serving' | 'Completed' | 'No-show' | 'Cancelled';
export type Booking = {
  id: string; revision?: number; token?: string; requestId: string; eventId: string;
  email: string; wcode: string; nickname: string; school: string;
  level: Level; subject: Subject; date: string; periodId: string; mode: string;
  questions: string[]; seminarConfirmed: boolean;
  queueNo: string | null; orders: Partial<Record<Stream, number>>;
  currentSubject: Stream; status: Status; checkedInAt: string | null;
  stages: Partial<Record<Stream, { calledAt?: string; startedAt?: string; endedAt?: string }>>;
  archivedAt?: string; archivedBy?: string; completedAt?: string; cancelledAt?: string; cancellationReason?: string;
  counter: string; staff: string; createdAt: string; updatedAt: string;
  emailStatus: 'Not configured' | 'Pending' | 'Sent' | 'Failed' | 'Cancelled';
};
export type BookingInput = Pick<Booking, 'email' | 'wcode' | 'nickname' | 'school' | 'level' | 'subject' | 'date' | 'periodId' | 'questions' | 'seminarConfirmed' | 'requestId'>;
export type Period = { id: string; start: string; end: string; capacity: number };
export type Audit = { id: string; bookingId: string; action: string; actor: string; at: string; previous: string; next: string };
export type Store = { version: 1; bookings: Booking[]; counters: Record<string, number>; audit: Audit[] };
export type ParentView = { queuesUntilTurn: number | null; queueCountState: 'ready' | 'current' | 'inactive' | 'awaiting-check-in' | 'awaiting-order' | 'staff-review'; queueNo: string | null; currentQueues: { queueNo: string; state: 'Calling' | 'Serving' }[]; currentSubject: Stream; status: Status; counter: string };
export type QueueScope = { level: Level; date: string; periodId: string; stream: Stream };
// Staff-only projection from the server: when the queued slot should start.
export type SlotEstimate = { startAt: string; late?: boolean };
