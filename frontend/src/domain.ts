import { EVENT } from './config';
import type { Booking, QueueScope } from './types';
export function inScope(b: Booking, scope: QueueScope) { return b.eventId === EVENT.id && b.level === scope.level && b.date === scope.date && b.periodId === scope.periodId && b.currentSubject === scope.stream; }
export function queueCompare(a: Booking, b: Booking) { return (a.orders[a.currentSubject] ?? Infinity) - (b.orders[b.currentSubject] ?? Infinity) || a.createdAt.localeCompare(b.createdAt); }
