# Consult-day storyboard capture (Playwright)

These scripts drive the real app with Playwright and write the screenshots and
captions used in [`docs/e2e-2026-11-06-full-flow/`](../../docs/e2e-2026-11-06-full-flow/STORYBOARD.md):
parent booking → tracking link → staff check-in → queue/ETA → call → serve
(timer/overrun) → complete → Combined handoff → no-show → cancel/restore →
Admin recovery/delete → swap → operator view → public display/kiosk → mobile →
edge states.

The capture runs against an **isolated local server and scratch PostgreSQL
database** with mail and Sheets sync disabled. It never touches Railway, the
production database, email or live Sheets.

## Prerequisites

- Node.js 20+ and a local Chrome (the scripts use `channel: 'chrome'`, so no
  browser download is needed)
- PostgreSQL 14+ reachable as the current user
- Go toolchain (the repo's backend module)
- A production build of the frontend: `cd frontend && npm ci && npm run build`

## One-time demo environment

```bash
mkdir -p /tmp/warwick-flow && cd /tmp/warwick-flow

# Server binary and operator password hash
(cd <repo>/backend && go build -o /tmp/warwick-flow/server ./cmd/server)

# Capture-friendly demo schedule: same event, levels, subjects and real 7 Nov
# Beginner rounds; the 6 November High Level rounds move to 14:00-15:30 /
# 15:30-17:00 and the day is On-site so the walkthrough shows room queues.
cp <repo>/scripts/e2e-flow/event.demo.json /tmp/warwick-flow/event.demo.json

cat > /tmp/warwick-flow/env <<EOF
APP_SECRET=$(openssl rand -base64 32)
DATABASE_URL=postgres://<user>@localhost:5432/warwick_flow_demo?sslmode=disable
APP_ENV=development
PUBLIC_URL=http://127.0.0.1:18081
HTTP_ADDR=127.0.0.1:18081
EVENT_CONFIG=/tmp/warwick-flow/event.demo.json
FRONTEND_DIST=<repo>/frontend/dist
STAFF_USERNAME=warwick-admin
STAFF_PASSWORD=flow-demo-password-123
EOF
```

Install the driver once:

```bash
cd <repo>/scripts/e2e-flow && npm i playwright-core
```

## Capture

```bash
cd <repo>/scripts/e2e-flow
./reset.sh                 # fresh database + server on 127.0.0.1:18081
node c1-parent.mjs         # chapters 1-2: booking form, tracking link
node c2-staff.mjs          # chapters 3-9: sign-in, check-in, queue, call, serve, settings, complete
node c3-cases.mjs          # chapters 10-14: Combined, no-show, cancel/restore, recovery/delete, swap
node c4-misc.mjs           # chapters 15-20: export, operator, display/kiosk, mobile, edge states
node storyboard.mjs        # rebuild STORYBOARD.md + gallery.html from captions
```

`reset.sh` must run before a full capture: the chapters seed their own families
and expect a clean database. State (booking ids/tokens) and captions are kept in
`/tmp/warwick-flow/`; override with `FLOW_STATE` / `FLOW_CAPS`. Other overrides:
`FLOW_BASE`, `FLOW_DB`, `FLOW_DIR`, `FLOW_OUT`.

## Notes

- `event.demo.json` is a demo-only schedule (times moved into the capture
  window, 6 November marked On-site). The committed `backend/config/event.json`
  and `event.production.json` are unchanged.
- `c2-staff.mjs` simulates elapsed time by moving the saved `startedAt`/
  `calledAt` stamps into the past, so the overrun screenshots are deterministic
  instead of waiting 20 minutes.
- The public shell intentionally probes `/api/auth/me` (401) and the SDK CSS
  references CDN fonts blocked by the CSP; only uncaught page errors count as
  failures.
