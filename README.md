# visto-easy

Portale di gestione richieste visto (MVP tecnico) con API multi-ruolo, workflow pratiche, documenti e pagamento webhook.

## Features MVP implementate

- Auth JWT + refresh token (`RICHIEDENTE`, `OPERATORE`, `SUPERVISORE`, `ADMIN`)
- Pratiche richiedente: create/list/get/patch/delete (solo in `BOZZA`)
- Documenti pratica: upload metadata + lista
- Presigned upload URL per documenti (S3/MinIO)
- Backoffice: lista pratiche, cambio stato, link pagamento, invio visto, stats dashboard
- Pagamenti: creazione sessione, query stato, webhook `payment.succeeded`
- Webhook pagamenti idempotente con deduplicazione `event_id`/`id`
- Notifiche email transazionali via SendGrid (quando configurato)
- Audit sicurezza login/rate-limit con endpoint backoffice
- Denylist IP backoffice con persistenza datastore (riavvio-safe)
- Macchina stati pratica con validazione transizioni
- Frontend web completo responsive in `web/` con moduli Auth, Richiedente e Backoffice

## API principali

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/refresh`
- `POST /api/auth/2fa/setup`
- `POST /api/auth/2fa/enable`
- `POST /api/auth/2fa/disable`
- `POST /api/pratiche/`
- `GET /api/pratiche/`
- `GET /api/pratiche/{id}`
- `PATCH /api/pratiche/{id}`
- `DELETE /api/pratiche/{id}`
- `POST /api/pratiche/{id}/documenti`
- `POST /api/pratiche/{id}/documenti/presign`
- `GET /api/pratiche/{id}/documenti`
- `GET /api/pratiche/{id}/documenti/{docId}`
- `DELETE /api/pratiche/{id}/documenti/{docId}`
- `GET /api/pratiche/{id}/eventi`
- `GET /api/bo/pratiche`
- `GET /api/bo/utenti`
- `GET /api/bo/utenti/{id}/sessioni`
- `POST /api/bo/utenti/{id}/sessioni/revoca-all`
- `POST /api/bo/sessioni/{id}/revoca`
- `POST /api/bo/pagamenti/{token}/rimborso`
- `GET /api/bo/security/allowed-ips`
- `POST /api/bo/security/allowed-ips/allow`
- `POST /api/bo/security/allowed-ips/revoke`
- `POST /api/bo/security/allowed-ips/revoke-bulk`
- `GET /api/bo/security/blocked-ips`
- `GET /api/bo/security/evaluate-ip`
- `POST /api/bo/security/blocked-ips/block`
- `POST /api/bo/security/blocked-ips/unblock`
- `POST /api/bo/security/blocked-ips/unblock-bulk`
- `GET /api/bo/security-events`
- `GET /api/bo/security-events/stats`
- `GET /api/bo/security-events/stream`
- `GET /api/bo/security-events/{id}`
- `GET /api/bo/security-events/report.csv`
- `GET /api/bo/audit-events`
- `GET /api/bo/audit-events/{id}`
- `GET /api/bo/audit-events/report.csv`
- `GET /api/bo/report.csv`
- `GET /api/bo/notifications/stream`
- `PATCH /api/bo/pratiche/{id}/stato`
- `POST /api/bo/pratiche/{id}/link-pagamento`
- `POST /api/bo/pratiche/{id}/invia-visto`
- `GET /api/bo/dashboard/stats`
- `POST /api/pagamento/webhook`

Payload webhook supportato (compatibile anche con schema Stripe base):
- `event` oppure `type` (es. `payment.succeeded`)
- `event_id` oppure `id`
- `token` diretto oppure `data.object.metadata.token` / `data.object.client_reference_id`

2FA backoffice (TOTP):
- setup: `POST /api/auth/2fa/setup` (richiede token access backoffice)
- enable: `POST /api/auth/2fa/enable` payload `{ "code": "123456" }`
- disable: `POST /api/auth/2fa/disable` payload `{ "code": "123456" }`
- login backoffice con 2FA attiva: includere `otp` nel payload login

## Query params utili (backoffice)

- `page`, `page_size`
- `sort_by`, `sort_order`
- `/api/bo/pratiche`: `stato`, `priorita`, `tipo_visto`, `paese_dest`, `operatore_id`, `q`, `from`, `to`
- `/api/bo/utenti`: `ruolo`, `q`
- `/api/bo/security-events`: `type`, `outcome`, `q`, `from`, `to`, `page`, `page_size`
- `/api/bo/security-events/stats`: stessi filtri di `/api/bo/security-events`
- `/api/bo/security-events/report.csv`: stessi filtri di `/api/bo/security-events`
- `/api/bo/audit-events`: `action`, `resource`, `actor_id`, `q`, `from`, `to`, `page`, `page_size`
- `/api/bo/audit-events/report.csv`: stessi filtri di `/api/bo/audit-events`
- `/api/bo/report.csv`: stessi filtri di `/api/bo/pratiche`

Payload blocklist IP:
- `POST /api/bo/security/allowed-ips/allow`: `{ "ip": "10.0.0.0/24", "reason": "trusted office", "ttl_minutes": 240 }`
- `POST /api/bo/security/allowed-ips/revoke`: `{ "ip": "10.0.0.0/24" }`
- `POST /api/bo/security/allowed-ips/revoke-bulk`: `{ "targets": ["10.0.0.0/24", "10.0.1.4"] }` oppure `{ "revoke_all": true }`
- `POST /api/bo/security/blocked-ips/block`: `{ "ip": "1.2.3.4", "reason": "bruteforce", "ttl_minutes": 120 }`
- `POST /api/bo/security/blocked-ips/block`: `{ "ip": "203.0.113.0/24", "reason": "abuse subnet", "ttl_minutes": 180 }`
- `POST /api/bo/security/blocked-ips/unblock`: `{ "ip": "1.2.3.4" }` oppure `{ "ip": "203.0.113.0/24" }`
- `POST /api/bo/security/blocked-ips/unblock-bulk`: `{ "targets": ["1.2.3.4", "203.0.113.0/24"] }` oppure `{ "unblock_all": true }`
- `GET /api/bo/security/evaluate-ip?ip=203.0.113.7` valuta la policy effettiva e la regola matchata

Payload rimborso pagamento:
- `POST /api/bo/pagamenti/{token}/rimborso`: `{ "amount": 25.0, "reason": "customer_request" }` (amount opzionale)

Precedence policy IP:
- `block` exact > `allow` exact > CIDR con prefisso piu specifico; in caso di pareggio CIDR vince `block`

## Run locale

1. Copia `.env.example` in `.env` e imposta almeno `JWT_SECRET`.
2. Avvia:

```bash
go run .
```

Health:

```bash
curl http://localhost:8080/api/v1/health
```

Frontend web:

```bash
http://localhost:8080/
```

Il frontend include:
- autenticazione (login/register/forgot/reset/logout/refresh)
- area richiedente (creazione pratica, submit, documenti)
- area backoffice (pratiche, utenti, audit eventi, cambio stato, note)

## Deploy (Coolify / Railway)

- Docker multi-stage pronto in `Dockerfile`
- Variabili consigliate (pronte per Coolify):

```bash
PORT=8080
ENVIRONMENT=production
LOG_LEVEL=info
JWT_SECRET=change_me_with_32_plus_chars_minimum
SESSION_SECRET=change_me_with_32_plus_chars_minimum

