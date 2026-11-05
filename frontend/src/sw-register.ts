/**
 * Registers the app-shell service worker for offline support.
 *
 * Deliberately defensive: it only ever registers in production builds over
 * http(s), it waits for the window 'load' event, and it swallows every error.
 * Registration must never break or delay the app.
 */
export function registerServiceWorker(): void {
  try {
    // Never register during `vite dev` (import.meta.env.PROD is false there).
    if (!import.meta.env.PROD) return;

    if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return;
    if (typeof window === 'undefined' || typeof document === 'undefined') return;

    const protocol = window.location.protocol;
    if (protocol !== 'https:' && protocol !== 'http:') return;

    const register = (): void => {
      try {
        navigator.serviceWorker.register('/sw.js').catch(() => {
          /* registration failed (private mode, stale file, ...): ignore */
        });
      } catch {
        /* synchronous failure: ignore */
      }
    };

    if (document.readyState === 'complete') {
      register();
    } else {
      window.addEventListener('load', register, { once: true });
    }
  } catch {
    /* never throw */
  }
}
