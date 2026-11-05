// Executes the real shipping service worker (frontend/public/sw.js) inside a
// node:vm context with mocked ServiceWorkerGlobalScope globals.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const SRC = fs.readFileSync(path.join(__dirname, '../frontend/public/sw.js'), 'utf8');
const ORIGIN = 'http://example.test';
const CACHE = 'spr-shell-v1';

function html(body, status = 200) {
  return new Response(body, { status, headers: { 'content-type': 'text/html' } });
}

function fixture() {
  const handlers = new Map(), cacheMap = new Map(), fetched = [], state = { skipped: 0, claimed: 0 };
  let network = async () => html('<!doctype html><html>shell</html>');

  const absolute = (key) => (typeof key === 'string' ? new URL(key, ORIGIN).href : key.url);
  const fetchNow = async (request) => {
    const url = typeof request === 'string' ? new URL(request, ORIGIN).href : request.url;
    fetched.push(url);
    return network(url);
  };
  const openCache = (name) => {
    if (!cacheMap.has(name)) {
      const store = new Map();
      cacheMap.set(name, {
        store,
        async add(url) {
          const response = await fetchNow(url);
          if (!response || response.status >= 400) throw new TypeError(`cache add failed: ${url}`);
          store.set(absolute(url), response.clone());
        },
        async addAll(urls) {
          const responses = await Promise.all(urls.map(fetchNow));
          if (responses.some((response) => !response || response.status >= 400)) throw new TypeError('cache addAll failed');
          urls.forEach((url, i) => store.set(absolute(url), responses[i].clone()));
        },
        async put(key, response) {
          store.set(absolute(key), response.clone());
        },
        async match(key) {
          const hit = store.get(absolute(key));
          return hit ? hit.clone() : undefined;
        },
      });
    }
    return cacheMap.get(name);
  };

  const caches = {
    open: async (name) => openCache(name),
    keys: async () => [...cacheMap.keys()],
    delete: async (name) => cacheMap.delete(name),
    match: async (key) => {
      for (const cache of cacheMap.values()) {
        const hit = await cache.match(key);
        if (hit) return hit;
      }
      return undefined;
    },
  };
  const self = {
    location: { origin: ORIGIN },
    addEventListener(type, handler) {
      if (!handlers.has(type)) handlers.set(type, []);
      handlers.get(type).push(handler);
    },
    skipWaiting() {
      state.skipped++;
    },
    clients: {
      claim() {
        state.claimed++;
      },
    },
  };
  const context = vm.createContext({ self, caches, fetch: fetchNow, Response, URL, console: { log() {} } });
  vm.runInContext(SRC, context);

  const dispatch = (type, event) => (handlers.get(type) || []).forEach((handler) => handler(event));
  const runLifecycle = async (type) => {
    const event = { waits: [] };
    event.waitUntil = (promise) => event.waits.push(promise);
    dispatch(type, event);
    await Promise.all(event.waits);
    return event;
  };
  const fireFetch = async (request) => {
    const event = { request, responds: [] };
    event.respondWith = (promise) => event.responds.push(promise);
    dispatch('fetch', event);
    if (!event.responds.length) return { intercepted: false };
    return { intercepted: true, response: await event.responds[0] };
  };
  const match = async (key) => {
    for (const cache of cacheMap.values()) {
      const hit = await cache.match(key);
      if (hit) return hit;
    }
    return undefined;
  };
  const request = (url, options = {}) => ({
    url: (options.origin || ORIGIN) + url,
    method: options.method || 'GET',
    mode: options.mode || 'same-origin',
  });

  return {
    state,
    fetched,
    fireFetch,
    request,
    match,
    cacheNames: () => [...cacheMap.keys()],
    openCache,
    setNetwork: (fn) => { network = fn; },
    install: (fail = []) => {
      network = async (url) => (fail.some((f) => new URL(url).pathname === f) ? html('missing', 404) : html('<!doctype html><html>shell</html>'));
      return runLifecycle('install');
    },
    activate: () => runLifecycle('activate'),
  };
}

