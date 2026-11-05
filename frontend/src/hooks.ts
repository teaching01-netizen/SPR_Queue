import { useCallback, useEffect, useRef, useState } from 'react';
import { createPoller, LIVE_POLL_INTERVAL_MS } from './polling';

type LiveOptions = { enabled?: boolean; refreshOnResume?: boolean };
export function useLive<T>(fetcher: () => Promise<T>, { enabled = true, refreshOnResume = false }: LiveOptions = {}) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [updated, setUpdated] = useState<Date | null>(null);
  const pollerRef = useRef<ReturnType<typeof createPoller> | null>(null);
  const refresh = useCallback(() => pollerRef.current?.refresh() ?? Promise.resolve(), []);

  useEffect(() => {
    if (!enabled) return;
    let active = true, requestEpoch = 0;
    setData(null); setError(''); setLoading(true); setUpdated(null);
    const poller = createPoller({
      available: () => !document.hidden && navigator.onLine !== false,
      interval: () => LIVE_POLL_INTERVAL_MS,
      async fetch() {
        const epoch = requestEpoch;
        try {
          const next = await fetcher();
          if (active && epoch === requestEpoch) { setData(next); setError(''); setUpdated(new Date()); }
        } catch (e) {
          if (active && epoch === requestEpoch) setError(e instanceof Error ? e.message : 'โหลดข้อมูลไม่สำเร็จ');
          throw e;
        } finally { if (active && epoch === requestEpoch) setLoading(false); }
      },
    });
    pollerRef.current = poller;
    const resume = () => {
      if (refreshOnResume && !document.hidden && navigator.onLine !== false) { requestEpoch++; setLoading(true); setError(''); }
      poller.resume(refreshOnResume);
    };
    const invalidate = () => poller.invalidate();
    document.addEventListener('visibilitychange', resume);
    window.addEventListener('focus', resume);
    window.addEventListener('online', resume);
    window.addEventListener('pageshow', resume);
    window.addEventListener('offline', resume);
    window.addEventListener('queue-updated', invalidate);
    void poller.refresh();
    return () => {
      active = false; poller.stop(); pollerRef.current = null;
      document.removeEventListener('visibilitychange', resume);
      window.removeEventListener('focus', resume);
      window.removeEventListener('online', resume);
      window.removeEventListener('pageshow', resume);
      window.removeEventListener('offline', resume);
      window.removeEventListener('queue-updated', invalidate);
    };
  }, [fetcher, enabled, refreshOnResume]);
  return { data, error, loading, updated, refresh };
}
