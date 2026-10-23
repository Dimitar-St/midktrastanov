# БулДез Контрол

Company-scoped visit scheduling with a Vue frontend, Cloudflare Worker gateway, Go API, and PostgreSQL. Users sign in with their username and password; each account can read only its company's schedule, users, events, and address history.

## Run locally

Requirements: Node 22.12+, Go 1.25+, and PostgreSQL 17+, or Docker Compose.

```sh
npm install
cp .env.example .env
cp .dev.vars.example .dev.vars
cp backend/.env.example backend/.env
docker compose up -d postgres
```

The supplied development credentials are `buldez` / `change-me` for database `buldez`; they match root `.env`, `backend/.env`, and Docker Compose. Replace them and the secrets before deployment, then start the services:

```sh
# terminal 1
cd backend
set -a
. ./.env
set +a
go run .

# terminal 2, Vite hot reload and direct development API proxy
npm run dev
```

Open `http://localhost:5173`, then either register a company and its first account or sign in with `SEED_ADMIN_USERNAME` and `SEED_ADMIN_PASSWORD`. The seed administrator is inserted only if that username does not already exist, so changing the seed password does not overwrite an existing account.

To run the database and API together in Docker, keep the values in root `.env` and run `docker compose up --build`; start the Worker separately with `npm run worker:dev`. Docker Compose builds `postgres/Dockerfile`, starts the `database` service, waits for its health check, then starts `backend` with `DATABASE_URL` pointing to `database:5432` on the internal Compose network.

## Pages

- `/day` and `/week`: company schedule for the selected date range.
- `/addresses`: every company address with its most recent and next visit.
- `/events/new`: create an event with address, start/end time, creator, assignee, and optional monthly recurrence.

The company name appears in the upper-right header. There is no duplicate upper-right "new event" action; event creation is available from the navigation.

## Configuration

| Variable | Used by | Purpose |
|---|---|---|
| `VITE_API_BASE_URL` | frontend | Browser API base; use `/api` through the Worker. |
| `VITE_GOOGLE_MAPS_API_KEY` | frontend | Browser-restricted Google Maps JavaScript API key for the address map. |
| `DEV_API_PROXY_TARGET` | Vite | Local Worker URL. |
| `BACKEND_URL` | Worker | Go API URL. |
| `FRONTEND_ORIGIN` | Worker and API | Allowed frontend origins. |
| `BACKEND_API_TOKEN` | Worker and API | Private Worker-to-API gateway token. |
| `DATABASE_URL` | API | PostgreSQL connection URL. |
| `SESSION_SECRET` | API | Long random secret for signed user sessions. |
| `SEED_COMPANY_NAME` | API | Initial company name. |
| `SEED_ADMIN_NAME` | API | Initial administrator display name. |
| `SEED_ADMIN_USERNAME` | API | Initial administrator username. |
| `SEED_ADMIN_PASSWORD` | API | Initial administrator password. |
| `PORT` | API | HTTP port, default `8080`. |

Do not use `VITE_` for secrets. `BACKEND_API_TOKEN` is injected only by the Worker as `X-Gateway-Token`; it is not sent to browsers. The Worker forwards the browser's signed user session to the API.

`postgres/Dockerfile` has development defaults for the database name, user, and password. Compose overrides them with `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`; set strong values for all secrets before deployment.

## Cloudflare deployment

`npm run deploy` first type-checks and builds the Vue application, then deploys the Worker and `dist/` assets to the default `buldez-scheduler.<your-subdomain>.workers.dev` domain. The Worker name and `workers.dev` publishing are configured in `wrangler.jsonc`.

Before deploying the API proxy, host the Go API at a publicly reachable HTTPS URL, then configure its Worker bindings. Do not commit these values:

```sh
npx wrangler secret put BACKEND_URL
npx wrangler secret put BACKEND_API_TOKEN
npx wrangler secret put FRONTEND_ORIGIN
```

Set `FRONTEND_ORIGIN` to the deployed `https://buldez-scheduler.<your-subdomain>.workers.dev` URL and configure the Go API with that same URL as `FRONTEND_ORIGIN`.

Pushes to `main` run the same build and deployment through GitHub Actions. Add `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` as GitHub repository secrets before relying on this workflow. The API token needs Workers Scripts edit permissions for the selected account.

## Database

The API applies each SQL file from `backend/migrations/` once on startup and records it in `scheduler.schema_migrations`. PostgreSQL uses the dedicated `scheduler` schema and has these primary tables:

- `companies`: one company per tenant.
- `users`: username, bcrypt password hash, display name, and company ownership.
- `events`: company address, start/end timestamps, creator, assigned user, and optional monthly recurrence.

Indexes cover schedule lookup by company/date and address lookup by company/address. Recurring occurrences are calculated at read time, including month-end clamping such as January 31 to February 28/29.

## Verification

```sh
npm run build
npm test
cd backend
go test ./...
go vet ./...
```