test('install precaches / and /index.html then calls skipWaiting', async () => {
  const worker = fixture();
  await worker.install();
  assert.equal(await worker.match('/index.html') instanceof Response, true);
  assert.equal(await worker.match('/') instanceof Response, true);
  assert.equal(worker.state.skipped, 1);
});

test('install survives a losing precache URL and still activates', async () => {
  const worker = fixture();
  await worker.install(['/index.html']);
  assert.equal(await worker.match('/') instanceof Response, true);
  assert.equal(worker.state.skipped, 1);
});

test('activate deletes stale spr-shell-* caches, keeps the current one and claims clients', async () => {
  const worker = fixture();
  await worker.openCache('spr-shell-v0');
  await worker.openCache('spr-shell-v1');
  await worker.activate();
  assert.equal(worker.cacheNames().includes('spr-shell-v0'), false);
  assert.equal(worker.cacheNames().includes('spr-shell-v1'), true);
  assert.equal(worker.state.claimed, 1);
});

test('/api/ requests are never intercepted or cached', async () => {
  const worker = fixture();
  const result = await worker.fireFetch(worker.request('/api/track/xyz'));
  assert.equal(result.intercepted, false);
  assert.equal(worker.fetched.length, 0);
});

test('successful navigation is served from network and cached under /index.html', async () => {
  const worker = fixture();
  worker.setNetwork(async () => html('<!doctype html><html>network shell</html>'));
  const result = await worker.fireFetch(worker.request('/#/track/abc', { mode: 'navigate' }));
  assert.equal(result.intercepted, true);
  assert.equal(result.response.status, 200);
  assert.match(await result.response.text(), /network shell/);
  const shell = await worker.match('/index.html');
  assert.match(await shell.text(), /network shell/);
});

test('failed navigation falls back to the cached /index.html shell', async () => {
  const worker = fixture();
  await (await worker.openCache(CACHE)).put('/index.html', html('<!doctype html><html>cached shell</html>'));
  worker.setNetwork(async () => { throw new TypeError('offline'); });
  const result = await worker.fireFetch(worker.request('/#/track/abc', { mode: 'navigate' }));
  assert.equal(result.intercepted, true);
  assert.equal(result.response.status, 200);
  assert.match(await result.response.text(), /cached shell/);
});

test('failed navigation with an empty cache resolves Response.error(), never undefined', async () => {
  const worker = fixture();
  await worker.install();
  await worker.fireFetch(worker.request('/#/first'));
  await (worker.openCache(CACHE)).store.clear();
  worker.setNetwork(async () => { throw new TypeError('offline'); });
  const result = await worker.fireFetch(worker.request('/#/track', { mode: 'navigate' }));
  assert.equal(result.intercepted, true);
  assert.equal(result.response.type, 'error');
});

test('/assets/x.js is cache-first and survives offline once cached', async () => {
  const worker = fixture();
  worker.setNetwork(async () => new Response('console.log(1)', { status: 200 }));
  const miss = await worker.fireFetch(worker.request('/assets/x.js'));
  assert.equal(miss.intercepted, true);
  assert.match(await miss.response.text(), /console\.log/);
  assert.equal(worker.fetched.length, 1);
  worker.setNetwork(async () => { throw new TypeError('offline'); });
  const hit = await worker.fireFetch(worker.request('/assets/x.js'));
  assert.equal(worker.fetched.length, 1, 'cache hit must not touch the network');
  assert.match(await hit.response.text(), /console\.log/);
});

test('cross-origin and non-GET requests are never intercepted', async () => {
  const worker = fixture();
  assert.equal((await worker.fireFetch(worker.request('/api/x', { origin: 'https://other.example' }))).intercepted, false);
  assert.equal((await worker.fireFetch(worker.request('/api/x', { method: 'POST' }))).intercepted, false);
});
