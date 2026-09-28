<h1 style="text-align:center">The Wish List</h1>

This small repository contains a simple website that shows a list of presents for friends and family to buy!

## Running the app

There are a few ways of running this locally, plus a Vercel deployment.

### Debugging with Go Air

To test the project with hot reload I'm using [Go Air](https://github.com/air-verse/air)

- Clone the repository to your machine.
- Get all the packages with `go mod tidy`
- Run the command `air`

By default this uses a local JSON file (`data/wishlist.json`) for storage — no database required.

### Plain `go run`

- `go run cmd/main.go`

### Docker image

- `docker build . -t thewishlist`
- `docker run -p 43067:43067 thewishlist`

### Vercel (production)

Vercel is the deployment target: the app runs as a serverless function (`api/index.go`, routed via `vercel.json`), backed by PostgreSQL instead of the local JSON file. See `docs/design.md` for the architecture. The app was previously deployed to Azure (Container App / Web App) — that's been retired in favor of Vercel.

## Environment variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `PORT` | No | Port to listen on (checked first). Only used by `cmd/main.go`'s own server loop, not by Vercel. |
| `WEBSITES_PORT` | No | Fallback port. Left over from the retired Azure App Service deployment — harmless to keep, but no longer meaningful. Falls back to `43067` if neither `PORT` nor this is set. |
| `STORE_TYPE` | No | Set to `postgres` to use PostgreSQL instead of the local JSON file. Any other value (or unset) uses `data/wishlist.json`. |
| `DATABASE_URL` | Only if `STORE_TYPE=postgres` | Postgres connection string. On Neon, use the pooled/pgbouncer variant. |
| `WISHLIST_PASSWORD` | No, but strongly recommended for any real deployment | Enables HTTP Basic Auth for every route when set. If unset, the site is completely open — a startup log line makes this visible either way. |
| `WISHLIST_USERNAME` | No | Basic Auth username. Defaults to `family` if `WISHLIST_PASSWORD` is set but this isn't. |
