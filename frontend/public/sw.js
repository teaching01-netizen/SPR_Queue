/*
 * SPR_Queue app-shell service worker.
 *
 * Deployment: served from the site root as /sw.js. Plain ES5-ish JavaScript —
 * no imports, no bundler, no dependencies.
 *
 * Strategy summary:
 *   - install:  precache the app shell ('/' and '/index.html'), then activate
 *               immediately via skipWaiting().
 *   - activate: drop stale 'spr-shell-*' caches, then claim open clients.
 *   - fetch:    same-origin GET only, and never touch '/api/'.
 *               navigate        -> network-first, fall back to cached shell.
 *               '/assets/*'     -> cache-first (Vite hashed, immutable).
 *               anything else   -> stale-while-revalidate.
 *
 * Every cache operation is wrapped in a catch so a quota/storage failure can
 * never break the page. Fetch handlers always resolve to a real Response.
 */

const CACHE = 'spr-shell-v1';
const SHELL_KEY = '/index.html';
const SHELL_FALLBACK_KEY = '/';
const PRECACHE_URLS = ['/', '/index.html'];

/**
 * Cache a URL, swallowing any failure (offline, 404, quota, opaque, ...).
 * @param {Cache} cache
 * @param {string} url
 */
function safeAdd(cache, url) {
  try {
    return cache.add(url).catch(function () {
      /* individual URL failed to cache: ignore */
    });
  } catch (error) {
    return Promise.resolve();
  }
}

self.addEventListener('install', function (event) {
  event.waitUntil(
    (async function () {
      let cache = null;
      try {
        cache = await caches.open(CACHE);
      } catch (error) {
        cache = null;
      }

      if (cache) {
        // Prefer a single atomic addAll; if any URL fails (e.g. one 404),
        // fall back to caching whatever succeeds so install still completes.
        try {
          await cache.addAll(PRECACHE_URLS);
        } catch (error) {
          await Promise.all(
            PRECACHE_URLS.map(function (url) {
              return safeAdd(cache, url);
            }),
          );
        }
      }

      await self.skipWaiting();
    })(),
  );
});

self.addEventListener('activate', function (event) {
  event.waitUntil(
    (async function () {
      try {
        const cacheNames = await caches.keys();
        await Promise.all(
          cacheNames.map(function (name) {
            if (name.indexOf('spr-shell-') === 0 && name !== CACHE) {
              return caches.delete(name).catch(function () {
                /* ignore */
              });
            }
            return Promise.resolve();
          }),
        );
      } catch (error) {
        /* cache cleanup must never break activation */
      }

      await self.clients.claim();
    })(),
  );
});

/**
 * Open the shell cache. Resolves to null when storage is unavailable.
 * @returns {Promise<Cache|null>}
 */
function openShellCache() {
  return caches.open(CACHE).catch(function () {
    return null;
  });
}

/**
 * Store a response clone under the given key if it is worth caching.
 * @param {Cache|null} cache
 * @param {string|Request} key
 * @param {Response} response
 */
function safePut(cache, key, response) {
  if (!cache) return;
  try {
    if (!response || response.status !== 200 || response.type === 'opaque') return;
    cache.put(key, response.clone()).catch(function () {
      /* storage failure: ignore */
    });
  } catch (error) {
    /* cloning/putting failed: ignore */
  }
}

/**
 * Match a key across every cache without ever rejecting.
 * @param {string|Request} key
 * @returns {Promise<Response|undefined>}
 */
function safeCachesMatch(key) {
  try {
    return caches.match(key).catch(function () {
      return undefined;
    });
  } catch (error) {
    return Promise.resolve(undefined);
  }
}

/**
 * Read from cache without ever rejecting.
 * @param {Cache|null} cache
 * @param {string|Request} key
 * @returns {Promise<Response|undefined>}
 */
function safeMatch(cache, key) {
  if (!cache) return Promise.resolve(undefined);
  try {
    return cache.match(key).catch(function () {
      return undefined;
    });
  } catch (error) {
    return Promise.resolve(undefined);
  }
}

/**
 * Network-first navigation: freshest shell when online, cached shell offline.
 * @param {Request} request
 */
function handleNavigate(request) {
  return (async function () {
    try {
      const response = await fetch(request);
      if (response && response.status === 200) {
        const cache = await openShellCache();
        safePut(cache, SHELL_KEY, response);
      }
      return response;
    } catch (error) {
      const shell = await safeCachesMatch(SHELL_KEY);
      if (shell) return shell;

      const root = await safeCachesMatch(SHELL_FALLBACK_KEY);
      if (root) return root;

      return Response.error();
    }
  })();
}

/**
 * Cache-first for immutable build assets ('/assets/...').
 * @param {Request} request
 */
function handleAsset(request) {
  return (async function () {
    const cache = await openShellCache();
    const cached = await safeMatch(cache, request);
    if (cached) return cached;

    try {
      const response = await fetch(request);
      safePut(cache, request, response);
      return response;
    } catch (error) {
      // A racing sibling request may have filled the cache in the meantime.
      const retry = await safeMatch(cache, request);
      return retry || Response.error();
    }
  })();
}

/**
 * Stale-while-revalidate for everything else (favicon, static images, ...).
 * @param {Request} request
 */
function handleStaleWhileRevalidate(request) {
  return (async function () {
    const cache = await openShellCache();
    const cached = await safeMatch(cache, request);

    const revalidation = fetch(request)
      .then(function (response) {
        safePut(cache, request, response);
        return response;
      })
      .catch(function () {
        return undefined;
      });

    if (cached) {
      // Update in the background; the page gets the cached copy right away.
      revalidation.catch(function () {
        /* ignore */
      });
      return cached;
    }

    const fresh = await revalidation;
    return fresh || Response.error();
  })();
}

self.addEventListener('fetch', function (event) {
  const request = event.request;

  // Only same-origin GET requests are ours to handle.
  if (!request || request.method !== 'GET') return;
  try {
    const url = new URL(request.url);
    if (url.origin !== self.location.origin) return;

    // API traffic must always hit the network untouched (tracking/polling).
    if (url.pathname.indexOf('/api/') === 0) return;

    if (request.mode === 'navigate') {
      event.respondWith(handleNavigate(request));
      return;
    }

    if (url.pathname.indexOf('/assets/') === 0) {
      event.respondWith(handleAsset(request));
      return;
    }

    event.respondWith(handleStaleWhileRevalidate(request));
  } catch (error) {
    // Malformed URL or unexpected platform error: let the browser go to network.
  }
});
