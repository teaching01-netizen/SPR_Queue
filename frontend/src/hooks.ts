import { useCallback, useEffect, useRef, useState } from 'react';
import { createPoller, LIVE_POLL_INTERVAL_MS } from './polling';
import { ApiError } from './api';
import { clearSnapshot, readSnapshot, writeSnapshot } from './storage';

type LiveOptions = { enabled?: boolean; refreshOnResume?: boolean; cacheKey?: string };

// Live data hook. The cache is a gap filler only: it paints the last known snapshot
// immediately, then the poller corrects it and refreshes on resume/reconnect.
export function useLive<T>(fetcher: () => Promise<T>, { enabled = true, refreshOnResume = false, cacheKey }: LiveOptions = {}) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState('');
  const [errorStatus, setErrorStatus] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [updated, setUpdated] = useState<Date | null>(null);
  const pollerRef = useRef<ReturnType<typeof createPoller> | null>(null);
  const refresh = useCallback(() => pollerRef.current?.refresh() ?? Promise.resolve(), []);

  useEffect(() => {
    if (!enabled) return;
    let active = true, requestEpoch = 0;
    const cached = cacheKey ? readSnapshot<T>(cacheKey) : null;
    setData(cached?.data ?? null);
    setUpdated(cached ? new Date(cached.fetchedAt) : null);
    setError('');
    setErrorStatus(null);
    setLoading(!cached);
    const poller = createPoller({
      available: () => !document.hidden && navigator.onLine !== false,
      interval: () => LIVE_POLL_INTERVAL_MS,
      async fetch() {
        const epoch = requestEpoch;
        try {
          const next = await fetcher();
          if (cacheKey) writeSnapshot(cacheKey, next);
          if (active && epoch === requestEpoch) { setData(next); setError(''); setErrorStatus(null); setUpdated(new Date()); }
        } catch (e) {
          const status = e instanceof ApiError ? e.status : null;
          if (status === 404 && cacheKey) clearSnapshot(cacheKey);
          if (active && epoch === requestEpoch) {
            if (status === 404) { setData(null); setUpdated(null); }
            setError(e instanceof Error ? e.message : 'โหลดข้อมูลไม่สำเร็จ');
            setErrorStatus(status);
          }
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
  }, [fetcher, enabled, refreshOnResume, cacheKey]);
  return { data, error, errorStatus, loading, updated, refresh };
}

// Ticks a clock so freshness bands and "last updated" labels stay accurate without polling.
export function useNow(intervalMs = 5000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}

export function useOnline(): boolean {
  const [online, setOnline] = useState(() => (typeof navigator === 'undefined' ? true : navigator.onLine !== false));
  useEffect(() => {
    const update = () => setOnline(navigator.onLine !== false);
    window.addEventListener('online', update);
    window.addEventListener('offline', update);
    update();
    return () => {
      window.removeEventListener('online', update);
      window.removeEventListener('offline', update);
    };
  }, []);
  return online;
}

// Keeps an unattended kiosk screen awake where the browser allows it.
export function useWakeLock(enabled: boolean): void {
  useEffect(() => {
    if (!enabled) return;
    let sentinel: WakeLockSentinel | null = null;
    let cancelled = false;
    const request = async () => {
      try {
        if (!('wakeLock' in navigator) || document.hidden) return;
        sentinel = await navigator.wakeLock.request('screen');
      } catch {
        sentinel = null;
      }
    };
    const onVisibility = () => { if (!document.hidden && !cancelled) void request(); };
    void request();
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      cancelled = true;
      document.removeEventListener('visibilitychange', onVisibility);
      if (sentinel) void sentinel.release().catch(() => {});
    };
  }, [enabled]);
}
