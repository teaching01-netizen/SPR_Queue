import { useState } from 'react';
import type { FormEvent } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Button as UIAction } from '@openai/apps-sdk-ui/components/Button';
import { Input as UIInput } from '@openai/apps-sdk-ui/components/Input';
import { api } from '../api';
import type { StaffUser } from '../api';
import { Notice } from '../ui';

const message = (e: unknown) => (e instanceof Error ? e.message : 'เกิดข้อผิดพลาด กรุณาลองอีกครั้ง');

// Staff entry form. Rendered at the bare site URL and on #/staff when signed out.
export function StaffLogin({ onSuccess }: { onSuccess: (user: StaffUser) => void }) {
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  async function login(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setBusy(true);
    setError('');
    try {
      onSuccess(await api.login(String(form.get('username')), String(form.get('password'))));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="staff-login">
      <div className="round-icon"><ShieldCheck size={28} /></div>
      <h1>Staff Login</h1>
      <p>สำหรับทีม Warwick เพื่อจัดลำดับและควบคุมคิว</p>
      <form onSubmit={login}>
        <label htmlFor="staff-username">ชื่อผู้ใช้<UIInput id="staff-username" size="xl" name="username" required autoComplete="username" maxLength={100} /></label>
        <label className="login-password" htmlFor="staff-password">รหัสผ่าน<UIInput id="staff-password" size="xl" type="password" name="password" required autoComplete="current-password" maxLength={72} /></label>
        {error && <Notice error>{error}</Notice>}
        <UIAction color="primary" pill={false} size="xl" type="submit" className="btn primary" disabled={busy}>{busy ? 'กำลังเข้าสู่ระบบ…' : 'เข้าสู่ระบบ'}</UIAction>
      </form>
      <p className="staff-login-note">ผู้ปกครองไม่ต้องเข้าสู่ระบบ · เปิดลิงก์ติดตามคิวจากอีเมลได้เลย</p>
    </div>
  );
}
