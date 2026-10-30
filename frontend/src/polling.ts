export const LIVE_POLL_INTERVAL_MS = 5000;

// Schedule after completion: slow requests never overlap or accumulate timer ticks.
export function createPoller({ fetch: fetchData, interval, available,
  now = Date.now, schedule = (fn: () => void, ms: number) => setTimeout(fn, ms), cancel = clearTimeout,
}: {
  fetch: () => Promise<void>; interval: () => number; available: () => boolean;
  now?: () => number; schedule?: (fn: () => void, ms: number) => ReturnType<typeof setTimeout>;
  cancel?: (timer: ReturnType<typeof setTimeout>) => void;
}) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let flight: Promise<void> | null = null;
  let stopped = false, dirty = false, failures = 0, lastSuccess = -Infinity, retryAt = 0;
  const clear = () => { if (timer !== undefined) cancel(timer); timer = undefined; };
  const delay = () => failures ? Math.min(120000, 15000 * 2 ** Math.min(failures, 3)) : interval();
  const plan = () => {
    clear();
    if (!stopped && available()) timer = schedule(() => { void refresh(); }, delay());
  };
  function refresh(): Promise<void> {
    if (stopped || !available()) return Promise.resolve();
    if (flight) return flight;
    clear();
    flight = (async () => {
      do {
        dirty = false;
        try { await fetchData(); failures = 0; lastSuccess = now(); retryAt = 0; }
        catch { failures++; retryAt = now() + delay(); }
      } while (dirty && !stopped && available() && failures === 0);
    })().finally(() => { flight = null; plan(); });
    return flight;
  }
  return {
    refresh,
    // A mutation during a read needs one fresh read afterwards; clicks/focus do not.
    invalidate() { dirty = true; if (!flight) void refresh(); },
    resume() {
      clear();
      if (stopped || !available() || flight) return;
      if (dirty || (failures ? now() >= retryAt : now() - lastSuccess >= interval())) void refresh();
      else timer = schedule(() => { void refresh(); }, failures ? retryAt - now() : interval() - (now() - lastSuccess));
    },
    stop() { stopped = true; clear(); },
  };
}
