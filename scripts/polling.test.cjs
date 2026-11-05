const { test } = require('node:test');
const assert = require('node:assert/strict');
const mod = import('../frontend/src/polling.ts');
async function harness(interval = 5000, fetch) {
  const { createPoller } = await mod;
  let time = 0, visible = true, calls = 0, id = 0;
  const tasks = new Map();
  const tick = async ms => {
    const end = time + ms;
    while (true) {
      const next = [...tasks].filter(([,t])=>t.at <= end).sort((a,b)=>a[1].at-b[1].at)[0];
      if (!next) break;
      tasks.delete(next[0]); time = next[1].at; next[1].fn();
      await flush();
    }
    time = end; await flush();
  };
  const poller = createPoller({
    async fetch() { calls++; if (fetch) await fetch(); }, interval: () => typeof interval === 'function' ? interval() : interval,
    available: () => visible, now: () => time,
    schedule(fn,ms) { const key = ++id; tasks.set(key,{at:time+ms,fn});return key; },
    cancel(key) { tasks.delete(key); },
  });
  return { poller, tick, get calls(){return calls;}, get tasks(){return tasks.size;},
    visible(value) { visible=value; poller.resume(); } };
}
async function flush() { for(let i=0;i<12;i++) await Promise.resolve(); }
function deferred() { let resolve; const promise = new Promise(r => resolve=r);return { promise, resolve }; }

test('five-second live polling maintains one timer and stops cleanly',async()=>{
  const { LIVE_POLL_INTERVAL_MS }=await mod;
  const h=await harness(LIVE_POLL_INTERVAL_MS);await h.poller.refresh();
  await h.tick(4999);assert.equal(h.calls,1);
  await h.tick(1);assert.equal(h.calls,2);
  await h.tick(55000);assert.equal(h.calls,13);assert.equal(h.tasks,1);
  h.poller.stop();await h.tick(60000);assert.equal(h.calls,13);assert.equal(h.tasks,0);
});
test('hidden/offline stops all timers, returns stale data immediately, focus is deduplicated',async()=>{
  const h=await harness();await h.poller.refresh();h.visible(false);await h.tick(60000);
  assert.equal(h.calls,1);assert.equal(h.tasks,0);
  h.visible(true);h.poller.resume();h.poller.resume();await flush();assert.equal(h.calls,2);
  await h.tick(1000);h.poller.resume();await flush();assert.equal(h.calls,2);
  await h.tick(4000);assert.equal(h.calls,3);h.poller.stop();
});
test('slow reads never overlap; one mutation followup coalesces duplicate refreshes',async()=>{
  const gates=[deferred(),deferred()];let n=0;
  const h=await harness(5000,()=>gates[n++]?.promise);
  const flight=h.poller.refresh();await h.tick(30000);assert.equal(h.calls,1);
  h.poller.invalidate();h.poller.invalidate();const same=h.poller.refresh();assert.equal(flight,same);
  gates[0].resolve();await flush();assert.equal(h.calls,2);
  gates[1].resolve();await flight;assert.equal(h.calls,2);
  await h.tick(4999);assert.equal(h.calls,2);await h.tick(1);assert.equal(h.calls,3);h.poller.stop();
});
test('errors back off 30/60/120 seconds and successful retry restores normal cadence',async()=>{
  let fail=true;const h=await harness(5000,async()=>{if(fail)throw new Error('offline');});
  await h.poller.refresh();await h.tick(29999);assert.equal(h.calls,1);await h.tick(1);assert.equal(h.calls,2);
  await h.tick(60000);assert.equal(h.calls,3);await h.tick(119999);assert.equal(h.calls,3);
  fail=false;await h.tick(1);assert.equal(h.calls,4);await h.tick(5000);assert.equal(h.calls,5);h.poller.stop();
});
test('focus does not bypass retry delay; explicit refresh can retry immediately',async()=>{
  const h=await harness(5000,async()=>{throw new Error('failed');});await h.poller.refresh();
  h.poller.resume();h.poller.resume();await flush();assert.equal(h.calls,1);
  await h.poller.refresh();assert.equal(h.calls,2);h.poller.stop();
});
test('cleanup during a request cannot schedule another read or mutation followup',async()=>{
  const gate=deferred();const h=await harness(5000,()=>gate.promise);const f=h.poller.refresh();
  h.poller.invalidate();h.poller.stop();gate.resolve();await f;await h.tick(60000);
  assert.equal(h.calls,1);assert.equal(h.tasks,0);
});

test('forced parent foreground refresh reads immediately inside five seconds and restarts normal cadence',async()=>{
  const h=await harness();await h.poller.refresh();await h.tick(1000);
  h.poller.resume(true);await flush();assert.equal(h.calls,2);
  await h.tick(4999);assert.equal(h.calls,2);await h.tick(1);assert.equal(h.calls,3);h.poller.stop();
});
test('foreground events during a slow read coalesce and force one newer read without overlap',async()=>{
  const gates=[deferred(),deferred()];let n=0;
  const h=await harness(5000,()=>gates[n++]?.promise);const first=h.poller.refresh();
  h.poller.resume(true);h.poller.resume(true);await flush();assert.equal(h.calls,1);
  gates[0].resolve();await flush();assert.equal(h.calls,2);
  gates[1].resolve();await first;assert.equal(h.calls,2);assert.equal(h.tasks,1);h.poller.stop();
});
test('foreground refresh queued during a failed old read retries immediately once',async()=>{
  const gate=deferred();let n=0;
  const h=await harness(5000,async()=>{if(n++===0){await gate.promise;throw new Error('old request failed');}});
  const first=h.poller.refresh();h.poller.resume(true);gate.resolve();await first;
  assert.equal(h.calls,2);await h.tick(5000);assert.equal(h.calls,3);h.poller.stop();
});
test('forced foreground refresh respects hidden state and stopped cleanup',async()=>{
  const h=await harness();await h.poller.refresh();h.visible(false);h.poller.resume(true);await flush();assert.equal(h.calls,1);
  h.visible(true);h.poller.resume(true);await flush();assert.equal(h.calls,2);
  h.poller.stop();h.poller.resume(true);await h.tick(10000);assert.equal(h.calls,2);
});
