import type { ReactNode } from 'react';
import { CircleHelp } from 'lucide-react';
import { Badge as UIBadge } from '@openai/apps-sdk-ui/components/Badge';
import { EVENT, STATUS_LABELS } from './config';
import type { Status } from './types';

export function Badge({ status }: { status: Status }) {
  return (
    <UIBadge
      color={status === 'Completed' ? 'success' : status === 'Calling' ? 'danger' : status === 'Serving' ? 'info' : status === 'Waiting' ? 'warning' : 'secondary'}
      className={`badge status-${status.toLowerCase()}`}
    >
      {['Calling', 'Serving'].includes(status) && <span className="dot" />}
      {STATUS_LABELS[status]}
    </UIBadge>
  );
}

export function Notice({ children, error = false }: { children: ReactNode; error?: boolean }) {
  return (
    <div role={error ? 'alert' : 'status'} className={`notice ${error ? 'error' : ''}`}>
      <CircleHelp size={17} />
      <div>{children}</div>
    </div>
  );
}

export function Header({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children?: ReactNode }) {
  return (
    <header className="page-heading">
      <div>
        <div className="eyebrow">{eyebrow}</div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {children}
    </header>
  );
}

export function EventBanner() {
  return (
    <div className="sample-banner">
      <span>{EVENT.sample ? (EVENT.id.startsWith('local-test-') ? 'โหมดทดลองในเครื่อง · ข้อมูลทดสอบ' : 'ตารางกิจกรรมตัวอย่าง · สำหรับทดลองระบบ') : EVENT.title}</span>
      <span>เชื่อมต่อระบบจองและคิวของ Warwick</span>
    </div>
  );
}
