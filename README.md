# visto-easy

Portale di gestione richieste visto (MVP tecnico) con API multi-ruolo, workflow pratiche, documenti e pagamento webhook.

## Features MVP implementate

- Auth JWT + refresh token (`RICHIEDENTE`, `OPERATORE`, `SUPERVISORE`, `ADMIN`)
- Pratiche richiedente: create/list/get/patch/delete (solo in `BOZZA`)
- Documenti pratica: upload metadata + lista
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
- `GET /api/pratiche/{id}/documenti`
- `GET /api/bo/pratiche`
- `PATCH /api/bo/pratiche/{id}/stato`
- `POST /api/bo/pratiche/{id}/link-pagamento`
- `POST /api/bo/pratiche/{id}/invia-visto`
- `GET /api/bo/dashboard/stats`
- `POST /api/pagamento/webhook`

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
- Variabili minime:

```bash
PORT=8080
ENVIRONMENT=production
LOG_LEVEL=info
JWT_SECRET=change_me_with_32_plus_chars_minimum
```

## Note architetturali

Questa versione implementa un MVP operativo in memoria (senza PostgreSQL/Redis/S3 reali).
Gli endpoint e i contratti sono già strutturati per la successiva integrazione dei servizi esterni previsti dalla specifica.
