# visto-easy

Portale di gestione richieste visto (MVP tecnico) con API multi-ruolo, workflow pratiche, documenti e pagamento webhook.

## Features MVP implementate

- Auth JWT + refresh token (`RICHIEDENTE`, `OPERATORE`, `SUPERVISORE`, `ADMIN`)
- Pratiche richiedente: create/list/get/patch/delete (solo in `BOZZA`)
- Documenti pratica: upload metadata + lista
- Presigned upload URL per documenti (S3/MinIO)
- Backoffice: lista pratiche, cambio stato, link pagamento, invio visto, stats dashboard
- Pagamenti: creazione sessione, query stato, webhook `payment.succeeded`
- Macchina stati pratica con validazione transizioni

## API principali

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/refresh`
- `POST /api/pratiche/`
- `GET /api/pratiche/`
- `GET /api/pratiche/{id}`
- `PATCH /api/pratiche/{id}`
- `DELETE /api/pratiche/{id}`
- `POST /api/pratiche/{id}/documenti`
- `POST /api/pratiche/{id}/documenti/presign`
- `GET /api/pratiche/{id}/documenti`
- `GET /api/pratiche/{id}/eventi`
- `GET /api/bo/pratiche`
- `GET /api/bo/utenti`
- `GET /api/bo/report.csv`
- `GET /api/bo/notifications/stream`
- `PATCH /api/bo/pratiche/{id}/stato`
- `POST /api/bo/pratiche/{id}/link-pagamento`
- `POST /api/bo/pratiche/{id}/invia-visto`
- `GET /api/bo/dashboard/stats`
- `POST /api/pagamento/webhook`

## Query params utili (backoffice)

- `page`, `page_size`
- `sort_by`, `sort_order`
- `/api/bo/pratiche`: `stato`, `priorita`, `tipo_visto`, `paese_dest`, `operatore_id`, `q`, `from`, `to`
- `/api/bo/utenti`: `ruolo`, `q`
- `/api/bo/report.csv`: stessi filtri di `/api/bo/pratiche`

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

# Hardening auth (opzionali)
AUTH_RATE_LIMIT_RPM=30
AUTH_LOCK_MAX_ATTEMPTS=5
AUTH_LOCK_WINDOW_MINUTES=15
```

## Note architetturali

Questa versione implementa un MVP operativo con persistenza su MongoDB (document store) e placeholder per Redis/S3/Stripe/SendGrid.

MongoDB richiesto all'avvio:

```bash
MONGODB_URI=mongodb+srv://user:password@cluster.mongodb.net/?retryWrites=true&w=majority
MONGODB_DB_NAME=visto-easy
```
