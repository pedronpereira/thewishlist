<h1 style="text-align:center">The Wish List</h1>

This small repository contains a simple website that shows a list of presents for friends and family to buy!

The app supports multiple named lists (e.g. `/wishlist/pedro`, `/wishlist/wife`), switchable via a tab bar in the header.

An admin login can add, edit, and remove items directly in the browser. Other logins can browse the lists and mark items as bought.

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

## Scripts

The `scripts/` folder holds helper tools that work against a running server through its HTTP API. They use only the Python standard library, so there's nothing to install beyond Python 3.

### `scripts/import_items.py`

Creates items on a server from a wishlist JSON file in the same format as `data/wishlist.json`, or updates items that already exist there. Useful for bringing a deployed site in line with local data without adding items by hand.

It needs the admin login from the environment, so the password never appears on the command line:

```powershell
$env:WISHLIST_ADMIN_PASSWORD = "your-admin-password"
python scripts/import_items.py --base-url https://your-site.example
```

`WISHLIST_ADMIN_USERNAME` is optional and defaults to `admin`.

| Option | Default | Purpose |
| --- | --- | --- |
| `--base-url` | (required) | The server to talk to, e.g. `https://your-site.example` |
| `--list` | `pedro` | Slug of the list to sync |
| `--file` | `data/wishlist.json` | Wishlist JSON to read items from |
| `--update` | off | Update items that exist on the server but differ from the file, instead of creating missing ones |
| `--apply` | off | Actually write the changes |

Behavior to know about:

- **Dry run by default.** Without `--apply` it prints what it would change and writes nothing.
- **Create mode** (the default) adds items whose `id` isn't on the server yet. Items keep the `id` from the file, so re-running is safe.
- **Update mode** (`--update`) only changes items whose fields differ from the file. A purchase status set on the server is always kept, so an update never un-marks an item.
- **Stops on the first error** and reports which item failed. Anything already written stays, and re-running picks up where it left off.

Examples:

```powershell
# See what would be created
python scripts/import_items.py --base-url https://your-site.example

# Create the missing items
python scripts/import_items.py --base-url https://your-site.example --apply

# See which existing items differ from the file
python scripts/import_items.py --base-url https://your-site.example --update
```

The target server must already be running the code that matches the file. Otherwise the CSRF check or the update route may reject the requests.

### Updating an environment with the scripts

The scripts talk to whichever server `--base-url` points at, so the same commands work for a local server or a deployed one. Each environment has its own data: the local server reads `data/wishlist.json`, while a deployed site reads its database. The scripts never touch either one directly.

To bring a deployed site in line with the local data file:

1. **Deploy the code first.** The deployed server needs the code that matches the file, including the admin routes the scripts call. Check that the deployment is live, for example by loading a static file such as `/css/index-v1.css`.
2. **Create missing items.** Run a dry run, check the list it prints, then run it again with `--apply`:
   ```powershell
   python scripts/import_items.py --base-url https://your-site.example
   python scripts/import_items.py --base-url https://your-site.example --apply
   ```
3. **Update changed items.** Dry run first with `--update`. Only the fields that differ are listed. If the list is what you expect, add `--apply`:
   ```powershell
   python scripts/import_items.py --base-url https://your-site.example --update
   python scripts/import_items.py --base-url https://your-site.example --update --apply
   ```

Always read the dry-run output before applying. If an update lists a field you didn't intend to change, the local data file is the likely cause: fix the file, then dry-run again. The scripts only add or change items; they never delete anything on the server.

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