MONGODB_URI=mongodb+srv://user:password@cluster.mongodb.net/?retryWrites=true&w=majority
MONGODB_DB_NAME=visto-easy

# Alias opzionali
DATABASE_NAME=visto-easy
DATABASE_URL=
MONGO_URL=
MONGODB_URL=

REDIS_URL=
S3_ENDPOINT=
S3_BUCKET=
S3_ACCESS_KEY=
S3_SECRET_KEY=
S3_USE_SSL=true
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
SENDGRID_API_KEY=
SENDGRID_FROM_EMAIL=
SENDGRID_FROM_NAME=Visto Easy
SENDGRID_API_BASE=
FRONTEND_RESET_PASSWORD_URL=

# Hardening auth (opzionali)
AUTH_RATE_LIMIT_RPM=30
AUTH_LOCK_MAX_ATTEMPTS=5
AUTH_LOCK_WINDOW_MINUTES=15

# Security alerting (opzionali)
SECURITY_ALERT_WINDOW_MINUTES=15
SECURITY_ALERT_FAILED_THRESHOLD=5
SECURITY_BLOCK_IP_DEFAULT_TTL_MINUTES=120
SECURITY_ALLOW_IP_DEFAULT_TTL_MINUTES=240

# CORS & security headers
CORS_ALLOWED_ORIGINS=*
CORS_ALLOWED_METHODS=GET,POST,PATCH,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Authorization,Content-Type
CORS_ALLOW_CREDENTIALS=false
SECURITY_HEADERS_HSTS=false
```

## Note architetturali

Questa versione implementa un MVP operativo con persistenza su MongoDB (document store) e placeholder per Redis/S3/Stripe/SendGrid.

Notifiche email transazionali (se `SENDGRID_API_KEY` + `SENDGRID_FROM_EMAIL` sono valorizzati):
- reset password richiesto
- link pagamento pratica
- conferma pagamento/visto emesso

MongoDB richiesto all'avvio:

```bash
MONGODB_URI=mongodb+srv://user:password@cluster.mongodb.net/?retryWrites=true&w=majority
MONGODB_DB_NAME=visto-easy
```
