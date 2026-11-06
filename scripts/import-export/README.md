# Import a Staff Excel export into a local database

Local-only tooling for loading a real `SPR_Queue_Registrations_*.xlsx` export
into a scratch PostgreSQL database so the app can be tested with real
registrations. It keeps the exported booking IDs, statuses, checked-in times and
stage timestamps, and generates fresh valid tracking tokens for each booking.

## Steps

```bash
# 1. Create the scratch database and point backend/.env at it
createdb warwick_local_import
# backend/.env (gitignored): DATABASE_URL=postgres://<user>@localhost:5432/warwick_local_import?sslmode=disable
#                            APP_SECRET=<base64 32 bytes>, PUBLIC_URL=http://127.0.0.1:8080
#                            EVENT_CONFIG=config/event.production.json, APP_ENV=development

# 2. Convert the xlsx (needs openpyxl)
python3 scripts/import-export/xlsx_to_rows.py \
  "/path/SPR_Queue_Registrations_20261106_034247Z.xlsx" \
  backend/config/event.production.json \
  /tmp/warwick-import/rows.json

# 3. Import (must run inside backend/ so the config paths resolve)
cd backend
set -a; . ./.env; set +a
go run ./cmd/importregistrations \
  -rows /tmp/warwick-import/rows.json \
  -config config/event.production.json \
  -out /tmp/warwick-import/tracking-links.tsv \
  -confirm-local

# 4. Start the server and open http://127.0.0.1:8080
go run ./cmd/server
```

`-replace` clears the event's existing local bookings first. The importer
refuses non-local database hosts and external booking IDs/tokens stay local:
`tracking-links.tsv` contains one working parent link per booking (mode 0600) so
you can open tracking screens directly.

The export excludes cancelled/archived bookings, so the local dataset mirrors
the visible staff list. Email sending and Google Sheets sync stay disabled in
this setup.
