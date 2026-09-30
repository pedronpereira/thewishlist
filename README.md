<h1 style="text-align:center">The Wish List</h1>

This small repository contains a simple website that shows a list of presents for friends and family to buy!

The app supports multiple named lists (e.g. `/wishlist/pedro`, `/wishlist/wife`), switchable via a tab bar in the header.

## Running the app

There are a few ways of running this locally, plus a Vercel deployment.

### Debugging with Go Air

To test the project with hot reload I'm using [Go Air](https://github.com/air-verse/air)

- Clone the repository to your machine.
- Get all the packages with `go mod tidy`
- Run the command `air`

By default this uses a local JSON file (`data/wishlist.json`) for storage — no database required.

To test with things like Basic Auth locally, copy `.env.example` to `.env` and fill in whatever values you want — `air` and `go run cmd/main.go` both load it automatically (via [godotenv](https://github.com/joho/godotenv)), so you don't need to `$env:` export anything by hand each session. `.env` is gitignored; never commit real credentials into `.env.example`.

### Plain `go run`

- `go run cmd/main.go`

### Docker image

- `docker build . -t thewishlist`
- `docker run -p 43067:43067 thewishlist`

### Vercel (production)

Vercel is the deployment target: the app runs as a serverless function (`api/index.go`, routed via `vercel.json`), backed by PostgreSQL instead of the local JSON file. The app was previously deployed to Azure (Container App / Web App) — that's been retired in favor of Vercel.

## Environment variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `PORT` | No | Port to listen on (checked first). Only used by `cmd/main.go`'s own server loop, not by Vercel. |
| `WEBSITES_PORT` | No | Fallback port. Left over from the retired Azure App Service deployment — harmless to keep, but no longer meaningful. Falls back to `43067` if neither `PORT` nor this is set. |
| `STORE_TYPE` | No | Set to `postgres` to use PostgreSQL instead of the local JSON file. Any other value (or unset) uses `data/wishlist.json`. |
| `DATABASE_URL` | Only if `STORE_TYPE=postgres` | Postgres connection string. On Neon, use the pooled/pgbouncer variant. |
| `WISHLIST_PASSWORD` | No, but strongly recommended for any real deployment | Enables HTTP Basic Auth for every route when set. If unset, the site is completely open — a startup log line makes this visible either way. |
| `WISHLIST_USERNAME` | No | Basic Auth username for the shared family login. Defaults to `family` if `WISHLIST_PASSWORD` is set but this isn't. |
| `WISHLIST_ADMIN_PASSWORD` | No | A second Basic Auth login with elevated request-level access. Only reachable if `WISHLIST_PASSWORD` is also set. |
| `WISHLIST_ADMIN_USERNAME` | No | Admin username. Defaults to `admin` if `WISHLIST_ADMIN_PASSWORD` is set but this isn't. |
