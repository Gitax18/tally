# Self-hosting

tally has two parts:

- **The service** (Go, in this repository's root). It serves the tracking script, collects
  events into Postgres, and answers the dashboard's questions. It runs on Heroku with one dyno and
  a Postgres add-on.
- **The dashboard** (Next.js, in `web/`). It runs on Vercel and talks to the service with a shared
  secret. Visitors' browsers never see that secret, and the dashboard never connects to Postgres
  itself.

```
your site ── <script defer src="https://a.example.com/s/k3j9x2a0bq.js">
    │  POST /e
    ▼
Heroku: Go service (1 Basic dyno) ──► Heroku Postgres (Essential)
    ▲  /v1/*  (Bearer API_SECRET)
    │
Vercel: Next.js dashboard  ◄── you, signed in with the admin password
```

| Piece | Plan | Cost per month |
|---|---|---|
| Vercel (dashboard) | Hobby | $0 |
| Heroku dyno | Basic. Eco is cheaper but sleeps after 30 idle minutes, so the first visit after a quiet spell waits for the dyno to wake | $7 |
| Heroku Postgres | Essential-0 (1 GB, about 2 million pageviews); move up when the storage meter says so | $5 |

That is about $12 a month for any number of sites, up to roughly a million pageviews a month.
Prices change; check Heroku's before you sign up.

## 1. Make the admin password hash

The dashboard has one password. The service stores only its scrypt hash. With Go 1.27 or newer:

```sh
go run github.com/Gitax18/tally/cmd/server@latest hash-password
# or, in a clone of this repository:
go run ./cmd/server hash-password
```

It asks for the password twice and prints one line starting with `scrypt$`. Keep it for the next
step. Use a long password; sign-in attempts are limited to 5 per minute.

## 2. Deploy the service to Heroku

**With the button:** [![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://www.heroku.com/deploy?template=https://github.com/Gitax18/tally)

It creates the app with a Basic dyno and an Essential-0 database, generates `API_SECRET`, and asks
for `ADMIN_PASSWORD_HASH`. Database migrations run automatically before every release.

**With the CLI**, from a clone of this repository:

```sh
heroku create my-tally --region us          # or --region eu
heroku addons:create heroku-postgresql:essential-0
heroku config:set API_SECRET="$(openssl rand -hex 32)" \
                  ADMIN_PASSWORD_HASH='scrypt$...'   # single quotes: the hash contains $
git push heroku main
heroku ps:type web=basic
```

Check it: `curl https://my-tally-xxxx.herokuapp.com/healthz` answers `ok`. Note the
`API_SECRET` for the dashboard: `heroku config:get API_SECRET`.

### Use your own domain

Ad blockers commonly block analytics requests to `*.herokuapp.com`. A neutral subdomain of your
own avoids most of that (the script and endpoint paths, `/s/` and `/e`, are neutral already):

```sh
heroku domains:add a.example.com     # prints a DNS target
# add a CNAME record: a.example.com → that DNS target
heroku certs:auto:enable             # free TLS certificate
```

From here on, the service's URL is `https://a.example.com`.

## 3. Deploy the dashboard to Vercel

**With the button:** [![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2FGitax18%2Ftally&root-directory=web&project-name=tally-dashboard&env=API_URL,API_SECRET,SESSION_SECRET&envDescription=API_URL%20is%20the%20service%27s%20public%20URL%2C%20API_SECRET%20the%20same%20value%20as%20on%20Heroku%2C%20SESSION_SECRET%20any%2032%2B%20random%20characters&envLink=https%3A%2F%2Fgithub.com%2FGitax18%2Ftally%2Fblob%2Fmain%2Fdocs%2Fself-hosting.md%233-deploy-the-dashboard-to-vercel)

**By hand:** import the repository in Vercel and set **Root Directory** to `web`. The framework
(Next.js) and package manager (pnpm) are detected.

Either way, set these environment variables:

| Variable | Value |
|---|---|
| `API_URL` | The service's public URL, for example `https://a.example.com`. The snippet the dashboard shows is built from it, so use the custom domain, not the herokuapp.com one |
| `API_SECRET` | The same value as on Heroku |
| `SESSION_SECRET` | 32 or more random characters: `openssl rand -hex 32`. Signs the login cookie; changing it signs you out |

Then, under **Settings → Functions**, set the function region next to your Heroku region:
`iad1` (Washington, D.C.) for Heroku's US region, `dub1` (Dublin) for the EU region. Every
dashboard page makes several calls to the service, so this keeps it fast.

Optionally give the dashboard its own domain too, such as `stats.example.com`.

## 4. Add a site

Sign in, choose **Add site**, and enter the domain without `www` (subdomains are included
automatically) and the time zone your days should start in. Paste the snippet into the `<head>`
of every page:

```html
<script defer src="https://a.example.com/s/k3j9x2a0bq.js"></script>
```

The page keeps waiting until the first event arrives. Then:

- `curl -I https://a.example.com/s/k3j9x2a0bq.js` shows `Cache-Control: public, max-age=3600`.
- Open the site on your phone; the dashboard's current-visitor count goes up within 30 seconds,
  with the right country and device.
- Open any page of your site once with `?tally=ignore` to stop counting your own visits in that
  browser (`?tally=track` undoes it).

If the site sends a Content-Security-Policy, allow the service's origin in both `script-src` and
`connect-src`. Single-page apps need nothing extra: the script follows `history.pushState`.

## Configuration

### Service (Heroku)

| Variable | Default | |
|---|---|---|
| `DATABASE_URL` | (set by the add-on) | Postgres connection URL |
| `API_SECRET` | (required) | At least 32 characters: letters, digits, `-` and `_` |
| `ADMIN_PASSWORD_HASH` | (required) | Output of `hash-password` |
| `RETENTION_MONTHS` | `24` | Data older than this is deleted daily (1 to 120) |
| `DB_LIMIT_MB` | `1024` | The database plan's storage limit, for the dashboard's meter. Essential-0: 1024, Essential-1: 10240, Essential-2: 32768 |
| `GEOIP_DOWNLOAD` | `true` | Download DB-IP's free country database when the service starts. `false` turns country lookups off unless `GEOIP_PATH` is set |
| `GEOIP_PATH` | | Use this local `.mmdb` country database instead of downloading one |
| `TRUST_PROXY` | `true` on Heroku, else `false` | Take the client IP from the last `X-Forwarded-For` entry. Only enable it behind a proxy that appends the real client address there |
| `PORT`, `LISTEN_HOST` | `8090`, all interfaces | Heroku sets `PORT` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `SHUTDOWN_TIMEOUT` | `25s` | How long to drain queued events on shutdown (Heroku allows 30s) |

### Dashboard (Vercel)

| Variable | |
|---|---|
| `API_URL` | The service's public URL |
| `API_SECRET` | Same as the service's |
| `SESSION_SECRET` | 32 or more characters; signs the 30-day login cookie |
| `TRUST_FORWARDED_FOR` | Only when hosting the dashboard outside Vercel behind a proxy: `true` makes the sign-in rate limit use the `X-Forwarded-For` / `X-Real-IP` address |

## Running it

- **Updating.** Pull the new version and `git push heroku main`; migrations run in Heroku's
  release phase before the new version takes traffic. Vercel redeploys the dashboard on push.
- **Changing the password.** Make a new hash and `heroku config:set ADMIN_PASSWORD_HASH='...'`.
  Browsers that are already signed in stay signed in until their cookie expires; change
  `SESSION_SECRET` on Vercel as well to sign everyone out.
- **Storage.** The sites page shows how full the database is and warns at 80%. Heroku Essential
  databases lose write access when they go over their limit, and new events would then be lost.
  Before that, either move to a bigger plan (see Heroku's guide to changing Postgres plans) and
  update `DB_LIMIT_MB`, or lower `RETENTION_MONTHS`.
- **Backups.** `heroku pg:backups:capture` takes one now, and
  `heroku pg:backups:schedule DATABASE_URL --at '04:00 UTC'` takes one daily. How many are kept
  depends on the plan.
- **Logs.** `heroku logs --tail`. Every 5 minutes the service logs its ingest counters: events
  accepted, stored, dropped (queue full), failed (database error), rejected (bad request, unknown
  site or wrong hostname), bots and rate-limited. The same numbers are in `GET /v1/usage`.
- **Restarts.** Heroku restarts dynos daily. On shutdown the service stops accepting requests and
  writes every queued event before exiting, and visits in progress continue after the restart.
- **Retention** runs once a day inside the service. With more than one dyno, a Postgres advisory
  lock makes sure only one of them does it.

## Troubleshooting

**Nothing shows up.** Open the site in a normal browser window and look at the request to `/e` in
the developer tools' Network tab:

- No request at all: the page is on `localhost` (add a site with the domain `localhost` to test
  locally), the browser is automated, this browser opted out (open a page with `?tally=track`), or
  a content blocker stopped the script.
- `202`: accepted. Events appear on the dashboard within a second or two.
- `403` with `hostname_mismatch`: the page's hostname is neither the site's domain nor a
  subdomain of it. Check the domain in the site's settings.
- `404` with `unknown_site`: the site ID in the snippet does not exist (was the site deleted?).
- `429`: more than 30 requests in 10 seconds from one address.

**The dashboard says the server could not be reached.** `API_URL` is wrong or the service is
down (`heroku ps`, `heroku logs`). A `401` from the service in Vercel's logs means `API_SECRET`
differs between the two.

**Days or hours look shifted.** Set the site's time zone in its settings. Stored data is in UTC,
so changing it re-groups everything, past data included.

## Hosting elsewhere

Nothing is Heroku-specific apart from `app.json` and the `Procfile`.

- **Service:** Go 1.27 and Postgres (tested with 17). Build with `go build -o bin/ ./cmd/...`,
  then run `bin/migrate` (applies migrations) and `bin/server` with the variables above. Behind
  a reverse proxy that appends the client address to `X-Forwarded-For` (nginx with
  `$proxy_add_x_forwarded_for`, Caddy, most load balancers), set `TRUST_PROXY=true`; otherwise
  every visitor would share the proxy's address.
- **Dashboard:** Node 24 and pnpm. In `web/`: `pnpm install && pnpm build && pnpm start`
  (port 3010), with the variables above.
